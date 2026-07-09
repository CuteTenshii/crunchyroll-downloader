# Phase 3: Usability — Configuration & Validation - Pattern Map

**Mapped:** 2026-07-09
**Files analyzed:** 11 (2 new, 9 modified)
**Analogs found:** 11 / 11

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/config/config.go` | utility | CRUD | `internal/drm/drm.go` | role-match |
| `internal/config/config_test.go` | test | CRUD | `internal/drm/drm_test.go` | exact |
| `main.go` | controller | request-response | itself (existing pattern to extend) | exact |
| `main_test.go` | test | request-response | itself (existing pattern to extend) | exact |
| `internal/api/auth.go` | utility | request-response | itself (existing line 28 to modify) | exact |
| `internal/api/client.go` | service | request-response | itself (existing constructor pattern) | exact |
| `internal/drm/drm.go` | service | file-I/O | itself (existing sync.Once + env patterns) | exact |
| `internal/drm/drm_test.go` | test | file-I/O | itself (existing test helpers to extend) | exact |
| `internal/download/episode.go` | service | file-I/O | itself (existing output path + sanitize patterns) | exact |
| `internal/download/season.go` | service | request-response | itself (existing function signature pattern) | exact |
| `internal/download/episode_test.go` | test | file-I/O | itself (existing test pattern to extend) | exact |

## Pattern Assignments

### `internal/config/config.go` (utility, CRUD) — NEW FILE

**Analog:** `internal/drm/drm.go` (lines 1-39) for package structure, constants, and error pattern. Also `internal/media/manifest.go` (lines 1-9) for import/package convention.

**Imports pattern** — stdlib-only internal package convention. Copy from `internal/locale/locale.go` (lines 1-2) and `internal/media/manifest.go` (lines 1-9):
```go
package locale  // → package config

// imports: stdlib only, no external deps for internal/ packages
// locale.go has NO imports (pure data)
// manifest.go uses only stdlib + go-mpd (external dep for API package)
```

**Internal package convention** (no external deps unless necessary) — `internal/locale/locale.go` and `internal/media/manifest.go` both use stdlib by default. Only `internal/drm/drm.go` imports external packages. For `internal/config/config.go`, pattern is **stdlib only**:
```go
import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)
```

**Error handling pattern** — copy from `internal/drm/drm.go` lines 172-174 and `internal/download/episode.go` lines 40-42:
```go
// drm.go pattern for error wrapping with context
func missingWidevineDeviceError() error {
    return errors.New("no Widevine device configured: set WIDEVINE_DEVICE_PATH to a .wvd file, ...")
}

// episode.go pattern for fmt.Errorf with %w wrapping
if err != nil {
    return fmt.Errorf("creating output directory: %w", err)
}
```

**Pointer-field struct pattern for config** — new pattern derived from D-05, modeled after Go's `encoding/json` conventions (not directly in codebase yet, but follows stdlib idiom):
```go
// Config struct with *string/*int pointer fields for explicit-only overrides
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
```

**File I/O pattern** — copy from `internal/drm/drm.go` lines 134-142 (os.Open, os.IsNotExist) and `internal/download/episode.go` lines 40-41 (os.MkdirAll):
```go
// drm.go pattern for file open with IsNotExist handling
file, err := os.Open(path)
if errors.Is(err, os.ErrNotExist) {
    return values, nil  // graceful not-found
}
if err != nil {
    return nil, fmt.Errorf("read %s: %w", path, err)
}

// episode.go pattern for MkdirAll
if err := os.MkdirAll(dir, 0755); err != nil {
    return fmt.Errorf("creating config directory: %w", err)
}
```

**JSON marshal pattern** — already used in `internal/api/auth.go` line 46 and `internal/api/types.go` (assumed from imports). Use `json.MarshalIndent` for config writing:
```go
// auth.go line 46 — existing json.Unmarshal usage
var result CrunchyrollTokenResponse
if err := json.Unmarshal(res, &result); err != nil { ... }

// For config writing (new):
data, err := json.MarshalIndent(skeleton, "", "  ")
if err != nil { return fmt.Errorf("encoding skeleton config: %w", err) }
return os.WriteFile(path, data, 0644)
```

---

### `internal/config/config_test.go` (test, CRUD) — NEW FILE

**Analog:** `internal/drm/drm_test.go` (all 239 lines)

**Test package pattern** — same-package test (`package config`), copy from `internal/drm/drm_test.go` line 1:
```go
package drm  // → package config  (NOT package config_test — uses unexported symbols)
```

**Test helpers pattern** (`t.Helper()`, `t.TempDir()`, `t.Cleanup()`) — copy from `internal/drm/drm_test.go` lines 199-231:
```go
func clearWidevineEnv(t *testing.T) {
    t.Helper()
    for _, key := range []string{widevineDevicePathEnv, widevineClientIDPathEnv, widevinePrivateKeyPathEnv} {
        oldValue, hadValue := os.LookupEnv(key)
        if err := os.Unsetenv(key); err != nil {
            t.Fatalf("Unsetenv(%s): %v", key, err)
        }
        t.Cleanup(func() {
            if hadValue { _ = os.Setenv(key, oldValue) }
        })
    }
}

func chdirTemp(t *testing.T) {
    t.Helper()
    originalCWD, err := os.Getwd()
    if err != nil { t.Fatalf("Getwd(): %v", err) }
    tempDir := t.TempDir()
    if err := os.Chdir(tempDir); err != nil { t.Fatalf("Chdir(%s): %v", tempDir, err) }
    t.Cleanup(func() { _ = os.Chdir(originalCWD) })
}

func writeFile(t *testing.T, path, content string) {
    t.Helper()
    if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
        t.Fatalf("WriteFile(%s): %v", path, err)
    }
}
```

**Table-driven test pattern** — copy from `internal/drm/drm_test.go` lines 112-148:
```go
func TestDiscoverWidevineDeviceConfigRequiresRawPairTogether(t *testing.T) {
    tests := []struct {
        name string
        env  map[string]string
    }{
        { name: "client id without private key", env: map[string]string{widevineClientIDPathEnv: "/tmp/client_id.bin"} },
        // ...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // setup
        })
    }
}
```

---

### `main.go` (controller, request-response) — MODIFY

**Analog:** Itself. The existing patterns for flags, URL processing, and main orchestration are all here.

**Flag definition pattern** (lines 17-26) — add `--output-dir` and `--widevine-device` flags following the same pattern:
```go
var (
    audioLang     = flag.String("audio-lang", "ja-JP", "Audio language(s)...")
    subtitlesLang = flag.String("subs-lang", "en-US", "Subtitle language(s)...")
    videoQuality  = flag.String("video-quality", "1080p", "Video quality")
    audioQuality  = flag.String("audio-quality", "192k", "Audio quality")
    seasonNumber  = flag.Int("season", 0, "Season number...")
    etpRt         = flag.String("etp-rt", "", "The \"etp_rt\" cookie value...")
    debug         = flag.Bool("debug-manifest", false, "Log raw...")
    workers       = flag.Int("workers", 10, "Number of concurrent...")
    // NEW FLAGS:
    outputDir     = flag.String("output-dir", "", "Custom output directory for downloads")
    widevineDev   = flag.String("widevine-device", "", "Path to .wvd file or directory with client_id.bin + private_key.pem")
)
```

**`flag.Parse()` + validation guard pattern** (lines 127-132) — extend the guard for new requirements:
```go
// existing pattern — guard after flag.Parse()
if *url == "" && *urlsFile == "" {
    flag.Usage()
    os.Exit(1)
}
```

**`flag.Visit()` explicit-set detection** — new pattern for precedence hierarchy (derived from research, not yet in codebase):
```go
explicitFlags := make(map[string]bool)
flag.Visit(func(f *flag.Flag) {
    explicitFlags[f.Name] = true
})
```

**`os.LookupEnv` pattern** — copy from `internal/drm/drm.go` lines 126-131:
```go
func envValue(key string, envFileValues map[string]string) string {
    if value, ok := os.LookupEnv(key); ok {
        return strings.TrimSpace(value)
    }
    return strings.TrimSpace(envFileValues[key])
}
```

**`exec.LookPath` pattern** — new, but references `internal/mux/mux.go` line 19 (`exec.CommandContext`), line 90 (`ffmpegCommand(ctx, "ffmpeg", args...)`):
```go
// New FFmpeg check function (main.go)
func checkFFmpeg() error {
    path, err := exec.LookPath("ffmpeg")
    if err != nil {
        return fmt.Errorf("FFmpeg not found: install FFmpeg ...")
    }
    cmd := exec.Command("ffmpeg", "-version")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("FFmpeg found at %s but failed to run: %w", path, err)
    }
    return nil
}
```

**`processURL` current URL validation pattern** (lines 38-53) — target for QOL-04 (&&→||) and QOL-05 (url.Parse()):
```go
// CURRENT (lines 39-48) — will be refactored
parts := strings.Split(url, "/")
if len(parts) < 5 {
    fmt.Printf("Invalid URL format: %s\n", url)
    return
}
contentType := parts[3]
contentID := parts[4]
if len(contentID) < 9 || len(contentID) > 14 {   // QOL-04: was &&, now ||
    fmt.Printf("Invalid URL format: %s\n", url)
    return
}
```

**`processURL` parseLangs call pattern** (lines 55-65) — target for QOL-08 (call once at config resolution time, pass as parameter):
```go
// CURRENT (lines 55-65) — QOL-08 target: move parseLangs after config resolution
audioLangs := parseLangs(*audioLang)
subsLangs := parseLangs(*subtitlesLang)

// AFTER QOL-08: parseLangs called once in main(), result passed to processURL
func processURL(ctx context.Context, client *api.Client, url string, audioLangs, subsLangs []string) {
    // no parseLangs call here
    primaryAudio := audioLangs[0]
    ...
}
```

**`download.Episode()` call pattern** (line 73) — modify when adding `outputDir` parameter:
```go
// CURRENT
if err := download.Episode(ctx, client, contentID, info, audioLangs, subsLangs, videoQuality, audioQuality, *workers); err != nil { ... }

// AFTER: add outputDir from resolved config
if err := download.Episode(ctx, client, contentID, info, audioLangs, subsLangs, videoQuality, audioQuality, *workers, outputDir); err != nil { ... }
```

**`download.Season()` call pattern** (lines 101, 113) — modify when adding `outputDir` parameter:
```go
// CURRENT
if err := download.Season(ctx, client, videoQuality, audioQuality, audioLangs, subsLangs, episodes, *workers); err != nil { ... }
```

---

### `main_test.go` (test, request-response) — MODIFY

**Analog:** Itself (lines 1-54)

**`captureMainStdout` test helper pattern** (lines 32-54) — reuse for all new main_test tests:
```go
func captureMainStdout(t *testing.T, fn func()) string {
    t.Helper()
    original := os.Stdout
    readPipe, writePipe, err := os.Pipe()
    if err != nil { t.Fatalf("Pipe(): %v", err) }
    os.Stdout = writePipe
    fn()
    if err := writePipe.Close(); err != nil { t.Fatalf("close stdout pipe: %v", err) }
    os.Stdout = original
    var buf bytes.Buffer
    if _, err := io.Copy(&buf, readPipe); err != nil { t.Fatalf("read stdout pipe: %v", err) }
    return buf.String()
}
```

**Test function pattern** (lines 12-30):
```go
func TestProcessURLRejectsInvalidContentIDLength(t *testing.T) {
    output := captureMainStdout(t, func() {
        processURL(context.Background(), nil, "https://www.crunchyroll.com/watch/short/title")
    })
    if !strings.Contains(output, "Invalid URL format") {
        t.Fatalf("processURL() output = %q, want invalid URL format message", output)
    }
}
```

---

### `internal/api/auth.go` (utility, request-response) — MODIFY

**Analog:** Itself. Modify line 28's hardcoded credential.

**Current hardcoded auth pattern** (line 28):
```go
req.Header.Set("Authorization", "Basic bm9haWhkZXZtXzZpeWcwYThsMHE6")
```

**Target pattern** — add env var resolution with fallback to default:
```go
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

---

### `internal/api/client.go` (service, request-response) — MODIFY

**Analog:** Itself. The `NewWithContext` constructor (lines 35-55) and `configuredHTTPClient` (lines 57-75) patterns.

**Constructor pattern** (lines 35-55) — etpRt validation is already here:
```go
func NewWithContext(ctx context.Context, etpRt string) (*Client, error) {
    if etpRt == "" {
        return nil, fmt.Errorf("etp_rt cookie is required")
    }
    c := &Client{
        httpClient: configuredHTTPClient(),
        etpRt:      etpRt,
        baseURL:    defaultBaseURL,
        authURL:    defaultAuthURL,
        licenseURL: defaultLicenseURL,
    }
    token, err := c.fetchAccessToken(ctx)
    if err != nil { return nil, fmt.Errorf("fetching access token: %w", err) }
    c.token = token
    return c, nil
}
```

**http.Client construction pattern** (lines 57-75) — remains unchanged:
```go
func configuredHTTPClient() *http.Client {
    return &http.Client{
        Transport: &http.Transport{
            Proxy: http.ProxyFromEnvironment,
            DialContext: (&net.Dialer{
                Timeout:   30 * time.Second,
                KeepAlive: 30 * time.Second,
            }).DialContext,
            ForceAttemptHTTP2:     true,
            MaxIdleConns:          100,
            MaxIdleConnsPerHost:   20,
            IdleConnTimeout:       90 * time.Second,
            TLSHandshakeTimeout:   10 * time.Second,
            ResponseHeaderTimeout: 30 * time.Second,
            ExpectContinueTimeout: 1 * time.Second,
        },
        Timeout: 60 * time.Second,
    }
}
```

---

### `internal/drm/drm.go` (service, file-I/O) — MODIFY

**Analog:** Itself. The key patterns to modify/extend are highlighted below.

**`sync.Once` device caching pattern** (lines 28-33, 56-62) — add `SetWidevinePath()` BEFORE the Once fires:
```go
var (
    widevineDeviceOnce   sync.Once
    widevineDevice       *widevine.Device
    widevineDeviceErr    error
    widevineDeviceLoader = loadWidevineDevice
)

func GetWidevineDevice() (*widevine.Device, error) {
    widevineDeviceOnce.Do(func() {
        widevineDevice, widevineDeviceErr = widevineDeviceLoader()
    })
    return widevineDevice, widevineDeviceErr
}
```

**New `SetWidevinePath` pattern** — add a package-level variable + setter:
```go
var widevineDevicePath string  // set before first GetWidevineDevice call

func SetWidevinePath(path string) {
    widevineDevicePath = path
}
```

**`envValue` env var fallback pattern** (lines 126-132) — keep for legacy env var support:
```go
func envValue(key string, envFileValues map[string]string) string {
    if value, ok := os.LookupEnv(key); ok {
        return strings.TrimSpace(value)
    }
    return strings.TrimSpace(envFileValues[key])
}
```

**`readDotEnv` pattern** (lines 134-170) — REMOVE entire function per D-16.

**Env var constant pattern** (lines 21-26) — keep constants but remove `widevineEnvFile`:
```go
const (
    widevineDevicePathEnv     = "WIDEVINE_DEVICE_PATH"
    widevineClientIDPathEnv   = "WIDEVINE_CLIENT_ID_PATH"
    widevinePrivateKeyPathEnv = "WIDEVINE_PRIVATE_KEY_PATH"
    // REMOVE: widevineEnvFile = ".env"
)
```

**Error message pattern** (lines 172-174):
```go
func missingWidevineDeviceError() error {
    return errors.New("no Widevine device configured: set WIDEVINE_DEVICE_PATH to a .wvd file, or set WIDEVINE_CLIENT_ID_PATH and WIDEVINE_PRIVATE_KEY_PATH together")
}
```

---

### `internal/drm/drm_test.go` (test, file-I/O) — MODIFY

**Analog:** Itself. Extend with `TestDetectDevicePath`.

**Existing test helpers to reuse** — all helpers at lines 183-239 (`resetWidevineDeviceCache`, `clearWidevineEnv`, `chdirTemp`, `writeFile`).

**`clearWidevineEnv` pattern** (lines 199-215) — reuse as-is. For `TestDetectDevicePath`, create files in `t.TempDir()`:
```go
func TestDetectDevicePathAcceptsWVD(t *testing.T) {
    wvdPath := filepath.Join(t.TempDir(), "device.wvd")
    writeFile(t, wvdPath, "some-wvd-content")
    
    format, err := DetectDevicePath(wvdPath)
    if err != nil { t.Fatalf("DetectDevicePath() error = %v", err) }
    if format != FormatWVD { t.Fatalf("format = %d, want FormatWVD", format) }
}

func TestDetectDevicePathAcceptsRawDir(t *testing.T) {
    dir := t.TempDir()
    writeFile(t, filepath.Join(dir, "client_id.bin"), "client")
    writeFile(t, filepath.Join(dir, "private_key.pem"), "key")
    
    format, err := DetectDevicePath(dir)
    if err != nil { t.Fatalf("DetectDevicePath() error = %v", err) }
    if format != FormatRawDir { t.Fatalf("format = %d, want FormatRawDir", format) }
}
```

---

### `internal/download/episode.go` (service, file-I/O) — MODIFY

**Analog:** Itself. Modify `sanitizeFilename` (QOL-07) and `Episode` output path.

**Current `sanitizeFilename` pattern** (lines 21-34) — QOL-07 target: replace O(n²) loop with `regexp.MustCompile`:
```go
// CURRENT
func sanitizeFilename(s string) string {
    if s == "" { return "Unknown" }
    illegal := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|", "'", "’", "`", "“", "”"}
    res := s
    for _, char := range illegal {
        res = strings.ReplaceAll(res, char, "_")
    }
    for strings.Contains(res, "__") {
        res = strings.ReplaceAll(res, "__", "_")
    }
    return strings.TrimRight(res, " .")
}

// AFTER QOL-07 — replace __ loop with regexp
import "regexp"

var multiUnderscore = regexp.MustCompile(`_{2,}`)

func sanitizeFilename(s string) string {
    if s == "" { return "Unknown" }
    illegal := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|", "'", "’", "`", "“", "”"}
    res := s
    for _, char := range illegal {
        res = strings.ReplaceAll(res, char, "_")
    }
    res = multiUnderscore.ReplaceAllString(res, "_")
    return strings.TrimRight(res, " .")
}
```

**Current output path construction pattern** (lines 36-55) — add `outputDir` parameter:
```go
// CURRENT — series subdir in CWD
func Episode(..., workers int) error {
    cleanSeriesTitle := sanitizeFilename(info.EpisodeMetadata.SeriesTitle)
    ...
    if err := os.MkdirAll(cleanSeriesTitle, 0777); err != nil {
        return fmt.Errorf("creating output directory: %w", err)
    }
    outputFile := filepath.Join(cleanSeriesTitle, fmt.Sprintf("%s S%02dE%02d - %s [%s].mkv", ...))
}

// AFTER — outputDir parameter, join if non-empty
func Episode(..., workers int, outputDir string) error {
    cleanSeriesTitle := sanitizeFilename(info.EpisodeMetadata.SeriesTitle)
    ...
    outputBase := cleanSeriesTitle
    if outputDir != "" {
        outputBase = filepath.Join(outputDir, cleanSeriesTitle)
    }
    if err := os.MkdirAll(outputBase, 0777); err != nil {
        return fmt.Errorf("creating output directory: %w", err)
    }
    outputFile := filepath.Join(outputBase, fmt.Sprintf("%s S%02dE%02d - %s [%s].mkv", ...))
}
```

---

### `internal/download/season.go` (service, request-response) — MODIFY

**Analog:** Itself. Add `outputDir` parameter throughout the call chain.

**Current function signature pattern** (lines 27-31):
```go
type episodeDownloader func(ctx context.Context, client *api.Client, baseContentID string, info *api.EpisodeInfo, audioLangs, subsLangs []string, videoQuality, audioQuality *string, workers int) error

func Season(ctx context.Context, client *api.Client, videoQuality, audioQuality *string, audioLangs, subsLangs []string, episodes []api.SeasonEpisode, workers int) error {
    return runSeason(ctx, client, videoQuality, audioQuality, audioLangs, subsLangs, episodes, workers, Episode)
}
```

**Target pattern** — add `outputDir string` parameter:
```go
type episodeDownloader func(ctx context.Context, client *api.Client, baseContentID string, info *api.EpisodeInfo, audioLangs, subsLangs []string, videoQuality, audioQuality *string, workers int, outputDir string) error

func Season(ctx context.Context, client *api.Client, videoQuality, audioQuality *string, audioLangs, subsLangs []string, episodes []api.SeasonEpisode, workers int, outputDir string) error {
    return runSeason(ctx, client, videoQuality, audioQuality, audioLangs, subsLangs, episodes, workers, outputDir, Episode)
}
```

---

### `internal/download/episode_test.go` (test, file-I/O) — MODIFY

**Analog:** Itself. Extend with `TestSanitizeFilenameRegex`.

**Existing test pattern** (lines 12-35):
```go
func TestEpisodeReturnsErrorForUnavailableAudioLocale(t *testing.T) {
    t.Chdir(t.TempDir())
    // ... setup ...
    err := Episode(context.Background(), nil, "base-content-id", info, []string{"en-US"}, nil, &videoQuality, &audioQuality, 2)
    if err == nil { t.Fatal("Episode() error = nil, want ... error") }
    if !strings.Contains(err.Error(), "audio locale en-US is not available") {
        t.Fatalf("Episode() error = %q, want ...", err)
    }
}
```

**New test for sanitizeFilename** — table-driven test:
```go
func TestSanitizeFilenameCollapsesMultiUnderscore(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string
    }{
        {name: "single underscore unchanged",  input: "a_b",   want: "a_b"},
        {name: "double underscore collapses",   input: "a__b",  want: "a_b"},
        {name: "triple underscore collapses",   input: "a___b", want: "a_b"},
        {name: "trailing space trimmed",        input: "a ",    want: "a"},
        {name: "empty returns Unknown",         input: "",      want: "Unknown"},
        {name: "illegal chars become underscore", input: "a:b", want: "a_b"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := sanitizeFilename(tt.input)
            if got != tt.want {
                t.Fatalf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

---

## Shared Patterns

### Package Structure Convention
**Source:** `internal/locale/locale.go`, `internal/drm/drm.go`, `internal/media/manifest.go`
**Apply to:** `internal/config/config.go`, `internal/config/config_test.go`
```go
package config  // same-package (not config_test) for access to unexported symbols

import (
    // stdlib imports only — no external deps
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)
```

### Error Wrapping with `fmt.Errorf`
**Source:** `internal/drm/drm.go:73`, `internal/download/episode.go:41`, `internal/api/client.go:50`
**Apply to:** All modified files
```go
// Always use %w for error wrapping
return fmt.Errorf("creating output directory: %w", err)
return fmt.Errorf("fetching access token: %w", err)
return fmt.Errorf("open %s: %w", widevineDevicePathEnv, err)
```

### `os.LookupEnv` Env Var Read
**Source:** `internal/drm/drm.go:127`
**Apply to:** `main.go`, `internal/config/config.go`, `internal/api/auth.go`
```go
if value, ok := os.LookupEnv(key); ok {
    return strings.TrimSpace(value)
}
```

### `os.MkdirAll` Directory Creation
**Source:** `internal/download/episode.go:40`
**Apply to:** `internal/config/config.go` (config directory), `internal/download/episode.go` (output dir)
```go
if err := os.MkdirAll(dir, 0755); err != nil {
    return fmt.Errorf("creating directory: %w", err)
}
```

### `os.Stat` Existence Check
**Source:** `internal/download/episode.go:52`
**Apply to:** `internal/drm/drm.go` (DetectDevicePath), `internal/config/config.go` (config file)
```go
if _, err := os.Stat(path); err == nil {
    // exists
} else if os.IsNotExist(err) {
    // does not exist
} else {
    return fmt.Errorf("accessing path: %w", err)
}
```

### `os.WriteFile` with 0644 Permissions
**Source:** `internal/drm/drm_test.go:236`
**Apply to:** `internal/config/config.go` (config skeleton write)
```go
if err := os.WriteFile(path, data, 0644); err != nil {
    return fmt.Errorf("writing file: %w", err)
}
```

### `flag.String` / `flag.Int` Definition Pattern
**Source:** `main.go:17-26`
**Apply to:** `main.go` (new `--output-dir`, `--widevine-device` flags)
```go
var (
    flagName = flag.String("flag-name", "default", "Description")
    flagNum  = flag.Int("flag-num", 0, "Description")
)
```

### `os.Exit(1)` on Fatal Error
**Source:** `main.go:131, 136, 144`
**Apply to:** `main.go` (FFmpeg check failure, config errors)
```go
if err != nil {
    fmt.Printf("Error message: %v\n", err)
    os.Exit(1)
}
```

---

## No Analog Found

All files in this phase have a direct analog in the existing codebase. No pure-unknown patterns.

The `internal/config/config.go` package is structurally new but follows established conventions from `internal/drm/drm.go` (stdlib-first, error wrapping, env var reading). The pointer-field Config struct is a new pattern but follows Go stdlib conventions (same as `encoding/json` pointer field idiom used throughout the codebase).

---

## Metadata

**Analog search scope:** `/home/fmoura/Documents/Applications/crunchyroll-downloader/**/*.go`
**Files scanned:** 18 source files + test files
**Pattern extraction date:** 2026-07-09
