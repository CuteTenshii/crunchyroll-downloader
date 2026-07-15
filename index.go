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

const rawASSParserVersion = "raw-ass/v1"
const minimumIndexDelaySeconds = 2

var openIndexPlayback playbackOpener = getEpisode

const (
	indexStatusPending         = "pending"
	indexStatusIndexed         = "indexed"
	indexStatusSubtitleCached  = "subtitle_cached"
	indexStatusSubtitleMissing = "subtitle_missing"
	indexStatusSubtitleFailed  = "subtitle_failed"
)

// indexFile is a durable, self-contained checkpoint. Every terminal episode
// state is followed by an atomic rewrite of this full snapshot.
type indexFile struct {
	Series    string        `json:"series"`
	SeriesURL string        `json:"series_url"`
	IndexedAt string        `json:"indexed_at"`
	SubsLang  string        `json:"subs_lang,omitempty"`
	Seasons   []indexSeason `json:"seasons"`
}

type indexSeason struct {
	Season   int            `json:"season"`
	Episodes []indexEpisode `json:"episodes"`
}

type indexEpisode struct {
	Episode               int      `json:"episode"`
	Title                 string   `json:"title"`
	Description           string   `json:"description,omitempty"`
	ID                    string   `json:"id"`
	URL                   string   `json:"url"`
	AirDate               string   `json:"air_date,omitempty"`
	AudioLangs            []string `json:"audio_langs,omitempty"`
	SubtitleLangs         []string `json:"subtitle_langs,omitempty"`
	Status                string   `json:"status"`
	SubtitleFile          string   `json:"subtitle_file,omitempty"`
	SubtitleLocale        string   `json:"subtitle_locale,omitempty"`
	SubtitleSHA256        string   `json:"subtitle_sha256,omitempty"`
	SubtitleParserVersion string   `json:"subtitle_parser_version,omitempty"`
	Error                 string   `json:"error,omitempty"`
}

func (episode indexEpisode) isVerifiedRawASS() bool {
	return episode.Status == indexStatusSubtitleCached &&
		episode.SubtitleFile != "" &&
		episode.SubtitleLocale != "" &&
		episode.SubtitleSHA256 != "" &&
		episode.SubtitleParserVersion == rawASSParserVersion
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
	AvailableLangs []string
	RawASS         []byte
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
				err = fmt.Errorf("open playback for %s: %v", episodeID, recovered)
			}
		}()
		episode = openPlaybackWithRetry(episodeID, openIndexPlayback, *playback4294Retries, *playback4294Backoff, sleepPlaybackRetry)
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
				released = deleteStream(episodeID, episode.Token)
			}()
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
		Series:    seriesTitle,
		SeriesURL: seriesURL,
		SubsLang:  subsLocale,
		Seasons:   make([]indexSeason, 0, len(seasons)),
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
			}
			// Preserve only a locally checksum-verified, exact-locale raw ASS
			// checkpoint. Missing/failed/stale entries stay pending and will be
			// retried serially after this complete initial snapshot is durable.
			if fetchSubs && canResumeRawASS(prior[indexEpisodeKey(ep.ID)], subsLocale) {
				cached := prior[indexEpisodeKey(ep.ID)]
				entry.Status = indexStatusSubtitleCached
				entry.SubtitleFile = cached.SubtitleFile
				entry.SubtitleSHA256 = cached.SubtitleSHA256
				entry.SubtitleParserVersion = cached.SubtitleParserVersion
				entry.SubtitleLangs = cached.SubtitleLangs
			}
			catalog.Seasons[seasonIndex].Episodes = append(catalog.Seasons[seasonIndex].Episodes, entry)
			work = append(work, indexEpisodeWork{SeasonIndex: seasonIndex, EpisodeIndex: len(catalog.Seasons[seasonIndex].Episodes) - 1, Episode: ep})
		}
	}
	return catalog, work
}

func replaceCatalogEpisode(catalog *indexFile, work indexEpisodeWork, entry indexEpisode) {
	catalog.Seasons[work.SeasonIndex].Episodes[work.EpisodeIndex] = entry
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
	if fetchSubs {
		priorityIDs := parseIndexPriorityIDs(*indexPriority)
		var err error
		work, err = reorderIndexWork(work, priorityIDs)
		if err != nil {
			return err
		}
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

	for _, item := range work {
		entry := catalog.Seasons[item.SeasonIndex].Episodes[item.EpisodeIndex]
		var needsDelay bool
		entry, needsDelay = checkpointSubtitle(item.Episode, entry, primarySubs, subsDir, prior[indexEpisodeKey(item.Episode.ID)])
		replaceCatalogEpisode(&catalog, item, entry)
		// Each terminal state updates the already-complete catalog atomically.
		if err := snapshotIndex(indexPath, &catalog); err != nil {
			return fmt.Errorf("cannot checkpoint %s: %w", indexPath, err)
		}
		// A checksum-valid raw ASS resume did not touch playback or the
		// subtitle endpoint. Only actual attempts need the provider pacing delay.
		if needsDelay {
			delaySeconds := *indexDelay
			if delaySeconds < minimumIndexDelaySeconds {
				delaySeconds = minimumIndexDelaySeconds
			}
			time.Sleep(time.Duration(delaySeconds) * time.Second)
		}
	}

	fmt.Printf("Index written: %s\n", indexPath)
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

func checkpointSubtitleWithFetcher(ep SeasonEpisode, entry indexEpisode, locale, subsDir string, prior indexEpisode, fetch subtitleFetcher) (indexEpisode, bool) {
	entry.SubtitleLocale = locale
	if canResumeRawASS(prior, locale) {
		entry.Status = indexStatusSubtitleCached
		entry.SubtitleFile = prior.SubtitleFile
		entry.SubtitleSHA256 = prior.SubtitleSHA256
		entry.SubtitleParserVersion = rawASSParserVersion
		entry.SubtitleLangs = prior.SubtitleLangs
		return entry, false
	}

	result, err := fetch(ep.ID, locale)
	entry.SubtitleLangs = result.AvailableLangs
	if err != nil {
		var missing *SubtitleLocaleError
		if errors.As(err, &missing) {
			entry.Status = indexStatusSubtitleMissing
		} else {
			entry.Status = indexStatusSubtitleFailed
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
	var output strings.Builder
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
		if strings.HasPrefix(line, "Format:") {
			fields = splitASSFields(strings.TrimSpace(strings.TrimPrefix(line, "Format:")))
			continue
		}
		if !strings.HasPrefix(line, "Dialogue:") {
			continue
		}
		start, text, ok := parseASSDialogue(strings.TrimSpace(strings.TrimPrefix(line, "Dialogue:")), fields)
		if !ok {
			continue
		}
		text = strings.TrimSpace(assNewlineRe.ReplaceAllString(assOverrideRe.ReplaceAllString(text, ""), " "))
		if text != "" {
			fmt.Fprintf(&output, "[%s] %s\n", formatASSTimestamp(start), text)
		}
	}
	return output.String()
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
	value = strings.TrimSpace(value)
	if dot := strings.IndexByte(value, '.'); dot >= 0 {
		value = value[:dot]
	}
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return "00:00:00"
	}
	hour, errHour := strconv.Atoi(parts[0])
	minute, errMinute := strconv.Atoi(parts[1])
	second, errSecond := strconv.Atoi(parts[2])
	if errHour != nil || errMinute != nil || errSecond != nil || hour < 0 || minute < 0 || second < 0 {
		return "00:00:00"
	}
	return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
}
