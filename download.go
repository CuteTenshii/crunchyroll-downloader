package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	widevine "github.com/iyear/gowidevine"
	"github.com/unki2aut/go-mpd"
)

const maxWorkers = 10

func buildUrl(base, representationId, file string, partNum *int64) string {
	if partNum != nil {
		file = strings.ReplaceAll(file, "$Number$", fmt.Sprintf("%05d", *partNum))
		file = strings.ReplaceAll(file, "$Number%05d$", fmt.Sprintf("%05d", *partNum))
	}
	return base + strings.ReplaceAll(file, "$RepresentationID$", representationId)
}

func downloadPart(url string) ([]byte, error) {
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Origin", "https://static.crunchyroll.com")
		req.Header.Set("Referer", "https://static.crunchyroll.com/")
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if attempt < maxRetries-1 {
				continue
			}
			return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			if attempt < maxRetries-1 {
				continue
			}
			return nil, fmt.Errorf("failed after %d retries, status: %d", maxRetries, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if attempt < maxRetries-1 {
				continue
			}
			return nil, fmt.Errorf("failed reading body after %d retries: %w", maxRetries, err)
		}
		return body, nil
	}
	return nil, fmt.Errorf("failed after %d retries", maxRetries)
}

func getFilename(set *mpd.AdaptationSet) string {
	if set == nil {
		f, _ := os.CreateTemp("", "crdl-subs-*.ass")
		return f.Name()
	}
	for _, representation := range set.Representations {
		if representation.Height != nil {
			f, _ := os.CreateTemp("", "crdl-video-*.mp4")
			return f.Name()
		} else if representation.Bandwidth != nil {
			f, _ := os.CreateTemp("", "crdl-audio-*.mp3")
			return f.Name()
		}
	}
	return ""
}

type segmentJob struct {
	index int
	url   string
}

func downloadParts(baseUrl, representationId *string, set *mpd.AdaptationSet) (string, error) {
	initUrl := buildUrl(*baseUrl, *representationId, *set.SegmentTemplate.Initialization, nil)
	initData, err := downloadPart(initUrl)
	if err != nil {
		return "", err
	}

	timeline := expandTimeline(set.SegmentTemplate.SegmentTimeline.S, 1)
	total := len(timeline)
	results := make([][]byte, total)
	var downloadErr error
	var errOnce sync.Once
	var done atomic.Int64

	jobs := make(chan segmentJob, total)
	var wg sync.WaitGroup

	for w := 0; w < maxWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				data, err := downloadPart(job.url)
				if err != nil {
					errOnce.Do(func() { downloadErr = err })
					return
				}
				results[job.index] = data
				count := done.Add(1)
				fmt.Printf("\rDownloaded %v of %v segments (%v%%)", count, total, (100*count)/int64(total))
			}
		}()
	}

	for i, item := range timeline {
		url := buildUrl(*baseUrl, *representationId, *set.SegmentTemplate.Media, &item)
		jobs <- segmentJob{index: i, url: url}
	}
	close(jobs)
	wg.Wait()

	if downloadErr != nil {
		return "", downloadErr
	}

	fmt.Println("\nFinished downloading!")

	var parts []byte
	parts = append(parts, initData...)
	for _, data := range results {
		parts = append(parts, data...)
	}

	filename := getFilename(set)
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if err = widevine.DecryptMP4Auto(io.NopCloser(bytes.NewReader(parts)), keys, file); err != nil {
		return "", fmt.Errorf("widevine.DecryptMP4Auto: %w", err)
	}

	return filename, nil
}

func downloadSubs(url string) string {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	filename := getFilename(nil)
	file, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	file.Write(body)
	file.Close()

	return filename
}

func downloadEpisode(contentId string, videoQuality, audioQuality, subtitlesLang *string, info EpisodeInfo) {
	renamed := strings.ReplaceAll(info.EpisodeMetadata.SeriesTitle, "'", "_")
	renamed = strings.ReplaceAll(renamed, "/", "_")
	renamed = strings.ReplaceAll(renamed, ":", "_")
	if _, err := os.Stat(renamed); err != nil {
		_ = os.MkdirAll(renamed, 0777)
	}
	outputFile := fmt.Sprintf("%s/%s S%02vE%02v [%s].mkv",
		renamed, info.EpisodeMetadata.SeriesTitle, info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber,
		*videoQuality,
	)

	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("Episode %v is already downloaded, skipping...\n", info.EpisodeMetadata.EpisodeNumber)
		return
	}

	episode := getEpisode(contentId)
	fmt.Printf("Downloading: %s (S%02vE%02v) from %s\n", info.Title, info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber, info.EpisodeMetadata.SeriesTitle)

	manifest := parseManifest(episode.ManifestURL)
	pssh := getPssh(manifest)
	if pssh == nil {
		panic("PSSH not found")
	}
	videoSet := manifest.Period[0].AdaptationSets[0]
	audioSet := manifest.Period[0].AdaptationSets[1]

	err := getLicense(*pssh, contentId, episode.Token)
	if err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}

	subtitles := episode.Subtitles[*subtitlesLang]
	var subsFile string
	if subtitles != nil {
		fmt.Printf("Downloading subtitles for %s language...\n", languageNames[*subtitlesLang])
		subsFile = downloadSubs(subtitles.URL)
		fmt.Println("Downloaded subtitles!")
	}

	baseUrl, representationId := getBaseUrl(videoSet, true, *videoQuality)
	if baseUrl == nil {
		print("Failed to get the video base URL, maybe the video quality you entered is wrong?\n")
		os.Exit(1)
	}
	videoFile, err := downloadParts(baseUrl, representationId, videoSet)
	if err != nil {
		panic(err)
	}

	audioBaseUrl, audioRepresentationId := getBaseUrl(audioSet, false, *audioQuality)
	if audioBaseUrl == nil {
		print("Failed to get the audio base URL, maybe the audio quality you entered is wrong?\n")
		os.Exit(1)
	}
	audioFile, err := downloadParts(audioBaseUrl, audioRepresentationId, audioSet)
	if err != nil {
		panic(err)
	}

	if success := deleteStream(contentId, episode.Token); !success {
		print("Failed to remove the player stream, you will probably have issues downloading other episodes.\n")
	}

	mergeEverything(videoFile, audioFile, subsFile, outputFile, subtitlesLang, info)
}

func downloadSeason(videoQuality, audioQuality, subtitlesLang *string, episodes []SeasonEpisode) {
	fmt.Printf("Downloading season %v of %s (%v episodes)\n\n", episodes[0].SeasonNumber, episodes[0].SeriesTitle, len(episodes))

	for _, episode := range episodes {
		info := EpisodeInfo{
			EpisodeMetadata: EpisodeMetadata{
				SeriesTitle:        episode.SeriesTitle,
				SeasonNumber:       episode.SeasonNumber,
				EpisodeNumber:      episode.EpisodeNumber,
				AudioLocale:        episode.AudioLocale,
				Versions:           episode.Versions,
				AvailabilityStarts: episode.AvailabilityStarts,
			},
			Title: episode.Title,
		}
		downloadEpisode(episode.ID, videoQuality, audioQuality, subtitlesLang, info)
	}
}
