package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	runtimedebug "runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iyear/gowidevine"
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

func downloadParts(title string, baseUrl, representationId *string, set *mpd.AdaptationSet, keys []*widevine.Key) (string, error) {
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

	bar := newProgressBar(title, int64(total), "segments")
	var totalBytes atomic.Int64
	fetch := func(url string) ([]byte, error) {
		data, err := downloadPart(url)
		if err == nil {
			totalBytes.Add(int64(len(data)))
		}
		return data, err
	}
	err = streamSegments(encFile, urls, fetch, func(fetched int64) {
		bar.update(fetched, totalBytes.Load())
	})
	bar.finish()
	if err != nil {
		return "", err
	}

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

// downloadAudioTrack downloads a version's audio representation into a temporary
// file. sets is non-nil only for on-demand manifests.
func downloadAudioTrack(manifest *mpd.MPD, sets []onDemandAdaptationSet, locale, quality string, keys []*widevine.Key) (string, error) {
	title := "Downloading " + trackTitle(locale) + " audio"
	if isOnDemand(manifest) {
		return downloadOnDemandAdaptation(title, sets, false, quality, keys)
	}
	audioSet := manifest.Period[0].AdaptationSets[1]
	audioBaseUrl, audioRepresentationId := getBaseUrl(audioSet, false, quality)
	if audioBaseUrl == nil {
		return "", fmt.Errorf("failed to get the audio base URL for %s, maybe the audio quality you entered is wrong?", locale)
	}
	return downloadParts(title, audioBaseUrl, audioRepresentationId, audioSet, keys)
}

// downloadVideoTrack downloads the video representation into a temporary file.
// The video track is identical across dubs, so it is downloaded only once.
func downloadVideoTrack(manifest *mpd.MPD, sets []onDemandAdaptationSet, quality string, keys []*widevine.Key) (string, error) {
	if isOnDemand(manifest) {
		return downloadOnDemandAdaptation("Downloading video", sets, true, quality, keys)
	}
	videoSet := manifest.Period[0].AdaptationSets[0]
	baseUrl, representationId := getBaseUrl(videoSet, true, quality)
	if baseUrl == nil {
		return "", fmt.Errorf("failed to get the video base URL, maybe the video quality you entered is wrong?")
	}
	return downloadParts("Downloading video", baseUrl, representationId, videoSet, keys)
}

func downloadSubs(url, format string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	filename := getFilename(nil, format)
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	if _, err = file.Write(body); err != nil {
		file.Close()
		return "", err
	}
	file.Close()

	return filename, nil
}

// buildGuidByLocale maps each available audio locale to its playback GUID. The
// episode's "versions" list is authoritative; the episode's own content ID is
// only a fallback for single-audio content where the API lists no versions.
// Crunchyroll sets "audio_locale" to the preferred language (which may be a dub)
// while the episode ID still points at the original, so mapping audio_locale to
// the content ID directly would resolve the dub to the wrong (original) stream.
func buildGuidByLocale(info EpisodeInfo, baseContentId string) map[string]string {
	guidByLocale := map[string]string{}
	for _, v := range info.EpisodeMetadata.Versions {
		guidByLocale[v.AudioLocale] = v.GUID
	}
	if len(guidByLocale) == 0 && info.EpisodeMetadata.AudioLocale != "" {
		guidByLocale[info.EpisodeMetadata.AudioLocale] = baseContentId
	}
	return guidByLocale
}

// filterAvailableLangs drops subtitle/caption locales that the episode does not
// offer, warning about each one instead of aborting the whole download. Subtitles
// are optional, so a missing locale (e.g. the default "en-US" on a movie) should
// not prevent the video and audio from being saved.
func filterAvailableLangs(langs []string, available map[string]*Subtitle, kind string, episode int) []string {
	var filtered []string
	for _, locale := range langs {
		if available[locale] == nil {
			fmt.Printf("! %s locale %s is not available for episode %v, skipping it.\n", kind, locale, episode)
			continue
		}
		filtered = append(filtered, locale)
	}
	return filtered
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
	guidByLocale := buildGuidByLocale(info, baseContentId)

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
			fmt.Printf("! Audio locale %s is not available for episode %v, skipping it.\n", locale, info.EpisodeMetadata.EpisodeNumber)
			continue
		}
		versions = append(versions, audioVersion{locale: locale, contentId: guid})
	}
	if len(versions) == 0 {
		fmt.Printf("! None of the requested audio locales are available for episode %v, aborting this episode.\n", info.EpisodeMetadata.EpisodeNumber)
		return
	}

	fmt.Printf("Downloading: %s (S%02vE%02v) from %s\n", info.Title, info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber, info.EpisodeMetadata.SeriesTitle)

	// activeStreams tracks every playback token we open so we can release them
	// all if anything fails partway through.
	var (
		streamsMu     sync.Mutex
		activeStreams = map[string]string{}
	)
	defer func() {
		print("Cleaning up...\n")

		streamsMu.Lock()
		streams := activeStreams
		activeStreams = map[string]string{}
		streamsMu.Unlock()

		for id, sToken := range streams {
			deleteStream(id, sToken)
		}
		if r := recover(); r != nil {
			fmt.Printf("Recovered from error: %v\n%s\n", r, runtimedebug.Stack())
		}
	}()

	// Fetch the first version's playback first so we can validate subtitle
	// and caption availability before downloading anything heavy.
	firstEpisode, err := getEpisode(versions[0].contentId)
	if err != nil {
		panic(err)
	}
	streamsMu.Lock()
	activeStreams[versions[0].contentId] = firstEpisode.Token
	streamsMu.Unlock()

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

	subsLangs = filterAvailableLangs(subsLangs, firstEpisode.Subtitles, "Subtitle", info.EpisodeMetadata.EpisodeNumber)
	ccLangs = filterAvailableLangs(ccLangs, firstEpisode.Captions, "Closed caption", info.EpisodeMetadata.EpisodeNumber)

	// Build the list of subtitle and caption downloads.
	type subJob struct {
		url    string
		format string
		locale string
		isCC   bool
	}
	var subJobs []subJob
	for _, locale := range subsLangs {
		sub := firstEpisode.Subtitles[locale]
		subJobs = append(subJobs, subJob{url: sub.URL, format: sub.Format, locale: locale})
	}
	for _, locale := range ccLangs {
		cc := firstEpisode.Captions[locale]
		subJobs = append(subJobs, subJob{url: cc.URL, format: cc.Format, locale: locale, isCC: true})
	}

	// Download every subtitle, every audio dub and the video track concurrently.
	// Each dub has its own playback token and license keys, so they no longer
	// need to be serialized behind a single global key set.
	subTracks := make([]mediaTrack, len(subJobs))
	audioTracks := make([]mediaTrack, len(versions))
	var videoFile string

	var (
		wg       sync.WaitGroup
		errMu    sync.Mutex
		firstErr error
	)
	fail := func(err error) {
		errMu.Lock()
		if firstErr == nil && err != nil {
			firstErr = err
		}
		errMu.Unlock()
	}

	for j, job := range subJobs {
		wg.Add(1)
		go func(j int, job subJob) {
			defer wg.Done()
			file, err := downloadSubs(job.url, job.format)
			if err != nil {
				fail(fmt.Errorf("downloading subtitles for %s: %w", trackTitle(job.locale), err))
				return
			}
			subTracks[j] = mediaTrack{file: file, locale: job.locale, format: job.format, isCC: job.isCC}
		}(j, job)
	}

	for i, version := range versions {
		wg.Add(1)
		go func(i int, version audioVersion) {
			defer wg.Done()

			episode := firstEpisode
			if i > 0 {
				var err error
				episode, err = getEpisode(version.contentId)
				if err != nil {
					fail(fmt.Errorf("getEpisode for %s: %w", version.locale, err))
					return
				}
				streamsMu.Lock()
				activeStreams[version.contentId] = episode.Token
				streamsMu.Unlock()
			}

			manifest, body, err := parseManifest(episode.ManifestURL)
			if err != nil {
				fail(fmt.Errorf("parsing manifest for %s: %w", version.locale, err))
				return
			}
			pssh := getPssh(manifest)
			if pssh == nil {
				fail(fmt.Errorf("PSSH not found for %s", version.locale))
				return
			}
			keys, err := getLicense(*pssh, version.contentId, episode.Token)
			if err != nil {
				fail(fmt.Errorf("getLicense for %s: %w", version.locale, err))
				return
			}

			var sets []onDemandAdaptationSet
			if isOnDemand(manifest) {
				var err error
				sets, err = parseOnDemand(body)
				if err != nil {
					fail(err)
					return
				}
			}

			if i == 0 {
				// The video and the first audio track share this version's keys,
				// so they download concurrently with each other and everything
				// else.
				type trackResult struct {
					file string
					err  error
				}
				videoCh := make(chan trackResult, 1)
				go func() {
					file, err := downloadVideoTrack(manifest, sets, *videoQuality, keys)
					videoCh <- trackResult{file, err}
				}()

				audioFile, err := downloadAudioTrack(manifest, sets, version.locale, *audioQuality, keys)
				if err != nil {
					// Join the video download before unwinding so its progress
					// bar can't race the deferred cleanup.
					<-videoCh
					fail(err)
					return
				}
				audioTracks[i] = mediaTrack{file: audioFile, locale: version.locale}

				video := <-videoCh
				if video.err != nil {
					fail(video.err)
					return
				}
				videoFile = video.file
			} else {
				audioFile, err := downloadAudioTrack(manifest, sets, version.locale, *audioQuality, keys)
				if err != nil {
					fail(err)
					return
				}
				audioTracks[i] = mediaTrack{file: audioFile, locale: version.locale}
			}

			if success := deleteStream(version.contentId, episode.Token); !success {
				print("Failed to remove the player stream, you will probably have issues downloading other episodes.\n")
			}
			streamsMu.Lock()
			delete(activeStreams, version.contentId)
			streamsMu.Unlock()
		}(i, version)
	}

	wg.Wait()
	if firstErr != nil {
		panic(firstErr)
	}
	if len(subTracks) > 0 {
		fmt.Println("Downloaded subtitles!")
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
