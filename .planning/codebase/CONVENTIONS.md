# Coding Conventions

**Analysis Date:** 2026-07-08

## Language

This project is written entirely in **Go 1.25** (`go.mod` line 3). All source files belong to `package main` — there are no sub-packages.

## Naming Patterns

**Files:**
- All Go source files are flat in the project root: `main.go`, `download.go`, `episode.go`, `season.go`, `http_request.go`, `token.go`, `drm.go`, `mpd.go`, `output.go`, `utils.go`
- Each file is named by its primary concern (lowercase, snake_case with no separators within words)
- No test files exist (`*_test.go` not found anywhere)

**Functions/Methods:**
- **Exported** functions use **PascalCase**: `GetAccessToken`, `DoRequest` (see `token.go:21`, `http_request.go:7`)
- **Unexported** functions use **mixedCase** (camelCase): `processUrl`, `parseLangs`, `downloadEpisode`, `getEpisode`, `getSeasons`, `parseManifest`, `mergeEverything`
- Acronyms within function names are inconsistently cased:
  - `processUrl` (mixed) — `token.go`
  - `getPssh` (lowercase acronym) — `drm.go:21`
  - `getBaseUrl` (mixed) — `mpd.go:40`
  - `expandTimeline` (camelCase) — `mpd.go:74`
  - `GetAccessToken` (PascalCase with mixed acronym) — `token.go:21`

**Variables:**
- Function-scoped variables use **mixedCase** (camelCase): `audioLangs`, `subsLangs`, `primaryAudio`, `seasonId`, `cleanSeriesTitle` (`main.go`, `download.go`)
- Parameters use **mixedCase**: `contentId`, `audio_locale` (note: `season.go:25` uses snake_case `audio_locale` — **inconsistent**)
- Error return names use single-letter `err` consistently
- Loop variables use single-letter: `i`, `j`, `w`
- Boolean flags use `is` prefix: `isVideoSet` (`mpd.go:40`)

**Types:**
- Exported structs use **PascalCase**: `Episode`, `EpisodeInfo`, `EpisodeMetadata`, `Subtitle`, `DubVersion`, `SeasonEpisode`, `Season`, `CrunchyrollTokenResponse`, `CrunchyrollWidevineLicenseResponse` (`episode.go`, `season.go`, `drm.go`)
- Unexported types use **mixedCase**: `mediaTrack`, `audioVersion`, `segmentJob` (`output.go:12`, `download.go:89`, `download.go:255`)
- JSON tags use **snake_case**: `"audio_locale"`, `"series_title"`, `"availability_starts"`, `"access_token"`, `"episode_number"` (consistent across all structs)

**Global/package-level variables:**
- `token` (string) — `main.go:12` — authentication token, updated via `DoRequest`
- `deviceId` (string) — `token.go:14` — generated UUID, package-level init
- `keys` ([]*widevine.Key) — `drm.go:18` — Widevine decryption keys
- `languageNames` (map) — `utils.go:3` — locale to display name mapping
- `languageCodes` (map) — `utils.go:34` — locale to ISO 639-2 mapping
- All flags defined as `flag.String(...)` / `flag.Bool(...)` / `flag.Int(...)` — `main.go:13-19`

**Constants:**
- `maxWorkers` (untyped int constant) — `download.go:20`

## Code Style

**Formatting:**
- Standard Go formatting conventions (implied, no `.golangci.yml` or `.editorconfig` found)
- No custom formatter configuration detected — relies on `gofmt` default behavior

**Linting:**
- No linter configuration found (no `.golangci.yml`, no `make lint` target, no CI lint step)
- The CI workflow (`release.yml`) only builds — no lint or vet step

**Line length:**
- No enforced limit observed. Long lines exist, e.g.:
  - `season.go:34` — URL construction, ~180 chars
  - `episode.go:88` — URL construction, ~170 chars
  - `download.go:191` — function signature spanning ~140 chars

## Import Organization

**Order:**
1. Standard library packages (no blank line separator)
2. Third-party packages (`github.com/...`) — grouped with a blank line separator
3. No internal project imports (single package project)

Example from `download.go`:
```go
import (
    "bytes"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "sync/atomic"
    "time"

    widevine "github.com/iyear/gowidevine"
    "github.com/unki2aut/go-mpd"
)
```

**Named imports:**
- `widevine "github.com/iyear/gowidevine"` — used for namespace clarity (`drm.go`, `download.go`)

## Error Handling

**Primary pattern: `panic(err)`** — used extensively throughout the codebase:

```go
// episode.go:32
req, err := http.NewRequest(http.MethodGet, ..., nil)
if err != nil {
    panic(err)
}
```

This pattern is used for:
- HTTP request creation failures
- Response body read failures
- JSON unmarshal failures
- File operation failures
- Widevine CDM failures

**Error returns** are used in:
- Package-level helper functions: `downloadPart` returns `([]byte, error)` — `download.go:30`
- HTTP request functions: `sendChallenge` returns `([]byte, error)` — `drm.go:40`
- Widevine device loading: `getWidevineDevice` returns `(*widevine.Device, error)` — `drm.go:76`
- `downloadParts` returns `(string, error)` — `download.go:94`

**Error wrapping:**
- Uses `fmt.Errorf("...: %w", err)` for wrapping — `download.go:49`, `drm.go:119`
- Some error messages are prefixed with the function name: `fmt.Errorf("widevine.DecryptMP4Auto: %w", err)` — `download.go:154`

**Recovery:**
- A single `recover()` in `downloadEpisode`: `defer` block catches panics and prints a message (`download.go:280-282`)
- No structured error propagation to callers in most cases

**HTTP retry:**
- `downloadPart` implements exponential backoff retry (5 attempts, `attempt * 2` second sleep) — `download.go:30-69`
- `DoRequest` implements token-refresh-and-retry on 401 — `http_request.go:7-23`

## Logging

**Framework:** `fmt.Print` / `fmt.Printf` — no structured logging library used

**Patterns:**
- Progress output: `fmt.Printf("\rDownloaded %v of %v segments (%v%%)", ...)` — `download.go:123`
- Status messages: `fmt.Printf("Downloading: %s ...\n", ...)` — `download.go:269`
- Debug output (guarded by `*debug` flag): `fmt.Printf("\n%s\n", string(body))` — `episode.go:56`, `mpd.go:34`
- Error prefix: `fmt.Printf("Failed to open URLs file: %s\n", err)` — `main.go:113`
- Warning prefix (`!`): `fmt.Printf("! Audio locale %s is not available...\n", ...)` — `download.go:263`

## Comments

**When to Comment:**
- Struct fields with non-obvious meaning receive doc comments:
  ```go
  // Token to give to the Widevine CDM challenge
  Token string `json:"token"`
  ```
- Functions with side effects get single-line explanations:
  ```go
  // deleteStream removes the stream to make Crunchyroll think we "left" the playback
  ```
- Complex logic blocks get multi-line explanations:
  ```go
  // The season/series API endpoints take a single preferred locale; use the
  // primary (first) requested one. All dub versions are still listed per
  // episode, so the other languages remain resolvable.
  ```
- Business rules and edge cases are explained inline.

**Doc comments style:**
- Exported functions (`GetAccessToken`, `DoRequest`): single-line `// FunctionName does X`
- Unexported functions: often uncommented (e.g., `processUrl`, `parseLangs`, `buildUrl`)
- No `go doc` format observed — uses plain English sentences

## Function Design

**Size:**
- Small focused functions (5-50 lines): `parseLangs`, `buildUrl`, `getPssh`, `trackTitle`
- Large orchestrating functions (100-350 lines): `downloadEpisode` (~183 lines), `mergeEverything` (~82 lines)
- `downloadEpisode` (`download.go:190-373`) is the largest function, handling episode resolution, subtitle download, audio/video download, and stream cleanup

**Parameters:**
- Functions often take 4-7 parameters, especially orchestrators: `downloadEpisode(baseContentId, info, audioLangs, subsLangs, videoQuality, audioQuality)` — 6 parameters
- Similar for `downloadSeason(videoQuality, audioQuality, audioLangs, subsLangs, episodes)` — 5 parameters
- Pointer parameters used for shared mutation: `*string`, `*int64` (`buildUrl`, `downloadParts`)

**Return Values:**
- Error returns use `(value, error)` Go convention
- Boolean returns for existence checks: `deleteStream` returns `bool`
- Functions that never fail use `panic` instead of returning error

## Module Design

**Exports:**
- Minimal — only `DoRequest` and `GetAccessToken` are exported
- All other functions are package-private (lowercase first letter)

**Barrel Files:**
- Not applicable — single package, no barrel pattern

**File grouping by concern:**
| File | Concern |
|------|---------|
| `main.go` | CLI entry, URL routing, flag parsing |
| `download.go` | Segment downloading, parallel downloader, orchestration |
| `episode.go` | Episode metadata & playback API |
| `season.go` | Season/series listing API |
| `http_request.go` | HTTP client wrapper with auth refresh |
| `token.go` | Crunchyroll OAuth token acquisition |
| `drm.go` | Widevine DRM, PSSH extraction, license challenge |
| `mpd.go` | DASH manifest parsing, timeline expansion |
| `output.go` | FFmpeg MKV muxing, track metadata |
| `utils.go` | Locale-to-name/code mappings |

## Cross-Cutting Patterns

**HTTP request construction:**
- Consistent use of `http.NewRequest(http.MethodGet/Post/Delete, ...)` 
- Headers are always set with `req.Header.Set()` — `Authorization`, `User-Agent`, `Origin`, `Referer`
- User-Agent is always `"Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0"`
- Two HTTP clients used: `http.DefaultClient` and `&http.Client{}` (in `DoRequest`)

**JSON handling:**
- Standard `encoding/json` with `json.Unmarshal` — no streaming decoder use
- All response bodies read via `io.ReadAll` before unmarshaling

**Deferred cleanup:**
- `resp.Body.Close()` deferred after successful requests
- Temporary file cleanup deferred or performed after FFmpeg completes
- Stream tokens released in `defer` block in `downloadEpisode`

**Concurrency:**
- Worker pool pattern with `sync.WaitGroup` and channel-based job distribution (`download.go:108-133`)
- `sync.Once` for first-error capture across goroutines
- `atomic.Int64` for progress tracking across workers

**Closure usage:**
- `sanitize` closure defined inside `downloadEpisode` for filename sanitization (`download.go:191-207`)
- Worker goroutines use closure over loop variables correctly

---

*Convention analysis: 2026-07-08*
