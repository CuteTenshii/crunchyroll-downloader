package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func boundedCatalog(ids ...string) (indexFile, []indexEpisodeWork) {
	episodes := make([]SeasonEpisode, 0, len(ids))
	for position, id := range ids {
		episodes = append(episodes, SeasonEpisode{
			ID: id, SeriesTitle: "Series", SeasonNumber: 1,
			EpisodeNumber:      len(ids) - position,
			AvailabilityStarts: time.Date(2026, 7, 21-position, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		})
	}
	return newIndexCatalog(
		"Series", "https://example.test/series", "en-US",
		[]Season{{ID: "season", SeasonNumber: 1}},
		map[string][]SeasonEpisode{"season": episodes}, nil, true,
	)
}

func boundedOptions(now time.Time) indexRunOptions {
	return indexRunOptions{
		Window: 5, Circuit4294Limit: 2, CircuitCooldown: 30 * time.Minute,
		Sleep: func(time.Duration) {}, Now: func() time.Time { return now }, StartedAt: now,
	}
}

func TestProcessIndexWorkBoundedOpensGlobalCircuitAndStopsPlayback(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	catalog, work := boundedCatalog("first", "second", "must-not-open")
	var calls []string
	var snapshots int
	fetch := func(id, _ string) (subtitleFetchResult, error) {
		calls = append(calls, id)
		return subtitleFetchResult{PlaybackAttempts: 1}, &PlaybackAPIError{
			EpisodeID: id, Code: playback4294Code, HTTPStatus: 403, RetryAfter: "120",
		}
	}
	summary, err := processIndexWorkBounded(&catalog, work, nil, "en-US", t.TempDir(), fetch, func(*indexFile) error {
		snapshots++
		return nil
	}, boundedOptions(now))
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("playback calls = %#v, want exactly first and second", calls)
	}
	if snapshots != 2 || summary.AttemptedIdentities != 2 || summary.ProviderCalls != 2 || summary.PlaybackOpenCalls != 2 {
		t.Fatalf("unexpected bounded telemetry: snapshots=%d summary=%#v", snapshots, summary)
	}
	wantRetry := now.Add(2 * time.Minute).Format(time.RFC3339Nano)
	if summary.Outcome != "throttled" || summary.Circuit.State != "open" || summary.NextRetryAt != wantRetry {
		t.Fatalf("unexpected circuit summary: %#v", summary)
	}
	if got := catalog.Seasons[0].Episodes[2].Status; got != indexStatusPending {
		t.Fatalf("breaker swept a third identity: status=%q", got)
	}
}

func TestProcessIndexWorkBoundedHonorsPersistedCooldownWithoutProviderCalls(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	catalog, work := boundedCatalog("blocked")
	next := now.Add(20 * time.Minute).Format(time.RFC3339Nano)
	catalog.Checkpoint.Circuit = indexCircuitState{State: "open", Consecutive4294: 2, NextEligibleAttemptAt: next}
	called := false
	summary, err := processIndexWorkBounded(&catalog, work, nil, "en-US", t.TempDir(), func(string, string) (subtitleFetchResult, error) {
		called = true
		return subtitleFetchResult{}, errors.New("must not be called")
	}, func(*indexFile) error { return nil }, boundedOptions(now))
	if err != nil {
		t.Fatal(err)
	}
	if called || summary.AttemptedIdentities != 0 || summary.Outcome != "throttled" || summary.NextRetryAt != next {
		t.Fatalf("persisted cooldown was not honored: called=%v summary=%#v", called, summary)
	}
}

func TestProcessIndexWorkBoundedPreservesCacheAndCountsProviderBoundaries(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	raw := []byte("[Script Info]\n[Events]\nFormat: Start, Text\n")
	cachedPath := rawASSPath(dir, 1, 2, "cached")
	if err := atomicWriteFile(cachedPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	digest, err := sha256File(cachedPath)
	if err != nil {
		t.Fatal(err)
	}
	prior := map[string]indexEpisode{"cached": {
		Status: indexStatusSubtitleCached, SubtitleFile: cachedPath, SubtitleLocale: "en-US",
		SubtitleSHA256: digest, SubtitleParserVersion: rawASSParserVersion,
	}}
	seasons := []Season{{ID: "season", SeasonNumber: 1}}
	episodes := map[string][]SeasonEpisode{"season": {
		{ID: "cached", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 2},
		{ID: "fresh", SeriesTitle: "Series", SeasonNumber: 1, EpisodeNumber: 1},
	}}
	catalog, work := newIndexCatalog("Series", "https://example.test/series", "en-US", seasons, episodes, prior, true)
	var fetched []string
	options := boundedOptions(now)
	options.Window = 1
	summary, err := processIndexWorkBounded(&catalog, work, prior, "en-US", dir, func(id, locale string) (subtitleFetchResult, error) {
		fetched = append(fetched, id)
		return subtitleFetchResult{
			AvailableLangs: []string{locale}, RawASS: raw,
			PlaybackAttempts: 1, SubtitleFetchAttempts: 1, StreamReleaseAttempts: 1,
			StreamReleaseOutcome: "released",
		}, nil
	}, func(*indexFile) error { return nil }, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetched) != 1 || fetched[0] != "fresh" {
		t.Fatalf("verified cache was fetched or fresh work was skipped: %#v", fetched)
	}
	if summary.NewSuccesses != 1 || summary.ProviderCalls != 3 || summary.PlaybackOpenCalls != 1 || summary.SubtitleFetchCalls != 1 || summary.StreamReleaseCalls != 1 {
		t.Fatalf("provider-call telemetry = %#v", summary)
	}
	actual, err := os.ReadFile(cachedPath)
	if err != nil || string(actual) != string(raw) {
		t.Fatalf("cached bytes changed: err=%v bytes=%q", err, actual)
	}
}

func TestCheckpointSubtitleClassifiesTypedTerminalOutcomes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "missing locale", err: &SubtitleLocaleError{EpisodeID: "episode", Locale: "en-US"}, want: indexStatusSubtitleMissing},
		{name: "locale mismatch", err: &SubtitleLocaleMismatchError{Requested: "en-US", Actual: "ja-JP"}, want: indexStatusPermanentFailed},
		{name: "provider 404", err: &HTTPStatusError{URL: "https://cdn.example", StatusCode: 404}, want: indexStatusPermanentFailed},
		{name: "provider 503", err: &HTTPStatusError{URL: "https://cdn.example", StatusCode: 503}, want: indexStatusRetryableFailed},
		{name: "playback 4294", err: &PlaybackAPIError{EpisodeID: "episode", Code: playback4294Code}, want: indexStatusThrottled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, attempted := checkpointSubtitleWithFetcher(
				SeasonEpisode{ID: "episode", SeasonNumber: 1, EpisodeNumber: 1}, indexEpisode{},
				"en-US", t.TempDir(), indexEpisode{},
				func(string, string) (subtitleFetchResult, error) {
					return subtitleFetchResult{PlaybackAttempts: 1}, test.err
				},
			)
			if !attempted || entry.Status != test.want {
				t.Fatalf("status=%q attempted=%v want=%q", entry.Status, attempted, test.want)
			}
		})
	}
}

func TestWriteIndexRunSummaryIsPrivateAndMachineReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary.json")
	summary := emptyIndexRunSummary(time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC))
	summary.FinishedAt = "2026-07-21T12:00:01Z"
	summary.Outcome = "partial"
	if err := writeIndexRunSummary(path, summary); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("summary mode=%#o want 0600", info.Mode().Perm())
	}
	var parsed indexRunSummary
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Schema != indexRunSummarySchema || parsed.Outcome != "partial" {
		t.Fatalf("unexpected terminal summary: %#v", parsed)
	}
}
