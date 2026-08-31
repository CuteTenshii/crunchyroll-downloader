package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

func getFilename(set *mpd.AdaptationSet, subExt string) string {
	if set == nil {
		if subExt == "" {
			subExt = "ass"
		}
		f, _ := os.CreateTemp("", "crdl-subs-*."+subExt)
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

// maxBufferedSegments bounds how many segments may be resident in memory at
// once, counting both in-flight downloads and payloads still waiting to be
// written. Without it the workers race arbitrarily far ahead of the sequential
// writer whenever an early segment is slow, which is what exhausted memory on
// movie-length titles.
const maxBufferedSegments = maxWorkers * 2

// streamSegments fetches every url concurrently but writes the payloads to w
// strictly in index order, releasing each one as soon as it has been written.
// Peak memory is therefore bounded by maxBufferedSegments rather than growing
// with the length of the media.
//
// onProgress, if non-nil, is called with the running count of fetched segments.
func streamSegments(w io.Writer, urls []string, fetch func(string) ([]byte, error), onProgress func(fetched int64)) error {
	total := len(urls)
	if total == 0 {
		return nil
	}

	var (
		mu      sync.Mutex
		cond    = sync.NewCond(&mu)
		payload = make([][]byte, total)
		ready   = make([]bool, total)
		failure error
	)

	abort := make(chan struct{})
	var abortOnce sync.Once
	fail := func(err error) {
		mu.Lock()
		if failure == nil {
			failure = err
		}
		mu.Unlock()
		cond.Broadcast()
		abortOnce.Do(func() { close(abort) })
	}

	// A worker claims a slot before claiming an index, so the indices held at
	// any moment are always the lowest outstanding ones. That ordering is what
	// guarantees the writer's next index is always held by a live worker and
	// can never be starved by later segments hogging every slot.
	slots := make(chan struct{}, maxBufferedSegments)
	var next atomic.Int64
	var fetched atomic.Int64

	var wg sync.WaitGroup
	for n := 0; n < maxWorkers; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case slots <- struct{}{}:
				case <-abort:
					return
				}

				i := int(next.Add(1) - 1)
				if i >= total {
					<-slots
					return
				}

				data, err := fetch(urls[i])
				if err != nil {
					fail(err)
					return
				}

				mu.Lock()
				payload[i] = data
				ready[i] = true
				mu.Unlock()
				cond.Broadcast()

				if onProgress != nil {
					onProgress(fetched.Add(1))
				}
			}
		}()
	}

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for i := 0; i < total; i++ {
			mu.Lock()
			for !ready[i] && failure == nil {
				cond.Wait()
			}
			if failure != nil {
				mu.Unlock()
				return
			}
			data := payload[i]
			payload[i] = nil
			mu.Unlock()

			_, err := w.Write(data)
			<-slots
			if err != nil {
				fail(fmt.Errorf("writing segment %d: %w", i, err))
				return
			}
		}
	}()

	wg.Wait()
	<-writerDone

	mu.Lock()
	defer mu.Unlock()
	return failure
}

func downloadParts(baseUrl, representationId *string, set *mpd.AdaptationSet) (string, error) {
	initUrl := buildUrl(*baseUrl, *representationId, *set.SegmentTemplate.Initialization, nil)
	initData, err := downloadPart(initUrl)
	if err != nil {
		return "", err
	}

	timeline := expandTimeline(set.SegmentTemplate.SegmentTimeline.S, 1)
	total := len(timeline)
	urls := make([]string, total)
	for i, item := range timeline {
		urls[i] = buildUrl(*baseUrl, *representationId, *set.SegmentTemplate.Media, &item)
	}

	filename := getFilename(set, "")
	encPath := filename + ".enc"
	encFile, err := os.Create(encPath)
	if err != nil {
		return "", err
	}
	defer os.Remove(encPath)
	defer encFile.Close()

	if _, err = encFile.Write(initData); err != nil {
		return "", fmt.Errorf("writing init segment: %w", err)
	}

	err = streamSegments(encFile, urls, downloadPart, func(fetched int64) {
		fmt.Printf("\rDownloaded %v of %v segments (%v%%)", fetched, total, (100*fetched)/int64(total))
	})
	if err != nil {
		return "", err
	}

	fmt.Println("\nFinished downloading!")

	if _, err = encFile.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewinding %s: %w", encPath, err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if err = decryptMP4(initData, encFile, keys, file); err != nil {
		return "", fmt.Errorf("decryptMP4: %w", err)
	}

	return filename, nil
}

func downloadSubs(url, format string) string {
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

	filename := getFilename(nil, format)
	file, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	file.Write(body)
	file.Close()

	return filename
}

func downloadEpisode(baseContentId string, info EpisodeInfo, audioLangs, subsLangs, ccLangs []string, videoQuality, audioQuality *string) {
	cleanSeriesTitle := sanitizeFilename(info.EpisodeMetadata.SeriesTitle)
	cleanEpisodeTitle := sanitizeFilename(info.Title)

	if _, err := os.Stat(cleanSeriesTitle); err != nil {
		_ = os.MkdirAll(cleanSeriesTitle, 0777)
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
		return
	}

	// Resolve each requested audio locale to its version GUID. Each dub is a
	// separate playback stream with its own manifest, token and Widevine keys.
	guidByLocale := map[string]string{}
	if info.EpisodeMetadata.AudioLocale != "" {
		guidByLocale[info.EpisodeMetadata.AudioLocale] = baseContentId
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
			fmt.Printf("! Audio locale %s is not available for episode %v, aborting this episode.\n", locale, info.EpisodeMetadata.EpisodeNumber)
			return
		}
		versions = append(versions, audioVersion{locale: locale, contentId: guid})
	}

	fmt.Printf("Downloading: %s (S%02vE%02v) from %s\n", info.Title, info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber, info.EpisodeMetadata.SeriesTitle)

	// activeStreams tracks every playback token we open so we can release them
	// all if anything fails partway through.
	activeStreams := map[string]string{}
	defer func() {
		print("Cleaning up...")

		for id, sToken := range activeStreams {
			deleteStream(id, sToken)
		}
		if r := recover(); r != nil {
			fmt.Println("Recovered from error:", r)
		}
	}()

	// Fetch the first version's playback first so we can validate subtitle
	// and caption availability before downloading anything heavy.
	firstEpisode := getEpisode(versions[0].contentId)
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
	if len(ccLangs) == 1 && ccLangs[0] == "all" {
		ccLangs = make([]string, 0, len(firstEpisode.Captions))
		for locale, cc := range firstEpisode.Captions {
			if cc != nil && cc.URL != "" {
				ccLangs = append(ccLangs, locale)
			}
		}
		sort.Strings(ccLangs)
	}

	fmt.Printf("Audio locales: %s | Subtitle locales: %s | CC locales: %s\n",
		strings.Join(audioLangs, ", "), strings.Join(subsLangs, ", "), strings.Join(ccLangs, ", "))

	for _, locale := range subsLangs {
		if firstEpisode.Subtitles[locale] == nil {
			fmt.Printf("! Subtitle locale %s is not available for episode %v, aborting this episode.\n", locale, info.EpisodeMetadata.EpisodeNumber)
			return
		}
	}
	for _, locale := range ccLangs {
		if firstEpisode.Captions[locale] == nil {
			fmt.Printf("! Closed caption locale %s is not available for episode %v, aborting this episode.\n", locale, info.EpisodeMetadata.EpisodeNumber)
			return
		}
	}

	var subTracks []mediaTrack
	for _, locale := range subsLangs {
		fmt.Printf("Downloading subtitles for %s...\n", trackTitle(locale))
		sub := firstEpisode.Subtitles[locale]
		subTracks = append(subTracks, mediaTrack{
			file:   downloadSubs(sub.URL, sub.Format),
			locale: locale,
			format: sub.Format,
		})
	}
	for _, locale := range ccLangs {
		fmt.Printf("Downloading closed captions for %s...\n", trackTitle(locale))
		cc := firstEpisode.Captions[locale]
		subTracks = append(subTracks, mediaTrack{
			file:   downloadSubs(cc.URL, cc.Format),
			locale: locale,
			format: cc.Format,
			isCC:   true,
		})
	}
	if len(subTracks) > 0 {
		fmt.Println("Downloaded subtitles!")
	}

	var videoFile string
	var audioTracks []mediaTrack

	for i, version := range versions {
		episode := firstEpisode
		if i > 0 {
			episode = getEpisode(version.contentId)
			activeStreams[version.contentId] = episode.Token
		}

		manifest := parseManifest(episode.ManifestURL)
		pssh := getPssh(manifest)
		if pssh == nil {
			panic("PSSH not found")
		}
		// getLicense stores the keys in the global "keys" used by downloadParts,
		// so audio for this version must be downloaded before the next license.
		if err := getLicense(*pssh, version.contentId, episode.Token); err != nil {
			panic(fmt.Sprintf("getLicense for %s: %s", version.locale, err))
		}

		audioSet := manifest.Period[0].AdaptationSets[1]
		fmt.Printf("Downloading %s audio...\n", trackTitle(version.locale))
		audioBaseUrl, audioRepresentationId := getBaseUrl(audioSet, false, *audioQuality)
		if audioBaseUrl == nil {
			panic(fmt.Sprintf("failed to get the audio base URL for %s, maybe the audio quality you entered is wrong?", version.locale))
		}
		audioFile, err := downloadParts(audioBaseUrl, audioRepresentationId, audioSet)
		if err != nil {
			panic(err)
		}
		audioTracks = append(audioTracks, mediaTrack{file: audioFile, locale: version.locale})

		// The video track is identical across dubs, so download it once using
		// the first version's keys (already loaded above).
		if i == 0 {
			videoSet := manifest.Period[0].AdaptationSets[0]
			fmt.Println("Downloading video...")
			baseUrl, representationId := getBaseUrl(videoSet, true, *videoQuality)
			if baseUrl == nil {
				panic("failed to get the video base URL, maybe the video quality you entered is wrong?")
			}
			videoFile, err = downloadParts(baseUrl, representationId, videoSet)
			if err != nil {
				panic(err)
			}
		}

		if success := deleteStream(version.contentId, episode.Token); !success {
			print("Failed to remove the player stream, you will probably have issues downloading other episodes.\n")
		}
		delete(activeStreams, version.contentId)
	}

	mergeEverything(videoFile, audioTracks, subTracks, outputFile, info)
}

func downloadSeason(videoQuality, audioQuality *string, audioLangs, subsLangs, ccLangs []string, episodes []SeasonEpisode) {
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

		downloadEpisode(episode.ID, info, audioLangs, subsLangs, ccLangs, videoQuality, audioQuality)
	}
}
