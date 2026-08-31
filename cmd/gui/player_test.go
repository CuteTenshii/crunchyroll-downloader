package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"crunchyroll-downloader/internal/engine"
)

func TestMissingPlayerErr(t *testing.T) {
	err := missingPlayerErr()
	if err == nil {
		t.Fatal("expected error")
	}
	if !libmpvError(err) {
		t.Fatalf("libmpvError(%v) = false", err)
	}
	if !strings.Contains(err.Error(), "player library missing") {
		t.Fatalf("error %q missing required string", err)
	}
	if libmpvError(nil) {
		t.Fatal("libmpvError(nil) = true")
	}
}

func TestMissingMpvHostMethods(t *testing.T) {
	h := newMissingMpvHost()
	if err := h.Attach(0); !libmpvError(err) {
		t.Fatalf("Attach: %v", err)
	}
	if err := h.LoadFile("x"); !libmpvError(err) {
		t.Fatalf("LoadFile: %v", err)
	}
	if err := h.Pause(true); !libmpvError(err) {
		t.Fatalf("Pause: %v", err)
	}
	if err := h.Seek(1); !libmpvError(err) {
		t.Fatalf("Seek: %v", err)
	}
	if _, err := h.Position(); !libmpvError(err) {
		t.Fatalf("Position: %v", err)
	}
	if _, err := h.Duration(); !libmpvError(err) {
		t.Fatalf("Duration: %v", err)
	}
	if err := h.SetVolume(50); !libmpvError(err) {
		t.Fatalf("SetVolume: %v", err)
	}
	if err := h.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}

func TestStartPlayFactoryError(t *testing.T) {
	orig := mpvHostFactory
	mpvHostFactory = func() (MpvHost, error) { return nil, missingPlayerErr() }
	defer func() { mpvHostFactory = orig }()

	a := NewApp()
	err := a.StartPlay(PlayRequest{})
	if !libmpvError(err) {
		t.Fatalf("StartPlay: %v", err)
	}
	if a.playGen != 1 {
		t.Fatalf("playGen=%d want 1", a.playGen)
	}
	if err := a.StopPlay(); err != nil {
		t.Fatalf("StopPlay: %v", err)
	}
}

type countingHost struct {
	destroys int
}

func (c *countingHost) Attach(uintptr) error       { return nil }
func (c *countingHost) LoadFile(string) error      { return nil }
func (c *countingHost) Pause(bool) error           { return nil }
func (c *countingHost) Seek(float64) error         { return nil }
func (c *countingHost) Position() (float64, error) { return 0, nil }
func (c *countingHost) Duration() (float64, error) { return 0, nil }
func (c *countingHost) SetVolume(int) error        { return nil }
func (c *countingHost) Destroy() error {
	c.destroys++
	return nil
}

func TestStopPlayMatchingGenDestroysHost(t *testing.T) {
	h := &countingHost{}
	a := NewApp()
	a.playHost = h
	a.playGen = 2
	if err := a.StopPlay(); err != nil {
		t.Fatalf("StopPlay: %v", err)
	}
	if h.destroys != 1 {
		t.Fatalf("destroys=%d want 1", h.destroys)
	}
	if a.playHost != nil {
		t.Fatal("host still set")
	}
}

func TestStopPlayStaleGenIsNoop(t *testing.T) {
	old := &countingHost{}
	a := NewApp()
	a.playHost = old
	a.playGen = 4

	a.playMu.Lock()
	gen := a.playGen
	a.playMu.Unlock()

	newer := &countingHost{}
	a.playMu.Lock()
	a.playGen++
	a.playHost = newer
	a.playMu.Unlock()

	if err := a.clearPlayIfGen(gen); err != nil {
		t.Fatalf("clearPlayIfGen: %v", err)
	}
	if old.destroys != 0 || newer.destroys != 0 {
		t.Fatalf("stale stop destroyed hosts old=%d new=%d", old.destroys, newer.destroys)
	}
	if a.playHost != newer {
		t.Fatal("new session host was cleared")
	}
}

func TestStartPlayBumpsGenBeforeFactoryError(t *testing.T) {
	orig := mpvHostFactory
	mpvHostFactory = func() (MpvHost, error) { return nil, missingPlayerErr() }
	defer func() { mpvHostFactory = orig }()

	a := NewApp()
	a.playGen = 7
	stale := a.playGen
	if err := a.StartPlay(PlayRequest{}); !libmpvError(err) {
		t.Fatalf("StartPlay: %v", err)
	}
	if a.playGen <= stale {
		t.Fatalf("playGen=%d not greater than stale %d", a.playGen, stale)
	}
	if err := a.clearPlayIfGen(stale); err != nil {
		t.Fatalf("stale clear: %v", err)
	}
	if a.playGen != stale+1 {
		t.Fatalf("playGen=%d want %d", a.playGen, stale+1)
	}
}

type scriptedHost struct {
	pos, dur    float64
	paused, eof bool
	destroys    int
	loadPath    string
}

func (s *scriptedHost) Attach(uintptr) error       { return nil }
func (s *scriptedHost) LoadFile(path string) error { s.loadPath = path; return nil }
func (s *scriptedHost) Pause(p bool) error         { s.paused = p; return nil }
func (s *scriptedHost) Seek(seconds float64) error { s.pos = seconds; return nil }
func (s *scriptedHost) Position() (float64, error) { return s.pos, nil }
func (s *scriptedHost) Duration() (float64, error) { return s.dur, nil }
func (s *scriptedHost) SetVolume(int) error        { return nil }
func (s *scriptedHost) Destroy() error             { s.destroys++; return nil }
func (s *scriptedHost) playFlags() (paused, eof bool) {
	return s.paused, s.eof
}

type postSink struct {
	mu    sync.Mutex
	posts []playheadPost
}

func (s *postSink) snapshot() []playheadPost {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]playheadPost, len(s.posts))
	copy(out, s.posts)
	return out
}

func stubPlayheadNetwork(t *testing.T) *postSink {
	t.Helper()
	sink := &postSink{}
	origAuth := playAuthenticate
	origGet := playGetPlayheads
	origAcct := playAccountIDFn
	origPost := playPostPlayhead
	playAuthenticate = func(string) error { return nil }
	playGetPlayheads = func(string, []string) (map[string]engine.PlayheadInfo, error) {
		return map[string]engine.PlayheadInfo{}, nil
	}
	playAccountIDFn = func() string { return "acct" }
	playPostPlayhead = func(accountID, contentID string, playheadSec float64, locale, audioLang string) error {
		sink.mu.Lock()
		sink.posts = append(sink.posts, playheadPost{
			accountID: accountID,
			contentID: contentID,
			seconds:   playheadSec,
			locale:    locale,
			audioLang: audioLang,
		})
		sink.mu.Unlock()
		return nil
	}
	t.Cleanup(func() {
		playAuthenticate = origAuth
		playGetPlayheads = origGet
		playAccountIDFn = origAcct
		playPostPlayhead = origPost
	})
	return sink
}

func TestStartPlayNoLocalFile(t *testing.T) {
	stubPlayheadNetwork(t)
	a := NewApp()
	a.prefs.CookieFile = "cookie.txt"
	a.prefs.OutputDir = t.TempDir()
	err := a.StartPlay(PlayRequest{
		EpisodeID:     "GWep",
		SeriesTitle:   "Show",
		SeasonNumber:  1,
		EpisodeNumber: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "no local file") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveLocalMKVMatchesSeasonEpisode(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Show")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "Show S01E02 - Title [1080p].mkv")
	if err := os.WriteFile(want, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Show S01E03 - Other [1080p].mkv"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveLocalMKV(root, "Show", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveLocalMKVPrefersSanitizedSeries(t *testing.T) {
	root := t.TempDir()
	other := filepath.Join(root, "Other")
	series := filepath.Join(root, "Mushoku Tensei_ Jobless Reincarnation")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(series, 0o755); err != nil {
		t.Fatal(err)
	}
	decoy := filepath.Join(other, "Other S01E01 - X [1080p].mkv")
	want := filepath.Join(series, "Mushoku Tensei_ Jobless Reincarnation S01E01 - Jobless Reincarnation [max].mkv")
	if err := os.WriteFile(decoy, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveLocalMKV(root, "Mushoku Tensei: Jobless Reincarnation", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPlayPausePostsPlayhead(t *testing.T) {
	sink := stubPlayheadNetwork(t)
	h := &scriptedHost{pos: 10.4, dur: 100, paused: false}
	a := NewApp()
	a.playHost = h
	a.playEpisodeID = "GWep"
	a.playAccountID = "acct"
	a.playLocale = "pt-BR"
	a.playAudioLang = "ja-JP"
	a.playPaused = false
	a.playDebounce = engine.NewPlayheadDebouncer(time.Second)

	if err := a.PlayPause(); err != nil {
		t.Fatal(err)
	}
	if !h.paused {
		t.Fatal("expected paused")
	}
	posts := sink.snapshot()
	if len(posts) != 1 {
		t.Fatalf("posts=%d want 1", len(posts))
	}
	if posts[0].contentID != "GWep" || posts[0].seconds != 10.4 || posts[0].locale != "pt-BR" || posts[0].audioLang != "ja-JP" {
		t.Fatalf("%+v", posts[0])
	}

	if err := a.PlayPause(); err != nil {
		t.Fatal(err)
	}
	if h.paused {
		t.Fatal("expected unpaused")
	}
	if n := len(sink.snapshot()); n != 1 {
		t.Fatalf("unpause posted %d", n)
	}
}

func TestStopPlayPostsPlayhead(t *testing.T) {
	sink := stubPlayheadNetwork(t)
	h := &scriptedHost{pos: 42, dur: 100}
	a := NewApp()
	a.playHost = h
	a.playGen = 1
	a.playEpisodeID = "GWep"
	a.playAccountID = "acct"
	a.playLocale = "pt-BR"
	a.playAudioLang = "ja-JP"
	a.playDebounce = engine.NewPlayheadDebouncer(time.Second)

	if err := a.StopPlay(); err != nil {
		t.Fatal(err)
	}
	posts := sink.snapshot()
	if len(posts) != 1 {
		t.Fatalf("posts=%d want 1", len(posts))
	}
	if posts[0].seconds != 42 {
		t.Fatalf("seconds %v", posts[0].seconds)
	}
	if h.destroys != 1 {
		t.Fatalf("destroys=%d", h.destroys)
	}
}

func TestEmitPlayStateEOFPostsFinishSeconds(t *testing.T) {
	sink := stubPlayheadNetwork(t)
	h := &scriptedHost{pos: 95, dur: 100, eof: true}
	a := NewApp()
	a.playHost = h
	a.playEpisodeID = "GWep"
	a.playAccountID = "acct"
	a.playLocale = "pt-BR"
	a.playAudioLang = "ja-JP"
	a.playDebounce = engine.NewPlayheadDebouncer(time.Second)

	a.emitPlayState(nil, h)
	posts := sink.snapshot()
	if len(posts) != 1 {
		t.Fatalf("posts=%d want 1", len(posts))
	}
	if posts[0].seconds != 100 {
		t.Fatalf("finish seconds %v want 100", posts[0].seconds)
	}

	a.emitPlayState(nil, h)
	if n := len(sink.snapshot()); n != 1 {
		t.Fatalf("eof tick posted again n=%d", n)
	}
}

func TestEmitPlayStateTickDoesNotPost(t *testing.T) {
	sink := stubPlayheadNetwork(t)
	h := &scriptedHost{pos: 12, dur: 100, paused: false, eof: false}
	a := NewApp()
	a.playHost = h
	a.playEpisodeID = "GWep"
	a.playAccountID = "acct"
	a.playDebounce = engine.NewPlayheadDebouncer(time.Second)

	a.emitPlayState(nil, h)
	a.emitPlayState(nil, h)
	if n := len(sink.snapshot()); n != 0 {
		t.Fatalf("tick posted n=%d", n)
	}
}

func TestPlayPauseThenStopDebouncesSameSecond(t *testing.T) {
	sink := stubPlayheadNetwork(t)
	h := &scriptedHost{pos: 10, dur: 100}
	a := NewApp()
	a.playHost = h
	a.playGen = 1
	a.playEpisodeID = "GWep"
	a.playAccountID = "acct"
	a.playPaused = false
	a.playDebounce = engine.NewPlayheadDebouncer(time.Second)

	if err := a.PlayPause(); err != nil {
		t.Fatal(err)
	}
	if err := a.StopPlay(); err != nil {
		t.Fatal(err)
	}
	if n := len(sink.snapshot()); n != 1 {
		t.Fatalf("pause-then-close posts=%d want 1", n)
	}
}

func TestResolveLocalMKVDoesNotMatchE10InsideE100(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Show")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "Show S01E10 - a.mkv")
	decoy := filepath.Join(dir, "Show S01E100 - b.mkv")
	if err := os.WriteFile(decoy, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveLocalMKV(root, "Show", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveLocalMKVPrefersSeriesFolderNotSubstring(t *testing.T) {
	root := t.TempDir()
	bleach := filepath.Join(root, "Bleach")
	tybw := filepath.Join(root, "Bleach Thousand Year Blood War")
	if err := os.MkdirAll(bleach, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tybw, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(bleach, "Bleach S01E01 - X [1080p].mkv")
	decoy := filepath.Join(tybw, "Bleach Thousand Year Blood War S01E01 - Y [1080p].mkv")
	if err := os.WriteFile(decoy, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveLocalMKV(root, "Bleach", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPlayPauseFailedPostThenStopRetries(t *testing.T) {
	sink := stubPlayheadNetwork(t)
	calls := 0
	inner := playPostPlayhead
	playPostPlayhead = func(accountID, contentID string, playheadSec float64, locale, audioLang string) error {
		calls++
		if calls == 1 {
			return fmt.Errorf("playhead POST HTTP 500")
		}
		return inner(accountID, contentID, playheadSec, locale, audioLang)
	}
	h := &scriptedHost{pos: 10, dur: 100}
	a := NewApp()
	a.playHost = h
	a.playGen = 1
	a.playEpisodeID = "GWep"
	a.playAccountID = "acct"
	a.playPaused = false
	a.playDebounce = engine.NewPlayheadDebouncer(time.Second)

	if err := a.PlayPause(); err == nil {
		t.Fatal("expected pause POST error")
	}
	if n := len(sink.snapshot()); n != 0 {
		t.Fatalf("failed pause posted n=%d", n)
	}
	if err := a.StopPlay(); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("posts=%d want 2", calls)
	}
	if n := len(sink.snapshot()); n != 1 {
		t.Fatalf("close posts=%d want 1", n)
	}
}

func TestStopPlayAbandonsInFlightStartPlay(t *testing.T) {
	sink := stubPlayheadNetwork(t)
	started := make(chan struct{})
	release := make(chan struct{})
	origAuth := playAuthenticate
	playAuthenticate = func(string) error {
		close(started)
		<-release
		return nil
	}
	t.Cleanup(func() { playAuthenticate = origAuth })

	factoryCalls := 0
	origFactory := mpvHostFactory
	mpvHostFactory = func() (MpvHost, error) {
		factoryCalls++
		return &scriptedHost{}, nil
	}
	t.Cleanup(func() { mpvHostFactory = origFactory })

	dir := t.TempDir()
	path := filepath.Join(dir, "Show S01E01 - X [1080p].mkv")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := NewApp()
	a.prefs.CookieFile = "cookie.txt"
	a.prefs.OutputDir = dir

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.StartPlay(PlayRequest{
			EpisodeID:     "GWep",
			SeriesTitle:   "Show",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			FilePath:      path,
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("auth did not start")
	}

	if err := a.StopPlay(); err != nil {
		t.Fatal(err)
	}
	close(release)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("StartPlay: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StartPlay did not return")
	}

	if a.playHost != nil {
		t.Fatal("host set after abandon")
	}
	if a.playCancel != nil {
		t.Fatal("ticker running after abandon")
	}
	if factoryCalls != 0 {
		t.Fatalf("factoryCalls=%d", factoryCalls)
	}
	if n := len(sink.snapshot()); n != 0 {
		t.Fatalf("posts=%d", n)
	}
}
