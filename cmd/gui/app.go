package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"crunchyroll-downloader/internal/engine"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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
			wailsruntime.EventsEmit(parent, "progress", ev)
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

// BuildIndex runs the engine metadata/subtitle index for a series URL
// (fetchSubs=false → catalog only; true → --index-subs). Uses the cookie path
// and language hints from saved preferences. Index tooling is CDM-free.
func (a *App) BuildIndex(url string, fetchSubs bool) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("series URL is required")
	}

	a.mu.Lock()
	prefs := a.prefs
	a.mu.Unlock()

	cookieFile := strings.TrimSpace(prefs.CookieFile)
	if cookieFile == "" {
		return fmt.Errorf("cookie file path is not configured")
	}
	if err := engine.AuthenticateFromCookieFile(cookieFile); err != nil {
		return err
	}

	cfg := runtimeConfigFromPrefs(prefs)
	primaryAudio := "ja-JP"
	if len(prefs.AudioLangs) > 0 && strings.TrimSpace(prefs.AudioLangs[0]) != "" {
		primaryAudio = prefs.AudioLangs[0]
	}
	primarySubs := "en-US"
	if len(prefs.SubtitleLangs) > 0 && strings.TrimSpace(prefs.SubtitleLangs[0]) != "" {
		primarySubs = prefs.SubtitleLangs[0]
	}

	return engine.BuildSeriesIndex(url, primaryAudio, primarySubs, fetchSubs, cfg)
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

// PickCookieFile opens a native file dialog for selecting the etp_rt cookie file.
// Returns an empty string when the user cancels.
func (a *App) PickCookieFile() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app not started")
	}
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select etp_rt cookie file",
	})
}

// PickOutputDir opens a native directory dialog for the download output folder.
// Returns an empty string when the user cancels.
func (a *App) PickOutputDir() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app not started")
	}
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Output folder",
	})
}

// PickDeviceFile opens a native file dialog with the given title (Widevine paths, etc.).
// Returns an empty string when the user cancels.
func (a *App) PickDeviceFile(title string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app not started")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Select file"
	}
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: title,
	})
}

// OpenOutputFolder reveals path in the system file manager.
// Creates the directory when missing. On Windows uses explorer; elsewhere
// falls back to a file:// URL via the Wails runtime.
func (a *App) OpenOutputFolder(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("output path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat output path: %w", err)
		}
		if mkErr := os.MkdirAll(abs, 0o755); mkErr != nil {
			return fmt.Errorf("create output directory %q: %w", abs, mkErr)
		}
		info, err = os.Stat(abs)
		if err != nil {
			return fmt.Errorf("stat created output path: %w", err)
		}
	}
	if !info.IsDir() {
		return fmt.Errorf("output path is not a directory: %s", abs)
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("explorer", abs)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("open explorer: %w", err)
		}
		return nil
	}

	if a.ctx == nil {
		return fmt.Errorf("app not started")
	}
	// file:/// with forward slashes works on macOS/Linux file managers via browser helper.
	url := "file://" + filepath.ToSlash(abs)
	if !strings.HasPrefix(abs, "/") {
		url = "file:///" + filepath.ToSlash(abs)
	}
	wailsruntime.BrowserOpenURL(a.ctx, url)
	return nil
}
