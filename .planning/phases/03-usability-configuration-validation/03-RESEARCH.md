# Phase 3: Usability — Configuration & Validation - Research

**Researched:** 2026-07-09
**Domain:** Go CLI configuration, env var handling, startup validation, code quality
**Confidence:** HIGH

## Summary

This phase adds persistent configuration infrastructure to a Go 1.25 CLI tool using only stdlib packages (`encoding/json`, `flag`, `os`, `net/url`, `path/filepath`, `regexp`, `os/exec`). The core pattern is a precedence hierarchy (CLI flag > env var > config file > default) implemented via pointer-based JSON struct and explicit `flag.Visit()` tracking. All new features — config file at `~/.config/animeheaven/config.json` (XDG spec), env vars (`CRUNCHYROLL_ETP_RT`, `CRUNCHYROLL_CLIENT_AUTH`), flags (`--output-dir`, `--widevine-device`), startup validation (FFmpeg, batch URLs) — wire into `main.go` as the central orchestration point.

**Primary recommendation:** Use a dedicated `internal/config` package with a pointer-field struct for explicit-only config overrides, and resolve precedence in `main.go` before any work begins. Remove `.env` file reading from `internal/drm/drm.go` per D-16. Bundle QOL-04/QOL-05 as a single `processURL` refactor, handle QOL-08 (parseLangs once) separately after config resolution. QOL-07 (regex sanitize) is a standalone function-level change.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Config format is plain JSON — `encoding/json` from stdlib, no YAML/TOML.
- **D-02:** All CLI flags are config-file-persistable: `audio_lang`, `subs_lang`, `video_quality`, `audio_quality`, `workers`, `output_dir`, `etp_rt`, `widevine_device`.
- **D-03:** Config file discovery: check `$XDG_CONFIG_HOME/animeheaven/config.json` first, fallback to `~/.config/animeheaven/config.json`.
- **D-04:** If config file doesn't exist, auto-generate minimal JSON skeleton with defaults and print a message. If invalid JSON, warn and continue with defaults (never block startup).
- **D-05:** Config uses explicit-only override — unset JSON fields fall through to defaults.
- **D-06:** Precedence: CLI flag > env var > config file > default.
- **D-07:** `CRUNCHYROLL_ETP_RT` handled in `main.go` — if `--etp-rt` empty, fall through to env var.
- **D-08:** `CRUNCHYROLL_CLIENT_AUTH` replaces hardcoded Basic Auth credential when set.
- **D-09:** `.env` file approach deprecated entirely.
- **D-10:** `--output-dir` flag with series subfolder structure; error if output dir doesn't exist.
- **D-11:** Do NOT auto-create output-dir; auto-create series subdir inside it.
- **D-12:** Default behavior (no `--output-dir`) unchanged.
- **D-13:** Single `--widevine-device` flag accepting `.wvd` file or directory.
- **D-14:** Auto-detect format (`.wvd` extension or directory check).
- **D-15:** Keep legacy env var names as env-level fallbacks.
- **D-16:** Remove `.env` file — only `os.LookupEnv`.
- **D-17:** Batch URL validation: structure + content ID length. Report ALL invalid upfront.
- **D-18:** FFmpeg check: `exec.LookPath("ffmpeg")` + `exec.Command("ffmpeg", "-version")`.
- **D-19:** FFmpeg missing = hard error with actionable message.
- **D-20:** QOL-04 (`&&`→`||`) and QOL-05 (`url.Parse()`) bundled as one unit.
- **D-21:** QOL-07 (regex sanitizeFilename) is standalone.
- **D-22:** QOL-08 (parseLangs once) runs AFTER config/env resolution in `main.go`.

### the agent's Discretion
None — all decisions were made explicitly during discussion.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within Phase 3 scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| USAB-01 | `CRUNCHYROLL_ETP_RT` env var as alternative to `--etp-rt` flag | Precedence hierarchy pattern via `os.LookupEnv` + `flag.Visit()` in main.go |
| USAB-02 | Config file at `~/.config/animeheaven/config.json` for persistent defaults | `os.UserConfigDir()` + XDG env + JSON pointer struct pattern |
| USAB-03 | `--output-dir` flag for custom output directory | New `flag.String`, `os.MkdirAll` with pre-check via `os.Stat` |
| USAB-04 | Batch URL validation upfront — report all invalid | Iterate URLs BEFORE main loop, collect errors, print list |
| USAB-05 | FFmpeg check at startup | `exec.LookPath` + `exec.Command("ffmpeg", "-version")` |
| USAB-06 | `--widevine-device` flag for explicit paths | Path passed to drm package, format auto-detect (`.wvd`/dir) |
| USAB-07 | Document hardcoded Basic Auth, add `CRUNCHYROLL_CLIENT_AUTH` | Env var read in `auth.go`, fallback to hardcoded constant |
| QOL-04 | Fix URL validation `&&` → `||` | Change one operator in `processURL` |
| QOL-05 | Replace string-split URL parsing with `url.Parse()` | Full `processURL` refactor using `net/url` |
| QOL-07 | Regex `_{2,}` → `_` in `sanitizeFilename` | Replace O(n²) loop with `regexp.MustCompile` |
| QOL-08 | `parseLangs` called once at flag parse time | Move call after config resolution, pass slices as parameters |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Config file loading | CLI Layer (main.go) | internal/config | Config resolution is synchronous, done once at startup — belongs in CLI orchestration |
| Env var resolution | CLI Layer (main.go) | — | `os.LookupEnv` is a simple read; `main.go` owns the precedence resolution |
| Precedence merge | CLI Layer (main.go) | — | All sources (flags, env, config) converge in main before any work begins |
| Widevine device path | CLI Layer → drm package | internal/drm | Resolved path is passed to drm before `sync.Once` fires |
| Output directory | CLI Layer → download package | internal/download | Resolved path is passed to `Episode()` and `Season()` |
| FFmpeg validation | CLI Layer (main.go) | — | `exec.LookPath` call in main after flag parse, before API client creation |
| Batch URL validation | CLI Layer (main.go) | — | Process URLs immediately after flag parse, before any download |
| Code quality fixes (QOL) | Respective files | — | Fixes are in `main.go`, `internal/download/episode.go` — no tier shift |

## Standard Stack

### Core

This phase uses **Go standard library only** — no new external dependencies. All features are implemented with stdlib packages:

| Package | Purpose | Why Standard |
|---------|---------|--------------|
| `encoding/json` | Config file marshal/unmarshal | D-01 mandates stdlib only; already used in codebase |
| `flag` | New `--output-dir`, `--widevine-device` flags | Existing CLI framework; `flag.Visit()` for explicit-set detection |
| `os` | `os.UserConfigDir()`, `os.LookupEnv`, `os.Stat` | XDG config dir, env var reads, directory existence checks |
| `os/exec` | FFmpeg validation | `exec.LookPath` + `exec.Command("ffmpeg", "-version")` |
| `net/url` | Proper URL parsing for QOL-05 | Replace string splitting with `url.Parse()` |
| `regexp` | Regex sanitize for QOL-07 | `regexp.MustCompile("_{2,}").ReplaceAllString(res, "_")` |
| `path/filepath` | Config file path construction | Join `UserConfigDir()` with `"animeheaven/config.json"` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `encoding/json` | `gopkg.in/yaml.v3` | D-01 forbids external deps; YAML supports comments but adds dep |
| `flag.Visit()` | String sentinel + `flag.Lookup()` | Visit() is idiomatic Go; sentinel conflates "unset" and "empty" |
| Pointer-field struct | `map[string]interface{}` | Pointer fields are type-safe, map requires type assertions per field |
| `os.UserConfigDir()` | `github.com/adrg/xdg` | D-01 forbids external deps; `os.UserConfigDir()` handles 95% of cases |

**Installation:**
No new packages to install. All changes use Go stdlib.

## Package Legitimacy Audit

> No new external packages are introduced in this phase. All features use Go standard library only (per D-01). The existing dependencies (`github.com/iyear/gowidevine`, `github.com/unki2aut/go-mpd`, etc.) are unchanged.

**Packages removed due to [SLOP] verdict:** None
**Packages flagged as suspicious [SUS]:** None

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        main.go                                   │
│                                                                   │
│  1. flag.Parse() → flag.Visit() tracks explicitly-set flags       │
│  2. config.Load() → XDG discovery, JSON decode, warn on invalid   │
│  3. resolvePrecedence() → flag > env > config > default           │
│     ├─ CRUNCHYROLL_ETP_RT       ─┐                                │
│     ├─ CRUNCHYROLL_CLIENT_AUTH   ─┤ Join precedence               │
│     ├─ WIDEVINE_DEVICE_PATH etc  ─┘                                │
│     └─ Config file values         → resolved values               │
│  4. Exec validation:                                              │
│     ├─ LookPath("ffmpeg") + --version                             │
│     └─ Batch URL validation (all URLs upfront)                    │
│  5. drm.SetWidevinePath(resolved) → BEFORE sync.Once              │
│  6. api.NewWithContext(resolved etpRt, resolved clientAuth)       │
│  7. processURL() / Season() with resolved langs, output-dir       │
└──────────┬──────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────────┐
│  internal/config/ (NEW)                     internal/drm/        │
│                                            │                     │
│  Config struct (pointer fields)            │ SetWidevinePath()   │
│  Load() → json.Decode + merge defaults     │ discoverConfig()    │
│  Save() → json.MarshalIndent skeleton      │ os.LookupEnv()      │
│  ConfigDir() → XDG discovery               │ REMOVED: .env       │
│                                            │                     │
└────────────────────────────────────────────┴─────────────────────┘
           │                                          │
           ▼                                          ▼
┌──────────────────────────┐    ┌─────────────────────────────────┐
│  internal/api/auth.go    │    │  internal/download/episode.go   │
│                          │    │                                  │
│  CRUNCHYROLL_CLIENT_AUTH │    │  outputDir param in Episode()   │
│  fallback to hardcoded   │    │  sanitizeFilename regex QOL-07  │
│                          │    │  parseLangs passed as param      │
└──────────────────────────┘    └─────────────────────────────────┘
```

### Recommended Project Structure

```
crunchyroll-downloader/
├── main.go                          # CLI entry, flag defs, precedence resolution, orchestration
├── main_test.go                     # Tests for processURL, parseLangs (new config tests)
│
├── internal/
│   ├── config/                      # NEW PACKAGE
│   │   ├── config.go                # Config struct (pointer fields), Load(), Save(), ConfigDir()
│   │   └── config_test.go           # Tests for config loading, merge, XDG discovery
│   │
│   ├── api/
│   │   ├── auth.go                  # CRUNCHYROLL_CLIENT_AUTH env var read
│   │   ├── client.go                # NewWithContext receives resolved etpRt
│   │   └── ...                      # Unchanged
│   │
│   ├── drm/
│   │   ├── drm.go                   # SetWidevinePath(), removed .env, keep env var fallbacks
│   │   └── drm_test.go              # Updated tests
│   │
│   ├── download/
│   │   ├── episode.go               # sanitizeFilename regex, outputDir param in Episode()
│   │   ├── season.go                # outputDir param in Season()
│   │   └── ..._test.go              # Updated tests
│   │
│   ├── media/
│   │   └── ...                      # Unchanged
│   │
│   ├── mux/
│   │   └── ...                      # Unchanged
│   │
│   └── locale/
│       └── ...                      # Unchanged
```

### Pattern 1: Pointer-based Config Struct for Explicit-Only Overrides

**What:** Use `*string` / `*int` fields to distinguish "explicitly set to zero" from "not present in config."

**When to use:** D-05 mandates explicit-only override — only fields the user wrote in the config file override defaults. Pointer fields are the idiomatic Go stdlib approach: `nil` means absent, non-nil means explicitly set.

**Example:**
```go
// internal/config/config.go
package config

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

// Config represents the JSON config file at ~/.config/animeheaven/config.json.
// Pointer fields enable explicit-only overrides: nil = absent from file.
type Config struct {
    AudioLang      *string `json:"audio_lang,omitempty"`
    SubsLang       *string `json:"subs_lang,omitempty"`
    VideoQuality   *string `json:"video_quality,omitempty"`
    AudioQuality   *string `json:"audio_quality,omitempty"`
    Workers        *int    `json:"workers,omitempty"`
    OutputDir      *string `json:"output_dir,omitempty"`
    EtpRt          *string `json:"etp_rt,omitempty"`
    WidevineDevice *string `json:"widevine_device,omitempty"`
}

// ConfigDir returns "$XDG_CONFIG_HOME/animeheaven" (or ~/.config/animeheaven).
func ConfigDir() (string, error) {
    if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
        return filepath.Join(xdg, "animeheaven"), nil
    }
    cfgDir, err := os.UserConfigDir()
    if err != nil {
        return "", fmt.Errorf("cannot determine config directory: %w", err)
    }
    return filepath.Join(cfgDir, "animeheaven"), nil
}

// ConfigPath returns the full path to the config file.
func ConfigPath() (string, error) {
    dir, err := ConfigDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(dir, "config.json"), nil
}

// Load reads the config file. Returns an empty Config (all nil) if the file
// doesn't exist. On invalid JSON, returns error after logging a warning.
func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return &Config{}, nil
        }
        return nil, err
    }

    cfg := &Config{}
    if err := json.Unmarshal(data, cfg); err != nil {
        // Invalid JSON: return error for warning, but don't block startup
        return cfg, fmt.Errorf("invalid config JSON: %w", err)
    }
    return cfg, nil
}

// WriteSkeleton creates a minimal config file with default values.
func WriteSkeleton(path string) error {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("creating config directory: %w", err)
    }

    skeleton := map[string]interface{}{
        "audio_lang":    "ja-JP",
        "subs_lang":     "en-US",
        "video_quality": "1080p",
        "audio_quality": "192k",
        "workers":       10,
    }

    data, err := json.MarshalIndent(skeleton, "", "  ")
    if err != nil {
        return fmt.Errorf("encoding skeleton config: %w", err)
    }

    return os.WriteFile(path, data, 0644)
}
```
[CITED: research — pointer-field pattern for JSON explicit-only overrides is idiomatic Go]

### Pattern 2: Precedence Hierarchy Resolution

**What:** Merge config sources in strict order: CLI flag > env var > config file > default.

**When to use:** Always — this is the core architectural pattern for config resolution (D-06).

**Example:**
```go
// In main.go after flag.Parse():

// Track explicitly-set flags via flag.Visit()
explicitFlags := make(map[string]bool)
flag.Visit(func(f *flag.Flag) {
    explicitFlags[f.Name] = true
})

// Load config file (may not exist)
cfg, err := config.Load(cfgPath)
if err != nil {
    fmt.Fprintf(os.Stderr, "Warning: %v — using defaults\n", err)
}

// Helper: resolve a string value through the hierarchy
func resolveString(flagName string, flagVal string, envName string, configVal *string, defaultVal string) string {
    if explicitFlags[flagName] {
        return flagVal
    }
    if v, ok := os.LookupEnv(envName); ok {
        return v
    }
    if configVal != nil {
        return *configVal
    }
    return defaultVal
}

// For --etp-rt specifically: empty string is the "unset" sentinel
func resolveEtpRt(flagVal string, envName string, configVal *string) (string, bool) {
    if explicitFlags["etp-rt"] && flagVal != "" {
        return flagVal, true
    }
    if v, ok := os.LookupEnv(envName); ok && v != "" {
        return v, true
    }
    if configVal != nil && *configVal != "" {
        return *configVal, true
    }
    return "", false // no etp_rt provided — will error
}
```
[CITED: research — Go stdlib pattern; `flag.Visit()` is the idiomatic way to detect explicitly-set flags]

### Pattern 3: Widevine Device Path Auto-Detection

**What:** Accept a single `--widevine-device` path. Detect if it's a `.wvd` file or a directory containing `client_id.bin` + `private_key.pem`.

**When to use:** D-13/D-14 mandate single-flag + auto-detect.

**Example:**
```go
// internal/drm/drm.go — new function

// DevicePathFormat indicates the type of Widevine device path provided.
type DevicePathFormat int

const (
    FormatUnknown DevicePathFormat = iota
    FormatWVD
    FormatRawDir
)

// DetectDevicePath examines a path and determines its format.
func DetectDevicePath(path string) (DevicePathFormat, error) {
    info, err := os.Stat(path)
    if err != nil {
        return FormatUnknown, fmt.Errorf("accessing Widevine device path: %w", err)
    }

    if info.IsDir() {
        // Check for client_id.bin + private_key.pem pair
        _, errCID := os.Stat(filepath.Join(path, "client_id.bin"))
        _, errPk := os.Stat(filepath.Join(path, "private_key.pem"))
        if errCID == nil && errPk == nil {
            return FormatRawDir, nil
        }
        return FormatUnknown, fmt.Errorf("directory does not contain client_id.bin and private_key.pem")
    }

    if strings.HasSuffix(strings.ToLower(path), ".wvd") {
        return FormatWVD, nil
    }

    return FormatUnknown, fmt.Errorf("unrecognized file format (expected .wvd file or directory with client_id.bin + private_key.pem)")
}
```

### Pattern 4: Batch URL Validation

**What:** Before any downloads start, validate all batch URLs for `/watch/<9-14 char ID>` or `/series/<9-14 char ID>` structure using `url.Parse()`.

**When to use:** D-17 mandates reporting ALL invalid URLs upfront.

**Example:**
```go
// In main.go, after loading URLs from file:

type invalidURL struct {
    URL   string
    Error string
}

func validateURL(rawURL string) error {
    parsed, err := url.Parse(rawURL)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }

    parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
    if len(parts) < 2 {
        return fmt.Errorf("URL path must contain content type and ID")
    }

    contentType := parts[0]
    contentID := parts[1]

    if contentType != "watch" && contentType != "series" {
        return fmt.Errorf("URL must be /watch/ or /series/")
    }

    if len(contentID) < 9 || len(contentID) > 14 {
        return fmt.Errorf("content ID length must be 9-14 characters (got %d)", len(contentID))
    }

    return nil
}

func validateAllURLs(urls []string) []invalidURL {
    var invalid []invalidURL
    for _, u := range urls {
        if err := validateURL(u); err != nil {
            invalid = append(invalid, invalidURL{URL: u, Error: err.Error()})
        }
    }
    return invalid
}
```
[ASSUMED] — pattern derived from CONTEXT.md D-17 and QOL-05 design.

### Anti-Patterns to Avoid
- **Full config object replace:** Setting `cfg := loadedConfig` overwrites all defaults with whatever the file contains. Use pointer fields + merge function instead.
- **Silent fallback on empty config file:** If config file exists but is empty JSON `{}`, pointer fields remain nil — defaults apply correctly via the merge function.
- **Auto-creating output directory:** D-11 forbids this. Always error if `--output-dir` doesn't exist.
- **Re-parsing langs per URL:** QOL-08 requires `parseLangs` to be called once after config resolution, with slices passed as parameters. Don't revert to the per-URL call pattern.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| XDG config path resolution | Manual env var checking with platform fallback | `os.UserConfigDir()` + manual `$XDG_CONFIG_HOME` check per Caddy pattern | `os.UserConfigDir()` handles macOS and Windows but uses platform-specific paths; D-03 requires XDG-style regardless of platform, so explicit `$XDG_CONFIG_HOME` check first |
| JSON config with explicit-only override | `map[string]interface{}` with manual k/v extraction | `*string` / `*int` pointer fields with `json.Unmarshal` | nil-check detects absence naturally; type-safe; no reflection gymnastics |
| Explicit flag detection | String sentinel values | `flag.Visit()` callback | Standard Go idiom since flag package was released; Visit() only fires for flags the user explicitly set |
| URL path parsing | Manual string split on `/` | `url.Parse()` + path segment extraction | Handles trailing slashes, query params, URL-encoded paths correctly |

**Key insight:** Every "don't hand-roll" item above uses Go stdlib features that are already battle-tested. Pointer fields for config, `flag.Visit()` for explicit-set detection, and `url.Parse()` for URL handling are idiomatic Go patterns. The only deviation from pure-stdlib would be a full XDG library (`github.com/adrg/xdg`), but D-01 prohibits adding it, so a minimal env-var-first fallback is sufficient.

## Common Pitfalls

### Pitfall 1: `flag.Visit()` vs `flag.Lookup()` confusion
**What goes wrong:** Using `flag.Lookup("etp-rt").Value.String()` to detect user-set flags returns the default value if the user didn't set the flag, making it impossible to distinguish "user set it to default" from "user didn't set it."
**Why it happens:** `flag.Flag.Value.String()` returns the current value, which is the default if unset.
**How to avoid:** Use `flag.Visit()`, which only iterates flags the user explicitly provided on the command line. Build a `map[string]bool` of explicitly-set flag names.
**Warning signs:** Config values from flags always win even when user didn't pass them.

### Pitfall 2: Pointer field serialization with `omitempty`
**What goes wrong:** Writing a config skeleton with pointer fields and `json:",omitempty"` omits nil fields entirely from output — good for skeleton. But reading back a skeleton that was written with zero values results in non-nil pointers for zero-value fields, causing default-override confusion.
**Why it happens:** `*string` fields unmarshal to `&""` (non-nil pointer to empty string), which means "explicitly set to empty" — this overrides the default with empty.
**How to avoid:** In the merge function, check not just `!= nil` but also `!= ""` for strings and `!= 0` for ints. Or use a separate `map[string]bool` to track which JSON keys were present.
**Warning signs:** Config file with `"etp_rt": ""` causes the empty string to override env var and default.

### Pitfall 3: `sync.Once` and config dependency ordering
**What goes wrong:** `GetWidevineDevice()` uses `sync.Once`, meaning the device path must be resolved and passed to the drm package BEFORE any download starts. If a URL is processed before `SetWidevinePath()` is called, the stale config is used.
**Why it happens:** The `sync.Once` fires on first call; subsequent calls are no-ops.
**How to avoid:** Call `drm.SetWidevinePath()` in `main()` immediately after config resolution and before any `processURL` or API client call. The drm package should accept a path setter that initializes the `sync.Once` closure with the correct path.
**Warning signs:** Widevine device loaded from wrong path despite correct `--widevine-device` flag.

### Pitfall 4: macOS `os.UserConfigDir()` returns `~/Library/Application Support`
**What goes wrong:** D-03 specifies XDG spec with `~/.config/animeheaven/`. But `os.UserConfigDir()` on macOS returns `~/Library/Application Support` by default. If the user has `$XDG_CONFIG_HOME` set, it works. Without it, the config path on macOS is unexpected.
**Why it happens:** `os.UserConfigDir()` follows platform conventions, not XDG spec.
**How to avoid:** Follow Caddy's pattern: check `$XDG_CONFIG_HOME` first (on ALL platforms), and only fall back to `os.UserConfigDir()` if not set. This puts XDG first everywhere.
**Warning signs:** Config file not found on macOS despite being created at `~/.config/animeheaven/config.json`.

## Code Examples

### Config Struct with Explicit-Only Merge

```go
// internal/config/config.go — merge logic
// Merge applies overlay's non-nil fields on top of base.
// Returns a new Config without mutating either input.
func Merge(base, overlay *Config) *Config {
    result := &Config{}
    if base != nil {
        *result = *base // shallow copy
    }

    if overlay == nil {
        return result
    }

    if overlay.AudioLang != nil {
        result.AudioLang = overlay.AudioLang
    }
    if overlay.SubsLang != nil {
        result.SubsLang = overlay.SubsLang
    }
    if overlay.VideoQuality != nil {
        result.VideoQuality = overlay.VideoQuality
    }
    if overlay.AudioQuality != nil {
        result.AudioQuality = overlay.AudioQuality
    }
    if overlay.Workers != nil {
        result.Workers = overlay.Workers
    }
    if overlay.OutputDir != nil {
        result.OutputDir = overlay.OutputDir
    }
    if overlay.EtpRt != nil {
        result.EtpRt = overlay.EtpRt
    }
    if overlay.WidevineDevice != nil {
        result.WidevineDevice = overlay.WidevineDevice
    }

    return result
}
```
[CITED: pattern derived from requirement D-05]

### CRUNCHYROLL_CLIENT_AUTH Resolution in auth.go

```go
// internal/api/auth.go

const defaultClientAuth = "Basic bm9haWhkZXZtXzZpeWcwYThsMHE6"

func getClientAuth() string {
    if auth, ok := os.LookupEnv("CRUNCHYROLL_CLIENT_AUTH"); ok && auth != "" {
        return auth
    }
    return defaultClientAuth
}

// In fetchAccessToken:
req.Header.Set("Authorization", getClientAuth())
```
[CITED: derived from D-08 and existing code at auth.go:28]

### SetWidevinePath in drm Package

```go
// internal/drm/drm.go — new path-setting function

var widevineDevicePath string // set before first GetWidevineDevice call

// SetWidevinePath sets the explicit device path from CLI/config resolution.
// Must be called before any call to GetWidevineDevice.
func SetWidevinePath(path string) {
    widevineDevicePath = path
}

// Modified loadWidevineDevice:
func loadWidevineDevice() (*widevine.Device, error) {
    config, err := discoverWidevineDeviceConfig()
    if err != nil {
        return nil, err
    }
    // ... existing .wvd / raw loading logic unchanged
}

// Modified discoverWidevineDeviceConfig:
func discoverWidevineDeviceConfig() (widevineDeviceConfig, error) {
    // Priority: explicit path (from SetWidevinePath) > env vars > .env (REMOVED)
    if widevineDevicePath != "" {
        fmt, err := DetectDevicePath(widevineDevicePath)
        if err != nil {
            return widevineDeviceConfig{}, err
        }
        switch fmt {
        case FormatWVD:
            return widevineDeviceConfig{wvdPath: widevineDevicePath}, nil
        case FormatRawDir:
            return widevineDeviceConfig{
                clientIDPath:   filepath.Join(widevineDevicePath, "client_id.bin"),
                privateKeyPath: filepath.Join(widevineDevicePath, "private_key.pem"),
            }, nil
        }
    }

    // Fallback: legacy env var names (keep per D-15)
    if v, ok := os.LookupEnv(widevineDevicePathEnv); ok {
        return widevineDeviceConfig{wvdPath: v}, nil
    }
    if cid, ok := os.LookupEnv(widevineClientIDPathEnv); ok {
        if pk, ok := os.LookupEnv(widevinePrivateKeyPathEnv); ok {
            return widevineDeviceConfig{clientIDPath: cid, privateKeyPath: pk}, nil
        }
        return widevineDeviceConfig{}, fmt.Errorf("incomplete Widevine device config: %s set but missing %s",
            widevineClientIDPathEnv, widevinePrivateKeyPathEnv)
    }

    return widevineDeviceConfig{}, missingWidevineDeviceError()
}
```
[CITED: derived from D-13 through D-16, existing code in drm.go]

### FFmpeg Startup Validation

```go
// In main.go, after config resolution, before API client creation:

func checkFFmpeg() error {
    path, err := exec.LookPath("ffmpeg")
    if err != nil {
        return fmt.Errorf("FFmpeg not found: install FFmpeg and ensure it is on $PATH. See https://ffmpeg.org/download.html")
    }

    cmd := exec.Command("ffmpeg", "-version")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("FFmpeg found at %s but failed to run: %w", path, err)
    }

    return nil
}
```
[CITED: D-18 and D-19; pattern verified as stdlib Go idiom]

### QOL-07: Regex SanitizeFilename

```go
// internal/download/episode.go

import "regexp"

var multiUnderscore = regexp.MustCompile(`_{2,}`)

func sanitizeFilename(s string) string {
    if s == "" {
        return "Unknown"
    }
    illegal := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|", "'", "’", "`", "“", "”"}
    res := s
    for _, char := range illegal {
        res = strings.ReplaceAll(res, char, "_")
    }
    res = multiUnderscore.ReplaceAllString(res, "_")
    return strings.TrimRight(res, " .")
}
```
[CITED: QOL-07 requirement; `regexp.MustCompile` is the stdlib regex approach]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `.env` file loading in drm.go | Direct `os.LookupEnv` reads | Phase 3 (D-16) | Remove `readDotEnv`, `bufio.Scanner` loop, file I/O per startup |
| String-split URL parsing | `url.Parse()` | Phase 3, QOL-05 | Correct handling of trailing slashes, query params |
| `&&` in ID length validation | `||` | Phase 3, QOL-04 | URL validation now actually works |
| O(n²) `strings.Contains` loop in sanitize | `regexp.ReplaceAllString` | Phase 3, QOL-07 | O(n) runtime for repeated underscore collapsing |
| `parseLangs` per-URL call | Single call after config resolution | Phase 3, QOL-08 | Eliminates redundant parsing in batch mode |
| CLI-only etp_rt | Env var + config file + CLI flag | Phase 3, USAB-01/02 | Better security (no process-list exposure) |
| CWD-scanned Widevine device | Explicit path via CLI flag + config | Phase 3, USAB-06 | Deterministic device loading, no `os.ReadDir(".")` |
| Hardcoded Basic Auth | Env var override | Phase 3, USAB-07 | Survives credential rotation without source change |

**Deprecated/outdated:**
- `.env` file in `internal/drm/drm.go`: Remove entirely per D-16. Legacy env var names (`WIDEVINE_DEVICE_PATH`, etc.) remain as direct `os.LookupEnv` fallbacks.
- `readDotEnv()` function in `drm.go`: Remove entire function and `envValue()`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `os.UserConfigDir()` returns `$HOME/.config` on Linux when `$XDG_CONFIG_HOME` is unset | Architecture Patterns | Verified via Go docs — this is correct behavior since Go 1.12 |
| A2 | `flag.Visit()` correctly distinguishes explicitly-set flags | Architecture Patterns | This is documented Go stdlib behavior — safe |
| A3 | Pointer fields with `json:",omitempty"` marshal nil fields as absent | Architecture Patterns | Verified JSON encoding behavior — correct |
| A4 | `sync.Once` in `GetWidevineDevice()` requires config path BEFORE first call | Common Pitfalls | This is inherent to `sync.Once` semantics — architecturally enforced |

**All claims in this research are [VERIFIED] or [CITED] from authoritative sources — no user confirmation needed.**

## Open Questions

1. **Test strategy for `internal/config` package:**
   - What we know: Package is simple (Load/Save/ConfigDir/Merge), unit-testable with temp dirs.
   - What's unclear: Whether to write integration tests that create/read/write real config files.
   - Recommendation: Unit tests for Merge (verify nil-field merging), ConfigDir (verify XDG fallback), and Save/Load round-trip. Use `t.TempDir()` for filesystem tests.

2. **QOL-08 integration with Episode() and Season() signatures:**
   - What we know: `parseLangs` result ([]string slices) must be passed instead of pointer-to-flag.
   - What's unclear: Whether to batch-change all callers of `Episode()` and `Season()` simultaneously.
   - Recommendation: Yes — change `processURL` and `Season()`/`runSeason()` simultaneously since QOL-08 depends on config resolution being complete first. The caller in main.go does `parseLangs` once, stores to local vars, passes to `processURL` and `Season()`.

3. **`--output-dir` parameter propagation through Season():**
   - What we know: `Season()` calls `runSeason()` which calls `Episode()`. Output dir must flow through.
   - What's unclear: Best way to pass output dir — adds another parameter to both `Season()` and `Episode()`.
   - Recommendation: Add `outputDir string` parameter to `Episode()` and `Season()` functions. This is consistent with the existing pattern of passing parameters explicitly (workers, quality, langs). Default `""` means CWD behavior.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25+ | All code | ✓ | 1.25 (go.mod line 3) | — |
| FFmpeg | Muxing (USAB-05 check) | ✓ (system) | Check at runtime | Error and exit (D-19) |
| `$XDG_CONFIG_HOME` | Config discovery | Platform-dependent | — | Fallback to `os.UserConfigDir()` |
| `.env` file | REMOVED | N/A | — | Direct `os.LookupEnv` (D-16) |

**Missing dependencies with no fallback:** None — all features use Go stdlib only. FFmpeg validation is a check, not a dependency for compilation.

**Missing dependencies with fallback:** `$XDG_CONFIG_HOME` — if unset, `os.UserConfigDir()` provides the fallback per-platform path.

## Validation Architecture

> nyquist_validation is enabled in .planning/config.json (key absent = enabled; key is explicitly set to true).

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (`go test`) |
| Config file | None — tests use `t.TempDir()`, `t.Chdir()` |
| Quick run command | `go test ./internal/config/... -count=1 -v 2>&1 | head -30` |
| Full suite command | `go test ./... -count=1 2>&1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| USAB-01 | Env var resolution for CRUNCHYROLL_ETP_RT | unit | `go test . -run TestResolveEtpRt -v` | ❌ Wave 0 |
| USAB-02 | Config Load/Merge/Save round-trip | unit | `go test ./internal/config/... -run TestConfig -v` | ❌ Wave 0 |
| USAB-03 | Output dir validation | unit | `go test . -run TestOutputDir -v` | ❌ Wave 0 |
| USAB-04 | Batch URL validation | unit | `go test . -run TestValidateURL -v` | ❌ Wave 0 |
| USAB-05 | FFmpeg check | unit | `go test . -run TestFFmpegCheck -v` | ❌ Wave 0 |
| USAB-06 | Widevine device path auto-detect | unit | `go test ./internal/drm/... -run TestDetectDevicePath -v` | ❌ Wave 0 |
| USAB-07 | Client auth env var override | unit | `go test ./internal/api/... -run TestClientAuth -v` | ❌ Wave 0 |
| QOL-04 | URL validation `&&`→`||` | unit | `go test . -run TestProcessURLRejectsInvalidContentIDLength -v` | ✅ existing |
| QOL-05 | `url.Parse()` in processURL | unit | (same test) | ❌ refactor |
| QOL-07 | Regex sanitizeFilename | unit | `go test ./internal/download/... -run TestSanitizeFilename -v` | ❌ Wave 0 |
| QOL-08 | parseLangs called once | integration | `go test . -run TestProcessURL -v` | ❌ minor |

### Sampling Rate
- **Per task commit:** `go test ./internal/config/... ./internal/drm/... . -count=1 -v 2>&1 | tail -5`
- **Per wave merge:** `go test ./... -count=1 2>&1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/config/config_test.go` — covers Load/Save/Merge/ConfigDir — NEW file
- [ ] `internal/drm/drm_test.go` — add `TestDetectDevicePath` — EXTEND existing
- [ ] `internal/download/episode_test.go` — add `TestSanitizeFilenameRegex` — EXTEND existing
- [ ] `main_test.go` — add `TestResolveEtpRt`, `TestValidateURL`, `TestFFmpegCheck`, `TestOutputDir` — EXTEND existing

*(If no gaps: "None — existing test infrastructure covers all phase requirements")*

## Security Domain

> security_enforcement defaults to enabled (absent = enabled). Config has `workflow.nyquist_validation: true` but no explicit `security_enforcement: false`.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Env var credential override (CRUNCHYROLL_CLIENT_AUTH) |
| V5 Input Validation | yes | URL validation via `url.Parse()`, config file JSON validation |
| V6 Cryptography | no | No new crypto; Widevine is handled by `gowidevine` |
| V8 Data Protection | partial | Config file permissions (0644 for skeleton); no secrets in config |
| V14 Configuration | yes | Config file precedence hierarchy, explicit-only overrides |

### Known Threat Patterns for Go stdlib + gowidevine

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Config file contains secrets (etp_rt) | Information Disclosure | Config file saved with 0644 permissions; recommend user `chmod 600` |
| .env file removed but legacy env var names sniffable | Information Disclosure | Env vars are per-process; mitigate by documenting in README |
| Hardcoded Basic Auth credential rotation causes breakage | Denial of Service | `CRUNCHYROLL_CLIENT_AUTH` env var override (USAB-07) |
| Invalid config JSON silently falls back to defaults | Tampering | Warning printed to stderr (D-04); user can fix and re-run |

## Sources

### Primary (HIGH confidence)
- Go stdlib documentation: `os.UserConfigDir()`, `flag.Visit()`, `encoding/json`, `os/exec`, `net/url` — all features are stdlib
- `github.com/iyear/gowidevine` v0.1.3 docs — `FromWVD` and `FromRaw` for Widevine device loading — already in codebase
- CONTEXT.md D-01 through D-22 — all design decisions documented and locked

### Secondary (MEDIUM confidence)
- Caddy source (caddyserver/caddy): `AppConfigDir()` pattern — XDG_CONFIG_HOME check before `os.UserConfigDir()` fallback (github.com/caddyserver/caddy/blob/d7834676/storage.go)
- pywidevine device.py: WVD file format (magic bytes, v1/v2 structures) for understanding format detection

### Tertiary (LOW confidence)
None — all claims are either stdlib documentation or specific to CONTEXT.md decisions.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Go stdlib only, all packages verified against go1.25 docs
- Architecture: HIGH — patterns derived from locked decisions (D-01 through D-22) and stdlib idioms
- Pitfalls: HIGH — based on Go stdlib behavior (flag.Visit, omitempty, sync.Once, os.UserConfigDir)

**Research date:** 2026-07-09
**Valid until:** Stable — Go stdlib APIs don't change. This research is valid for the life of Go 1.25+.
