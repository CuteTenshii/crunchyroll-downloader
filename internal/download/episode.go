package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/drm"
	"crunchyroll-downloader/internal/media"
	"crunchyroll-downloader/internal/mux"
	"github.com/unki2aut/go-mpd"
	"golang.org/x/sync/errgroup"
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
	var tempFiles []string
	completed := false
	defer func() {
		fmt.Print("Cleaning up...")
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelCleanup()
		for id, sToken := range activeStreams {
			if _, err := client.DeleteStream(cleanupCtx, id, sToken); err != nil {
				fmt.Printf("\nFailed to remove stream %s: %v\n", id, err)
			}
		}
		if !completed {
			cleanupEpisodeArtifacts(outputFile, tempFiles)
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
		tempFiles = append(tempFiles, file)
		subTracks = append(subTracks, mux.MediaTrack{File: file, Locale: locale})
	}
	if len(subTracks) > 0 {
		fmt.Println("Downloaded subtitles!")
	}

	var videoFile string
	var audioTracks []mux.MediaTrack

	// ===== Phase A: Video + first audio (sequential, i==0) =====
	version := versions[0]

	var manifest *mpd.MPD
	manifestData, err := client.FetchManifest(ctx, firstEpisode.ManifestURL)
	if err != nil {
		return fmt.Errorf("fetching manifest for %s: %w", version.locale, err)
	}

	manifest, err = media.ParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("parsing manifest for %s: %w", version.locale, err)
	}
	media.SetCachedManifest(version.contentId, manifest)

	pssh := drm.GetPssh(manifest)
	if pssh == nil {
		return fmt.Errorf("PSSH not found for %s", version.locale)
	}

	keys, err := drm.GetLicense(ctx, client, *pssh, version.contentId, firstEpisode.Token)
	if err != nil {
		return fmt.Errorf("getting license for %s: %w", version.locale, err)
	}

	audioSet := manifest.Period[0].AdaptationSets[1]
	fmt.Printf("Downloading %s audio...\n", mux.TrackTitle(version.locale))
	audioBaseUrl, audioRepresentationId := media.GetAudioBaseUrl(audioSet, *audioQuality)
	if audioBaseUrl == nil {
		return fmt.Errorf("failed to get audio base URL for %s", version.locale)
	}

	audioFile, err := media.DownloadParts(ctx, client, audioBaseUrl, audioRepresentationId, audioSet, keys, workers)
	if err != nil {
		return fmt.Errorf("downloading audio for %s: %w", version.locale, err)
	}
	tempFiles = append(tempFiles, audioFile)
	audioTracks = append(audioTracks, mux.MediaTrack{File: audioFile, Locale: version.locale})

	// Download video (always first version)
	videoSet := manifest.Period[0].AdaptationSets[0]
	fmt.Println("Downloading video...")
	baseUrl, representationId := media.GetVideoBaseUrl(videoSet, *videoQuality)
	if baseUrl == nil {
		return fmt.Errorf("failed to get video base URL")
	}
	videoFile, err = media.DownloadParts(ctx, client, baseUrl, representationId, videoSet, keys, workers)
	if err != nil {
		return fmt.Errorf("downloading video: %w", err)
	}
	tempFiles = append(tempFiles, videoFile)

	// ===== Phase B: Parallel audio (versions [1..N]) =====
	if len(versions) > 1 {
		g, gctx := errgroup.WithContext(ctx)
		var mu sync.Mutex

		for idx := 1; idx < len(versions); idx++ {
			idx := idx
			version := versions[idx]
			g.Go(func() error {
				manifest := media.GetCachedManifest(version.contentId)
				var episodeToken string
				if manifest == nil {
					episode, err := client.GetEpisode(gctx, version.contentId)
					if err != nil {
						return fmt.Errorf("fetching episode for %s: %w", version.locale, err)
					}
					episodeToken = episode.Token

					mu.Lock()
					activeStreams[version.contentId] = episode.Token
					mu.Unlock()

					manifestData, err := client.FetchManifest(gctx, episode.ManifestURL)
					if err != nil {
						return fmt.Errorf("fetching manifest for %s: %w", version.locale, err)
					}

					manifest, err = media.ParseManifest(manifestData)
					if err != nil {
						return fmt.Errorf("parsing manifest for %s: %w", version.locale, err)
					}
					media.SetCachedManifest(version.contentId, manifest)
				}

				pssh := drm.GetPssh(manifest)
				if pssh == nil {
					return fmt.Errorf("PSSH not found for %s", version.locale)
				}

				if episodeToken == "" {
					mu.Lock()
					episodeToken = activeStreams[version.contentId]
					mu.Unlock()
				}

				keys, err := drm.GetLicense(gctx, client, *pssh, version.contentId, episodeToken)
				if err != nil {
					return fmt.Errorf("getting license for %s: %w", version.locale, err)
				}

				audioSet := manifest.Period[0].AdaptationSets[1]
				fmt.Printf("Downloading %s audio...\n", mux.TrackTitle(version.locale))
				audioBaseUrl, audioRepresentationId := media.GetAudioBaseUrl(audioSet, *audioQuality)
				if audioBaseUrl == nil {
					return fmt.Errorf("failed to get audio base URL for %s", version.locale)
				}

				audioFile, err := media.DownloadParts(gctx, client, audioBaseUrl, audioRepresentationId, audioSet, keys, workers)
				if err != nil {
					return fmt.Errorf("downloading audio for %s: %w", version.locale, err)
				}

				mu.Lock()
				tempFiles = append(tempFiles, audioFile)
				audioTracks = append(audioTracks, mux.MediaTrack{File: audioFile, Locale: version.locale})
				mu.Unlock()
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return fmt.Errorf("audio versions: %w", err)
		}
	}

	// Phase C: Stream cleanup is handled by the deferred function above.
	// All activeStreams entries are released in the deferred DeleteStream loop.

	if err := mux.MergeEverything(ctx, videoFile, audioTracks, subTracks, outputFile, info); err != nil {
		return fmt.Errorf("muxing episode: %w", err)
	}
	completed = true
	return nil
}

func cleanupEpisodeArtifacts(outputFile string, tempFiles []string) {
	for _, path := range append(tempFiles, outputFile) {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			fmt.Printf("\nWarning: failed to remove partial file %s: %v", path, err)
		}
	}
}
