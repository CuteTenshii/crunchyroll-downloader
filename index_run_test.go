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
	if summary.Outcome != "provider_rate_limited" || summary.Circuit.State != "open" || summary.NextRetryAt != wantRetry {
		t.Fatalf("unexpected circuit summary: %#v", summary)
	}
	if got := catalog.Seasons[0].Episodes[2].Status; got != indexStatusPending {
		t.Fatalf("breaker swept a third identity: status=%q", got)
	}
}

func TestProcessIndexWorkBoundedPreservesClosed4294StreakAcrossRuns(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	catalog, work := boundedCatalog("first", "second")
	options := boundedOptions(now)
	options.Window = 1
	fetch := func(id, _ string) (subtitleFetchResult, error) {
		return subtitleFetchResult{PlaybackAttempts: 1}, &PlaybackAPIError{EpisodeID: id, Code: playback4294Code, HTTPStatus: 403}
	}

	first, err := processIndexWorkBounded(&catalog, work, nil, "en-US", t.TempDir(), fetch, func(*indexFile) error { return nil }, options)
	if err != nil {
		t.Fatal(err)
	}
	if first.AttemptedIdentities != 1 || catalog.Checkpoint.Circuit.State != "closed" || catalog.Checkpoint.Circuit.Consecutive4294 != 1 {
		t.Fatalf("first bounded run lost circuit streak: summary=%#v circuit=%#v", first, catalog.Checkpoint.Circuit)
	}

	prior := priorEpisodes(catalog)
	second, err := processIndexWorkBounded(&catalog, work, prior, "en-US", t.TempDir(), fetch, func(*indexFile) error { return nil }, options)
	if err != nil {
		t.Fatal(err)
	}
	if second.AttemptedIdentities != 1 || second.Circuit.State != "open" || second.Circuit.Consecutive4294 != 2 {
		t.Fatalf("second bounded run did not open global circuit: summary=%#v", second)
	}
}

func TestProcessIndexWorkBoundedClearsExpiredOpenCircuit(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	catalog, work := boundedCatalog("terminal")
	entry := catalog.Seasons[0].Episodes[0]
	entry.Status = indexStatusSubtitleMissing
	replaceCatalogEpisode(&catalog, work[0], entry)
	catalog.Checkpoint.Circuit = indexCircuitState{
		State: "open", Consecutive4294: 2,
		NextEligibleAttemptAt: now.Add(-time.Minute).Format(time.RFC3339Nano),
		RestrictionEvidence:   providerRestrictionUnknown,
	}
	summary, err := processIndexWorkBounded(&catalog, work, priorEpisodes(catalog), "en-US", t.TempDir(), func(string, string) (subtitleFetchResult, error) {
		t.Fatal("expired circuit with terminal work must not fetch")
		return subtitleFetchResult{}, nil
	}, func(*indexFile) error { return nil }, boundedOptions(now))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Circuit.State != "closed" || summary.Circuit.Consecutive4294 != 0 || summary.Circuit.RestrictionEvidence != "" {
		t.Fatalf("expired circuit was not cleared: %#v", summary.Circuit)
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
	if called || summary.AttemptedIdentities != 0 || summary.Outcome != providerRestrictionUnknown || summary.NextRetryAt != next {
		t.Fatalf("persisted cooldown was not honored: called=%v summary=%#v", called, summary)
	}
}

func TestProcessIndexWorkBoundedReportsUnknownProviderPlaybackRestrictionWithoutRateEvidence(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	catalog, work := boundedCatalog("first", "second")
	summary, err := processIndexWorkBounded(&catalog, work, nil, "en-US", t.TempDir(), func(id, _ string) (subtitleFetchResult, error) {
		return subtitleFetchResult{PlaybackAttempts: 1}, &PlaybackAPIError{EpisodeID: id, Code: playback4294Code, HTTPStatus: 403}
	}, func(*indexFile) error { return nil }, boundedOptions(now))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Outcome != providerRestrictionUnknown || summary.Circuit.RestrictionEvidence != providerRestrictionUnknown {
		t.Fatalf("unsupported causal inference in summary: %#v", summary)
	}
	if got := catalog.Seasons[0].Episodes[0].Status; got != indexStatusUnknownProviderPlaybackRestriction {
		t.Fatalf("first status=%q", got)
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
		{name: "provider 429", err: &HTTPStatusError{URL: "https://cdn.example", StatusCode: 429}, want: indexStatusProviderRateLimited},
		{name: "provider 503", err: &HTTPStatusError{URL: "https://cdn.example", StatusCode: 503}, want: indexStatusRetryableFailed},
		{name: "playback 4294 without direct rate evidence", err: &PlaybackAPIError{EpisodeID: "episode", Code: playback4294Code}, want: indexStatusUnknownProviderPlaybackRestriction},
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

func TestCheckpointSubtitleClassifiesDirectRateLimitEvidenceSeparately(t *testing.T) {
	entry, attempted := checkpointSubtitleWithFetcher(
		SeasonEpisode{ID: "episode", SeasonNumber: 1, EpisodeNumber: 1}, indexEpisode{}, "en-US", t.TempDir(), indexEpisode{},
		func(string, string) (subtitleFetchResult, error) {
			return subtitleFetchResult{PlaybackAttempts: 1}, &PlaybackAPIError{EpisodeID: "episode", Code: playback4294Code, HTTPStatus: 403, RetryAfter: "60"}
		},
	)
	if !attempted || entry.Status != indexStatusProviderRateLimited || entry.ProviderRestriction != providerRestrictionRateLimited {
		t.Fatalf("direct rate evidence=%#v attempted=%v", entry, attempted)
	}
}

func TestProcessIndexWorkBoundedReportsDegradedCueQuality(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	catalog, work := boundedCatalog("episode")
	raw := []byte("[Events]\nFormat: Start, Text\nDialogue: 0:01:02.00,{\\i1}\n")
	summary, err := processIndexWorkBounded(&catalog, work, nil, "en-US", t.TempDir(), func(_ string, locale string) (subtitleFetchResult, error) {
		return subtitleFetchResult{AvailableLangs: []string{locale}, RawASS: raw, PlaybackAttempts: 1, SubtitleFetchAttempts: 1}, nil
	}, func(*indexFile) error { return nil }, boundedOptions(now))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Outcome != "degraded" || summary.CueQuality.Version != assCueQualityVersion || summary.CueQuality.CachedIdentities != 1 || summary.CueQuality.DegradedIdentities != 1 || summary.CueQuality.EmptyCues != 1 {
		t.Fatalf("cue-quality summary=%#v", summary)
	}
}

func TestProcessIndexWorkBoundedRecordsSnapshotForAttemptedSparseRecheck(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	catalog, work := boundedCatalog("episode")
	catalog.CatalogSnapshot = "catalog-snapshot/v1:current"
	entry := catalog.Seasons[0].Episodes[0]
	entry.SubtitleRecheckReason = "catalog_snapshot_changed"
	replaceCatalogEpisode(&catalog, work[0], entry)
	raw := []byte("[Events]\nFormat: Start, Text\nDialogue: 0:01:02.00,Recovered\n")
	summary, err := processIndexWorkBounded(&catalog, work, nil, "en-US", t.TempDir(), func(_ string, locale string) (subtitleFetchResult, error) {
		return subtitleFetchResult{AvailableLangs: []string{locale}, RawASS: raw, PlaybackAttempts: 1}, nil
	}, func(*indexFile) error { return nil }, boundedOptions(now))
	if err != nil {
		t.Fatal(err)
	}
	if summary.ScheduledTerminalRechecks != 1 || catalog.Seasons[0].Episodes[0].SubtitleRecheckSnapshot != catalog.CatalogSnapshot {
		t.Fatalf("sparse recheck checkpoint=%#v summary=%#v", catalog.Seasons[0].Episodes[0], summary)
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
