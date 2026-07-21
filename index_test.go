package main

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
	originalRetries := *playback4294Retries
	originalBackoff := *playback4294Backoff
	defer func() {
		openIndexPlayback = originalOpen
		sleepPlaybackRetry = originalSleep
		*playback4294Retries = originalRetries
		*playback4294Backoff = originalBackoff
	}()

	*playback4294Retries = 2
	*playback4294Backoff = 4 * time.Second
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

func TestParseASSPreservesCommasInText(t *testing.T) {
	raw := []byte("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:01:02.34,0:01:04.00,Default,,0,0,0,,Hello, world, again!\\N{\\i1}Still here\n")
	got := parseASS(raw)
	want := "[00:01:02] Hello, world, again! Still here\n"
	if got != want {
		t.Fatalf("parseASS() = %q, want %q", got, want)
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
	original := *playback4294Retries
	t.Cleanup(func() { *playback4294Retries = original })
	*playback4294Retries = 5
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
	if len(snapshots) != 3 || snapshots[0][0] != indexStatusThrottled || snapshots[0][1] != indexStatusPending || snapshots[2][2] != indexStatusSubtitleCached {
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
	oldPath := *etpRtFile
	defer func() { *etpRtFile = oldPath }()
	*etpRtFile = ""
	t.Setenv("CRUNCHYROLL_ETP_RT", "")

	err := run("https://www.crunchyroll.com/series/series-id", "")
	if err == nil || !strings.Contains(err.Error(), "Authentication setup failed") {
		t.Fatalf("run() error = %v, want authentication setup failure", err)
	}
}
