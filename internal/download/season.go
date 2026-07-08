package download

import (
	"fmt"

	"crunchyroll-downloader/internal/api"
)

func Season(client *api.Client, videoQuality, audioQuality *string, audioLangs, subsLangs []string, episodes []api.SeasonEpisode) {
	if len(episodes) == 0 {
		return
	}

	fmt.Printf("Downloading season %v of %s (%v episodes)\n\n", episodes[0].SeasonNumber, episodes[0].SeriesTitle, len(episodes))

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

		if err := Episode(client, episode.ID, info, audioLangs, subsLangs, videoQuality, audioQuality); err != nil {
			fmt.Printf("Error downloading episode %v: %v\n", episode.EpisodeNumber, err)
		}
	}
}
