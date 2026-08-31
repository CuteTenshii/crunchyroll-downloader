package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"crunchyroll-downloader/internal/engine"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const missingPlayerMsg = "player library missing"

// MpvHost is the in-window libmpv control surface. Video must draw into the
// attached child HWND (wid); implementations must never spawn a second window.
type MpvHost interface {
	Attach(hwnd uintptr) error
	LoadFile(path string) error
	Pause(p bool) error
	Seek(seconds float64) error
	Position() (float64, error)
	Duration() (float64, error)
	SetVolume(percent int) error
	Destroy() error
}

// PlayStageRect is #play-stage.getBoundingClientRect() in CSS pixels, relative
// to the WebView viewport. Wails fills the window client with WebView2, so
// these are client coordinates. Windows converts to physical pixels with
// GetDpiForWindow (scale = dpi/96).
type PlayStageRect struct {
	X, Y, W, H float64
}

type mpvStateReader interface {
	playFlags() (paused bool, eof bool)
}

// PlayRequest starts an in-window play session for one episode.
type PlayRequest struct {
	EpisodeID     string `json:"episodeId"`
	SeriesTitle   string `json:"seriesTitle"`
	EpisodeTitle  string `json:"episodeTitle"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	FilePath      string `json:"filePath"` // empty = resolve from output dir
	AudioLang     string `json:"audioLang"`
	Locale        string `json:"locale"`
}

type playheadPost struct {
	accountID string
	contentID string
	seconds   float64
	locale    string
	audioLang string
	debounce  *engine.PlayheadDebouncer
}

var mpvHostFactory = newMpvHost

var (
	playAuthenticate     = engine.AuthenticateFromCookieFile
	playGetPlayheads     = engine.GetPlayheads
	playAccountIDFn      = engine.GetAccountID
	playPostPlayhead     = engine.PostPlayhead
	resolvePlayFile      = resolveLocalMKV
	startProgressivePlay = engine.StartProgressivePlay
)

// playBufferSession is the progressive-play control surface used by StartPlay,
// PlaySeek, and StopPlay. *engine.PlaySession implements it.
type playBufferSession interface {
	SeekTarget(sec float64)
	BufferedEnd() float64
	Duration() float64
	PlayingPath() string
	AudioPath() string
	Close() error
}

type mpvAudioAdder interface {
	AddAudio(path string) error
}

func missingPlayerErr() error {
	return fmt.Errorf(missingPlayerMsg)
}

func libmpvError(err error) bool {
	return err != nil && strings.Contains(err.Error(), missingPlayerMsg)
}

type missingMpvHost struct{}

func newMissingMpvHost() MpvHost {
	return missingMpvHost{}
}

func (missingMpvHost) Attach(uintptr) error       { return missingPlayerErr() }
func (missingMpvHost) LoadFile(string) error      { return missingPlayerErr() }
func (missingMpvHost) Pause(bool) error           { return missingPlayerErr() }
func (missingMpvHost) Seek(float64) error         { return missingPlayerErr() }
func (missingMpvHost) Position() (float64, error) { return 0, missingPlayerErr() }
func (missingMpvHost) Duration() (float64, error) { return 0, missingPlayerErr() }
func (missingMpvHost) SetVolume(int) error        { return missingPlayerErr() }
func (missingMpvHost) Destroy() error             { return nil }
func (missingMpvHost) playFlags() (paused, eof bool) {
	return true, false
}

func playRequestNeedsFile(req PlayRequest) bool {
	return strings.TrimSpace(req.EpisodeID) != "" ||
		strings.TrimSpace(req.SeriesTitle) != "" ||
		strings.TrimSpace(req.EpisodeTitle) != "" ||
		req.SeasonNumber != 0 ||
		req.EpisodeNumber != 0
}

func playSeriesDirHint(seriesTitle string) string {
	s := strings.TrimSpace(seriesTitle)
	if s == "" {
		return ""
	}
	repl := strings.NewReplacer(
		`\`, "_", `/`, "_", `:`, "_", `*`, "_", `?`, "_", `"`, "_",
		`<`, "_", `>`, "_", `|`, "_", `'`, "_", "’", "_", "`", "_",
		"“", "_", "”", "_",
	)
	s = repl.Replace(s)
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return strings.TrimRight(s, " .")
}

func mkvMatchesEpisode(name string, season, episode int) bool {
	token := fmt.Sprintf("S%02dE%02d", season, episode)
	return strings.Contains(name, " "+token+" ") || strings.Contains(name, token+" -")
}

func resolveLocalMKV(outputDir, seriesTitle string, season, episode int) (string, error) {
	root := strings.TrimSpace(outputDir)
	if root == "" {
		root = "./Downloads"
	}
	seriesHint := playSeriesDirHint(seriesTitle)

	var matches []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if !strings.EqualFold(filepath.Ext(name), ".mkv") {
			return nil
		}
		if !mkvMatchesEpisode(name, season, episode) {
			return nil
		}
		matches = append(matches, path)
		return nil
	})
	if len(matches) == 0 {
		return "", fmt.Errorf("no local file")
	}
	if seriesHint != "" {
		for _, m := range matches {
			if filepath.Base(filepath.Dir(m)) == seriesHint {
				return m, nil
			}
		}
	}
	return matches[0], nil
}

// StartPlay authenticates, resolves a completed MKV, attaches the in-window
// mpv host, and loads the file. An empty PlayRequest still creates the surface
// (overlay chrome-only / tests). Missing local media returns "no local file".
func (a *App) StartPlay(req PlayRequest) error {
	a.mu.Lock()
	prefs := a.prefs
	a.mu.Unlock()

	episodeID := strings.TrimSpace(req.EpisodeID)
	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = discoverLocale(prefs)
	}
	audio := strings.TrimSpace(req.AudioLang)
	if audio == "" {
		if len(prefs.AudioLangs) > 0 && strings.TrimSpace(prefs.AudioLangs[0]) != "" {
			audio = strings.TrimSpace(prefs.AudioLangs[0])
		} else {
			audio = "ja-JP"
		}
	}

	a.playMu.Lock()
	a.playGen++
	mine := a.playGen
	prev, doPrev := a.queuePlayheadLocked()
	a.playMu.Unlock()

	commitPrev := func() { a.commitPlayhead(prev, doPrev) }
	if a.playAbandonedLocked(mine) {
		commitPrev()
		return nil
	}

	cookie := strings.TrimSpace(prefs.CookieFile)
	if episodeID != "" {
		if cookie == "" {
			commitPrev()
			return fmt.Errorf("cookie file path is not configured")
		}
		if err := playAuthenticate(cookie); err != nil {
			commitPrev()
			return err
		}
	} else if cookie != "" {
		_ = playAuthenticate(cookie)
	}
	if a.playAbandonedLocked(mine) {
		commitPrev()
		return nil
	}

	resumeSeconds := 0.0
	accountID := ""
	if episodeID != "" {
		accountID = strings.TrimSpace(playAccountIDFn())
		if accountID != "" {
			heads, err := playGetPlayheads(accountID, []string{episodeID})
			if err == nil {
				if info, ok := heads[episodeID]; ok {
					resumeSeconds = info.Playhead
				}
			}
		}
	}

	path := strings.TrimSpace(req.FilePath)
	var progSess *engine.PlaySession
	if path == "" {
		if playRequestNeedsFile(req) {
			outDir := strings.TrimSpace(prefs.OutputDir)
			if outDir == "" {
				outDir = "./Downloads"
			}
			found, err := resolvePlayFile(outDir, req.SeriesTitle, req.SeasonNumber, req.EpisodeNumber)
			if err != nil || found == "" {
				if episodeID == "" {
					commitPrev()
					return fmt.Errorf("no local file")
				}
				a.mu.Lock()
				downloading := a.cancel != nil
				a.mu.Unlock()
				if downloading {
					commitPrev()
					return fmt.Errorf("download already running")
				}
				applyWidevineEnvFromPrefs(prefs)
				cfg := runtimeConfigFromPrefs(prefs)
				pctx, pcancel := context.WithCancel(context.Background())
				a.playMu.Lock()
				if a.playAbandoned >= mine {
					a.playMu.Unlock()
					pcancel()
					commitPrev()
					return nil
				}
				a.playSessionCancel = pcancel
				a.playMu.Unlock()

				sess, perr := startProgressivePlay(pctx, episodeID, cfg, func(p engine.PlayProgress) {
					a.onPlayProgress(p)
				})
				if a.playAbandonedLocked(mine) {
					if sess != nil {
						_ = sess.Close()
					}
					pcancel()
					a.playMu.Lock()
					a.playSessionCancel = nil
					a.playMu.Unlock()
					commitPrev()
					return nil
				}
				if perr != nil {
					pcancel()
					a.playMu.Lock()
					if a.playSessionCancel != nil {
						a.playSessionCancel = nil
					}
					a.playMu.Unlock()
					commitPrev()
					return perr
				}
				progSess = sess
				path = sess.PlayingPath()
			} else {
				path = found
			}
		}
	} else {
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			commitPrev()
			return fmt.Errorf("no local file")
		}
	}

	a.playMu.Lock()
	if a.playAbandoned >= mine {
		a.playMu.Unlock()
		if progSess != nil {
			_ = progSess.Close()
		}
		commitPrev()
		return nil
	}

	if progSess != nil {
		a.playSession = progSess
		a.playBufferEnd = progSess.BufferedEnd()
		a.playDurationHint = progSess.Duration()
	}

	if a.playHost == nil {
		host, err := mpvHostFactory()
		if err != nil {
			_ = a.clearPlayLocked()
			a.playMu.Unlock()
			commitPrev()
			return missingPlayerErr()
		}
		a.playHost = host
	}

	hwnd, err := a.ensurePlaySurfaceLocked()
	if err != nil {
		_ = a.clearPlayLocked()
		a.playMu.Unlock()
		commitPrev()
		return missingPlayerErr()
	}
	if err := a.playHost.Attach(hwnd); err != nil {
		_ = a.clearPlayLocked()
		a.playMu.Unlock()
		commitPrev()
		if libmpvError(err) {
			return err
		}
		return missingPlayerErr()
	}
	if path != "" {
		if err := a.playHost.LoadFile(path); err != nil {
			_ = a.clearPlayLocked()
			a.playMu.Unlock()
			commitPrev()
			return err
		}
		a.playPaused = false
		if adder, ok := a.playHost.(mpvAudioAdder); ok {
			if a.playSession != nil {
				if audio := a.playSession.AudioPath(); audio != "" {
					_ = adder.AddAudio(audio)
				}
			}
		}
	}
	a.playEpisodeID = episodeID
	a.playLocale = locale
	a.playAudioLang = audio
	a.playAccountID = accountID
	a.playEOFPosted = false
	if a.playDebounce == nil {
		a.playDebounce = engine.NewPlayheadDebouncer(time.Second)
	}
	a.startPlayTickerLocked()
	ctx := a.ctx
	a.playMu.Unlock()

	commitPrev()
	if ctx != nil {
		wailsruntime.EventsEmit(ctx, "play-ready", map[string]any{
			"resumeSeconds": resumeSeconds,
		})
	}
	return nil
}

func (a *App) playAbandonedLocked(mine uint64) bool {
	a.playMu.Lock()
	ok := a.playAbandoned >= mine
	a.playMu.Unlock()
	return ok
}

// StopPlay tears down the host, child HWND, and play-state ticker for the
// generation captured at call start. A later StartPlay bumps playGen so a
// delayed StopPlay cannot destroy the new session.
func (a *App) StopPlay() error {
	a.playMu.Lock()
	a.playAbandoned = a.playGen
	gen := a.playGen
	a.playMu.Unlock()
	return a.clearPlayIfGen(gen)
}

func (a *App) clearPlayIfGen(gen uint64) error {
	a.playMu.Lock()
	if a.playGen != gen {
		a.playMu.Unlock()
		return nil
	}
	shot, doPost := a.queuePlayheadLocked()
	err := a.clearPlayLocked()
	a.playMu.Unlock()
	a.commitPlayhead(shot, doPost)
	return err
}

// PlayPause toggles the pause flag on the current host.
func (a *App) PlayPause() error {
	a.playMu.Lock()
	if a.playHost == nil {
		a.playMu.Unlock()
		return missingPlayerErr()
	}
	next := !a.playPaused
	if err := a.playHost.Pause(next); err != nil {
		a.playMu.Unlock()
		return err
	}
	a.playPaused = next
	var shot playheadPost
	doPost := false
	if next {
		shot, doPost = a.queuePlayheadLocked()
	}
	a.playMu.Unlock()
	if doPost {
		return a.commitPlayhead(shot, true)
	}
	return nil
}

// PlaySeek seeks the current file to seconds (absolute).
// When a progressive session is active and sec is past the contiguous buffer,
// workers are retargeted and mpv is not seeked until the prefix catches up.
func (a *App) PlaySeek(seconds float64) error {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	if a.playHost == nil {
		return missingPlayerErr()
	}
	if a.playSession != nil && seconds > a.playSession.BufferedEnd() {
		a.playSession.SeekTarget(seconds)
		return nil
	}
	return a.playHost.Seek(seconds)
}

// PlaySetVolume sets mpv volume to pct (0–100).
func (a *App) PlaySetVolume(pct int) error {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	if a.playHost == nil {
		return missingPlayerErr()
	}
	return a.playHost.SetVolume(pct)
}

// PlayLayout moves the child HWND to the CSS-pixel play-stage rect.
func (a *App) PlayLayout(x, y, w, h float64) error {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	a.playRect = PlayStageRect{X: x, Y: y, W: w, H: h}
	if a.playChild == 0 {
		return nil
	}
	return a.movePlaySurfaceLocked()
}

func (a *App) clearPlayLocked() error {
	a.stopPlayTickerLocked()
	if a.playSessionCancel != nil {
		a.playSessionCancel()
		a.playSessionCancel = nil
	}
	if a.playSession != nil {
		_ = a.playSession.Close()
		a.playSession = nil
	}
	a.playBufferEnd = 0
	a.playDurationHint = 0
	var err error
	if a.playHost != nil {
		_ = a.playHost.Pause(true)
		err = a.playHost.Destroy()
		a.playHost = nil
	}
	if derr := a.destroyPlaySurfaceLocked(); derr != nil && err == nil {
		err = derr
	}
	a.playPaused = true
	a.playEpisodeID = ""
	a.playLocale = ""
	a.playAudioLang = ""
	a.playAccountID = ""
	a.playEOFPosted = false
	return err
}

func (a *App) startPlayTickerLocked() {
	a.stopPlayTickerLocked()
	ctx, cancel := context.WithCancel(context.Background())
	a.playCancel = cancel
	wailsCtx := a.ctx
	host := a.playHost
	go func() {
		tick := time.NewTicker(250 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				a.emitPlayState(wailsCtx, host)
			}
		}
	}()
}

func (a *App) stopPlayTickerLocked() {
	if a.playCancel != nil {
		a.playCancel()
		a.playCancel = nil
	}
}

func (a *App) emitPlayState(wailsCtx context.Context, host MpvHost) {
	if host == nil {
		return
	}
	a.playMu.Lock()
	if a.playHost != host {
		a.playMu.Unlock()
		return
	}
	pos, _ := host.Position()
	dur, _ := host.Duration()
	paused := a.playPaused
	eof := false
	if r, ok := host.(mpvStateReader); ok {
		paused, eof = r.playFlags()
		a.playPaused = paused
	}
	shot, doPost := playheadPost{}, false
	if eof && !a.playEOFPosted {
		shot, doPost = a.queuePlayheadLocked()
		a.playEOFPosted = true
	}
	if a.playDurationHint > dur {
		dur = a.playDurationHint
	}
	bufEnd := a.playBufferEnd
	if a.playSession == nil {
		bufEnd = dur
	}
	payload := map[string]any{
		"position":  pos,
		"duration":  dur,
		"paused":    paused,
		"eof":       eof,
		"bufferEnd": bufEnd,
	}
	a.playMu.Unlock()

	if wailsCtx != nil {
		wailsruntime.EventsEmit(wailsCtx, "play-state", payload)
	}
	if doPost {
		if err := a.commitPlayhead(shot, true); err != nil {
			a.playMu.Lock()
			if a.playHost == host {
				a.playEOFPosted = false
			}
			a.playMu.Unlock()
		}
	}
}

func (a *App) queuePlayheadLocked() (playheadPost, bool) {
	id := strings.TrimSpace(a.playEpisodeID)
	if id == "" {
		return playheadPost{}, false
	}
	account := strings.TrimSpace(a.playAccountID)
	if account == "" {
		account = strings.TrimSpace(playAccountIDFn())
	}
	if account == "" {
		return playheadPost{}, false
	}
	pos, dur := 0.0, 0.0
	eof := false
	if a.playHost != nil {
		pos, _ = a.playHost.Position()
		dur, _ = a.playHost.Duration()
		if r, ok := a.playHost.(mpvStateReader); ok {
			_, eof = r.playFlags()
		}
	}
	if eof || engine.IsPlayFinished(pos, dur) {
		pos = engine.FinishPlayheadSeconds(dur)
	}
	if a.playDebounce == nil {
		a.playDebounce = engine.NewPlayheadDebouncer(time.Second)
	}
	if !a.playDebounce.ShouldPost(id, pos) {
		return playheadPost{}, false
	}
	return playheadPost{
		accountID: account,
		contentID: id,
		seconds:   pos,
		locale:    a.playLocale,
		audioLang: a.playAudioLang,
		debounce:  a.playDebounce,
	}, true
}

func (a *App) commitPlayhead(shot playheadPost, ok bool) error {
	if !ok || shot.contentID == "" || shot.accountID == "" {
		return nil
	}
	err := playPostPlayhead(shot.accountID, shot.contentID, shot.seconds, shot.locale, shot.audioLang)
	if err != nil {
		a.logPlayheadErr(err)
		return err
	}
	if shot.debounce != nil {
		shot.debounce.MarkPosted(shot.contentID, shot.seconds)
	}
	return nil
}

func (a *App) onPlayProgress(p engine.PlayProgress) {
	// Emit off the download worker so we cannot deadlock with StartPlay's playMu.
	go func() {
		a.playMu.Lock()
		a.playBufferEnd = p.BufferEndSec
		if p.DurationSec > a.playDurationHint {
			a.playDurationHint = p.DurationSec
		}
		ctx := a.ctx
		paused := a.playPaused
		a.playMu.Unlock()
		if ctx == nil {
			return
		}
		payload := map[string]any{
			"bufferEnd": p.BufferEndSec,
			"duration":  p.DurationSec,
			"paused":    paused,
			"eof":       false,
		}
		if p.Err != nil {
			payload["error"] = p.Err.Error()
		}
		wailsruntime.EventsEmit(ctx, "play-state", payload)
	}()
}

func (a *App) logPlayheadErr(err error) {
	if err == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "progress", engine.ProgressEvent{
		Phase:    engine.PhaseIdle,
		Message:  "playhead POST failed: " + err.Error(),
		Level:    "warn",
		Fraction: -1,
	})
}
