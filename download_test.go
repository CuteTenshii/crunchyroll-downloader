package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/unki2aut/go-mpd"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestFetchSubtitleASSRetainsRawBytesAndRejectsBadStatus(t *testing.T) {
	raw := []byte("\xEF\xBB\xBF[Script Info]\r\nTitle: raw\r\n[Events]\r\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\r\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,A, B\r\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/raw":
			_, _ = w.Write(raw)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	got, err := fetchSubtitleASS(server.URL + "/raw")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("raw subtitle bytes changed: got %q want %q", got, raw)
	}
	_, err = fetchSubtitleASS(server.URL + "/bad?signature=sentinel-secret")
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTPStatusError, got %v", err)
	}
	if strings.Contains(err.Error(), "sentinel-secret") || strings.Contains(statusErr.URL, "sentinel-secret") {
		t.Fatalf("signed subtitle URL leaked into error: %v", err)
	}
}

func TestFetchSubtitleASSTransportAndCheckpointErrorsRedactURLs(t *testing.T) {
	originalClient := subtitleHTTPClient
	subtitleHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport failure for https://cdn.example/sub.ass?signature=sentinel-secret")
	})}
	defer func() { subtitleHTTPClient = originalClient }()
	_, err := fetchSubtitleASS("https://cdn.example/sub.ass?signature=sentinel-secret")
	if !strings.Contains(err.Error(), "subtitle transport failed") || strings.Contains(err.Error(), "sentinel-secret") {
		t.Fatalf("unsafe transport error: %v", err)
	}
	if got := redactSensitiveURLs("cache failed https://cdn.example/sub.ass?signature=sentinel-secret"); strings.Contains(got, "sentinel-secret") {
		t.Fatalf("unsafe checkpoint error: %q", got)
	}
}

func TestProviderErrorRedactionRemovesPlaybackPathTokens(t *testing.T) {
	err := &HTTPStatusError{URL: "https://api.example/playback/v1/token/episode/path-sentinel?signature=query-sentinel#fragment", StatusCode: http.StatusUnauthorized}
	message := err.Error()
	if strings.Contains(message, "path-sentinel") || strings.Contains(message, "query-sentinel") || strings.Contains(message, "fragment") {
		t.Fatalf("provider path/query token leaked: %q", message)
	}
	if !strings.Contains(message, "https://api.example") {
		t.Fatalf("expected host-only diagnostic, got %q", message)
	}
}

func TestFetchSubtitleASSRejectsHTMLAndContentLengthMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<!doctype html><html><body>sign in</body></html>"))
	}))
	defer server.Close()
	_, err := fetchSubtitleASS(server.URL)
	var bodyErr *SubtitleBodyError
	if !errors.As(err, &bodyErr) || bodyErr.Problem != "body is not an ASS subtitle document" {
		t.Fatalf("expected non-ASS SubtitleBodyError, got %v", err)
	}

	originalClient := subtitleHTTPClient
	subtitleHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: 999,
			Body:          io.NopCloser(bytes.NewReader([]byte("[Script Info]\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"))),
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})}
	defer func() { subtitleHTTPClient = originalClient }()
	_, err = fetchSubtitleASS("https://example.invalid/subtitle.ass")
	if !errors.As(err, &bodyErr) || bodyErr.Problem != "body does not match declared Content-Length" {
		t.Fatalf("expected content-length SubtitleBodyError, got %v", err)
	}
}

func TestDownloadEpisodeReturnsRecoveredPanicAndReleasesActiveStream(t *testing.T) {
	originalOpen := openDownloadPlayback
	originalClose := closeDownloadPlayback
	originalParse := parseDownloadManifest
	defer func() {
		openDownloadPlayback = originalOpen
		closeDownloadPlayback = originalClose
		parseDownloadManifest = originalParse
	}()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(originalWD) }()

	var releasedID, releasedToken string
	openDownloadPlayback = func(string) Episode {
		return Episode{ManifestURL: "https://manifest.example/signed", Token: "stream-token"}
	}
	parseDownloadManifest = func(string) *mpd.MPD {
		panic(errors.New("provider E31 playback failure"))
	}
	closeDownloadPlayback = func(id, streamToken string) bool {
		releasedID, releasedToken = id, streamToken
		return true
	}

	err = downloadEpisode("GY19Q77GR", EpisodeInfo{
		EpisodeMetadata: EpisodeMetadata{SeriesTitle: "One Piece", AudioLocale: "ja-JP"},
		Title:           "Episode 31",
	}, []string{"ja-JP"}, nil, strPtr("1080p"), strPtr("192k"))
	if err == nil {
		t.Fatal("downloadEpisode swallowed the recovered panic")
	}
	if got, want := err.Error(), "download episode GY19Q77GR: panic recovered: provider E31 playback failure"; got != want {
		t.Fatalf("recovered error = %q, want %q", got, want)
	}
	if releasedID != "GY19Q77GR" || releasedToken != "stream-token" {
		t.Fatalf("active playback stream was not released: id=%q token=%q", releasedID, releasedToken)
	}
}

func TestProcessURLPropagatesDownloadFailure(t *testing.T) {
	originalInfo := getProcessEpisodeInfo
	originalOpen := openDownloadPlayback
	defer func() {
		getProcessEpisodeInfo = originalInfo
		openDownloadPlayback = originalOpen
	}()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(originalWD) }()

	getProcessEpisodeInfo = func(string) EpisodeInfo {
		return EpisodeInfo{
			EpisodeMetadata: EpisodeMetadata{SeriesTitle: "One Piece", AudioLocale: "ja-JP"},
			Title:           "Episode 31",
		}
	}
	openDownloadPlayback = func(string) Episode {
		panic(errors.New("provider E31 playback failure"))
	}

	err = processUrl("https://www.crunchyroll.com/watch/GY19Q77GR")
	if err == nil {
		t.Fatal("processUrl swallowed the episode download failure")
	}
	if got, want := err.Error(), "download episode GY19Q77GR: panic recovered: provider E31 playback failure"; got != want {
		t.Fatalf("processUrl error = %q, want %q", got, want)
	}
}

func TestDownloadEpisodeRetries4294ThenUsesSuccessfulPlayback(t *testing.T) {
	originalOpen := openDownloadPlayback
	originalClose := closeDownloadPlayback
	originalParse := parseDownloadManifest
	originalSleep := sleepPlaybackRetry
	originalRetries := *playback4294Retries
	originalBackoff := *playback4294Backoff
	defer func() {
		openDownloadPlayback = originalOpen
		closeDownloadPlayback = originalClose
		parseDownloadManifest = originalParse
		sleepPlaybackRetry = originalSleep
		*playback4294Retries = originalRetries
		*playback4294Backoff = originalBackoff
	}()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(originalWD) }()

	*playback4294Retries = 2
	*playback4294Backoff = 5 * time.Second
	var calls int
	var delays []time.Duration
	openDownloadPlayback = func(id string) Episode {
		calls++
		if calls == 1 {
			panic(&PlaybackAPIError{EpisodeID: id, Code: playback4294Code})
		}
		return Episode{ManifestURL: "https://manifest.example/signed", Token: "stream-token"}
	}
	sleepPlaybackRetry = func(delay time.Duration) { delays = append(delays, delay) }
	parseDownloadManifest = func(string) *mpd.MPD { panic(errors.New("stop after playback")) }
	closeDownloadPlayback = func(id, streamToken string) bool {
		return id == "GY19Q77GR" && streamToken == "stream-token"
	}

	err = downloadEpisode("GY19Q77GR", EpisodeInfo{
		EpisodeMetadata: EpisodeMetadata{SeriesTitle: "One Piece", AudioLocale: "ja-JP"},
		Title:           "Episode 31",
	}, []string{"ja-JP"}, nil, strPtr("1080p"), strPtr("192k"))
	if err == nil || !strings.Contains(err.Error(), "stop after playback") {
		t.Fatalf("downloadEpisode() error = %v", err)
	}
	if calls != 2 || len(delays) != 1 || delays[0] != 5*time.Second {
		t.Fatalf("calls=%d delays=%v", calls, delays)
	}
}

func TestDownloadEpisodeExhausted4294ReturnsError(t *testing.T) {
	originalOpen := openDownloadPlayback
	originalSleep := sleepPlaybackRetry
	originalRetries := *playback4294Retries
	originalBackoff := *playback4294Backoff
	defer func() {
		openDownloadPlayback = originalOpen
		sleepPlaybackRetry = originalSleep
		*playback4294Retries = originalRetries
		*playback4294Backoff = originalBackoff
	}()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(originalWD) }()

	*playback4294Retries = 1
	*playback4294Backoff = time.Second
	calls := 0
	openDownloadPlayback = func(id string) Episode {
		calls++
		panic(&PlaybackAPIError{EpisodeID: id, Code: playback4294Code})
	}
	sleepPlaybackRetry = func(time.Duration) {}

	err = downloadEpisode("GY19Q77GR", EpisodeInfo{
		EpisodeMetadata: EpisodeMetadata{SeriesTitle: "One Piece", AudioLocale: "ja-JP"},
		Title:           "Episode 31",
	}, []string{"ja-JP"}, nil, strPtr("1080p"), strPtr("192k"))
	if err == nil || !strings.Contains(err.Error(), "code 4294") || calls != 2 {
		t.Fatalf("downloadEpisode() error=%v calls=%d", err, calls)
	}
}

func strPtr(value string) *string { return &value }
