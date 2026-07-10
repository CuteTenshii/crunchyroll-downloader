package download

import (
	"context"
	"errors"
	"strings"
	"testing"

	"crunchyroll-downloader/internal/api"
)

func TestRunSeasonEmptyEpisodes(t *testing.T) {
	err := runSeason(context.Background(), nil, nil, nil, nil, nil, nil, 0, "", nil)
	if err != nil {
		t.Fatalf("runSeason(empty) error = %v, want nil", err)
	}
}

func TestSeasonErrorFormatting(t *testing.T) {
	err := &SeasonError{Failed: 2, Total: 5}
	want := "2 of 5 episode(s) failed"
	if got := err.Error(); got != want {
		t.Fatalf("SeasonError.Error() = %q, want %q", got, want)
	}
}

func TestFormatFailedList(t *testing.T) {
	failures := []episodeError{
		{Number: 1, Title: "First", Err: errors.New("network error")},
		{Number: 3, Title: "Third", Err: errors.New("timeout")},
	}
	result := formatFailedList(failures)
	if !strings.Contains(result, "episode 1") {
		t.Fatalf("formatFailedList() = %q, want episode 1", result)
	}
	if !strings.Contains(result, "network error") {
		t.Fatalf("formatFailedList() = %q, want network error", result)
	}
	if !strings.Contains(result, "episode 3") {
		t.Fatalf("formatFailedList() = %q, want episode 3", result)
	}
	if !strings.Contains(result, "timeout") {
		t.Fatalf("formatFailedList() = %q, want timeout", result)
	}
	if !strings.Contains(result, ";") {
		t.Fatalf("formatFailedList() = %q, want semicolon separator", result)
	}
}

func TestRunSeasonContinuesAfterEpisodeFailure(t *testing.T) {
	videoQuality := "1080p"
	audioQuality := "192k"
	episodes := []api.SeasonEpisode{
		{
			ID:            "episode-1",
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
			Title:         "First",
		},
		{
			ID:            "episode-2",
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 2,
			AudioLocale:   "ja-JP",
			Title:         "Second",
		},
	}

	firstErr := errors.New("first episode failed")
	var calls []int
	err := runSeason(context.Background(), nil, &videoQuality, &audioQuality, []string{"ja-JP"}, nil, episodes, 2, "",
		func(_ context.Context, _ *api.Client, _ string, info *api.EpisodeInfo, _ []string, _ []string, _ *string, _ *string, workers int, outputDir string, totalEpisodes int) error {
			if workers != 2 {
				t.Fatalf("workers = %d, want 2", workers)
			}
			if outputDir != "" {
				t.Fatalf("outputDir = %q, want \"\"", outputDir)
			}
			calls = append(calls, info.EpisodeMetadata.EpisodeNumber)
			if info.EpisodeMetadata.EpisodeNumber == 1 {
				return firstErr
			}
			return nil
		})

	if len(calls) != 2 {
		t.Fatalf("runSeason() called downloader %d times, want 2", len(calls))
	}
	if calls[0] != 1 || calls[1] != 2 {
		t.Fatalf("runSeason() calls = %v, want [1 2]", calls)
	}
	if err == nil {
		t.Fatal("runSeason() error = nil, want aggregate season error")
	}
	var seasonErr *SeasonError
	if !errors.As(err, &seasonErr) {
		t.Fatalf("runSeason() error type = %T, want *SeasonError", err)
	}
	if seasonErr.Failed != 1 || seasonErr.Total != 2 {
		t.Fatalf("SeasonError = failed %d total %d, want failed 1 total 2", seasonErr.Failed, seasonErr.Total)
	}
	if !errors.Is(err, firstErr) {
		t.Fatalf("runSeason() error does not wrap first episode failure: %v", err)
	}
}
