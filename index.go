package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	rawASSParserVersion         = "raw-ass/v1"
	assCueQualityVersion        = "ass-cue-quality/v1"
	indexCatalogSnapshotVersion = "catalog-snapshot/v1"
	minimumIndexDelaySeconds    = 2

	// These limits are deliberately persisted with each cue-quality record so
	// an operator can tell which policy produced a degraded result. A raw ASS
	// cache remains canonical and is never rewritten to satisfy this policy.
	maxMalformedASSCueRatio = 0.05
	maxEmptyASSCueRatio     = 0.20
	maxSkippedASSCueRatio   = 0.05
)

var openIndexPlayback playbackOpener = getEpisode

const (
	indexStatusPending         = "pending"
	indexStatusIndexed         = "indexed"
	indexStatusSubtitleCached  = "subtitle_cached"
	indexStatusSubtitleMissing = "subtitle_missing_locale"
	indexStatusRetryableFailed = "subtitle_retryable_failed"
	indexStatusPermanentFailed = "subtitle_permanent_failed"
	// A provider application code alone does not establish why playback was
	// restricted. Keep the causal claim neutral unless the HTTP response or
	// rate-limit headers independently establish it.
	indexStatusUnknownProviderPlaybackRestriction = "subtitle_unknown_provider_playback_restriction"
	indexStatusProviderRateLimited                = "subtitle_provider_rate_limited"
	// Deprecated source alias retained for older focused tests and callers;
	// serialized checkpoints use the typed retryable value above.
	indexStatusSubtitleFailed = indexStatusRetryableFailed
)

const legacyIndexStatusThrottled = "subtitle_throttled"

const (
	providerRestrictionUnknown     = "unknown_provider_playback_restriction"
	providerRestrictionRateLimited = "direct_rate_limit_evidence"
)

// indexFile is a durable, self-contained checkpoint. Every terminal episode
// state is followed by an atomic rewrite of this full snapshot.
type indexFile struct {
	Series          string          `json:"series"`
	SeriesURL       string          `json:"series_url"`
	IndexedAt       string          `json:"indexed_at"`
	SubsLang        string          `json:"subs_lang,omitempty"`
	CatalogSnapshot string          `json:"catalog_snapshot,omitempty"`
	Seasons         []indexSeason   `json:"seasons"`
	Checkpoint      indexCheckpoint `json:"checkpoint,omitempty"`
}

type indexCheckpoint struct {
	CursorProviderID string            `json:"cursor_provider_id,omitempty"`
	UpdatedAt        string            `json:"updated_at,omitempty"`
	Circuit          indexCircuitState `json:"circuit"`
}

type indexCircuitState struct {
	State                 string `json:"state"`
	Consecutive4294       int    `json:"consecutive_4294"`
	OpenedAt              string `json:"opened_at,omitempty"`
	NextEligibleAttemptAt string `json:"next_eligible_attempt_at,omitempty"`
	RestrictionEvidence   string `json:"restriction_evidence,omitempty"`
}

// subtitleCueQuality records a local, derived assessment of the preserved raw
// ASS bytes. It is telemetry only: the raw file and checksum remain the cache
// contract even when this outcome is degraded.
type subtitleCueQuality struct {
	Version              string  `json:"version"`
	TotalCues            int     `json:"total_cues"`
	UsableCues           int     `json:"usable_cues"`
	MalformedCues        int     `json:"malformed_cues"`
	EmptyCues            int     `json:"empty_cues"`
	SkippedCues          int     `json:"skipped_cues"`
	MaxMalformedCueRatio float64 `json:"max_malformed_cue_ratio"`
	MaxEmptyCueRatio     float64 `json:"max_empty_cue_ratio"`
	MaxSkippedCueRatio   float64 `json:"max_skipped_cue_ratio"`
	Outcome              string  `json:"outcome"`
}

type indexSeason struct {
	Season   int            `json:"season"`
	Episodes []indexEpisode `json:"episodes"`
}

type indexEpisode struct {
	Episode                      int                      `json:"episode"`
	Title                        string                   `json:"title"`
	Description                  string                   `json:"description,omitempty"`
	ID                           string                   `json:"id"`
	URL                          string                   `json:"url"`
	AirDate                      string                   `json:"air_date,omitempty"`
	AudioLangs                   []string                 `json:"audio_langs,omitempty"`
	SubtitleLangs                []string                 `json:"subtitle_langs,omitempty"`
	Status                       string                   `json:"status"`
	SubtitleFile                 string                   `json:"subtitle_file,omitempty"`
	SubtitleLocale               string                   `json:"subtitle_locale,omitempty"`
	SubtitleSHA256               string                   `json:"subtitle_sha256,omitempty"`
	SubtitleParserVersion        string                   `json:"subtitle_parser_version,omitempty"`
	SubtitleCueQuality           *subtitleCueQuality      `json:"subtitle_cue_quality,omitempty"`
	SubtitleAttemptCount         int                      `json:"subtitle_attempt_count,omitempty"`
	SubtitleFetchAttemptCount    int                      `json:"subtitle_fetch_attempt_count,omitempty"`
	StreamReleaseAttemptCount    int                      `json:"stream_release_attempt_count,omitempty"`
	SubtitleLastAttemptAt        string                   `json:"subtitle_last_attempt_at,omitempty"`
	SubtitleNextEligibleAt       string                   `json:"subtitle_next_eligible_at,omitempty"`
	HTTPStatus                   int                      `json:"http_status,omitempty"`
	ProviderApplicationCode      string                   `json:"provider_application_code,omitempty"`
	RetryAfter                   string                   `json:"retry_after,omitempty"`
	RateLimit                    ProviderRateLimitHeaders `json:"rate_limit,omitempty"`
	StreamReleaseOutcome         string                   `json:"stream_release_outcome,omitempty"`
	ProviderRestriction          string                   `json:"provider_restriction,omitempty"`
	SourceVersion                string                   `json:"source_version,omitempty"`
	SubtitleCheckedSourceVersion string                   `json:"subtitle_checked_source_version,omitempty"`
	SubtitleRecheckSnapshot      string                   `json:"subtitle_recheck_snapshot,omitempty"`
	SubtitleRecheckReason        string                   `json:"subtitle_recheck_reason,omitempty"`
	Error                        string                   `json:"error,omitempty"`
}

func (episode indexEpisode) isVerifiedRawASS() bool {
	return episode.Status == indexStatusSubtitleCached &&
		episode.SubtitleFile != "" &&
		episode.SubtitleLocale != "" &&
		episode.SubtitleSHA256 != "" &&
		episode.SubtitleParserVersion == rawASSParserVersion
}

func normalizeIndexStatus(status string) string {
	switch status {
	case legacyIndexStatusThrottled:
		// Older checkpoints inferred a rate cause from application code 4294.
		// Keep their durable attempts and files, but carry the neutral meaning
		// forward instead of preserving that unsupported inference.
		return indexStatusUnknownProviderPlaybackRestriction
	case "subtitle_failed":
		return indexStatusRetryableFailed
	default:
		return status
	}
}

func stableSnapshotHash(parts []string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = fmt.Fprintf(hash, "%d:", len(part))
		_, _ = hash.Write([]byte(part))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// episodeSourceVersion fingerprints only provider catalog fields that can
// change an episode's playable identity or subtitle availability context. It
// never uses wall-clock time, so an unchanged provider snapshot cannot cause a
// terminal row to be rechecked merely because another run occurred.
func episodeSourceVersion(ep SeasonEpisode) string {
	versions := make([]string, 0, len(ep.Versions))
	for _, version := range ep.Versions {
		if version == nil {
			continue
		}
		versions = append(versions, version.AudioLocale+"\x00"+version.GUID)
	}
	sort.Strings(versions)
	parts := []string{
		"episode-source/v1", ep.ID, strconv.Itoa(ep.SeasonNumber), strconv.Itoa(ep.EpisodeNumber),
		ep.AvailabilityStarts, ep.AudioLocale, ep.Title, ep.Description,
	}
	parts = append(parts, versions...)
	return "episode-source/v1:" + stableSnapshotHash(parts)
}

// catalogSnapshot is a stable identity for the discovered provider catalog.
// It lets terminal rows be revisited only when the source snapshot changes;
// selection itself remains explicitly capped elsewhere.
func catalogSnapshot(catalog indexFile) string {
	entries := make([]string, 0)
	for _, season := range catalog.Seasons {
		for _, episode := range season.Episodes {
			entries = append(entries, episode.ID+"\x00"+episode.SourceVersion)
		}
	}
	sort.Strings(entries)
	parts := []string{indexCatalogSnapshotVersion, catalog.SeriesURL, catalog.SubsLang}
	parts = append(parts, entries...)
	return indexCatalogSnapshotVersion + ":" + stableSnapshotHash(parts)
}

func copyCueQuality(quality *subtitleCueQuality) *subtitleCueQuality {
	if quality == nil {
		return nil
	}
	copied := *quality
	return &copied
}

func cueQualityForVerifiedRawASS(prior indexEpisode) *subtitleCueQuality {
	if prior.SubtitleCueQuality != nil && prior.SubtitleCueQuality.Version == assCueQualityVersion {
		return copyCueQuality(prior.SubtitleCueQuality)
	}
	raw, err := os.ReadFile(prior.SubtitleFile)
	if err != nil {
		// The checksum verification immediately before this local read is the
		// cache gate. Do not fall through to a provider redownload if a later
		// local read races or fails; keep the already-verified cache truthful.
		return nil
	}
	quality := assessASSCueQuality(raw)
	return &quality
}

func copyVerifiedRawASSCheckpoint(entry *indexEpisode, prior indexEpisode) {
	entry.Status = indexStatusSubtitleCached
	entry.SubtitleFile = prior.SubtitleFile
	entry.SubtitleSHA256 = prior.SubtitleSHA256
	entry.SubtitleParserVersion = rawASSParserVersion
	entry.SubtitleLangs = prior.SubtitleLangs
	entry.SubtitleCueQuality = cueQualityForVerifiedRawASS(prior)
	entry.SubtitleCheckedSourceVersion = checkedSourceVersion(prior)
	if entry.SubtitleCheckedSourceVersion == "" {
		entry.SubtitleCheckedSourceVersion = entry.SourceVersion
	}
}

func checkedSourceVersion(entry indexEpisode) string {
	if entry.SubtitleCheckedSourceVersion != "" {
		return entry.SubtitleCheckedSourceVersion
	}
	return entry.SourceVersion
}

func indexFilename(seriesTitle, contentID string) string {
	if strings.TrimSpace(seriesTitle) == "" {
		seriesTitle = contentID
	}
	return sanitize(seriesTitle) + "_index.json"
}

func indexEpisodeKey(id string) string { return id }

func priorEpisodes(file indexFile) map[string]indexEpisode {
	prior := make(map[string]indexEpisode)
	for _, season := range file.Seasons {
		for _, episode := range season.Episodes {
			if episode.ID != "" {
				prior[indexEpisodeKey(episode.ID)] = episode
			}
		}
	}
	return prior
}

func loadIndex(path string) (indexFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return indexFile{}, nil
	}
	if err != nil {
		return indexFile{}, fmt.Errorf("read index %s: %w", path, err)
	}
	var file indexFile
	if err := json.Unmarshal(data, &file); err != nil {
		return indexFile{}, fmt.Errorf("parse index %s: %w", path, err)
	}
	return file, nil
}

func marshalIndex(file indexFile) ([]byte, error) {
	return json.MarshalIndent(file, "", "  ")
}

// atomicWriteFile writes and fsyncs a same-directory temporary file before a
// rename, so an interrupted run keeps either the old complete checkpoint or
// the new complete checkpoint -- never a partial JSON/ASS file.
func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file next to %s: %w", path, err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if err := tmp.Chmod(mode); err != nil {
		cleanup()
		return fmt.Errorf("set temporary file mode: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("atomically replace %s: %w", path, err)
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func snapshotIndex(path string, file *indexFile) error {
	file.IndexedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := marshalIndex(*file)
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	return atomicWriteFile(path, data, 0644)
}

func sha256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func canResumeRawASS(prior indexEpisode, locale string) bool {
	if !prior.isVerifiedRawASS() || prior.SubtitleLocale != locale {
		return false
	}
	checksum, err := sha256File(prior.SubtitleFile)
	return err == nil && checksum == prior.SubtitleSHA256
}

func rawASSPath(subsDir string, season, episode int, episodeID string) string {
	return filepath.Join(subsDir, fmt.Sprintf("S%02dE%03d_%s.ass", season, episode, sanitize(episodeID)))
}

func collectAudioLangs(ep SeasonEpisode) []string {
	seen := map[string]bool{}
	var langs []string
	add := func(locale string) {
		if locale != "" && !seen[locale] {
			seen[locale] = true
			langs = append(langs, locale)
		}
	}
	add(ep.AudioLocale)
	for _, version := range ep.Versions {
		if version != nil {
			add(version.AudioLocale)
		}
	}
	sort.Strings(langs)
	return langs
}

func sortedSubtitleLocales(subtitles map[string]*Subtitle) []string {
	locales := make([]string, 0, len(subtitles))
	for locale, subtitle := range subtitles {
		if subtitle != nil && subtitle.URL != "" {
			locales = append(locales, locale)
		}
	}
	sort.Strings(locales)
	return locales
}

type subtitleFetchResult struct {
	AvailableLangs          []string
	RawASS                  []byte
	PlaybackAttempts        int
	SubtitleFetchAttempts   int
	StreamReleaseAttempts   int
	StreamReleaseOutcome    string
	HTTPStatus              int
	ProviderApplicationCode string
	RetryAfter              string
	RateLimit               ProviderRateLimitHeaders
}

func hasDirectRateLimitEvidence(httpStatus int, retryAfter string, rateLimit ProviderRateLimitHeaders) bool {
	if httpStatus == 429 || strings.TrimSpace(retryAfter) != "" {
		return true
	}
	return strings.TrimSpace(rateLimit.Remaining) == "0" &&
		(strings.TrimSpace(rateLimit.Limit) != "" || strings.TrimSpace(rateLimit.Reset) != "")
}

func providerRestrictionEvidence(httpStatus int, retryAfter string, rateLimit ProviderRateLimitHeaders) string {
	if hasDirectRateLimitEvidence(httpStatus, retryAfter, rateLimit) {
		return providerRestrictionRateLimited
	}
	return providerRestrictionUnknown
}

// SubtitleLocaleMismatchError is terminal: the provider map key is not
// enough to prove that the returned subtitle track has the requested locale.
type SubtitleLocaleMismatchError struct {
	Requested string
	Actual    string
}

func (e *SubtitleLocaleMismatchError) Error() string {
	return fmt.Sprintf("subtitle locale mismatch: requested %s, received %s", e.Requested, e.Actual)
}

func exactSubtitleForLocale(subtitles map[string]*Subtitle, locale string) (*Subtitle, error) {
	subtitle, ok := subtitles[locale]
	if !ok || subtitle == nil || subtitle.URL == "" {
		return nil, &SubtitleLocaleError{Locale: locale}
	}
	if subtitle.Language != locale {
		return nil, &SubtitleLocaleMismatchError{Requested: locale, Actual: subtitle.Language}
	}
	return subtitle, nil
}

// fetchSubtitles uses the exact requested locale. A different locale is never
// silently substituted: that would make a checkpoint claim a transcript it
// did not actually fetch. Playback streams are released before returning.
func fetchSubtitles(episodeID, locale string) (result subtitleFetchResult, err error) {
	if locale == "" {
		return result, fmt.Errorf("subtitle locale is required")
	}
	var episode Episode
	if err := func() (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				if recoveredErr, ok := recovered.(error); ok {
					err = fmt.Errorf("open playback for %s: %w", episodeID, recoveredErr)
				} else {
					err = fmt.Errorf("open playback for %s: %v", episodeID, recovered)
				}
			}
		}()
		episode = openPlaybackWithRetry(episodeID, func(id string) Episode {
			result.PlaybackAttempts++
			return openIndexPlayback(id)
		}, indexPlayback4294Retries(), *playback4294Backoff, sleepPlaybackRetry)
		return nil
	}(); err != nil {
		return result, err
	}

	result.AvailableLangs = sortedSubtitleLocales(episode.Subtitles)
	if episode.Token != "" {
		defer func() {
			var released bool
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				result.StreamReleaseAttempts++
				released = deleteStream(episodeID, episode.Token)
			}()
			if recovered != nil {
				result.StreamReleaseOutcome = "failed"
			} else if released {
				result.StreamReleaseOutcome = "released"
			} else {
				result.StreamReleaseOutcome = "not_released"
			}
			if err == nil && recovered != nil {
				err = fmt.Errorf("release playback stream for %s: %v", episodeID, recovered)
			} else if err == nil && !released {
				err = fmt.Errorf("release playback stream for %s", episodeID)
			}
		}()
	}

	subtitle, err := exactSubtitleForLocale(episode.Subtitles, locale)
	if err != nil {
		var missing *SubtitleLocaleError
		if errors.As(err, &missing) {
			missing.EpisodeID = episodeID
		}
		return result, err
	}
	raw, err := fetchSubtitleASS(subtitle.URL)
	result.SubtitleFetchAttempts++
	if err != nil {
		return result, err
	}
	result.RawASS = raw
	return result, nil
}

// SubtitleLocaleError records exact-locale absence as a normal terminal state.
type SubtitleLocaleError struct {
	EpisodeID string
	Locale    string
}

func (e *SubtitleLocaleError) Error() string {
	return fmt.Sprintf("subtitle locale %s is not available for episode %s", e.Locale, e.EpisodeID)
}

type indexEpisodeWork struct {
	SeasonIndex  int
	EpisodeIndex int
	Episode      SeasonEpisode
}

// sortIndexWorkNewestFirst returns a deterministic traversal plan without
// changing catalog presentation order. Provider availability time is the
// canonical newest signal; season and episode numbers break equal-date ties,
// and provider episode ID makes duplicate-number variants collision-stable.
func sortIndexWorkNewestFirst(work []indexEpisodeWork) []indexEpisodeWork {
	ordered := append([]indexEpisodeWork(nil), work...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i].Episode
		right := ordered[j].Episode
		if left.AvailabilityStarts != right.AvailabilityStarts {
			return left.AvailabilityStarts > right.AvailabilityStarts
		}
		if left.SeasonNumber != right.SeasonNumber {
			return left.SeasonNumber > right.SeasonNumber
		}
		if left.EpisodeNumber != right.EpisodeNumber {
			return left.EpisodeNumber > right.EpisodeNumber
		}
		return left.ID < right.ID
	})
	return ordered
}

// parseIndexPriorityIDs normalizes the comma-separated CLI value while
// preserving the user's first-declared order. Empty entries and duplicates do
// not add work or alter the existing episode list.
func parseIndexPriorityIDs(value string) []string {
	seen := make(map[string]struct{})
	var ids []string
	for _, raw := range strings.Split(value, ",") {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// IndexPriorityIDsError reports every requested ID that was not present in
// the discovered provider episode list, in deterministic declaration order.
type IndexPriorityIDsError struct {
	Unknown []string
}

func (e *IndexPriorityIDsError) Error() string {
	return fmt.Sprintf("unknown --index-priority-ids episode ID(s): %s", strings.Join(e.Unknown, ", "))
}

// reorderIndexWork moves requested IDs to the front in declaration order and
// leaves all other work items in their original order. It never filters work;
// duplicate provider IDs in the discovered list are all retained and moved as
// one group. Unknown requested IDs are rejected before any playback attempt.
func reorderIndexWork(work []indexEpisodeWork, priorityIDs []string) ([]indexEpisodeWork, error) {
	if len(priorityIDs) == 0 {
		return append([]indexEpisodeWork(nil), work...), nil
	}

	positionsByID := make(map[string][]int, len(work))
	for position, item := range work {
		positionsByID[item.Episode.ID] = append(positionsByID[item.Episode.ID], position)
	}
	unknown := make([]string, 0)
	used := make([]bool, len(work))
	reordered := make([]indexEpisodeWork, 0, len(work))
	seenPriority := make(map[string]struct{}, len(priorityIDs))
	for _, id := range priorityIDs {
		if _, seen := seenPriority[id]; seen {
			continue
		}
		seenPriority[id] = struct{}{}
		positions, ok := positionsByID[id]
		if !ok {
			unknown = append(unknown, id)
			continue
		}
		for _, position := range positions {
			reordered = append(reordered, work[position])
			used[position] = true
		}
	}
	if len(unknown) > 0 {
		return nil, &IndexPriorityIDsError{Unknown: unknown}
	}
	for position, item := range work {
		if !used[position] {
			reordered = append(reordered, item)
		}
	}
	return reordered, nil
}

// newIndexCatalog creates the complete metadata snapshot before any playback
// stream is opened. In subtitle mode, every discovered episode is explicitly
// pending so an interrupted run never publishes a misleading processed prefix.
func newIndexCatalog(seriesTitle, seriesURL, subsLocale string, seasons []Season, seasonEpisodes map[string][]SeasonEpisode, prior map[string]indexEpisode, fetchSubs bool) (indexFile, []indexEpisodeWork) {
	catalog := indexFile{
		Series:     seriesTitle,
		SeriesURL:  seriesURL,
		SubsLang:   subsLocale,
		Seasons:    make([]indexSeason, 0, len(seasons)),
		Checkpoint: indexCheckpoint{Circuit: indexCircuitState{State: "closed"}},
	}
	work := make([]indexEpisodeWork, 0)
	for _, season := range seasons {
		episodes := seasonEpisodes[season.ID]
		catalog.Seasons = append(catalog.Seasons, indexSeason{Season: season.SeasonNumber, Episodes: make([]indexEpisode, 0, len(episodes))})
		seasonIndex := len(catalog.Seasons) - 1
		for _, ep := range episodes {
			status := indexStatusIndexed
			if fetchSubs {
				status = indexStatusPending
			}
			entry := indexEpisode{
				Episode:        ep.EpisodeNumber,
				Title:          ep.Title,
				Description:    ep.Description,
				ID:             ep.ID,
				URL:            "https://www.crunchyroll.com/watch/" + ep.ID,
				AirDate:        ep.AvailabilityStarts,
				AudioLangs:     collectAudioLangs(ep),
				Status:         status,
				SubtitleLocale: subsLocale,
				SourceVersion:  episodeSourceVersion(ep),
			}
			// Preserve only a locally checksum-verified, exact-locale raw ASS
			// checkpoint. Terminal missing/permanent rows retain their outcome
			// until the bounded source/snapshot recheck policy explicitly promotes
			// a sparse subset back to pending.
			if fetchSubs && canResumeRawASS(prior[indexEpisodeKey(ep.ID)], subsLocale) {
				cached := prior[indexEpisodeKey(ep.ID)]
				copyVerifiedRawASSCheckpoint(&entry, cached)
				entry.SubtitleAttemptCount = cached.SubtitleAttemptCount
				entry.SubtitleFetchAttemptCount = cached.SubtitleFetchAttemptCount
				entry.StreamReleaseAttemptCount = cached.StreamReleaseAttemptCount
				entry.SubtitleLastAttemptAt = cached.SubtitleLastAttemptAt
			} else if fetchSubs {
				priorEntry := prior[indexEpisodeKey(ep.ID)]
				if priorEntry.SubtitleLocale == subsLocale {
					entry.SubtitleAttemptCount = priorEntry.SubtitleAttemptCount
					entry.SubtitleFetchAttemptCount = priorEntry.SubtitleFetchAttemptCount
					entry.StreamReleaseAttemptCount = priorEntry.StreamReleaseAttemptCount
					entry.SubtitleLastAttemptAt = priorEntry.SubtitleLastAttemptAt
					entry.SubtitleNextEligibleAt = priorEntry.SubtitleNextEligibleAt
					entry.HTTPStatus = priorEntry.HTTPStatus
					entry.ProviderApplicationCode = priorEntry.ProviderApplicationCode
					entry.RetryAfter = priorEntry.RetryAfter
					entry.RateLimit = priorEntry.RateLimit
					entry.StreamReleaseOutcome = priorEntry.StreamReleaseOutcome
					entry.ProviderRestriction = priorEntry.ProviderRestriction
					entry.SubtitleCheckedSourceVersion = checkedSourceVersion(priorEntry)
					entry.SubtitleRecheckSnapshot = priorEntry.SubtitleRecheckSnapshot
					entry.SubtitleRecheckReason = priorEntry.SubtitleRecheckReason
					entry.SubtitleCueQuality = copyCueQuality(priorEntry.SubtitleCueQuality)
					entry.Error = priorEntry.Error
					switch normalizeIndexStatus(priorEntry.Status) {
					case indexStatusSubtitleMissing, indexStatusRetryableFailed, indexStatusPermanentFailed, indexStatusUnknownProviderPlaybackRestriction, indexStatusProviderRateLimited:
						entry.Status = normalizeIndexStatus(priorEntry.Status)
					}
				}
			}
			catalog.Seasons[seasonIndex].Episodes = append(catalog.Seasons[seasonIndex].Episodes, entry)
			work = append(work, indexEpisodeWork{SeasonIndex: seasonIndex, EpisodeIndex: len(catalog.Seasons[seasonIndex].Episodes) - 1, Episode: ep})
		}
	}
	catalog.CatalogSnapshot = catalogSnapshot(catalog)
	return catalog, work
}

func replaceCatalogEpisode(catalog *indexFile, work indexEpisodeWork, entry indexEpisode) {
	catalog.Seasons[work.SeasonIndex].Episodes[work.EpisodeIndex] = entry
}

func isSparseRecheckTerminalStatus(status string) bool {
	return status == indexStatusSubtitleMissing || status == indexStatusPermanentFailed
}

// scheduleSparseTerminalRechecks promotes only a small, deterministic subset
// of missing-locale/permanent rows back to pending. The caller's priority and
// newest-first traversal order is authoritative across reason classes. The
// separately persisted checked source version keeps any source change that
// misses this run's quota eligible on a later run.
func scheduleSparseTerminalRechecks(catalog *indexFile, work []indexEpisodeWork, prior map[string]indexEpisode, previousSnapshot string, limit int) int {
	if limit <= 0 || catalog.CatalogSnapshot == "" {
		return 0
	}
	scheduled := 0
	for _, item := range work {
		if scheduled >= limit {
			break
		}
		entry := catalog.Seasons[item.SeasonIndex].Episodes[item.EpisodeIndex]
		if !isSparseRecheckTerminalStatus(entry.Status) {
			continue
		}
		priorEntry, ok := prior[indexEpisodeKey(item.Episode.ID)]
		if !ok || priorEntry.SubtitleLocale != entry.SubtitleLocale {
			continue
		}

		reason := ""
		checkedVersion := checkedSourceVersion(priorEntry)
		if checkedVersion == "" || checkedVersion != entry.SourceVersion {
			reason = "source_version_changed"
		} else if previousSnapshot != "" && previousSnapshot != catalog.CatalogSnapshot && priorEntry.SubtitleRecheckSnapshot != catalog.CatalogSnapshot {
			reason = "catalog_snapshot_changed"
		}
		if reason == "" {
			continue
		}

		entry.Status = indexStatusPending
		entry.SubtitleNextEligibleAt = ""
		entry.SubtitleRecheckReason = reason
		replaceCatalogEpisode(catalog, item, entry)
		scheduled++
	}
	return scheduled
}

// writeIndex creates a serial, resumable metadata/subtitle index. Legacy .txt
// files are intentionally ignored: they are derived artifacts without raw-byte
// hashes and are never upgraded or reported as raw ASS checkpoints.
func writeIndex(seriesURL, contentID, primaryAudio, primarySubs string, fetchSubs bool) error {
	return writeIndexWithLoaders(seriesURL, contentID, primaryAudio, primarySubs, fetchSubs, getSeasons, getSeasonEpisodes)
}

// writeIndexWithLoaders keeps metadata acquisition injectable for deterministic
// error-path tests while the production path uses the provider-backed loaders.
func writeIndexWithLoaders(seriesURL, contentID, primaryAudio, primarySubs string, fetchSubs bool, loadSeasons func(string, string, string) []Season, loadEpisodes func(string, string, string) []SeasonEpisode) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			switch value := recovered.(type) {
			case error:
				err = fmt.Errorf("index metadata/provider failure: %w", value)
			default:
				err = fmt.Errorf("index metadata/provider failure: %v", value)
			}
		}
	}()

	if seriesURL == "" {
		seriesURL = "https://www.crunchyroll.com/series/" + contentID
	}
	if primarySubs == "" {
		return errors.New("a subtitle locale is required for --index-subs")
	}

	seasons := loadSeasons(contentID, primaryAudio, primarySubs)
	if len(seasons) == 0 {
		return errors.New("no seasons found")
	}
	seriesTitle := ""
	seasonEpisodes := make(map[string][]SeasonEpisode, len(seasons))
	for _, season := range seasons {
		episodes := loadEpisodes(season.ID, primaryAudio, primarySubs)
		seasonEpisodes[season.ID] = episodes
		if seriesTitle == "" && len(episodes) > 0 && episodes[0].SeriesTitle != "" {
			seriesTitle = episodes[0].SeriesTitle
		}
	}
	indexPath := indexFilename(seriesTitle, contentID)
	previous, err := loadIndex(indexPath)
	if err != nil {
		return fmt.Errorf("cannot resume %s: %w", indexPath, err)
	}
	prior := priorEpisodes(previous)
	catalog, work := newIndexCatalog(seriesTitle, seriesURL, primarySubs, seasons, seasonEpisodes, prior, fetchSubs)
	catalog.Checkpoint = previous.Checkpoint
	if catalog.Checkpoint.Circuit.State == "" {
		catalog.Checkpoint.Circuit.State = "closed"
	}
	if fetchSubs {
		work = sortIndexWorkNewestFirst(work)
		priorityIDs := parseIndexPriorityIDs(*indexPriority)
		var err error
		work, err = reorderIndexWork(work, priorityIDs)
		if err != nil {
			return err
		}
		recheckWindow := *indexTerminalRecheckWindow
		if recheckWindow > *indexWindow {
			recheckWindow = *indexWindow
		}
		scheduleSparseTerminalRechecks(&catalog, work, prior, previous.CatalogSnapshot, recheckWindow)
	}
	if err := snapshotIndex(indexPath, &catalog); err != nil {
		return fmt.Errorf("cannot write initial index %s: %w", indexPath, err)
	}
	if !fetchSubs {
		fmt.Printf("Index written: %s\n", indexPath)
		return nil
	}

	subsDir := sanitize(seriesTitle)
	if subsDir == "Unknown" {
		subsDir = sanitize(contentID)
	}
	subsDir += "_subs"
	if err := os.MkdirAll(subsDir, 0755); err != nil {
		return fmt.Errorf("cannot create subtitle directory %s: %w", subsDir, err)
	}

	delaySeconds := *indexDelay
	if delaySeconds < minimumIndexDelaySeconds {
		delaySeconds = minimumIndexDelaySeconds
	}
	summaryPath := strings.TrimSpace(*indexSummaryPath)
	if summaryPath == "" {
		summaryPath = indexPath + ".run-summary.json"
	}
	startedAt := time.Now().UTC()
	summary, err := processIndexWorkBounded(&catalog, work, prior, primarySubs, subsDir, fetchSubtitles, func(file *indexFile) error {
		return snapshotIndex(indexPath, file)
	}, indexRunOptions{
		Window: *indexWindow, Circuit4294Limit: *indexCircuitLimit,
		CircuitCooldown: *indexCircuitCooldown,
		Delay:           time.Duration(delaySeconds) * time.Second, Sleep: time.Sleep,
		Now: func() time.Time { return time.Now().UTC() }, StartedAt: startedAt,
		ProviderMetrics: providerCallMetricsSnapshot,
	})
	if writeErr := writeIndexRunSummary(summaryPath, summary); writeErr != nil {
		return fmt.Errorf("cannot write terminal run summary %s: %w", summaryPath, writeErr)
	}
	if err != nil {
		return fmt.Errorf("cannot checkpoint %s: %w", indexPath, err)
	}

	fmt.Printf("Index written: %s\n", indexPath)
	return nil
}

type indexSnapshotter func(*indexFile) error

// processIndexWork is deliberately serial. Each identity reaches a terminal
// cached/missing/failed state and is durably snapshotted before the next
// identity is attempted. Fetch failures are record-local and never stop older
// work; only inability to persist the checkpoint stops traversal.
func processIndexWork(catalog *indexFile, work []indexEpisodeWork, prior map[string]indexEpisode, locale, subsDir string, fetch subtitleFetcher, snapshot indexSnapshotter, delay time.Duration, sleep playbackSleeper) error {
	for _, item := range work {
		entry := catalog.Seasons[item.SeasonIndex].Episodes[item.EpisodeIndex]
		entry, needsDelay := checkpointSubtitleWithFetcher(item.Episode, entry, locale, subsDir, prior[indexEpisodeKey(item.Episode.ID)], fetch)
		if needsDelay && entry.SubtitleRecheckReason != "" && catalog.CatalogSnapshot != "" {
			entry.SubtitleRecheckSnapshot = catalog.CatalogSnapshot
		}
		replaceCatalogEpisode(catalog, item, entry)
		if err := snapshot(catalog); err != nil {
			return err
		}
		if needsDelay {
			sleep(delay)
		}
	}
	return nil
}

// checkpointSubtitle returns whether a subtitle playback/fetch attempt was
// made. Callers use that signal to preserve the provider delay for every
// attempt (including failures) while allowing verified local checkpoints to
// advance without sleeping.
func checkpointSubtitle(ep SeasonEpisode, entry indexEpisode, locale, subsDir string, prior indexEpisode) (indexEpisode, bool) {
	return checkpointSubtitleWithFetcher(ep, entry, locale, subsDir, prior, fetchSubtitles)
}

type subtitleFetcher func(string, string) (subtitleFetchResult, error)

func indexPlayback4294Retries() int {
	// The persisted global circuit owns 4294 recovery during indexing. Retrying
	// inside one identity would hide consecutive responses from that circuit and
	// multiply provider calls before the cursor can be checkpointed.
	return 0
}

func checkpointSubtitleWithFetcher(ep SeasonEpisode, entry indexEpisode, locale, subsDir string, prior indexEpisode, fetch subtitleFetcher) (indexEpisode, bool) {
	entry.SubtitleLocale = locale
	if canResumeRawASS(prior, locale) {
		copyVerifiedRawASSCheckpoint(&entry, prior)
		return entry, false
	}

	result, err := fetch(ep.ID, locale)
	attempts := result.PlaybackAttempts
	if attempts < 1 {
		// Test and alternate fetchers predate playback-attempt telemetry, but a
		// fetch invocation is still one real acquisition attempt.
		attempts = 1
	}
	entry.SubtitleAttemptCount = prior.SubtitleAttemptCount + attempts
	entry.SubtitleFetchAttemptCount = prior.SubtitleFetchAttemptCount + result.SubtitleFetchAttempts
	entry.StreamReleaseAttemptCount = prior.StreamReleaseAttemptCount + result.StreamReleaseAttempts
	entry.SubtitleLastAttemptAt = time.Now().UTC().Format(time.RFC3339Nano)
	entry.SubtitleLangs = result.AvailableLangs
	entry.StreamReleaseOutcome = result.StreamReleaseOutcome
	entry.HTTPStatus = result.HTTPStatus
	entry.ProviderApplicationCode = result.ProviderApplicationCode
	entry.RetryAfter = result.RetryAfter
	entry.RateLimit = result.RateLimit
	entry.ProviderRestriction = ""
	entry.SubtitleCheckedSourceVersion = entry.SourceVersion
	if err != nil {
		var missing *SubtitleLocaleError
		var mismatch *SubtitleLocaleMismatchError
		var playbackErr *PlaybackAPIError
		var httpErr *HTTPStatusError
		if errors.As(err, &playbackErr) {
			entry.HTTPStatus = playbackErr.HTTPStatus
			entry.ProviderApplicationCode = playbackErr.Code
			entry.RetryAfter = playbackErr.RetryAfter
			entry.RateLimit = playbackErr.RateLimit
		}
		if errors.As(err, &httpErr) {
			entry.HTTPStatus = httpErr.StatusCode
			entry.RetryAfter = httpErr.RetryAfter
			entry.RateLimit = httpErr.RateLimit
		}
		if errors.As(err, &missing) {
			entry.Status = indexStatusSubtitleMissing
		} else if errors.As(err, &mismatch) {
			entry.Status = indexStatusPermanentFailed
		} else if errors.As(err, &playbackErr) && playbackErr.Code == playback4294Code {
			entry.ProviderRestriction = providerRestrictionEvidence(entry.HTTPStatus, entry.RetryAfter, entry.RateLimit)
			if entry.ProviderRestriction == providerRestrictionRateLimited {
				entry.Status = indexStatusProviderRateLimited
			} else {
				entry.Status = indexStatusUnknownProviderPlaybackRestriction
			}
		} else if errors.As(err, &httpErr) && hasDirectRateLimitEvidence(entry.HTTPStatus, entry.RetryAfter, entry.RateLimit) {
			entry.ProviderRestriction = providerRestrictionRateLimited
			entry.Status = indexStatusProviderRateLimited
		} else if errors.As(err, &httpErr) && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 && httpErr.StatusCode != 408 && httpErr.StatusCode != 429 {
			entry.Status = indexStatusPermanentFailed
		} else {
			entry.Status = indexStatusRetryableFailed
		}
		entry.Error = redactSensitiveURLs(err.Error())
		return entry, true
	}

	relativePath := rawASSPath(subsDir, ep.SeasonNumber, ep.EpisodeNumber, ep.ID)
	if err := atomicWriteFile(relativePath, result.RawASS, 0644); err != nil {
		entry.Status = indexStatusSubtitleFailed
		entry.Error = redactSensitiveURLs(fmt.Sprintf("cache raw ASS: %v", err))
		return entry, true
	}
	sum := sha256.Sum256(result.RawASS)
	entry.Status = indexStatusSubtitleCached
	entry.SubtitleFile = relativePath // Link it in the checkpoint immediately.
	entry.SubtitleSHA256 = hex.EncodeToString(sum[:])
	entry.SubtitleParserVersion = rawASSParserVersion
	quality := assessASSCueQuality(result.RawASS)
	entry.SubtitleCueQuality = &quality
	entry.Error = ""
	return entry, true
}

// parseASS is intentionally a derived-view helper only. The index never uses
// its output as a cache: raw .ass bytes and their checksum remain canonical.
// It parses a declared Events format and preserves commas in the Text field.
var (
	assOverrideRe = regexp.MustCompile(`\{[^}]*\}`)
	assNewlineRe  = regexp.MustCompile(`\\[NnHh]`)
)

func parseASS(data []byte) string {
	transcript, _ := parseASSWithCueQuality(data)
	return transcript
}

func newASSCueQuality() subtitleCueQuality {
	return subtitleCueQuality{
		Version:              assCueQualityVersion,
		MaxMalformedCueRatio: maxMalformedASSCueRatio,
		MaxEmptyCueRatio:     maxEmptyASSCueRatio,
		MaxSkippedCueRatio:   maxSkippedASSCueRatio,
		Outcome:              "healthy",
	}
}

// assessASSCueQuality is deliberately local and non-mutating. It can be run
// against an existing checksum-verified cache without opening playback or
// downloading the subtitle again.
func assessASSCueQuality(data []byte) subtitleCueQuality {
	_, quality := parseASSWithCueQuality(data)
	return quality
}

func parseASSWithCueQuality(data []byte) (string, subtitleCueQuality) {
	var output strings.Builder
	quality := newASSCueQuality()
	inEvents := false
	var fields []string
	for _, original := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(original)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inEvents = strings.EqualFold(line, "[Events]")
			continue
		}
		if !inEvents {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "format:") {
			fields = splitASSFields(strings.TrimSpace(line[len("Format:"):]))
			continue
		}
		if !strings.HasPrefix(strings.ToLower(line), "dialogue:") {
			continue
		}
		quality.TotalCues++
		start, text, ok := parseASSDialogue(strings.TrimSpace(line[len("Dialogue:"):]), fields)
		if !ok {
			quality.MalformedCues++
			continue
		}
		text = strings.TrimSpace(assNewlineRe.ReplaceAllString(assOverrideRe.ReplaceAllString(text, ""), " "))
		if text == "" {
			quality.EmptyCues++
			continue
		}
		formattedStart, validStart := formatASSTimestampChecked(start)
		if !validStart {
			quality.SkippedCues++
			continue
		}
		quality.UsableCues++
		fmt.Fprintf(&output, "[%s] %s\n", formattedStart, text)
	}
	quality.Outcome = cueQualityOutcome(quality)
	return output.String(), quality
}

func cueQualityOutcome(quality subtitleCueQuality) string {
	if quality.TotalCues == 0 || quality.UsableCues == 0 {
		return "degraded"
	}
	if exceedsCueQualityRatio(quality.MalformedCues, quality.TotalCues, quality.MaxMalformedCueRatio) ||
		exceedsCueQualityRatio(quality.EmptyCues, quality.TotalCues, quality.MaxEmptyCueRatio) ||
		exceedsCueQualityRatio(quality.SkippedCues, quality.TotalCues, quality.MaxSkippedCueRatio) {
		return "degraded"
	}
	return "healthy"
}

func exceedsCueQualityRatio(count, total int, maximum float64) bool {
	return total > 0 && float64(count)/float64(total) > maximum
}

func splitASSFields(format string) []string {
	parts := strings.Split(format, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// parseASSDialogue avoids the common SplitN(..., 3) bug, which treats a comma
// in Text as an early boundary. The ASS grammar specifies Text as the final
// field; reject a malformed/reordered declaration instead of guessing.
func parseASSDialogue(payload string, fields []string) (start, text string, ok bool) {
	if len(fields) == 0 {
		return "", "", false
	}
	startIndex, textIndex := -1, -1
	for i, field := range fields {
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "start":
			startIndex = i
		case "text":
			textIndex = i
		}
	}
	if startIndex < 0 || textIndex != len(fields)-1 {
		return "", "", false
	}
	parts := strings.SplitN(payload, ",", len(fields))
	if len(parts) != len(fields) {
		return "", "", false
	}
	return strings.TrimSpace(parts[startIndex]), parts[textIndex], true
}

func formatASSTimestamp(value string) string {
	formatted, ok := formatASSTimestampChecked(value)
	if !ok {
		return "00:00:00"
	}
	return formatted
}

func formatASSTimestampChecked(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if dot := strings.IndexByte(value, '.'); dot >= 0 {
		fraction := value[dot+1:]
		if fraction == "" {
			return "", false
		}
		for _, digit := range fraction {
			if digit < '0' || digit > '9' {
				return "", false
			}
		}
		value = value[:dot]
	}
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return "", false
	}
	hour, errHour := strconv.Atoi(parts[0])
	minute, errMinute := strconv.Atoi(parts[1])
	second, errSecond := strconv.Atoi(parts[2])
	if errHour != nil || errMinute != nil || errSecond != nil || hour < 0 || minute < 0 || minute >= 60 || second < 0 || second >= 60 {
		return "", false
	}
	return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second), true
}
