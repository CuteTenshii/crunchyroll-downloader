package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"crunchyroll-downloader/internal/engine"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound application shell: preferences, Inspect, and download jobs.
type App struct {
	ctx    context.Context
	cancel context.CancelFunc // job cancel only; nil when no download is running
	mu     sync.Mutex
	prefs  engine.Preferences
}

// NewApp constructs an empty App; prefs load on startup.
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	// Keep the Wails context for EventsEmit; job cancel is stored separately on a.cancel.
	a.ctx = ctx
	path, err := engine.DefaultPreferencesPath()
	if err != nil {
		return
	}
	prefs, err := engine.LoadPreferences(path)
	if err != nil {
		return
	}
	a.prefs = prefs
}

// GetPreferences returns the in-memory preferences snapshot.
func (a *App) GetPreferences() engine.Preferences {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.prefs
}

// SavePreferences persists p to the default preferences path and updates memory.
func (a *App) SavePreferences(p engine.Preferences) error {
	a.mu.Lock()
	a.prefs = p
	a.mu.Unlock()
	path, err := engine.DefaultPreferencesPath()
	if err != nil {
		return err
	}
	return engine.SavePreferences(path, p)
}

// Inspect probes a Crunchyroll URL for catalog/quality metadata via the engine.
func (a *App) Inspect(req engine.InspectRequest) (engine.InspectResult, error) {
	a.mu.Lock()
	prefs := a.prefs
	a.mu.Unlock()
	cfg := runtimeConfigFromPrefs(prefs)
	return engine.Inspect(req, cfg)
}

// StartDownload begins a multi-episode download job in a background goroutine.
// Progress is pushed as "progress" runtime events. Only one job may run at a time.
func (a *App) StartDownload(job engine.DownloadJob) error {
	a.mu.Lock()
	if a.cancel != nil {
		a.mu.Unlock()
		return fmt.Errorf("job already running")
	}
	if a.ctx == nil {
		a.mu.Unlock()
		return fmt.Errorf("app not started")
	}
	if len(job.EpisodeIDs) == 0 {
		a.mu.Unlock()
		return fmt.Errorf("no episodes in queue")
	}
	prefs := a.prefs
	parent := a.ctx
	ctx, cancel := context.WithCancel(parent)
	a.cancel = cancel
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.cancel = nil
			a.mu.Unlock()
		}()

		emit := func(ev engine.ProgressEvent) {
			runtime.EventsEmit(parent, "progress", ev)
		}

		// Authenticate from prefs cookie path (secrets stay on disk).
		cookieFile := strings.TrimSpace(prefs.CookieFile)
		if cookieFile == "" {
			emit(engine.ProgressEvent{
				Phase:    engine.PhaseDone,
				Message:  "cookie file path is not configured",
				Level:    "error",
				Fraction: -1,
			})
			return
		}
		if err := engine.AuthenticateFromCookieFile(cookieFile); err != nil {
			emit(engine.ProgressEvent{
				Phase:    engine.PhaseDone,
				Message:  err.Error(),
				Level:    "error",
				Fraction: -1,
			})
			return
		}

		applyWidevineEnvFromPrefs(prefs)
		cfg := runtimeConfigFromPrefs(prefs)

		err := engine.RunDownloadJob(ctx, job, cfg, emit)
		if err != nil {
			// RunDownloadJob may already have emitted per-episode errors; always
			// surface a terminal PhaseDone so the UI can leave the running state.
			level := "error"
			msg := err.Error()
			if ctx.Err() != nil {
				level = "warn"
				msg = "cancelled"
			}
			emit(engine.ProgressEvent{
				Phase:    engine.PhaseDone,
				Message:  msg,
				Level:    level,
				Fraction: -1,
			})
			return
		}
		// Success path already emits PhaseDone from RunDownloadJob; send a
		// concise UI terminal event as well (idempotent for the frontend).
		emit(engine.ProgressEvent{
			Phase:    engine.PhaseDone,
			Message:  "All done",
			Level:    "ok",
			Fraction: 1,
		})
	}()

	return nil
}

// CancelDownload requests cancellation of the active download job, if any.
func (a *App) CancelDownload() {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// runtimeConfigFromPrefs maps advanced preference fields onto RuntimeConfig.
func runtimeConfigFromPrefs(p engine.Preferences) engine.RuntimeConfig {
	cfg := engine.DefaultRuntimeConfig()
	if p.VideoQuality != "" {
		cfg.VideoQuality = p.VideoQuality
	}
	if p.AudioQuality != "" {
		cfg.AudioQuality = p.AudioQuality
	}
	if p.DebugManifest {
		cfg.DebugManifest = true
	}
	if p.Playback4294Retries > 0 {
		cfg.Playback4294Retries = p.Playback4294Retries
	}
	if p.Playback4294BackoffSec > 0 {
		cfg.Playback4294Backoff = time.Duration(p.Playback4294BackoffSec) * time.Second
	}
	if p.IndexWindow > 0 {
		cfg.IndexWindow = p.IndexWindow
	}
	if p.IndexCircuitLimit > 0 {
		cfg.IndexCircuitLimit = p.IndexCircuitLimit
	}
	return cfg
}

// applyWidevineEnvFromPrefs sets CRUNCHYROLL_WIDEVINE_* from preference paths when set.
// Empty paths leave any existing process environment untouched.
func applyWidevineEnvFromPrefs(p engine.Preferences) {
	if v := strings.TrimSpace(p.WVDPath); v != "" {
		_ = os.Setenv("CRUNCHYROLL_WIDEVINE_DEVICE_FILE", v)
	}
	if v := strings.TrimSpace(p.ClientIDPath); v != "" {
		_ = os.Setenv("CRUNCHYROLL_WIDEVINE_CLIENT_ID_FILE", v)
	}
	if v := strings.TrimSpace(p.PrivateKeyPath); v != "" {
		_ = os.Setenv("CRUNCHYROLL_WIDEVINE_PRIVATE_KEY_FILE", v)
	}
}
