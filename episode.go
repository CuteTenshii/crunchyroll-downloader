package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const playback4294Code = "4294"

// PlaybackAPIError is the safe, typed provider boundary for playback-open
// failures. It deliberately retains only the episode ID and parsed scalar
// code/message; the raw provider body is never stored or formatted.
type PlaybackAPIError struct {
	EpisodeID  string
	Code       string
	Message    string
	HTTPStatus int
	RetryAfter string
	RateLimit  ProviderRateLimitHeaders
}

func (e *PlaybackAPIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("playback API error for %s: code %s: %s", e.EpisodeID, e.Code, e.Message)
	}
	return fmt.Sprintf("playback API error for %s: code %s", e.EpisodeID, e.Code)
}

func safeProviderMessage(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	message = redactSensitiveURLs(message)
	const maxProviderMessageBytes = 160
	if len(message) > maxProviderMessageBytes {
		message = message[:maxProviderMessageBytes] + "..."
	}
	return message
}

func safeHeaderScalar(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 80 {
		return value[:80]
	}
	return value
}

type ProviderRateLimitHeaders struct {
	Limit     string `json:"limit,omitempty"`
	Remaining string `json:"remaining,omitempty"`
	Reset     string `json:"reset,omitempty"`
}

func providerCode(value any) string {
	switch value := value.(type) {
	case json.Number:
		return value.String()
	case string:
		if _, err := strconv.ParseInt(value, 10, 64); err == nil {
			return value
		}
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return "unknown"
}

func parsePlaybackAPIError(episodeID string, raw json.RawMessage) *PlaybackAPIError {
	parsed := &PlaybackAPIError{EpisodeID: episodeID, Code: "unknown"}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		parsed.Message = "provider returned an unreadable error"
		return parsed
	}
	switch value := value.(type) {
	case json.Number:
		parsed.Code = providerCode(value)
	case string:
		if code := providerCode(value); code != "unknown" {
			parsed.Code = code
		} else {
			parsed.Message = safeProviderMessage(value)
		}
	case map[string]any:
		parsed.Code = providerCode(value["code"])
		if message, ok := value["message"].(string); ok {
			parsed.Message = safeProviderMessage(message)
		}
	default:
		parsed.Message = "provider returned an unrecognized error"
	}
	return parsed
}

type playbackOpener func(string) Episode
type playbackSleeper func(time.Duration)

var sleepPlaybackRetry playbackSleeper = time.Sleep

func playbackRetryDelay(base time.Duration, retry int) time.Duration {
	delay := base
	for i := 0; i < retry && delay < maxPlayback4294Backoff; i++ {
		delay *= 2
		if delay > maxPlayback4294Backoff {
			delay = maxPlayback4294Backoff
		}
	}
	return delay
}

// openPlaybackWithRetry retries only the provider's typed 4294 response.
// Other panics retain their original value and propagate immediately.
func openPlaybackWithRetry(id string, open playbackOpener, retries int, backoff time.Duration, sleep playbackSleeper) Episode {
	for attempt := 0; ; attempt++ {
		var episode Episode
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			episode = open(id)
		}()
		if recovered == nil {
			return episode
		}
		err, ok := recovered.(error)
		if !ok {
			panic(recovered)
		}
		var playbackErr *PlaybackAPIError
		if !errors.As(err, &playbackErr) || playbackErr.Code != playback4294Code || attempt >= retries {
			panic(recovered)
		}
		sleep(playbackRetryDelay(backoff, attempt))
	}
}

type Episode struct {
	// Dash manifest file URL
	ManifestURL string `json:"url"`
	// List of .ass files (translation-style subtitles)
	Subtitles map[string]*Subtitle `json:"subtitles"`
	// List of .vtt files (closed captions, transcribing the dub audio)
	Captions map[string]*Subtitle `json:"captions"`
	// Token to give to the Widevine CDM challenge
	Token string `json:"token"`
	// Error, `nil` if there's no error. Crunchyroll returns this as a string
	// or sometimes a number, so use json.RawMessage to tolerate either.
	Error json.RawMessage `json:"error"`
}

type Subtitle struct {
	// Language represents a subtitle language in the "en-US" format
	Language string `json:"language"`
	// Format of the file, e.g. "ass" or "vtt"
	Format string `json:"format"`
	// Direct URL to the subtitle/caption file
	URL string `json:"url"`
}

func getEpisode(id string) Episode {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://www.crunchyroll.com/playback/v3/%s/web/firefox/play", id), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := DoRequest(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var episode Episode
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	decodeErr := json.Unmarshal(body, &episode)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if decodeErr == nil && len(episode.Error) > 0 && string(episode.Error) != "null" {
			providerErr := parsePlaybackAPIError(id, episode.Error)
			providerErr.HTTPStatus = resp.StatusCode
			providerErr.RetryAfter = safeHeaderScalar(resp.Header.Get("Retry-After"))
			providerErr.RateLimit = providerRateLimitHeaders(resp.Header)
			panic(providerErr)
		}
		panic(&HTTPStatusError{
			URL:        redactURL(req.URL.String()),
			StatusCode: resp.StatusCode,
			RetryAfter: safeHeaderScalar(resp.Header.Get("Retry-After")),
			RateLimit:  providerRateLimitHeaders(resp.Header),
		})
	}
	if decodeErr != nil {
		panic(decodeErr)
	}
	if len(episode.Error) > 0 && string(episode.Error) != "null" {
		// Panic rather than os.Exit so callers (download loop, index loop) can
		// recover and continue past a single bad episode instead of dying.
		providerErr := parsePlaybackAPIError(id, episode.Error)
		providerErr.HTTPStatus = resp.StatusCode
		providerErr.RetryAfter = safeHeaderScalar(resp.Header.Get("Retry-After"))
		providerErr.RateLimit = providerRateLimitHeaders(resp.Header)
		panic(providerErr)
	}

	if *debug {
		fmt.Printf("\n%s\n", string(body))
	}

	return episode
}

type EpisodeMetadataResponse struct {
	Data []EpisodeInfo `json:"data"`
}

type EpisodeInfo struct {
	EpisodeMetadata EpisodeMetadata `json:"episode_metadata"`
	// Episode title
	Title string `json:"title"`
}

type EpisodeMetadata struct {
	AudioLocale   string `json:"audio_locale"`
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	SeriesTitle   string `json:"series_title"`
	Description   string `json:"description"`
	// AvailabilityStarts represents the date when the episode was released on Crunchyroll
	AvailabilityStarts string        `json:"availability_starts"`
	Versions           []*DubVersion `json:"versions"`
}

type DubVersion struct {
	AudioLocale string `json:"audio_locale"`
	GUID        string `json:"guid"`
}

func getEpisodeInfo(id string) EpisodeInfo {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://www.crunchyroll.com/content/v2/cms/objects/%s?ratings=true&preferred_audio_language=ja-JP&locale=en-US", id), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := DoRequest(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var info EpisodeMetadataResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(body, &info); err != nil {
		panic(err)
	}

	return info.Data[0]
}

// deleteStream removes the stream to make Crunchyroll think we "left" the playback
func deleteStream(contentId, sToken string) bool {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("https://www.crunchyroll.com/playback/v1/token/%s/%s", contentId, sToken), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := DoRequest(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusNoContent
}
