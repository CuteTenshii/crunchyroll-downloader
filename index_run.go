package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

const indexRunSummarySchema = "crunchyroll-downloader.index-run/v1"

type indexRunSummary struct {
	Schema              string            `json:"schema"`
	StartedAt           string            `json:"started_at"`
	FinishedAt          string            `json:"finished_at"`
	Outcome             string            `json:"outcome"`
	Totals              map[string]int    `json:"totals"`
	NewSuccesses        int               `json:"new_successes"`
	ProviderCalls       int               `json:"provider_calls"`
	AuthenticationCalls int               `json:"authentication_calls"`
	CatalogCalls        int               `json:"catalog_calls"`
	PlaybackOpenCalls   int               `json:"playback_open_calls"`
	SubtitleFetchCalls  int               `json:"subtitle_fetch_calls"`
	StreamReleaseCalls  int               `json:"stream_release_calls"`
	AttemptedIdentities int               `json:"attempted_identities"`
	Circuit             indexCircuitState `json:"circuit"`
	CursorProviderID    string            `json:"cursor_provider_id,omitempty"`
	NextRetryAt         string            `json:"next_retry_at,omitempty"`
	Error               string            `json:"error,omitempty"`
}

type indexRunOptions struct {
	Window           int
	Circuit4294Limit int
	CircuitCooldown  time.Duration
	Delay            time.Duration
	Sleep            playbackSleeper
	Now              func() time.Time
	StartedAt        time.Time
	ProviderMetrics  func() providerCallMetrics
}

func emptyIndexRunSummary(startedAt time.Time) indexRunSummary {
	return indexRunSummary{
		Schema:    indexRunSummarySchema,
		StartedAt: startedAt.UTC().Format(time.RFC3339Nano),
		Totals:    map[string]int{},
		Circuit:   indexCircuitState{State: "closed"},
	}
}

func writeIndexRunSummary(path string, summary indexRunSummary) error {
	data, err := jsonMarshalIndentNewline(summary)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data, 0600)
}

func jsonMarshalIndentNewline(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func countIndexStatuses(catalog *indexFile) map[string]int {
	totals := map[string]int{}
	for _, season := range catalog.Seasons {
		for _, episode := range season.Episodes {
			totals[episode.Status]++
		}
	}
	return totals
}

func retryAfterTime(value string, now time.Time) time.Time {
	if value == "" {
		return time.Time{}
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return now.Add(time.Duration(seconds) * time.Second)
	}
	if parsed, err := http.ParseTime(value); err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}

func isEligibleEpisode(entry indexEpisode, now time.Time) bool {
	if entry.Status == indexStatusSubtitleCached || entry.Status == indexStatusSubtitleMissing || entry.Status == indexStatusPermanentFailed {
		return false
	}
	if entry.SubtitleNextEligibleAt == "" {
		return true
	}
	next, err := time.Parse(time.RFC3339Nano, entry.SubtitleNextEligibleAt)
	return err != nil || !next.After(now)
}

func processIndexWorkBounded(catalog *indexFile, work []indexEpisodeWork, prior map[string]indexEpisode, locale, subsDir string, fetch subtitleFetcher, snapshot indexSnapshotter, options indexRunOptions) (summary indexRunSummary, err error) {
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if options.Sleep == nil {
		options.Sleep = time.Sleep
	}
	summary = emptyIndexRunSummary(options.StartedAt)
	defer func() {
		summary.FinishedAt = now().UTC().Format(time.RFC3339Nano)
		summary.Totals = countIndexStatuses(catalog)
		summary.Circuit = catalog.Checkpoint.Circuit
		summary.CursorProviderID = catalog.Checkpoint.CursorProviderID
		if options.ProviderMetrics != nil {
			metrics := options.ProviderMetrics()
			summary.AuthenticationCalls = metrics.Authentication
			summary.CatalogCalls = metrics.Catalog
			summary.PlaybackOpenCalls = metrics.PlaybackOpen
			summary.SubtitleFetchCalls = metrics.SubtitleFetch
			summary.StreamReleaseCalls = metrics.StreamRelease
			summary.ProviderCalls = metrics.total()
		}
		if summary.NextRetryAt == "" {
			summary.NextRetryAt = catalog.Checkpoint.Circuit.NextEligibleAttemptAt
		}
		if err != nil {
			summary.Outcome = "failed"
			summary.Error = redactSensitiveURLs(err.Error())
		} else if summary.Circuit.State == "open" {
			summary.Outcome = "throttled"
		} else if summary.AttemptedIdentities == 0 {
			summary.Outcome = "unchanged"
		} else if summary.Totals[indexStatusRetryableFailed]+summary.Totals[indexStatusPermanentFailed]+summary.Totals[indexStatusThrottled] > 0 {
			summary.Outcome = "partial"
		} else {
			summary.Outcome = "succeeded"
		}
	}()

	if catalog.Checkpoint.Circuit.State == "open" && catalog.Checkpoint.Circuit.NextEligibleAttemptAt != "" {
		next, parseErr := time.Parse(time.RFC3339Nano, catalog.Checkpoint.Circuit.NextEligibleAttemptAt)
		if parseErr == nil && next.After(now()) {
			summary.NextRetryAt = next.UTC().Format(time.RFC3339Nano)
			return summary, nil
		}
	}
	catalog.Checkpoint.Circuit = indexCircuitState{State: "closed"}

	for _, item := range work {
		entry := catalog.Seasons[item.SeasonIndex].Episodes[item.EpisodeIndex]
		if !isEligibleEpisode(entry, now()) {
			continue
		}
		if summary.AttemptedIdentities >= options.Window {
			break
		}
		beforeCached := entry.Status == indexStatusSubtitleCached
		entry, attempted := checkpointSubtitleWithFetcher(item.Episode, entry, locale, subsDir, prior[indexEpisodeKey(item.Episode.ID)], fetch)
		if !attempted {
			continue
		}
		summary.AttemptedIdentities++
		playbackCalls := entry.SubtitleAttemptCount - prior[indexEpisodeKey(item.Episode.ID)].SubtitleAttemptCount
		if playbackCalls < 1 {
			playbackCalls = 1
		}
		subtitleCalls := entry.SubtitleFetchAttemptCount - prior[indexEpisodeKey(item.Episode.ID)].SubtitleFetchAttemptCount
		releaseCalls := entry.StreamReleaseAttemptCount - prior[indexEpisodeKey(item.Episode.ID)].StreamReleaseAttemptCount
		summary.PlaybackOpenCalls += playbackCalls
		summary.SubtitleFetchCalls += subtitleCalls
		summary.StreamReleaseCalls += releaseCalls
		summary.ProviderCalls += playbackCalls + subtitleCalls + releaseCalls
		if entry.Status == indexStatusSubtitleCached && !beforeCached {
			summary.NewSuccesses++
		}
		catalog.Checkpoint.CursorProviderID = item.Episode.ID
		catalog.Checkpoint.UpdatedAt = now().UTC().Format(time.RFC3339Nano)

		if entry.ProviderApplicationCode == playback4294Code {
			catalog.Checkpoint.Circuit.Consecutive4294++
		} else {
			catalog.Checkpoint.Circuit.Consecutive4294 = 0
		}
		if entry.Status == indexStatusThrottled || entry.Status == indexStatusRetryableFailed {
			next := retryAfterTime(entry.RetryAfter, now())
			if next.IsZero() {
				next = now().Add(options.CircuitCooldown)
			}
			entry.SubtitleNextEligibleAt = next.UTC().Format(time.RFC3339Nano)
			if summary.NextRetryAt == "" || entry.SubtitleNextEligibleAt < summary.NextRetryAt {
				summary.NextRetryAt = entry.SubtitleNextEligibleAt
			}
		}
		if catalog.Checkpoint.Circuit.Consecutive4294 >= options.Circuit4294Limit {
			next := retryAfterTime(entry.RetryAfter, now())
			if next.IsZero() {
				next = now().Add(options.CircuitCooldown)
			}
			catalog.Checkpoint.Circuit.State = "open"
			catalog.Checkpoint.Circuit.OpenedAt = now().UTC().Format(time.RFC3339Nano)
			catalog.Checkpoint.Circuit.NextEligibleAttemptAt = next.UTC().Format(time.RFC3339Nano)
			entry.SubtitleNextEligibleAt = catalog.Checkpoint.Circuit.NextEligibleAttemptAt
		}
		replaceCatalogEpisode(catalog, item, entry)
		if snapshotErr := snapshot(catalog); snapshotErr != nil {
			return summary, snapshotErr
		}
		if catalog.Checkpoint.Circuit.State == "open" {
			break
		}
		options.Sleep(options.Delay)
	}
	return summary, nil
}
