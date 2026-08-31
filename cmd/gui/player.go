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
}

var mpvHostFactory = newMpvHost

var (
	playAuthenticate = engine.AuthenticateFromCookieFile
	playGetPlayheads = engine.GetPlayheads
	playAccountIDFn  = engine.GetAccountID
	playPostPlayhead = engine.PostPlayhead
)

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

func resolveLocalMKV(outputDir, seriesTitle string, season, episode int) (string, error) {
	root := strings.TrimSpace(outputDir)
	if root == "" {
		root = "./Downloads"
	}
	needle := fmt.Sprintf("S%02dE%02d", season, episode)
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
		if !strings.Contains(name, needle) {
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
			if strings.Contains(filepath.Base(m), seriesHint) || strings.Contains(m, seriesHint) {
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

	cookie := strings.TrimSpace(prefs.CookieFile)
	if episodeID != "" {
		if cookie == "" {
			return fmt.Errorf("cookie file path is not configured")
		}
		if err := playAuthenticate(cookie); err != nil {
			return err
		}
	} else if cookie != "" {
		_ = playAuthenticate(cookie)
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
	if path == "" {
		if playRequestNeedsFile(req) {
			outDir := strings.TrimSpace(prefs.OutputDir)
			if outDir == "" {
				outDir = "./Downloads"
			}
			found, err := resolveLocalMKV(outDir, req.SeriesTitle, req.SeasonNumber, req.EpisodeNumber)
			if err != nil || found == "" {
				return fmt.Errorf("no local file")
			}
			path = found
		}
	} else {
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			return fmt.Errorf("no local file")
		}
	}

	a.playMu.Lock()
	prev, doPrev := a.queuePlayheadLocked()
	a.playGen++

	if a.playHost == nil {
		host, err := mpvHostFactory()
		if err != nil {
			a.playMu.Unlock()
			a.commitPlayhead(prev, doPrev)
			return missingPlayerErr()
		}
		a.playHost = host
	}

	hwnd, err := a.ensurePlaySurfaceLocked()
	if err != nil {
		_ = a.clearPlayLocked()
		a.playMu.Unlock()
		a.commitPlayhead(prev, doPrev)
		return missingPlayerErr()
	}
	if err := a.playHost.Attach(hwnd); err != nil {
		_ = a.clearPlayLocked()
		a.playMu.Unlock()
		a.commitPlayhead(prev, doPrev)
		if libmpvError(err) {
			return err
		}
		return missingPlayerErr()
	}
	if path != "" {
		if err := a.playHost.LoadFile(path); err != nil {
			a.playMu.Unlock()
			a.commitPlayhead(prev, doPrev)
			return err
		}
		a.playPaused = false
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

	a.commitPlayhead(prev, doPrev)
	if ctx != nil {
		wailsruntime.EventsEmit(ctx, "play-ready", map[string]any{
			"resumeSeconds": resumeSeconds,
		})
	}
	return nil
}

// StopPlay tears down the host, child HWND, and play-state ticker for the
// generation captured at call start. A later StartPlay bumps playGen so a
// delayed StopPlay cannot destroy the new session.
func (a *App) StopPlay() error {
	a.playMu.Lock()
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
func (a *App) PlaySeek(seconds float64) error {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	if a.playHost == nil {
		return missingPlayerErr()
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
	payload := map[string]any{
		"position": pos,
		"duration": dur,
		"paused":   paused,
		"eof":      eof,
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
	}, true
}

func (a *App) commitPlayhead(shot playheadPost, ok bool) error {
	if !ok || shot.contentID == "" || shot.accountID == "" {
		return nil
	}
	err := playPostPlayhead(shot.accountID, shot.contentID, shot.seconds, shot.locale, shot.audioLang)
	if err != nil {
		a.logPlayheadErr(err)
	}
	return err
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
