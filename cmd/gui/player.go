package main

import (
	"context"
	"fmt"
	"strings"
	"time"

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

var mpvHostFactory = newMpvHost

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

// StartPlay constructs the in-window mpv host if needed, attaches it to the
// child HWND covering #play-stage, and optionally loads path. An empty path
// still creates the surface (overlay chrome-only / later load).
func (a *App) StartPlay(path string) error {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	a.playGen++

	if a.playHost == nil {
		host, err := mpvHostFactory()
		if err != nil {
			return missingPlayerErr()
		}
		a.playHost = host
	}

	hwnd, err := a.ensurePlaySurfaceLocked()
	if err != nil {
		a.clearPlayLocked()
		return missingPlayerErr()
	}
	if err := a.playHost.Attach(hwnd); err != nil {
		a.clearPlayLocked()
		if libmpvError(err) {
			return err
		}
		return missingPlayerErr()
	}
	if path != "" {
		if err := a.playHost.LoadFile(path); err != nil {
			return err
		}
		a.playPaused = false
	}
	a.startPlayTickerLocked()
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
	defer a.playMu.Unlock()
	if a.playGen != gen {
		return nil
	}
	return a.clearPlayLocked()
}

// PlayPause toggles the pause flag on the current host.
func (a *App) PlayPause() error {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	if a.playHost == nil {
		return missingPlayerErr()
	}
	next := !a.playPaused
	if err := a.playHost.Pause(next); err != nil {
		return err
	}
	a.playPaused = next
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
	if wailsCtx == nil || host == nil {
		return
	}
	a.playMu.Lock()
	defer a.playMu.Unlock()
	if a.playHost != host {
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
	wailsruntime.EventsEmit(wailsCtx, "play-state", map[string]any{
		"position": pos,
		"duration": dur,
		"paused":   paused,
		"eof":      eof,
	})
}
