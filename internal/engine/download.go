package engine

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	widevine "github.com/iyear/gowidevine"
	"github.com/unki2aut/go-mpd"
)

const maxWorkers = 10

const maxSubtitleBytes int64 = 16 << 20

const providerHTTPTimeout = 30 * time.Second

// These variables are test-overridable; production provider requests remain
// bounded even when a provider stalls without returning an HTTP error.
var subtitleHTTPClient = &http.Client{Timeout: providerHTTPTimeout}
var segmentHTTPClient = &http.Client{Timeout: providerHTTPTimeout}

// These seams keep the error boundary testable without opening a live playback
// stream. Production always uses the concrete provider functions.
var openDownloadPlayback = getEpisode
var closeDownloadPlayback = deleteStream
var parseDownloadManifest = parseManifest

// HTTPStatusError reports a response that cannot be used by a caller. Keeping
// the status on the error lets the index checkpoint distinguish a missing or
// rejected subtitle from an interrupted local write.
type HTTPStatusError struct {
	URL        string
	StatusCode int
	RetryAfter string
	RateLimit  ProviderRateLimitHeaders
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d for %s", e.StatusCode, redactURL(e.URL))
}

func providerRateLimitHeaders(header http.Header) ProviderRateLimitHeaders {
	return ProviderRateLimitHeaders{
		Limit:     safeHeaderScalar(header.Get("RateLimit-Limit")),
		Remaining: safeHeaderScalar(header.Get("RateLimit-Remaining")),
		Reset:     safeHeaderScalar(header.Get("RateLimit-Reset")),
	}
}

// SubtitleBodyError means a subtitle response was structurally unsafe to
// cache. The body is deliberately never interpreted or rewritten here: ASS
// bytes are retained verbatim once validated.
type SubtitleBodyError struct {
	URL     string
	Problem string
}

func (e *SubtitleBodyError) Error() string {
	return fmt.Sprintf("invalid subtitle response for %s: %s", redactURL(e.URL), e.Problem)
}

type SubtitleTransportError struct{ Operation string }

func (e *SubtitleTransportError) Error() string {
	return "subtitle " + e.Operation + " failed"
}

var urlInMessageRe = regexp.MustCompile(`https?://[^\s]+`)

// redactURL preserves only scheme and host. Provider URL paths can contain
// playback/release tokens, so they are never safe to persist or report.
func redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "<redacted-url>"
	}
	return parsed.Scheme + "://" + parsed.Host
}

func redactSensitiveURLs(message string) string {
	return urlInMessageRe.ReplaceAllStringFunc(message, redactURL)
}

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
		resp, err := segmentHTTPClient.Do(req)
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

	filename := getFilename(set, "")

	// Stream encrypted data to disk instead of hoarding the full media in RAM
	// (movie-length assets previously spiked to 14GB+ and OOM'd).
	encFile, err := os.Create(filename + ".enc")
	if err != nil {
		return "", err
	}
	defer os.Remove(filename + ".enc")
	defer encFile.Close()

	if _, err := encFile.Write(initData); err != nil {
		return "", fmt.Errorf("write init segment: %w", err)
	}
	for i, data := range results {
		if _, err := encFile.Write(data); err != nil {
			return "", fmt.Errorf("write media segment %d: %w", i, err)
		}
		results[i] = nil // allow GC to reclaim each segment after it hits disk
	}
	if _, err := encFile.Seek(0, 0); err != nil {
		return "", fmt.Errorf("rewind encrypted temp file: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if err = widevine.DecryptMP4Auto(encFile, keys, file); err != nil {
		return "", fmt.Errorf("widevine.DecryptMP4Auto: %w", err)
	}

	return filename, nil
}

// fetchSubtitleASS validates and returns the raw ASS response. It intentionally
// does not parse, normalize, or transcode the response; the index cache is an
// exact-byte preservation store.
func fetchSubtitleASS(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, &SubtitleTransportError{Operation: "request construction"}
	}
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	recordProviderCall(req)
	resp, err := subtitleHTTPClient.Do(req)
	if err != nil {
		return nil, &SubtitleTransportError{Operation: "transport"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPStatusError{
			URL: redactURL(url), StatusCode: resp.StatusCode,
			RetryAfter: safeHeaderScalar(resp.Header.Get("Retry-After")),
			RateLimit:  providerRateLimitHeaders(resp.Header),
		}
	}
	if resp.ContentLength == 0 {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "empty body"}
	}
	if resp.ContentLength > maxSubtitleBytes {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "body exceeds size limit"}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSubtitleBytes+1))
	if err != nil {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "unable to read body"}
	}
	if len(body) == 0 {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "empty body"}
	}
	if int64(len(body)) > maxSubtitleBytes {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "body exceeds size limit"}
	}
	if resp.ContentLength >= 0 && int64(len(body)) != resp.ContentLength {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "body does not match declared Content-Length"}
	}
	if !isASS(body) {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "body is not an ASS subtitle document"}
	}
	return body, nil
}

// isASS accepts the standard UTF-8 BOM and CRLF line endings, but requires
// the two structural sections which distinguish an ASS document from a 200
// HTML/login/error page. It is validation only: the returned cache bytes are
// never normalized or rewritten.
func isASS(body []byte) bool {
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	lines := bytes.Split(body, []byte{'\n'})
	firstSection := ""
	seenEvents := false
	seenFormat := false
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == ';' {
			continue
		}
		if firstSection == "" {
			firstSection = string(line)
			if !strings.EqualFold(firstSection, "[Script Info]") {
				return false
			}
			continue
		}
		if strings.EqualFold(string(line), "[Events]") {
			seenEvents = true
			continue
		}
		if seenEvents && strings.HasPrefix(strings.ToLower(string(line)), "format:") {
			seenFormat = true
		}
	}
	return firstSection != "" && seenEvents && seenFormat
}

// fetchSubtitleBytes downloads a subtitle/caption payload with size limits and
// no format-specific validation. Used for non-ASS formats such as WebVTT closed
// captions. ASS transcripts continue to use fetchSubtitleASS so the index cache
// only ever stores verified ASS documents.
func fetchSubtitleBytes(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, &SubtitleTransportError{Operation: "request construction"}
	}
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	recordProviderCall(req)
	resp, err := subtitleHTTPClient.Do(req)
	if err != nil {
		return nil, &SubtitleTransportError{Operation: "transport"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPStatusError{
			URL: redactURL(url), StatusCode: resp.StatusCode,
			RetryAfter: safeHeaderScalar(resp.Header.Get("Retry-After")),
			RateLimit:  providerRateLimitHeaders(resp.Header),
		}
	}
	if resp.ContentLength == 0 {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "empty body"}
	}
	if resp.ContentLength > maxSubtitleBytes {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "body exceeds size limit"}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSubtitleBytes+1))
	if err != nil {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "unable to read body"}
	}
	if len(body) == 0 {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "empty body"}
	}
	if int64(len(body)) > maxSubtitleBytes {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "body exceeds size limit"}
	}
	if resp.ContentLength >= 0 && int64(len(body)) != resp.ContentLength {
		return nil, &SubtitleBodyError{URL: redactURL(url), Problem: "body does not match declared Content-Length"}
	}
	return body, nil
}

func downloadSubs(url, format string) (string, error) {
	if format == "" {
		format = "ass"
	}

	var body []byte
	var err error
	if format == "ass" {
		body, err = fetchSubtitleASS(url)
	} else {
		body, err = fetchSubtitleBytes(url)
	}
	if err != nil {
		return "", err
	}

	filename := getFilename(nil, format)
	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("create subtitle temp file: %w", err)
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		_ = os.Remove(filename)
		return "", fmt.Errorf("write subtitle temp file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(filename)
		return "", fmt.Errorf("close subtitle temp file: %w", err)
	}

	return filename, nil
}

// sanitize is the shared filename sanitizer used by download and index paths.
// Implementation lives in utils.go as sanitizeFilename (PR #30/#31).
func sanitize(s string) string {
	return sanitizeFilename(s)
}

func panicAsError(recovered any) error {
	switch value := recovered.(type) {
	case error:
		return fmt.Errorf("panic recovered: %w", value)
	default:
		return fmt.Errorf("panic recovered: %v", value)
	}
}

// releaseDownloadPlayback converts a panic in provider cleanup into an error so
// it cannot hide the original download failure or leave the caller believing a
// stream was released when it was not.
func releaseDownloadPlayback(contentID, streamToken string) (released bool, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("release playback stream for %s: %w", contentID, panicAsError(recovered))
		}
	}()
	return closeDownloadPlayback(contentID, streamToken), nil
}

func downloadEpisode(baseContentId string, info EpisodeInfo, audioLangs, subsLangs, ccLangs []string, videoQuality, audioQuality string) (err error) {
	cleanSeriesTitle := sanitizeFilename(info.EpisodeMetadata.SeriesTitle)
	cleanEpisodeTitle := sanitizeFilename(info.Title)

	if err := os.MkdirAll(cleanSeriesTitle, 0777); err != nil {
		return fmt.Errorf("create output directory %q: %w", cleanSeriesTitle, err)
	}

	outputFile := filepath.Join(cleanSeriesTitle, fmt.Sprintf("%s S%02dE%02d - %s [%s].mkv",
		cleanSeriesTitle,
		info.EpisodeMetadata.SeasonNumber,
		info.EpisodeMetadata.EpisodeNumber,
		cleanEpisodeTitle,
		videoQuality,
	))

	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("Episode %v is already downloaded, skipping...\n", info.EpisodeMetadata.EpisodeNumber)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat output file %q: %w", outputFile, err)
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
			return fmt.Errorf("audio locale %s is not available for episode %v", locale, info.EpisodeMetadata.EpisodeNumber)
		}
		versions = append(versions, audioVersion{locale: locale, contentId: guid})
	}
	if len(versions) == 0 {
		return fmt.Errorf("no audio locales were requested for episode %v", info.EpisodeMetadata.EpisodeNumber)
	}

	fmt.Printf("Downloading: %s (S%02vE%02v) from %s\n", info.Title, info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber, info.EpisodeMetadata.SeriesTitle)

	// activeStreams tracks every playback token we open so we can release them
	// all if anything fails partway through.
	activeStreams := map[string]string{}
	defer func() {
		fmt.Println("Cleaning up...")
		var cleanupErrs []error
		for id, sToken := range activeStreams {
			released, releaseErr := releaseDownloadPlayback(id, sToken)
			if releaseErr != nil {
				cleanupErrs = append(cleanupErrs, releaseErr)
				continue
			}
			if !released {
				cleanupErrs = append(cleanupErrs, fmt.Errorf("release playback stream for %s: provider rejected cleanup", id))
			}
		}
		if recovered := recover(); recovered != nil {
			err = panicAsError(recovered)
		}
		if cleanupErr := errors.Join(cleanupErrs...); cleanupErr != nil {
			if err == nil {
				err = cleanupErr
			} else {
				err = errors.Join(err, cleanupErr)
			}
		}
		if err != nil {
			err = fmt.Errorf("download episode %s: %w", baseContentId, err)
		}
	}()

	// Fetch the first version's playback first so we can validate subtitle
	// and caption availability before downloading anything heavy.
	firstEpisode := openPlaybackWithRetry(versions[0].contentId, openDownloadPlayback, activeConfig.Playback4294Retries, activeConfig.Playback4294Backoff, sleepPlaybackRetry)
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
			return fmt.Errorf("subtitle locale %s is not available for episode %v", locale, info.EpisodeMetadata.EpisodeNumber)
		}
	}
	for _, locale := range ccLangs {
		if firstEpisode.Captions[locale] == nil {
			return fmt.Errorf("closed caption locale %s is not available for episode %v", locale, info.EpisodeMetadata.EpisodeNumber)
		}
	}

	var subTracks []mediaTrack
	for _, locale := range subsLangs {
		fmt.Printf("Downloading subtitles for %s...\n", trackTitle(locale))
		sub := firstEpisode.Subtitles[locale]
		format := sub.Format
		if format == "" {
			format = "ass"
		}
		subtitleFile, err := downloadSubs(sub.URL, format)
		if err != nil {
			return fmt.Errorf("download subtitles for %s: %w", locale, err)
		}
		subTracks = append(subTracks, mediaTrack{
			file:   subtitleFile,
			locale: locale,
			format: format,
		})
	}
	for _, locale := range ccLangs {
		fmt.Printf("Downloading closed captions for %s...\n", trackTitle(locale))
		cc := firstEpisode.Captions[locale]
		format := cc.Format
		if format == "" {
			format = "vtt"
		}
		ccFile, err := downloadSubs(cc.URL, format)
		if err != nil {
			return fmt.Errorf("download closed captions for %s: %w", locale, err)
		}
		subTracks = append(subTracks, mediaTrack{
			file:   ccFile,
			locale: locale,
			format: format,
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
			episode = openPlaybackWithRetry(version.contentId, openDownloadPlayback, activeConfig.Playback4294Retries, activeConfig.Playback4294Backoff, sleepPlaybackRetry)
			activeStreams[version.contentId] = episode.Token
		}

		manifest := parseDownloadManifest(episode.ManifestURL)
		pssh := getPssh(manifest)
		if pssh == nil {
			return errors.New("PSSH not found")
		}
		// getLicense stores the keys in the global "keys" used by downloadParts,
		// so audio for this version must be downloaded before the next license.
		if err := getLicense(*pssh, version.contentId, episode.Token); err != nil {
			return fmt.Errorf("get license for %s: %w", version.locale, err)
		}

		audioSet := manifest.Period[0].AdaptationSets[1]
		fmt.Printf("Downloading %s audio...\n", trackTitle(version.locale))
		audioBaseUrl, audioRepresentationId := getBaseUrl(audioSet, false, audioQuality)
		if audioBaseUrl == nil {
			return fmt.Errorf("failed to get the audio base URL for %s; check the requested audio quality", version.locale)
		}
		audioFile, err := downloadParts(audioBaseUrl, audioRepresentationId, audioSet)
		if err != nil {
			return fmt.Errorf("download %s audio: %w", version.locale, err)
		}
		audioTracks = append(audioTracks, mediaTrack{file: audioFile, locale: version.locale})

		// The video track is identical across dubs, so download it once using
		// the first version's keys (already loaded above).
		if i == 0 {
			videoSet := manifest.Period[0].AdaptationSets[0]
			fmt.Println("Downloading video...")
			baseUrl, representationId := getBaseUrl(videoSet, true, videoQuality)
			if baseUrl == nil {
				return errors.New("failed to get the video base URL; check the requested video quality")
			}
			videoFile, err = downloadParts(baseUrl, representationId, videoSet)
			if err != nil {
				return fmt.Errorf("download video: %w", err)
			}
		}

		released, releaseErr := releaseDownloadPlayback(version.contentId, episode.Token)
		if releaseErr != nil {
			return releaseErr
		}
		if !released {
			return fmt.Errorf("release playback stream for %s: provider rejected cleanup", version.contentId)
		}
		delete(activeStreams, version.contentId)
	}

	mergeEverything(videoFile, audioTracks, subTracks, outputFile, info)
	return nil
}

func downloadSeason(videoQuality, audioQuality string, audioLangs, subsLangs, ccLangs []string, episodes []SeasonEpisode) error {
	if len(episodes) == 0 {
		return errors.New("season has no episodes")
	}
	fmt.Printf("Downloading season %v of %s (%v episodes)\n\n", episodes[0].SeasonNumber, episodes[0].SeriesTitle, len(episodes))

	for _, episode := range episodes {
		info := EpisodeInfo{
			EpisodeMetadata: EpisodeMetadata{
				SeriesTitle:        episode.SeriesTitle,
				SeasonNumber:       episode.SeasonNumber,
				EpisodeNumber:      episode.EpisodeNumber,
				AudioLocale:        episode.AudioLocale,
				Description:        episode.Description,
				Versions:           episode.Versions,
				AvailabilityStarts: episode.AvailabilityStarts,
			},
			Title: episode.Title,
		}

		if err := downloadEpisode(episode.ID, info, audioLangs, subsLangs, ccLangs, videoQuality, audioQuality); err != nil {
			return fmt.Errorf("download season %d episode %d: %w", episode.SeasonNumber, episode.EpisodeNumber, err)
		}
	}
	return nil
}
