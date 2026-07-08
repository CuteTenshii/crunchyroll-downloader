package download

import (
	"errors"
	"testing"

	"crunchyroll-downloader/internal/api"
)

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
	err := runSeason(nil, &videoQuality, &audioQuality, []string{"ja-JP"}, nil, episodes,
		func(_ *api.Client, _ string, info *api.EpisodeInfo, _ []string, _ []string, _ *string, _ *string) error {
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
