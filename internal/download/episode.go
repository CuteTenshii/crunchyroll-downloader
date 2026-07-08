package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/drm"
	"crunchyroll-downloader/internal/media"
	"crunchyroll-downloader/internal/mux"
)

func sanitizeFilename(s string) string {
	if s == "" {
		return "Unknown"
	}
	illegal := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|", "'", "’", "`", "“", "”"}
	res := s
	for _, char := range illegal {
		res = strings.ReplaceAll(res, char, "_")
	}
	for strings.Contains(res, "__") {
		res = strings.ReplaceAll(res, "__", "_")
	}
	return strings.TrimRight(res, " .")
}

func Episode(ctx context.Context, client *api.Client, baseContentID string, info *api.EpisodeInfo, audioLangs, subsLangs []string, videoQuality, audioQuality *string, workers int) error {
	cleanSeriesTitle := sanitizeFilename(info.EpisodeMetadata.SeriesTitle)
	cleanEpisodeTitle := sanitizeFilename(info.Title)

	if err := os.MkdirAll(cleanSeriesTitle, 0777); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	outputFile := filepath.Join(cleanSeriesTitle, fmt.Sprintf("%s S%02dE%02d - %s [%s].mkv",
		cleanSeriesTitle,
		info.EpisodeMetadata.SeasonNumber,
		info.EpisodeMetadata.EpisodeNumber,
		cleanEpisodeTitle,
		*videoQuality,
	))

	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("Episode %v is already downloaded, skipping...\n", info.EpisodeMetadata.EpisodeNumber)
		return nil
	}

	guidByLocale := map[string]string{}
	if info.EpisodeMetadata.AudioLocale != "" {
		guidByLocale[info.EpisodeMetadata.AudioLocale] = baseContentID
	}
	for _, v := range info.EpisodeMetadata.Versions {
		guidByLocale[v.AudioLocale] = v.GUID
	}

	if len(audioLangs) == 1 && audioLangs[0] == "all" {
		audioLangs = make([]string, 0, len(guidByLocale))
		if primaryLocale := info.EpisodeMetadata.AudioLocale; primaryLocale != "" {
			if _, ok := guidByLocale[primaryLocale]; ok {
				audioLangs = append(audioLangs, primaryLocale)
			}
		}
		for locale := range guidByLocale {
			if locale != info.EpisodeMetadata.AudioLocale {
				audioLangs = append(audioLangs, locale)
			}
		}
		if len(audioLangs) > 1 {
			sort.Strings(audioLangs[1:])
		}
	}

	type audioVersion struct {
		locale    string
		contentId string
	}
	var versions []audioVersion
	for _, locale := range audioLangs {
		guid, ok := guidByLocale[locale]
		if !ok {
			return fmt.Errorf("audio locale %s is not available for episode %v", locale, info.EpisodeMetadata.EpisodeNumber)
		}
		versions = append(versions, audioVersion{locale: locale, contentId: guid})
	}

	fmt.Printf("Downloading: %s (S%02vE%02v) from %s\n", info.Title, info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber, info.EpisodeMetadata.SeriesTitle)

	activeStreams := map[string]string{}
	defer func() {
		fmt.Print("Cleaning up...")
		for id, sToken := range activeStreams {
			if _, err := client.DeleteStream(ctx, id, sToken); err != nil {
				fmt.Printf("\nFailed to remove stream %s: %v\n", id, err)
			}
		}
	}()

	firstEpisode, err := client.GetEpisode(ctx, versions[0].contentId)
	if err != nil {
		return fmt.Errorf("fetching first episode: %w", err)
	}
	activeStreams[versions[0].contentId] = firstEpisode.Token

	if len(subsLangs) == 1 && subsLangs[0] == "all" {
		subsLangs = make([]string, 0, len(firstEpisode.Subtitles))
		for locale, sub := range firstEpisode.Subtitles {
			if sub != nil && sub.URL != "" {
				subsLangs = append(subsLangs, locale)
			}
		}
		sort.Strings(subsLangs)
	}

	fmt.Printf("Audio locales: %s | Subtitle locales: %s\n", strings.Join(audioLangs, ", "), strings.Join(subsLangs, ", "))

	for _, locale := range subsLangs {
		if firstEpisode.Subtitles[locale] == nil {
			return fmt.Errorf("subtitle locale %s is not available for episode %v", locale, info.EpisodeMetadata.EpisodeNumber)
		}
	}

	var subTracks []mux.MediaTrack
	for _, locale := range subsLangs {
		fmt.Printf("Downloading subtitles for %s...\n", mux.TrackTitle(locale))
		file, err := media.DownloadSubs(ctx, client, firstEpisode.Subtitles[locale].URL)
		if err != nil {
			return fmt.Errorf("downloading subtitles for %s: %w", locale, err)
		}
		subTracks = append(subTracks, mux.MediaTrack{File: file, Locale: locale})
	}
	if len(subTracks) > 0 {
		fmt.Println("Downloaded subtitles!")
	}

	var videoFile string
	var audioTracks []mux.MediaTrack

	for i, version := range versions {
		episode := firstEpisode
		if i > 0 {
			episode, err = client.GetEpisode(ctx, version.contentId)
			if err != nil {
				return fmt.Errorf("fetching episode for %s: %w", version.locale, err)
			}
			activeStreams[version.contentId] = episode.Token
		}

		manifestData, err := client.FetchManifest(ctx, episode.ManifestURL)
		if err != nil {
			return fmt.Errorf("fetching manifest for %s: %w", version.locale, err)
		}

		manifest, err := media.ParseManifest(manifestData)
		if err != nil {
			return fmt.Errorf("parsing manifest for %s: %w", version.locale, err)
		}

		pssh := drm.GetPssh(manifest)
		if pssh == nil {
			return fmt.Errorf("PSSH not found for %s", version.locale)
		}

		keys, err := drm.GetLicense(ctx, client, *pssh, version.contentId, episode.Token)
		if err != nil {
			return fmt.Errorf("getting license for %s: %w", version.locale, err)
		}

		audioSet := manifest.Period[0].AdaptationSets[1]
		fmt.Printf("Downloading %s audio...\n", mux.TrackTitle(version.locale))
		audioBaseUrl, audioRepresentationId := media.GetBaseUrl(audioSet, false, *audioQuality)
		if audioBaseUrl == nil {
			return fmt.Errorf("failed to get audio base URL for %s", version.locale)
		}

		audioFile, err := media.DownloadParts(ctx, client, audioBaseUrl, audioRepresentationId, audioSet, keys, workers)
		if err != nil {
			return fmt.Errorf("downloading audio for %s: %w", version.locale, err)
		}
		audioTracks = append(audioTracks, mux.MediaTrack{File: audioFile, Locale: version.locale})

		if i == 0 {
			videoSet := manifest.Period[0].AdaptationSets[0]
			fmt.Println("Downloading video...")
			baseUrl, representationId := media.GetBaseUrl(videoSet, true, *videoQuality)
			if baseUrl == nil {
				return fmt.Errorf("failed to get video base URL")
			}
			videoFile, err = media.DownloadParts(ctx, client, baseUrl, representationId, videoSet, keys, workers)
			if err != nil {
				return fmt.Errorf("downloading video: %w", err)
			}
		}

		if success, err := client.DeleteStream(ctx, version.contentId, episode.Token); err != nil {
			fmt.Printf("Failed to remove stream %s: %v\n", version.contentId, err)
		} else if !success {
			fmt.Print("Failed to remove the player stream, you will probably have issues downloading other episodes.\n")
		}
		delete(activeStreams, version.contentId)
	}

	if err := mux.MergeEverything(videoFile, audioTracks, subTracks, outputFile, info); err != nil {
		return fmt.Errorf("muxing episode: %w", err)
	}
	return nil
}
