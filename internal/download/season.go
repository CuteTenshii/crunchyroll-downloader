package download

import (
	"errors"
	"fmt"

	"crunchyroll-downloader/internal/api"
)

type episodeDownloader func(client *api.Client, baseContentID string, info *api.EpisodeInfo, audioLangs, subsLangs []string, videoQuality, audioQuality *string) error

type SeasonError struct {
	Failed int
	Total  int
	err    error
}

func (e *SeasonError) Error() string {
	return fmt.Sprintf("%d of %d episode(s) failed", e.Failed, e.Total)
}

func (e *SeasonError) Unwrap() error {
	return e.err
}

func Season(client *api.Client, videoQuality, audioQuality *string, audioLangs, subsLangs []string, episodes []api.SeasonEpisode) error {
	return runSeason(client, videoQuality, audioQuality, audioLangs, subsLangs, episodes, Episode)
}

func runSeason(client *api.Client, videoQuality, audioQuality *string, audioLangs, subsLangs []string, episodes []api.SeasonEpisode, downloadEpisode episodeDownloader) error {
	if len(episodes) == 0 {
		return nil
	}

	fmt.Printf("Downloading season %v of %s (%v episodes)\n\n", episodes[0].SeasonNumber, episodes[0].SeriesTitle, len(episodes))

	var failures []error
	for _, episode := range episodes {
		info := &api.EpisodeInfo{
			EpisodeMetadata: api.EpisodeMetadata{
				SeriesTitle:        episode.SeriesTitle,
				SeasonNumber:       episode.SeasonNumber,
				EpisodeNumber:      episode.EpisodeNumber,
				AudioLocale:        episode.AudioLocale,
				Versions:           episode.Versions,
				AvailabilityStarts: episode.AvailabilityStarts,
			},
			Title: episode.Title,
		}

		if err := downloadEpisode(client, episode.ID, info, audioLangs, subsLangs, videoQuality, audioQuality); err != nil {
			fmt.Printf("Error downloading episode %v: %v\n", episode.EpisodeNumber, err)
			failures = append(failures, fmt.Errorf("episode %v: %w", episode.EpisodeNumber, err))
		}
	}

	if len(failures) > 0 {
		return &SeasonError{
			Failed: len(failures),
			Total:  len(episodes),
			err:    errors.Join(failures...),
		}
	}
	return nil
}
