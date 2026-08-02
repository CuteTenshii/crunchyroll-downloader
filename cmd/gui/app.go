package main

import (
	"context"
	"time"

	"crunchyroll-downloader/internal/engine"
)

// App is the Wails-bound application shell. Task 5 exposes preferences and
// Inspect only; full download/job UI lands in later tasks.
type App struct {
	ctx    context.Context
	cancel context.CancelFunc
	prefs  engine.Preferences
}

// NewApp constructs an empty App; prefs load on startup.
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
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
	return a.prefs
}

// SavePreferences persists p to the default preferences path and updates memory.
func (a *App) SavePreferences(p engine.Preferences) error {
	a.prefs = p
	path, err := engine.DefaultPreferencesPath()
	if err != nil {
		return err
	}
	return engine.SavePreferences(path, p)
}

// Inspect probes a Crunchyroll URL for catalog/quality metadata via the engine.
func (a *App) Inspect(req engine.InspectRequest) (engine.InspectResult, error) {
	cfg := runtimeConfigFromPrefs(a.prefs)
	return engine.Inspect(req, cfg)
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
