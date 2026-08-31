package main

import (
	"context"
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

func TestStartPlayFallsBackToProgressive(t *testing.T) {
	stubPlayheadNetwork(t)
	dir := t.TempDir()
	playing := filepath.Join(dir, "playing.mp4")
	if err := os.WriteFile(playing, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	origProg := startProgressivePlay
	var called bool
	var gotID string
	startProgressivePlay = func(ctx context.Context, episodeID string, cfg engine.RuntimeConfig, emit func(engine.PlayProgress)) (*engine.PlaySession, error) {
		called = true
		gotID = episodeID
		if emit != nil {
			emit(engine.PlayProgress{BufferEndSec: 4, DurationSec: 24, Ready: true, PlayingPath: playing})
		}
		return &engine.PlaySession{EpisodeID: episodeID, Dir: dir, BufferEndSec: 4}, nil
	}
	t.Cleanup(func() { startProgressivePlay = origProg })

	h := &scriptedHost{}
	origFactory := mpvHostFactory
	mpvHostFactory = func() (MpvHost, error) { return h, nil }
	t.Cleanup(func() { mpvHostFactory = origFactory })

	a := NewApp()
	a.prefs.CookieFile = "cookie.txt"
	a.prefs.OutputDir = t.TempDir()
	err := a.StartPlay(PlayRequest{
		EpisodeID:     "GWep",
		SeriesTitle:   "Show",
		SeasonNumber:  1,
		EpisodeNumber: 1,
	})
	if !called || gotID != "GWep" {
		t.Fatalf("progressive play not used called=%v id=%q", called, gotID)
	}
	// Windows tests have no Wails HWND, so attach may return player-library-missing
	// after the session starts; that is still a successful fallback from glob-miss.
	if err != nil && !libmpvError(err) {
		t.Fatalf("StartPlay: %v", err)
	}
	if err == nil && h.loadPath != playing {
		t.Fatalf("loadPath=%q want %q", h.loadPath, playing)
	}
	if err := a.StopPlay(); err != nil {
		t.Fatal(err)
	}
}

func TestStartPlayProgressiveBlockedWhenDownloading(t *testing.T) {
	stubPlayheadNetwork(t)
	a := NewApp()
	a.prefs.CookieFile = "cookie.txt"
	a.prefs.OutputDir = t.TempDir()
	a.cancel = func() {}
	err := a.StartPlay(PlayRequest{
		EpisodeID:     "GWep",
		SeriesTitle:   "Show",
		SeasonNumber:  1,
		EpisodeNumber: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "download already running") {
		t.Fatalf("err=%v", err)
	}
}

type fakePlaySess struct {
	end, seek float64
	path      string
	audio     string
	closed    bool
}

func (f *fakePlaySess) SeekTarget(sec float64) { f.seek = sec }
func (f *fakePlaySess) BufferedEnd() float64   { return f.end }
func (f *fakePlaySess) Duration() float64      { return 100 }
func (f *fakePlaySess) PlayingPath() string    { return f.path }
func (f *fakePlaySess) AudioPath() string      { return f.audio }
func (f *fakePlaySess) Close() error           { f.closed = true; return nil }

func TestPlaySeekRetargetsWhenAheadOfBuffer(t *testing.T) {
	h := &scriptedHost{pos: 1, dur: 100}
	sess := &fakePlaySess{end: 10}
	a := NewApp()
	a.playHost = h
	a.playSession = sess

	if err := a.PlaySeek(40); err != nil {
		t.Fatal(err)
	}
	if sess.seek != 40 {
		t.Fatalf("SeekTarget=%v want 40", sess.seek)
	}
	if h.pos != 1 {
		t.Fatalf("mpv seeked to %v while past buffer", h.pos)
	}
	if a.playPendingSeek != 40 {
		t.Fatalf("pending=%v want 40", a.playPendingSeek)
	}

	if err := a.PlaySeek(5); err != nil {
		t.Fatal(err)
	}
	if h.pos != 5 {
		t.Fatalf("mpv pos=%v want 5", h.pos)
	}
	if a.playPendingSeek != 0 {
		t.Fatalf("pending=%v want 0 after in-buffer seek", a.playPendingSeek)
	}
}

func TestPlaySeekPendingAppliedWhenBufferCatchesUp(t *testing.T) {
	h := &scriptedHost{pos: 1, dur: 100}
	sess := &fakePlaySess{end: 10}
	a := NewApp()
	a.playHost = h
	a.playSession = sess
	a.playLastPos = 1

	if err := a.PlaySeek(40); err != nil {
		t.Fatal(err)
	}
	if h.pos != 1 {
		t.Fatalf("mpv seeked early to %v", h.pos)
	}

	sess.end = 40
	a.onPlayProgress(engine.PlayProgress{BufferEndSec: 40, DurationSec: 100}, a.playGen)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		a.playMu.Lock()
		pending := a.playPendingSeek
		pos := h.pos
		last := a.playLastPos
		a.playMu.Unlock()
		if pending == 0 && pos == 40 && last == 40 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("pending=%v pos=%v last=%v want seek 40", a.playPendingSeek, h.pos, a.playLastPos)
}

func TestEmitPlayStateAppliesPendingSeek(t *testing.T) {
	h := &scriptedHost{pos: 8, dur: 100}
	sess := &fakePlaySess{end: 10}
	a := NewApp()
	a.playHost = h
	a.playSession = sess
	a.playLastPos = 8

	if err := a.PlaySeek(40); err != nil {
		t.Fatal(err)
	}
	sess.end = 41
	a.emitPlayState(nil, h)
	if h.pos != 40 {
		t.Fatalf("mpv pos=%v want 40 after buffer catch-up", h.pos)
	}
	if a.playPendingSeek != 0 {
		t.Fatalf("pending=%v want 0", a.playPendingSeek)
	}
	if a.playLastPos != 40 {
		t.Fatalf("lastPos=%v want 40", a.playLastPos)
	}
}

func TestOnPlayProgressKeepsLastPosition(t *testing.T) {
	h := &scriptedHost{pos: 12.5, dur: 100, paused: false}
	sess := &fakePlaySess{end: 20}
	a := NewApp()
	a.playHost = h
	a.playSession = sess
	a.playLastPos = 12.5
	a.playPaused = false

	a.onPlayProgress(engine.PlayProgress{BufferEndSec: 24, DurationSec: 100}, a.playGen)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		a.playMu.Lock()
		last := a.playLastPos
		buf := a.playBufferEnd
		a.playMu.Unlock()
		if last == 12.5 && buf == 24 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("lastPos=%v bufferEnd=%v", a.playLastPos, a.playBufferEnd)
}

func TestStopPlayClosesProgressiveSession(t *testing.T) {
	h := &scriptedHost{}
	sess := &fakePlaySess{end: 8}
	a := NewApp()
	a.playHost = h
	a.playGen = 1
	a.playSession = sess
	if err := a.StopPlay(); err != nil {
		t.Fatal(err)
	}
	if !sess.closed {
		t.Fatal("session not closed")
	}
	if a.playSession != nil {
		t.Fatal("playSession still set")
	}
}

type orderHost struct {
	scriptedHost
	order *[]string
}

func (o *orderHost) Destroy() error {
	*o.order = append(*o.order, "destroy")
	return o.scriptedHost.Destroy()
}

type orderSess struct {
	fakePlaySess
	order *[]string
}

func (o *orderSess) Close() error {
	*o.order = append(*o.order, "close")
	return o.fakePlaySess.Close()
}

func TestStopPlayDestroysHostBeforeSessionClose(t *testing.T) {
	var order []string
	h := &orderHost{order: &order}
	sess := &orderSess{order: &order, fakePlaySess: fakePlaySess{end: 8}}
	a := NewApp()
	a.playHost = h
	a.playGen = 1
	a.playSession = sess
	if err := a.StopPlay(); err != nil {
		t.Fatal(err)
	}
	want := "destroy,close"
	got := strings.Join(order, ",")
	if got != want {
		t.Fatalf("order=%q want %q", got, want)
	}
	if !sess.closed {
		t.Fatal("session not closed")
	}
	if h.destroys != 1 {
		t.Fatalf("destroys=%d", h.destroys)
	}
}

type audioHost struct {
	scriptedHost
	mu    sync.Mutex
	added []string
}

func (h *audioHost) AddAudio(path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.added = append(h.added, path)
	return nil
}

func (h *audioHost) addedCopy() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.added))
	copy(out, h.added)
	return out
}

func waitPlay(t *testing.T, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out")
}

func TestOnPlayProgressAttachesLateAudioOnce(t *testing.T) {
	h := &audioHost{}
	sess := &fakePlaySess{end: 8}
	a := NewApp()
	a.playHost = h
	a.playSession = sess
	a.playGen = 4

	a.onPlayProgress(engine.PlayProgress{BufferEndSec: 8, DurationSec: 100}, 4)
	waitPlay(t, time.Second, func() bool {
		a.playMu.Lock()
		defer a.playMu.Unlock()
		return a.playBufferEnd == 8
	})
	if n := len(h.addedCopy()); n != 0 {
		t.Fatalf("AddAudio before path ready: %d", n)
	}

	sess.audio = filepath.Join(t.TempDir(), "audio.mp4")
	a.onPlayProgress(engine.PlayProgress{BufferEndSec: 10, DurationSec: 100}, 4)
	waitPlay(t, time.Second, func() bool { return len(h.addedCopy()) == 1 })
	if got := h.addedCopy(); got[0] != sess.audio {
		t.Fatalf("added %q want %q", got[0], sess.audio)
	}

	a.onPlayProgress(engine.PlayProgress{BufferEndSec: 12, DurationSec: 100}, 4)
	waitPlay(t, time.Second, func() bool {
		a.playMu.Lock()
		defer a.playMu.Unlock()
		return a.playBufferEnd == 12
	})
	if n := len(h.addedCopy()); n != 1 {
		t.Fatalf("AddAudio calls=%d want 1", n)
	}
}

func TestOnPlayProgressIgnoresStaleGen(t *testing.T) {
	h := &audioHost{}
	sess := &fakePlaySess{end: 8, audio: "late.mp4"}
	a := NewApp()
	a.playHost = h
	a.playSession = sess
	a.playGen = 9
	a.playBufferEnd = 3

	a.onPlayProgress(engine.PlayProgress{BufferEndSec: 50, DurationSec: 100}, 8)
	time.Sleep(50 * time.Millisecond)
	a.playMu.Lock()
	buf := a.playBufferEnd
	added := a.playAudioAdded
	a.playMu.Unlock()
	if buf != 3 {
		t.Fatalf("bufferEnd=%v want 3", buf)
	}
	if added || len(h.addedCopy()) != 0 {
		t.Fatal("stale progress attached audio")
	}
}

func TestStartPlayClosesPreviousSession(t *testing.T) {
	old := &fakePlaySess{end: 6}
	a := NewApp()
	a.playHost = &scriptedHost{}
	a.playSession = old
	_ = a.StartPlay(PlayRequest{})
	if !old.closed {
		t.Fatal("previous playSession not closed")
	}
}

func TestShutdownStopsPlay(t *testing.T) {
	h := &scriptedHost{}
	sess := &fakePlaySess{end: 4}
	a := NewApp()
	a.playHost = h
	a.playGen = 1
	a.playSession = sess
	a.shutdown(context.Background())
	if !sess.closed {
		t.Fatal("session not closed on shutdown")
	}
	if h.destroys != 1 {
		t.Fatalf("destroys=%d", h.destroys)
	}
	if a.playHost != nil || a.playSession != nil {
		t.Fatal("play state remains after shutdown")
	}
}

func TestStartPlayNoEpisodeIDStillNoLocalFile(t *testing.T) {
	a := NewApp()
	a.prefs.OutputDir = t.TempDir()
	err := a.StartPlay(PlayRequest{
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
