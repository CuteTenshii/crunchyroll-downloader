package engine

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/unki2aut/go-mpd"
)

// InspectRequest describes a catalog/quality probe against a Crunchyroll URL.
// JSON tags are required so Wails includes fields when binding from the frontend.
type InspectRequest struct {
	URL              string `json:"url"`
	ETPRTFile        string `json:"etpRtFile"`
	PrimaryAudioHint string `json:"primaryAudioHint"` // default ja-JP
	PrimarySubsHint  string `json:"primarySubsHint"`  // for CMS locale params only
	ProbePlayback    bool   `json:"probePlayback"`    // open playback once for subs/CC/qualities
	ProbeContentID   string `json:"probeContentId"`   // empty = first episode / watch id
}

// CatalogSeason is a series season entry returned by Inspect.
type CatalogSeason struct {
	ID           string `json:"id"`
	SeasonNumber int    `json:"seasonNumber"`
}

// CatalogEpisode is a catalog row for one episode (metadata only).
type CatalogEpisode struct {
	ID            string   `json:"id"`
	SeasonNumber  int      `json:"seasonNumber"`
	EpisodeNumber int      `json:"episodeNumber"`
	Title         string   `json:"title"`
	SeriesTitle   string   `json:"seriesTitle"`
	AudioLocales  []string `json:"audioLocales"` // from versions + primary
	// ThumbnailURL is a small CDN image URL already present in CMS JSON.
	// The WebView loads it from static CDN — not an extra playback/API call.
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

// InspectResult is the GUI/catalog snapshot for one URL.
type InspectResult struct {
	ContentType      string           `json:"contentType"` // watch | series
	ContentID        string           `json:"contentID"`
	Seasons          []CatalogSeason  `json:"seasons"`
	Episodes         []CatalogEpisode `json:"episodes"`
	AudioLocales     []string         `json:"audioLocales"`
	SubtitleLocales  []string         `json:"subtitleLocales"`
	CaptionLocales   []string         `json:"captionLocales"`
	VideoQualities   []string         `json:"videoQualities"` // e.g. 1080p
	AudioQualities   []string         `json:"audioQualities"` // e.g. 192k
	DefaultEpisodeID string           `json:"defaultEpisodeID"`
	OriginalAudio    string           `json:"originalAudio"`
	// PosterURL is the series/movie hero image (CDN). Series Inspect does one
	// extra CMS series GET for this; watch uses the object images already loaded.
	PosterURL   string `json:"posterUrl,omitempty"`
	DisplayTitle string `json:"displayTitle,omitempty"`
}

// Test seams so Inspect unit tests do not hit live Crunchyroll.
var (
	inspectRefreshAccessToken = refreshAccessToken
	inspectGetSeasons         = getSeasons
	inspectGetSeasonEpisodes  = getSeasonEpisodes
	inspectGetEpisodeInfo     = getEpisodeInfo
	inspectGetSeriesInfo      = getSeriesInfo
)

// ListVideoQualities returns unique video heights as "Np" labels, highest first.
func ListVideoQualities(manifest *mpd.MPD) []string {
	if manifest == nil {
		return nil
	}
	seen := map[uint64]struct{}{}
	var heights []uint64
	for _, period := range manifest.Period {
		if period == nil {
			continue
		}
		for _, set := range period.AdaptationSets {
			if set == nil {
				continue
			}
			for i := range set.Representations {
				rep := &set.Representations[i]
				if rep.Height == nil || *rep.Height == 0 {
					continue
				}
				h := *rep.Height
				if _, ok := seen[h]; ok {
					continue
				}
				seen[h] = struct{}{}
				heights = append(heights, h)
			}
		}
	}
	sort.Slice(heights, func(i, j int) bool { return heights[i] > heights[j] })
	out := make([]string, 0, len(heights))
	for _, h := range heights {
		out = append(out, strconv.FormatUint(h, 10)+"p")
	}
	return out
}

// ListAudioQualities returns detected audio quality labels (192k/128k/96k), highest first.
// Only audio representations are considered (Height == nil and ID looks like audio).
// Video reps with high bandwidth must not be labeled as audio bitrates.
func ListAudioQualities(manifest *mpd.MPD) []string {
	if manifest == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var qualities []string
	add := func(label string) {
		if label == "" {
			return
		}
		if _, ok := seen[label]; ok {
			return
		}
		seen[label] = struct{}{}
		qualities = append(qualities, label)
	}
	for _, period := range manifest.Period {
		if period == nil {
			continue
		}
		for _, set := range period.AdaptationSets {
			if set == nil {
				continue
			}
			for i := range set.Representations {
				rep := &set.Representations[i]
				if !isAudioRepresentation(rep) {
					continue
				}
				add(detectAudioQualityLabel(rep))
			}
		}
	}
	sort.Slice(qualities, func(i, j int) bool {
		return audioQualityRank(qualities[i]) > audioQualityRank(qualities[j])
	})
	return qualities
}

// isAudioRepresentation reports whether a DASH representation is an audio track.
// Video representations have Height set; Crunchyroll audio IDs typically contain "audio/".
func isAudioRepresentation(rep *mpd.Representation) bool {
	if rep == nil {
		return false
	}
	// Pure video (or any visual) stream — never treat as audio.
	if rep.Height != nil {
		return false
	}
	if rep.ID != nil {
		id := *rep.ID
		// Prefer explicit audio/ IDs (same check as getBaseUrl).
		if strings.Contains(id, "audio/") {
			return true
		}
		// Explicit video IDs without height still skip.
		if strings.Contains(id, "video/") {
			return false
		}
	}
	// Height-less rep without a video/ ID: treat as audio candidate
	// (bandwidth-only matching for non-standard IDs).
	return true
}

// detectAudioQualityLabel maps an audio representation to 192k/128k/96k.
// ID patterns are preferred (matching getBaseUrl's quality substring check on full
// labels like "192k"), then bandwidth heuristics. Bare "192"/"128"/"96" substrings
// are avoided so video IDs containing "1920" never match.
func detectAudioQualityLabel(rep *mpd.Representation) string {
	if rep == nil {
		return ""
	}
	if rep.ID != nil {
		if label := audioQualityFromID(*rep.ID); label != "" {
			return label
		}
	}
	if rep.Bandwidth != nil {
		bw := *rep.Bandwidth
		switch {
		case bw >= 192000:
			return "192k"
		case bw >= 128000:
			return "128k"
		case bw >= 96000:
			return "96k"
		}
	}
	return ""
}

// audioQualityFromID extracts a bitrate label from a representation ID.
// Matches full quality tokens used by getBaseUrl ("192k") and path suffixes
// like "/192k", never a bare "192" that would hit "1920" in video dimensions.
func audioQualityFromID(id string) string {
	// Prefer longest / most specific labels first.
	for _, label := range []string{"192k", "128k", "96k"} {
		if strings.Contains(id, label) {
			return label
		}
	}
	// Also accept trailing path bitrate without "k" only when delimited:
	// e.g. ".../192" or ".../192/" — not "1920".
	for _, num := range []struct {
		token string
		label string
	}{
		{"/192k", "192k"},
		{"/128k", "128k"},
		{"/96k", "96k"},
		{"/192/", "192k"},
		{"/128/", "128k"},
		{"/96/", "96k"},
	} {
		if strings.Contains(id, num.token) {
			return num.label
		}
	}
	for _, num := range []struct {
		suffix string
		label  string
	}{
		{"/192", "192k"},
		{"/128", "128k"},
		{"/96", "96k"},
	} {
		if strings.HasSuffix(id, num.suffix) {
			return num.label
		}
	}
	return ""
}

func audioQualityRank(label string) int {
	switch label {
	case "192k":
		return 192
	case "128k":
		return 128
	case "96k":
		return 96
	default:
		return 0
	}
}

// Inspect authenticates, loads catalog metadata for a watch/series URL, and
// optionally opens one playback stream to list qualities and subtitle locales.
func Inspect(req InspectRequest, cfg RuntimeConfig) (result InspectResult, err error) {
	SetRuntimeConfig(cfg)

	etpRT, err := loadETPRT(req.ETPRTFile)
	if err != nil {
		return result, fmt.Errorf("authentication setup failed: %w", err)
	}
	setETPRT(etpRT)
	if err := inspectRefreshAccessToken(); err != nil {
		return result, fmt.Errorf("authentication failed: %w", err)
	}

	contentType, contentID, err := parseInspectURL(req.URL)
	if err != nil {
		return result, err
	}
	result.ContentType = contentType
	result.ContentID = contentID

	primaryAudio := strings.TrimSpace(req.PrimaryAudioHint)
	if primaryAudio == "" {
		primaryAudio = "ja-JP"
	}
	primarySubs := strings.TrimSpace(req.PrimarySubsHint)
	if primarySubs == "" {
		primarySubs = "en-US"
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("inspect %s: %w", req.URL, panicAsError(recovered))
		}
	}()

	switch contentType {
	case "watch":
		if err := fillInspectWatch(&result, contentID); err != nil {
			return result, err
		}
	case "series":
		if err := fillInspectSeries(&result, contentID, primaryAudio, primarySubs); err != nil {
			return result, err
		}
	default:
		return result, fmt.Errorf("unsupported content type %q", contentType)
	}

	if req.ProbePlayback {
		probeID := strings.TrimSpace(req.ProbeContentID)
		if probeID == "" {
			probeID = result.DefaultEpisodeID
		}
		if probeID == "" {
			return result, fmt.Errorf("probe playback requested but no episode id is available")
		}
		if err := probeInspectPlayback(&result, probeID); err != nil {
			return result, err
		}
	}

	return result, nil
}

func parseInspectURL(raw string) (contentType, contentID string, err error) {
	url := raw
	if i := strings.IndexAny(url, "?#"); i >= 0 {
		url = url[:i]
	}
	url = strings.TrimRight(url, "/")

	parts := strings.Split(url, "/")
	if len(parts) < 5 {
		return "", "", fmt.Errorf("invalid URL format: %s", raw)
	}
	contentType = parts[3]
	contentID = parts[4]
	if !isValidContentID(contentID) {
		return "", "", fmt.Errorf("invalid URL format (bad content id %q): %s", contentID, raw)
	}
	if contentType != "watch" && contentType != "series" {
		return "", "", fmt.Errorf("invalid URL (must be /watch/ or /series/): %s", raw)
	}
	return contentType, contentID, nil
}

func fillInspectWatch(result *InspectResult, contentID string) error {
	info := inspectGetEpisodeInfo(contentID)
	locales := audioLocalesFromEpisodeInfo(info)
	thumb := thumbnailFromImages(info.Images)
	result.Episodes = []CatalogEpisode{{
		ID:            contentID,
		SeasonNumber:  info.EpisodeMetadata.SeasonNumber,
		EpisodeNumber: info.EpisodeMetadata.EpisodeNumber,
		Title:         info.Title,
		SeriesTitle:   info.EpisodeMetadata.SeriesTitle,
		AudioLocales:  locales,
		ThumbnailURL:  thumb,
	}}
	result.AudioLocales = locales
	result.DefaultEpisodeID = contentID
	result.OriginalAudio = info.EpisodeMetadata.AudioLocale
	result.PosterURL = posterFromImages(info.Images)
	if result.PosterURL == "" {
		result.PosterURL = thumb
	}
	if info.EpisodeMetadata.SeriesTitle != "" {
		result.DisplayTitle = info.EpisodeMetadata.SeriesTitle
	} else {
		result.DisplayTitle = info.Title
	}
	return nil
}

func fillInspectSeries(result *InspectResult, contentID, primaryAudio, primarySubs string) error {
	// Optional series poster/title — one extra CMS GET, no playback.
	if series, err := inspectGetSeriesInfo(contentID, primarySubs); err == nil {
		result.PosterURL = posterFromImages(series.Images)
		result.DisplayTitle = series.Title
	}

	seasons := inspectGetSeasons(contentID, primaryAudio, primarySubs)
	if len(seasons) == 0 {
		return fmt.Errorf("no seasons found")
	}

	result.Seasons = make([]CatalogSeason, 0, len(seasons))
	for _, season := range seasons {
		result.Seasons = append(result.Seasons, CatalogSeason{
			ID:           season.ID,
			SeasonNumber: season.SeasonNumber,
		})
	}

	// Prefer true season 1; otherwise use the lowest season number present.
	seasonID, seasonNumber := pickInspectSeason(seasons)
	episodes := inspectGetSeasonEpisodes(seasonID, primaryAudio, primarySubs)
	result.Episodes = make([]CatalogEpisode, 0, len(episodes))
	audioSeen := map[string]struct{}{}
	var audioLocales []string
	addAudio := func(locale string) {
		if locale == "" {
			return
		}
		if _, ok := audioSeen[locale]; ok {
			return
		}
		audioSeen[locale] = struct{}{}
		audioLocales = append(audioLocales, locale)
	}

	for _, ep := range episodes {
		locales := collectAudioLangs(ep)
		for _, locale := range locales {
			addAudio(locale)
		}
		if result.DisplayTitle == "" && ep.SeriesTitle != "" {
			result.DisplayTitle = ep.SeriesTitle
		}
		result.Episodes = append(result.Episodes, CatalogEpisode{
			ID:            ep.ID,
			SeasonNumber:  ep.SeasonNumber,
			EpisodeNumber: ep.EpisodeNumber,
			Title:         ep.Title,
			SeriesTitle:   ep.SeriesTitle,
			AudioLocales:  locales,
			ThumbnailURL:  thumbnailFromImages(ep.Images),
		})
	}
	sort.Strings(audioLocales)
	result.AudioLocales = audioLocales

	// Default to S1E1 (or first episode of the chosen season).
	if len(result.Episodes) > 0 {
		def := result.Episodes[0]
		for _, ep := range result.Episodes {
			if seasonNumber != 0 && ep.SeasonNumber == seasonNumber && ep.EpisodeNumber == 1 {
				def = ep
				break
			}
			if ep.EpisodeNumber < def.EpisodeNumber {
				def = ep
			}
		}
		result.DefaultEpisodeID = def.ID
		if def.AudioLocales != nil && len(def.AudioLocales) > 0 {
			// Prefer primary audio if present on the default episode.
			result.OriginalAudio = def.AudioLocales[0]
			for _, locale := range def.AudioLocales {
				if locale == primaryAudio {
					result.OriginalAudio = locale
					break
				}
			}
		}
		// Prefer the CMS primary audio locale field when available on the source row.
		for _, ep := range episodes {
			if ep.ID == def.ID && ep.AudioLocale != "" {
				result.OriginalAudio = ep.AudioLocale
				break
			}
		}
	}
	_ = seasonNumber
	return nil
}

func pickInspectSeason(seasons []Season) (id string, number int) {
	// Prefer season number 1 when present.
	for _, season := range seasons {
		if season.SeasonNumber == 1 {
			return season.ID, season.SeasonNumber
		}
	}
	// Otherwise pick the lowest positive season number (or first if unnumbered).
	best := seasons[0]
	for _, season := range seasons[1:] {
		if best.SeasonNumber <= 0 || (season.SeasonNumber > 0 && season.SeasonNumber < best.SeasonNumber) {
			best = season
		}
	}
	return best.ID, best.SeasonNumber
}

func audioLocalesFromEpisodeInfo(info EpisodeInfo) []string {
	seen := map[string]bool{}
	var langs []string
	add := func(locale string) {
		if locale != "" && !seen[locale] {
			seen[locale] = true
			langs = append(langs, locale)
		}
	}
	add(info.EpisodeMetadata.AudioLocale)
	for _, version := range info.EpisodeMetadata.Versions {
		if version != nil {
			add(version.AudioLocale)
		}
	}
	sort.Strings(langs)
	return langs
}

func probeInspectPlayback(result *InspectResult, contentID string) (err error) {
	var episode Episode
	var opened bool
	defer func() {
		if !opened || episode.Token == "" {
			return
		}
		released, releaseErr := releaseDownloadPlayback(contentID, episode.Token)
		if releaseErr != nil {
			if err == nil {
				err = releaseErr
			} else {
				err = fmt.Errorf("%w; %v", err, releaseErr)
			}
			return
		}
		if !released && err == nil {
			err = fmt.Errorf("release playback stream for %s: provider rejected cleanup", contentID)
		}
	}()

	if openErr := func() (openErr error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				openErr = fmt.Errorf("open playback for %s: %w", contentID, panicAsError(recovered))
			}
		}()
		episode = openPlaybackWithRetry(
			contentID,
			openDownloadPlayback,
			activeConfig.Playback4294Retries,
			activeConfig.Playback4294Backoff,
			sleepPlaybackRetry,
		)
		opened = true
		return nil
	}(); openErr != nil {
		return openErr
	}

	result.SubtitleLocales = sortedSubtitleLocales(episode.Subtitles)
	result.CaptionLocales = sortedSubtitleLocales(episode.Captions)

	if episode.ManifestURL == "" {
		return fmt.Errorf("playback for %s returned no manifest URL", contentID)
	}

	var manifest *mpd.MPD
	if parseErr := func() (parseErr error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				parseErr = fmt.Errorf("parse manifest for %s: %w", contentID, panicAsError(recovered))
			}
		}()
		manifest = parseDownloadManifest(episode.ManifestURL)
		return nil
	}(); parseErr != nil {
		return parseErr
	}

	result.VideoQualities = ListVideoQualities(manifest)
	result.AudioQualities = ListAudioQualities(manifest)
	return nil
}
