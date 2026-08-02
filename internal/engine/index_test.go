package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckpointSubtitleResumesOnlyMatchingRawASS(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("[Script Info]\r\nTitle: untouched\r\n[Events]\r\n")
	path := rawASSPath(dir, 1, 1, "episode")
	if err := atomicWriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	prior := indexEpisode{
		Status:                indexStatusSubtitleCached,
		SubtitleFile:          path,
		SubtitleLocale:        "en-US",
		SubtitleSHA256:        hex.EncodeToString(sum[:]),
		SubtitleParserVersion: rawASSParserVersion,
		SubtitleLangs:         []string{"en-US"},
	}
	got, needsDelay := checkpointSubtitle(SeasonEpisode{ID: "episode", SeasonNumber: 1, EpisodeNumber: 1}, indexEpisode{}, "en-US", dir, prior)
	if needsDelay {
		t.Fatal("verified cached checkpoint must not require a provider delay")
	}
	if got.Status != indexStatusSubtitleCached || got.SubtitleFile != path || got.SubtitleSHA256 != prior.SubtitleSHA256 {
		t.Fatalf("unexpected verified resume checkpoint: %#v", got)
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(raw) {
		t.Fatalf("raw ASS changed: got %q want %q", actual, raw)
	}
	if err := os.WriteFile(path, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if canResumeRawASS(prior, "en-US") {
		t.Fatal("checksum mismatch must not be resumed")
	}
}

func TestCheckpointSubtitleFetchSuccessRequiresDelay(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("[Script Info]\n[Events]\n")
	fetch := func(episodeID, locale string) (subtitleFetchResult, error) {
		if episodeID != "episode" || locale != "en-US" {
			t.Fatalf("unexpected fetch request: episode=%q locale=%q", episodeID, locale)
		}
		return subtitleFetchResult{AvailableLangs: []string{"en-US"}, RawASS: raw}, nil
	}

	got, needsDelay := checkpointSubtitleWithFetcher(
		SeasonEpisode{ID: "episode", SeasonNumber: 1, EpisodeNumber: 1},
		indexEpisode{}, "en-US", dir, indexEpisode{}, fetch,
	)
	if !needsDelay {
		t.Fatal("successful subtitle fetch must require a provider delay")
	}
	if got.Status != indexStatusSubtitleCached || got.SubtitleFile != rawASSPath(dir, 1, 1, "episode") {
		t.Fatalf("unexpected successful fetch checkpoint: %#v", got)
	}
}

func TestCheckpointSubtitleFetchFailureRequiresDelay(t *testing.T) {
	dir := t.TempDir()
	fetch := func(episodeID, locale string) (subtitleFetchResult, error) {
		if episodeID != "episode" || locale != "en-US" {
			t.Fatalf("unexpected fetch request: episode=%q locale=%q", episodeID, locale)
		}
		return subtitleFetchResult{AvailableLangs: []string{"en-US"}}, errors.New("provider unavailable")
	}

	got, needsDelay := checkpointSubtitleWithFetcher(
		SeasonEpisode{ID: "episode", SeasonNumber: 1, EpisodeNumber: 1},
		indexEpisode{}, "en-US", dir, indexEpisode{}, fetch,
	)
	if !needsDelay {
		t.Fatal("failed subtitle fetch must require a provider delay")
	}
	if got.Status != indexStatusSubtitleFailed || got.Error != "provider unavailable" {
		t.Fatalf("unexpected failed fetch checkpoint: %#v", got)
	}
}

func TestFetchSubtitlesSurfacesFirst4294ToGlobalCircuit(t *testing.T) {
	originalOpen := openIndexPlayback
	originalSleep := sleepPlaybackRetry
	originalRetries := activeConfig.Playback4294Retries
	originalBackoff := activeConfig.Playback4294Backoff
	defer func() {
		openIndexPlayback = originalOpen
		sleepPlaybackRetry = originalSleep
		activeConfig.Playback4294Retries = originalRetries
		activeConfig.Playback4294Backoff = originalBackoff
	}()

	activeConfig.Playback4294Retries = 2
	activeConfig.Playback4294Backoff = 4 * time.Second
	calls := 0
	var delays []time.Duration
	openIndexPlayback = func(id string) Episode {
		calls++
		panic(&PlaybackAPIError{EpisodeID: id, Code: playback4294Code})
	}
	sleepPlaybackRetry = func(delay time.Duration) { delays = append(delays, delay) }

	_, err := fetchSubtitles("episode", "en-US")
	var playbackErr *PlaybackAPIError
	if !errors.As(err, &playbackErr) || playbackErr.Code != playback4294Code {
		t.Fatalf("fetchSubtitles() error = %v, want typed 4294", err)
	}
	if calls != 1 || len(delays) != 0 {
		t.Fatalf("calls=%d delays=%v", calls, delays)
	}
}

func TestRawASSPathIncludesEpisodeID(t *testing.T) {
	first := rawASSPath("subs", 1, 1, "G123ABC")
	second := rawASSPath("subs", 1, 1, "G456DEF")
	if first == second {
		t.Fatalf("duplicate provider variants collide at %q", first)
	}
	if !strings.Contains(first, "G123ABC") || !strings.Contains(second, "G456DEF") {
		t.Fatalf("episode IDs missing from paths: %q, %q", first, second)
	}
}

func TestExactSubtitleForLocaleRejectsLanguageMismatch(t *testing.T) {
	_, err := exactSubtitleForLocale(map[string]*Subtitle{
		"en-US": {Language: "ja-JP", URL: "https://cdn.example/sub.ass"},
	}, "en-US")
	var mismatch *SubtitleLocaleMismatchError
	if !errors.As(err, &mismatch) || mismatch.Requested != "en-US" || mismatch.Actual != "ja-JP" {
		t.Fatalf("expected exact locale mismatch, got %v", err)
	}
}

func TestCatalogSnapshotRetainsLaterPendingEpisodes(t *testing.T) {
	seasons := []Season{{ID: "season", SeasonNumber: 1}}
	episodes := map[string][]SeasonEpisode{"season": {
		{ID: "first", EpisodeNumber: 1, Title: "First", SeriesTitle: "Series"},
		{ID: "second", EpisodeNumber: 2, Title: "Second", SeriesTitle: "Series"},
	}}
	catalog, work := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, episodes, nil, true)
	if len(work) != 2 || catalog.Seasons[0].Episodes[1].Status != indexStatusPending {
		t.Fatalf("catalog did not predeclare complete pending work: %#v", catalog)
	}
	first := catalog.Seasons[0].Episodes[0]
	first.Status = indexStatusSubtitleMissing
	replaceCatalogEpisode(&catalog, work[0], first)
	data, err := marshalIndex(catalog)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot indexFile
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if got := snapshot.Seasons[0].Episodes; len(got) != 2 || got[0].Status != indexStatusSubtitleMissing || got[1].ID != "second" || got[1].Status != indexStatusPending {
		t.Fatalf("terminal update published a partial/misleading snapshot: %#v", snapshot)
	}
}

func TestInitialCatalogPreservesOnlyVerifiedPriorCheckpoints(t *testing.T) {
	dir := t.TempDir()
	validPath := rawASSPath(dir, 1, 2, "second")
	raw := []byte("[Script Info]\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	if err := atomicWriteFile(validPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	seasons := []Season{{ID: "season", SeasonNumber: 1}}
	episodes := map[string][]SeasonEpisode{"season": {
		{ID: "first", EpisodeNumber: 1, Title: "First", SeriesTitle: "Series"},
		{ID: "second", EpisodeNumber: 2, Title: "Second", SeriesTitle: "Series"},
	}}
	prior := map[string]indexEpisode{
		"first": {
			Status:                indexStatusSubtitleCached,
			SubtitleFile:          validPath,
			SubtitleLocale:        "en-US",
			SubtitleSHA256:        "not-the-file-checksum",
			SubtitleParserVersion: rawASSParserVersion,
		},
		"second": {
			Status:                indexStatusSubtitleCached,
			SubtitleFile:          validPath,
			SubtitleLocale:        "en-US",
			SubtitleSHA256:        hex.EncodeToString(sum[:]),
			SubtitleParserVersion: rawASSParserVersion,
			SubtitleLangs:         []string{"en-US"},
		},
	}
	catalog, _ := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, episodes, prior, true)
	got := catalog.Seasons[0].Episodes
	if got[0].Status != indexStatusPending || got[0].SubtitleFile != "" {
		t.Fatalf("invalid checksum was carried into initial snapshot: %#v", got[0])
	}
	if got[1].Status != indexStatusSubtitleCached || got[1].SubtitleFile != validPath || got[1].SubtitleSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("valid later checkpoint was not retained: %#v", got[1])
	}
}

func TestInitialCatalogMigratesLegacy4294InferenceToNeutralRestriction(t *testing.T) {
	seasons := []Season{{ID: "season", SeasonNumber: 1}}
	episodes := map[string][]SeasonEpisode{"season": {{ID: "episode", EpisodeNumber: 1, SeriesTitle: "Series"}}}
	prior := map[string]indexEpisode{
		"episode": {Status: legacyIndexStatusThrottled, SubtitleLocale: "en-US", SubtitleAttemptCount: 4},
	}
	catalog, _ := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, episodes, prior, true)
	entry := catalog.Seasons[0].Episodes[0]
	if entry.Status != indexStatusUnknownProviderPlaybackRestriction || entry.SubtitleAttemptCount != 4 {
		t.Fatalf("legacy provider state was not safely normalized: %#v", entry)
	}
}

func TestParseASSPreservesCommasInText(t *testing.T) {
	raw := []byte("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:01:02.34,0:01:04.00,Default,,0,0,0,,Hello, world, again!\\N{\\i1}Still here\n")
	got := parseASS(raw)
	want := "[00:01:02] Hello, world, again! Still here\n"
	if got != want {
		t.Fatalf("parseASS() = %q, want %q", got, want)
	}
}

func TestAssessASSCueQualityReportsMalformedEmptyAndSkippedCues(t *testing.T) {
	raw := []byte("[Events]\nFormat: Start, Text\nDialogue: 0:01:02.34,Valid cue\nDialogue: malformed\nDialogue: 0:01:04.00,{\\i1}\nDialogue: invalid-time,Skipped cue\n")
	transcript, quality := parseASSWithCueQuality(raw)
	if transcript != "[00:01:02] Valid cue\n" {
		t.Fatalf("transcript=%q", transcript)
	}
	if quality.Version != assCueQualityVersion || quality.TotalCues != 4 || quality.UsableCues != 1 || quality.MalformedCues != 1 || quality.EmptyCues != 1 || quality.SkippedCues != 1 || quality.Outcome != "degraded" {
		t.Fatalf("cue quality=%#v", quality)
	}
	if quality.MaxMalformedCueRatio != maxMalformedASSCueRatio || quality.MaxEmptyCueRatio != maxEmptyASSCueRatio || quality.MaxSkippedCueRatio != maxSkippedASSCueRatio {
		t.Fatalf("cue-quality thresholds=%#v", quality)
	}
}

func TestAssessASSCueQualityAllowsCountsWithinExplicitThresholds(t *testing.T) {
	var raw strings.Builder
	raw.WriteString("[Events]\nFormat: Start, Text\n")
	for i := 0; i < 20; i++ {
		raw.WriteString("Dialogue: 0:01:02.00,Valid cue\n")
	}
	raw.WriteString("Dialogue: malformed\n")
	quality := assessASSCueQuality([]byte(raw.String()))
	if quality.MalformedCues != 1 || quality.TotalCues != 21 || quality.Outcome != "healthy" {
		t.Fatalf("cue quality=%#v", quality)
	}
}

func TestCheckpointSubtitleCachesDegradedRawASSWithoutChangingBytes(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("[Events]\nFormat: Start, Text\nDialogue: 0:01:02.00,{\\i1}\n")
	entry, attempted := checkpointSubtitleWithFetcher(
		SeasonEpisode{ID: "episode", SeasonNumber: 1, EpisodeNumber: 1}, indexEpisode{}, "en-US", dir, indexEpisode{},
		func(string, string) (subtitleFetchResult, error) {
			return subtitleFetchResult{AvailableLangs: []string{"en-US"}, RawASS: raw, PlaybackAttempts: 1, SubtitleFetchAttempts: 1}, nil
		},
	)
	if !attempted || entry.Status != indexStatusSubtitleCached || entry.SubtitleCueQuality == nil || entry.SubtitleCueQuality.Outcome != "degraded" {
		t.Fatalf("checkpoint=%#v attempted=%v", entry, attempted)
	}
	stored, err := os.ReadFile(entry.SubtitleFile)
	if err != nil || string(stored) != string(raw) {
		t.Fatalf("stored raw ASS err=%v bytes=%q", err, stored)
	}
}

func TestCheckpointSubtitleAddsCueTelemetryToVerifiedCacheWithoutRedownload(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("[Events]\nFormat: Start, Text\nDialogue: 0:01:02.00,Existing cache\n")
	path := rawASSPath(dir, 1, 1, "episode")
	if err := atomicWriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	prior := indexEpisode{
		Status: indexStatusSubtitleCached, SubtitleFile: path, SubtitleLocale: "en-US",
		SubtitleSHA256: hex.EncodeToString(sum[:]), SubtitleParserVersion: rawASSParserVersion,
	}
	entry, attempted := checkpointSubtitleWithFetcher(
		SeasonEpisode{ID: "episode", SeasonNumber: 1, EpisodeNumber: 1}, indexEpisode{}, "en-US", dir, prior,
		func(string, string) (subtitleFetchResult, error) {
			t.Fatal("verified cache must not redownload")
			return subtitleFetchResult{}, nil
		},
	)
	if attempted || entry.SubtitleCueQuality == nil || entry.SubtitleCueQuality.Outcome != "healthy" {
		t.Fatalf("checkpoint=%#v attempted=%v", entry, attempted)
	}
	stored, err := os.ReadFile(path)
	if err != nil || string(stored) != string(raw) {
		t.Fatalf("cached raw ASS changed err=%v bytes=%q", err, stored)
	}
}

func TestScheduleSparseTerminalRechecksIsSnapshotAwareAndBounded(t *testing.T) {
	seasons := []Season{{ID: "season", SeasonNumber: 1}}
	initialEpisodes := map[string][]SeasonEpisode{"season": {
		{ID: "old-newest", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 3, AvailabilityStarts: "2026-07-03T00:00:00Z"},
		{ID: "old-middle", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 2, AvailabilityStarts: "2026-07-02T00:00:00Z"},
		{ID: "old-oldest", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 1, AvailabilityStarts: "2026-07-01T00:00:00Z"},
	}}
	previous, _ := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, initialEpisodes, nil, true)
	prior := priorEpisodes(previous)
	for id, entry := range prior {
		entry.Status = indexStatusSubtitleMissing
		prior[id] = entry
	}

	currentEpisodes := map[string][]SeasonEpisode{"season": {
		{ID: "fresh", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 4, AvailabilityStarts: "2026-07-04T00:00:00Z"},
		initialEpisodes["season"][0], initialEpisodes["season"][1], initialEpisodes["season"][2],
	}}
	current, work := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, currentEpisodes, prior, true)
	work = sortIndexWorkNewestFirst(work)
	if got := scheduleSparseTerminalRechecks(&current, work, prior, previous.CatalogSnapshot, 2); got != 2 {
		t.Fatalf("scheduled=%d want=2", got)
	}
	byID := priorEpisodes(current)
	if byID["fresh"].Status != indexStatusPending || byID["old-newest"].Status != indexStatusPending || byID["old-middle"].Status != indexStatusPending || byID["old-oldest"].Status != indexStatusSubtitleMissing {
		t.Fatalf("sparse scheduling swept or reordered rows: %#v", byID)
	}
	if byID["old-newest"].SubtitleRecheckReason != "catalog_snapshot_changed" || byID["old-middle"].SubtitleRecheckReason != "catalog_snapshot_changed" {
		t.Fatalf("unexpected sparse recheck reasons: %#v", byID)
	}

	// Once an unchanged snapshot has been recorded for a terminal recheck, it
	// cannot trigger a repeat playback attempt on the next resumed run.
	for _, id := range []string{"old-newest", "old-middle"} {
		entry := byID[id]
		entry.Status = indexStatusSubtitleMissing
		entry.SubtitleRecheckSnapshot = current.CatalogSnapshot
		prior[id] = entry
	}
	stable, stableWork := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, currentEpisodes, prior, true)
	if got := scheduleSparseTerminalRechecks(&stable, sortIndexWorkNewestFirst(stableWork), prior, current.CatalogSnapshot, 2); got != 0 {
		t.Fatalf("unchanged snapshot rescheduled %d terminal rows", got)
	}
}

func TestScheduleSparseTerminalRechecksPrioritizesSourceVersionChange(t *testing.T) {
	seasons := []Season{{ID: "season", SeasonNumber: 1}}
	initialEpisodes := map[string][]SeasonEpisode{"season": {{ID: "episode", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 1, Title: "Original", AvailabilityStarts: "2026-07-01T00:00:00Z"}}}
	previous, _ := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, initialEpisodes, nil, true)
	prior := priorEpisodes(previous)
	entry := prior["episode"]
	entry.Status = indexStatusPermanentFailed
	prior["episode"] = entry

	updatedEpisodes := map[string][]SeasonEpisode{"season": {{ID: "episode", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 1, Title: "Updated", AvailabilityStarts: "2026-07-01T00:00:00Z"}}}
	current, work := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, updatedEpisodes, prior, true)
	if got := scheduleSparseTerminalRechecks(&current, sortIndexWorkNewestFirst(work), prior, previous.CatalogSnapshot, 1); got != 1 {
		t.Fatalf("scheduled=%d want=1", got)
	}
	updated := current.Seasons[0].Episodes[0]
	if updated.Status != indexStatusPending || updated.SubtitleRecheckReason != "source_version_changed" {
		t.Fatalf("source-version recheck=%#v", updated)
	}
}

func TestScheduleSparseTerminalRechecksKeepsPriorityOrderAndRetainsSourceTruth(t *testing.T) {
	seasons := []Season{{ID: "season", SeasonNumber: 1}}
	initialEpisodes := map[string][]SeasonEpisode{"season": {
		{ID: "snapshot-only", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 3, Title: "Stable", AvailabilityStarts: "2026-07-03T00:00:00Z"},
		{ID: "source-first", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 2, Title: "Original A", AvailabilityStarts: "2026-07-02T00:00:00Z"},
		{ID: "source-later", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 1, Title: "Original B", AvailabilityStarts: "2026-07-01T00:00:00Z"},
	}}
	previous, _ := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, initialEpisodes, nil, true)
	prior := priorEpisodes(previous)
	for id, entry := range prior {
		entry.Status = indexStatusSubtitleMissing
		entry.SubtitleCheckedSourceVersion = entry.SourceVersion
		prior[id] = entry
	}

	currentEpisodes := map[string][]SeasonEpisode{"season": {
		{ID: "fresh", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 4, Title: "Fresh", AvailabilityStarts: "2026-07-04T00:00:00Z"},
		initialEpisodes["season"][0],
		{ID: "source-first", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 2, Title: "Updated A", AvailabilityStarts: "2026-07-02T00:00:00Z"},
		{ID: "source-later", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 1, Title: "Updated B", AvailabilityStarts: "2026-07-01T00:00:00Z"},
	}}
	current, work := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, currentEpisodes, prior, true)
	ordered, err := reorderIndexWork(sortIndexWorkNewestFirst(work), []string{"snapshot-only"})
	if err != nil {
		t.Fatal(err)
	}
	if got := scheduleSparseTerminalRechecks(&current, ordered, prior, previous.CatalogSnapshot, 1); got != 1 {
		t.Fatalf("scheduled=%d want=1", got)
	}
	byID := priorEpisodes(current)
	if entry := byID["snapshot-only"]; entry.Status != indexStatusPending || entry.SubtitleRecheckReason != "catalog_snapshot_changed" {
		t.Fatalf("priority snapshot recheck did not win quota: %#v", entry)
	}
	if byID["source-first"].Status != indexStatusSubtitleMissing || byID["source-later"].Status != indexStatusSubtitleMissing {
		t.Fatalf("source changes exceeded the quota: %#v", byID)
	}
	if byID["source-first"].SubtitleCheckedSourceVersion == byID["source-first"].SourceVersion {
		t.Fatalf("deferred source change was erased: %#v", byID["source-first"])
	}

	resumedPrior := priorEpisodes(current)
	resumed, resumedWork := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, currentEpisodes, resumedPrior, true)
	resumedOrdered, err := reorderIndexWork(sortIndexWorkNewestFirst(resumedWork), []string{"snapshot-only"})
	if err != nil {
		t.Fatal(err)
	}
	if got := scheduleSparseTerminalRechecks(&resumed, resumedOrdered, resumedPrior, current.CatalogSnapshot, 1); got != 1 {
		t.Fatalf("resumed source change scheduled=%d want=1", got)
	}
	resumedByID := priorEpisodes(resumed)
	if entry := resumedByID["source-first"]; entry.Status != indexStatusPending || entry.SubtitleRecheckReason != "source_version_changed" {
		t.Fatalf("deferred source change was not recovered on resume: %#v", entry)
	}
	if resumedByID["source-later"].Status != indexStatusSubtitleMissing || resumedByID["source-later"].SubtitleCheckedSourceVersion == resumedByID["source-later"].SourceVersion {
		t.Fatalf("later source change was not retained beyond resumed quota: %#v", resumedByID["source-later"])
	}
}

func TestAtomicWriteFileLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	if err := atomicWriteFile(path, []byte("complete"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temporary file was not cleaned up: %s", entry.Name())
		}
	}
}

func indexWorkForIDs(ids ...string) []indexEpisodeWork {
	work := make([]indexEpisodeWork, 0, len(ids))
	for _, id := range ids {
		work = append(work, indexEpisodeWork{Episode: SeasonEpisode{ID: id}})
	}
	return work
}

func indexWorkIDs(work []indexEpisodeWork) []string {
	ids := make([]string, 0, len(work))
	for _, item := range work {
		ids = append(ids, item.Episode.ID)
	}
	return ids
}

func TestParseIndexPriorityIDsTrimsAndDeduplicates(t *testing.T) {
	got := parseIndexPriorityIDs(" second, first, second, , first ")
	want := []string{"second", "first"}
	if len(got) != len(want) {
		t.Fatalf("parseIndexPriorityIDs() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseIndexPriorityIDs() = %#v, want %#v", got, want)
		}
	}
}

func TestIndexPlayback4294RetriesAreOwnedByGlobalCircuit(t *testing.T) {
	original := activeConfig.Playback4294Retries
	t.Cleanup(func() { activeConfig.Playback4294Retries = original })
	activeConfig.Playback4294Retries = 5
	if got := indexPlayback4294Retries(); got != 0 {
		t.Fatalf("index playback retries = %d, want 0", got)
	}
}

func TestReorderIndexWorkPrioritizesDeclaredOrderAndRetainsAll(t *testing.T) {
	work := indexWorkForIDs("first", "second", "third", "fourth")
	got, err := reorderIndexWork(work, parseIndexPriorityIDs(" third, first "))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"third", "first", "second", "fourth"}
	if gotIDs := indexWorkIDs(got); len(gotIDs) != len(want) {
		t.Fatalf("reorderIndexWork() IDs = %#v, want %#v", gotIDs, want)
	} else {
		for i := range want {
			if gotIDs[i] != want[i] {
				t.Fatalf("reorderIndexWork() IDs = %#v, want %#v", gotIDs, want)
			}
		}
	}
	if gotIDs := indexWorkIDs(work); gotIDs[0] != "first" {
		t.Fatalf("reorderIndexWork() mutated original work: %#v", gotIDs)
	}
}

func TestReorderIndexWorkRejectsUnknownIDsDeterministically(t *testing.T) {
	work := indexWorkForIDs("first", "second")
	_, err := reorderIndexWork(work, parseIndexPriorityIDs("missing, second, absent, missing"))
	var priorityErr *IndexPriorityIDsError
	if !errors.As(err, &priorityErr) {
		t.Fatalf("expected IndexPriorityIDsError, got %v", err)
	}
	want := []string{"missing", "absent"}
	if len(priorityErr.Unknown) != len(want) {
		t.Fatalf("unknown IDs = %#v, want %#v", priorityErr.Unknown, want)
	}
	for i := range want {
		if priorityErr.Unknown[i] != want[i] {
			t.Fatalf("unknown IDs = %#v, want %#v", priorityErr.Unknown, want)
		}
	}
}

func TestReorderIndexWorkEmptyPriorityPreservesOriginal(t *testing.T) {
	work := indexWorkForIDs("first", "second")
	got, err := reorderIndexWork(work, parseIndexPriorityIDs(""))
	if err != nil {
		t.Fatal(err)
	}
	if gotIDs := indexWorkIDs(got); len(gotIDs) != 2 || gotIDs[0] != "first" || gotIDs[1] != "second" {
		t.Fatalf("empty priority reordered work: %#v", gotIDs)
	}
}

func TestSortIndexWorkNewestFirstUsesStableCanonicalTies(t *testing.T) {
	work := []indexEpisodeWork{
		{Episode: SeasonEpisode{ID: "older", SeasonNumber: 99, EpisodeNumber: 999, AvailabilityStarts: "2025-01-01T00:00:00Z"}},
		{Episode: SeasonEpisode{ID: "variant-z", SeasonNumber: 2, EpisodeNumber: 10, AvailabilityStarts: "2026-01-01T00:00:00Z"}},
		{Episode: SeasonEpisode{ID: "variant-a", SeasonNumber: 2, EpisodeNumber: 10, AvailabilityStarts: "2026-01-01T00:00:00Z"}},
		{Episode: SeasonEpisode{ID: "episode-nine", SeasonNumber: 2, EpisodeNumber: 9, AvailabilityStarts: "2026-01-01T00:00:00Z"}},
		{Episode: SeasonEpisode{ID: "season-one", SeasonNumber: 1, EpisodeNumber: 100, AvailabilityStarts: "2026-01-01T00:00:00Z"}},
	}
	got := indexWorkIDs(sortIndexWorkNewestFirst(work))
	want := []string{"variant-a", "variant-z", "episode-nine", "season-one", "older"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("newest-first IDs = %#v, want %#v", got, want)
	}
	if work[0].Episode.ID != "older" {
		t.Fatalf("sort mutated provider/catalog work: %#v", indexWorkIDs(work))
	}
}

func TestProcessIndexWorkSnapshotsEveryIdentityAndContinuesAfterFailure(t *testing.T) {
	seasons := []Season{{ID: "season", SeasonNumber: 1}}
	episodes := map[string][]SeasonEpisode{"season": {
		{ID: "newest", SeasonNumber: 1, EpisodeNumber: 3, AvailabilityStarts: "2026-01-03T00:00:00Z", Title: "Newest", SeriesTitle: "Series"},
		{ID: "middle", SeasonNumber: 1, EpisodeNumber: 2, AvailabilityStarts: "2026-01-02T00:00:00Z", Title: "Middle", SeriesTitle: "Series"},
		{ID: "oldest", SeasonNumber: 1, EpisodeNumber: 1, AvailabilityStarts: "2026-01-01T00:00:00Z", Title: "Oldest", SeriesTitle: "Series"},
	}}
	catalog, work := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, episodes, nil, true)
	work = sortIndexWorkNewestFirst(work)
	var fetched []string
	fetch := func(id, locale string) (subtitleFetchResult, error) {
		fetched = append(fetched, id)
		if id == "newest" {
			return subtitleFetchResult{}, &PlaybackAPIError{EpisodeID: id, Code: playback4294Code}
		}
		return subtitleFetchResult{AvailableLangs: []string{locale}, RawASS: []byte("[Events]\n")}, nil
	}
	var snapshots [][]string
	snapshot := func(file *indexFile) error {
		statuses := make([]string, 0, 3)
		for _, episode := range file.Seasons[0].Episodes {
			statuses = append(statuses, episode.Status)
		}
		snapshots = append(snapshots, statuses)
		return nil
	}
	var sleeps []time.Duration
	err := processIndexWork(&catalog, work, nil, "en-US", t.TempDir(), fetch, snapshot, 7*time.Second, func(delay time.Duration) {
		sleeps = append(sleeps, delay)
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(fetched, ",") != "newest,middle,oldest" {
		t.Fatalf("failure blocked older work: fetched=%#v", fetched)
	}
	if len(snapshots) != 3 || snapshots[0][0] != indexStatusUnknownProviderPlaybackRestriction || snapshots[0][1] != indexStatusPending || snapshots[2][2] != indexStatusSubtitleCached {
		t.Fatalf("per-record snapshots = %#v", snapshots)
	}
	if len(sleeps) != 3 || sleeps[0] != 7*time.Second {
		t.Fatalf("attempt pacing = %#v", sleeps)
	}
	if got := catalog.Seasons[0].Episodes[0].SubtitleAttemptCount; got != 1 {
		t.Fatalf("failed attempt count = %d, want 1", got)
	}
	if catalog.Seasons[0].Episodes[0].SubtitleLastAttemptAt == "" {
		t.Fatal("failed identity did not persist last-attempt time")
	}
}

func TestProcessIndexWorkRestartSkipsVerifiedCacheAndRetriesFailedIdentity(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("[Events]\n")
	cachedPath := rawASSPath(dir, 1, 2, "cached")
	if err := atomicWriteFile(cachedPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	prior := map[string]indexEpisode{
		"cached": {Status: indexStatusSubtitleCached, SubtitleFile: cachedPath, SubtitleLocale: "en-US", SubtitleSHA256: hex.EncodeToString(sum[:]), SubtitleParserVersion: rawASSParserVersion},
		"failed": {Status: indexStatusSubtitleFailed, SubtitleLocale: "en-US", SubtitleAttemptCount: 3, Error: "playback API error: code 4294"},
	}
	seasons := []Season{{ID: "season", SeasonNumber: 1}}
	episodes := map[string][]SeasonEpisode{"season": {
		{ID: "cached", SeasonNumber: 1, EpisodeNumber: 2, AvailabilityStarts: "2026-01-02T00:00:00Z", SeriesTitle: "Series"},
		{ID: "failed", SeasonNumber: 1, EpisodeNumber: 1, AvailabilityStarts: "2026-01-01T00:00:00Z", SeriesTitle: "Series"},
	}}
	catalog, work := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, episodes, prior, true)
	var fetched []string
	err := processIndexWork(&catalog, sortIndexWorkNewestFirst(work), prior, "en-US", dir, func(id, locale string) (subtitleFetchResult, error) {
		fetched = append(fetched, id)
		return subtitleFetchResult{AvailableLangs: []string{locale}, RawASS: raw, PlaybackAttempts: 2}, nil
	}, func(*indexFile) error { return nil }, time.Second, func(time.Duration) {})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(fetched, ",") != "failed" {
		t.Fatalf("restart fetched verified cache or skipped failure: %#v", fetched)
	}
	for _, episode := range catalog.Seasons[0].Episodes {
		if episode.Status != indexStatusSubtitleCached {
			t.Fatalf("restart did not converge to cached: %#v", catalog.Seasons[0].Episodes)
		}
		if episode.ID == "failed" && episode.SubtitleAttemptCount != 5 {
			t.Fatalf("restart attempt count = %d, want 5", episode.SubtitleAttemptCount)
		}
	}
}

func TestWriteIndexWithLoadersReturnsNoSeasonsError(t *testing.T) {
	episodesCalled := false
	err := writeIndexWithLoaders(
		"https://example.test/series/series-id", "series-id", "ja-JP", "en-US", false,
		func(string, string, string) []Season { return nil },
		func(string, string, string) []SeasonEpisode {
			episodesCalled = true
			return nil
		},
	)
	if err == nil || err.Error() != "no seasons found" {
		t.Fatalf("writeIndexWithLoaders() error = %v, want no seasons found", err)
	}
	if episodesCalled {
		t.Fatal("episode metadata loader called after no seasons")
	}
}

func TestWriteIndexWithLoadersConvertsProviderPanicToError(t *testing.T) {
	err := writeIndexWithLoaders(
		"https://example.test/series/series-id", "series-id", "ja-JP", "en-US", false,
		func(string, string, string) []Season { panic("provider unavailable") },
		func(string, string, string) []SeasonEpisode { return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "index metadata/provider failure: provider unavailable") {
		t.Fatalf("writeIndexWithLoaders() error = %v, want converted provider failure", err)
	}
}

func TestRunReturnsAuthenticationSetupErrorWithoutExiting(t *testing.T) {
	oldPath := activeCLI.EtpRtFile
	defer func() { activeCLI.EtpRtFile = oldPath }()
	activeCLI.EtpRtFile = ""
	t.Setenv("CRUNCHYROLL_ETP_RT", "")

	err := Run("https://www.crunchyroll.com/series/series-id", "")
	if err == nil || !strings.Contains(err.Error(), "Authentication setup failed") {
		t.Fatalf("Run() error = %v, want authentication setup failure", err)
	}
}
