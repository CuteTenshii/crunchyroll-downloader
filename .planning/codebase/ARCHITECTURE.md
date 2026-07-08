<!-- refreshed: 2026-07-08 -->
# Architecture

**Analysis Date:** 2026-07-08

## System Overview

```text
┌───────────────────────────────────────────────────────────┐
│                   CLI Layer (main.go)                      │
│   flag parsing │ URL dispatch │ batch file support         │
├──────────────────┬──────────────────┬──────────────────────┤
│   Auth Layer     │   Discovery       │   Download Engine    │
│  `token.go`      │  `season.go`      │   `download.go`      │
│                  │  `episode.go`     │   `mpd.go`           │
│                  │                   │   `drm.go`           │
└────────┬─────────┴────────┬──────────┴──────────┬───────────┘
         │                  │                     │
         ▼                  ▼                     ▼
┌───────────────────────────────────────────────────────────┐
│              HTTP/Network Layer (`http_request.go`)         │
│   Bearer auth injection │ auto-refresh on 401 │ retries    │
└────────────────────────┬──────────────────────────────────┘
                         │
                         ▼
┌───────────────────────────────────────────────────────────┐
│            Output/Muxing Layer (`output.go`)                │
│   ffmpeg subprocess │ MKV muxing │ temp file cleanup       │
└───────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| CLI Entry | Parse CLI flags, dispatch URLs, orchestrate batch/season/episode flows | `main.go` |
| Token Acquisition | Authenticate with Crunchyroll via `etp_rt` cookie, fetch bearer token | `token.go` |
| Season Discovery | List seasons for a series, list episodes within a season | `season.go` |
| Episode Metadata | Fetch playback data (manifest URL, subtitles, stream token) | `episode.go` |
| Segment Downloader | Download DASH segments with concurrent worker pool, Widevine decryption | `download.go` |
| MPD Parser | Parse MPD manifests, resolve representation/quality, expand segment timeline | `mpd.go` |
| DRM License | Extract PSSH, load Widevine device, get license challenge, parse response | `drm.go` |
| Output Muxer | Invoke ffmpeg to merge video + audio(s) + subtitle(s) into output MKV | `output.go` |
| HTTP Client | Centralized request dispatch with auth-token re-fetch on 401 | `http_request.go` |
| Language Maps | Locale → human name and ISO 639-2 code lookup tables | `utils.go` |

## Pattern Overview

**Overall:** Monolithic single-package CLI tool. All Go source files belong to `package main` with no sub-package decomposition. Each file owns a distinct domain concern (auth, download, DRM, output, etc.) but shares global variables and function-level composition via direct calls across files.

**Key Characteristics:**
- **Flat package structure** — All `.go` files in project root, `package main` only
- **Global state** — `token` (string), `keys` (Widevine keys), `deviceId` (UUID) are package-level globals shared across files
- **Synchronous orchestration** — Download workers use goroutines, but overall flow is sequential with `panic(err)` error handling
- **External subprocess for muxing** — ffmpeg invoked via `os/exec` as final packaging step
- **No test files** — 0 test files detected

## Layers

**CLI Entry Layer:**
- Purpose: Parse user input, validate URL format, orchestrate top-level download flow
- Location: `main.go`
- Contains: `main()`, `processUrl()`, `parseLangs()`
- Depends on: All other files (direct function calls)
- Used by: End user (terminal invocation)

**Discovery Layer:**
- Purpose: Fetch metadata from Crunchyroll CMS APIs
- Location: `season.go`, `episode.go`
- Contains: Type definitions (`Season`, `Episode`, `EpisodeInfo`), API calls (`getSeasons`, `getSeasonEpisodes`, `getEpisode`, `getEpisodeInfo`), stream teardown (`deleteStream`)
- Depends on: `http_request.go` (for `DoRequest`), globally shared `token`
- Used by: `main.go` (`processUrl`), `download.go` (`downloadEpisode`)

**Download Engine Layer:**
- Purpose: Download and decrypt DASH-encoded media segments
- Location: `download.go`, `mpd.go`, `drm.go`
- Contains: Segment download with worker pool (`downloadParts`), MPD manifest parsing (`parseManifest`, `getBaseUrl`, `expandTimeline`), Widevine license acquisition (`getLicense`, `sendChallenge`, `getWidevineDevice`), subtitle download (`downloadSubs`)
- Depends on: `http_request.go`, `episode.go` (via `getEpisode`), external dependencies `go-mpd`, `gowidevine`
- Used by: `main.go` (`processUrl` → `downloadEpisode`)

**Output Layer:**
- Purpose: Mux downloaded streams into a single MKV container
- Location: `output.go`
- Contains: `mergeEverything()`, `mediaTrack` type, `trackTitle()`
- Depends on: `ffmpeg` (external binary via `os/exec`), temp files from download layer
- Used by: `download.go` (`downloadEpisode`)

**Infrastructure Layer:**
- Purpose: Shared HTTP client with token refresh, shared data
- Location: `http_request.go`, `token.go`, `utils.go`
- Contains: `DoRequest()`, `GetAccessToken()`, language name/code maps
- Depends on: Go standard library (`net/http`)
- Used by: All other layers

## Data Flow

### Primary Request Path — Single Episode Download

1. CLI entry: `main()` parses flags, validates `--url` and `--etp-rt` (`main.go:93-136`)
2. Auth: `GetAccessToken(etpRt)` fetches bearer token from Crunchyroll (`token.go:21-51`)
3. URL dispatch: `processUrl(url)` parses `watch/EPISODE_ID` pattern (`main.go:34-91`)
4. Metadata: `getEpisodeInfo(id)` fetches CMS object metadata (`episode.go:87-110`)
5. Playback: `getEpisode(contentId)` fetches playback data (manifest URL, subtitles, stream token) (`episode.go:29-60`)
6. Subtitle download: `downloadSubs(url)` fetches `.ass` subtitle files (`download.go:160-188`)
7. Manifest parse: `parseManifest(url)` fetches and decodes MPD XML (`mpd.go:13-38`)
8. DRM: `getPssh(manifest)` extracts PSSH — `getLicense()` performs Widevine challenge-response (`drm.go:115-146`)
9. Segment download: `downloadParts()` downloads init segment + all timeline segments via 10-worker pool, then decrypts via `widevine.DecryptMP4Auto` (`download.go:94-158`)
10. Mux: `mergeEverything()` invokes `ffmpeg` to produce final `.mkv` (`output.go:27-109`)
11. Cleanup: `deleteStream()` releases the Crunchyroll playback token, temp files are removed (`episode.go:113-126`, `output.go:99-106`)

### Batch/Season Flow

1. URL dispatch: `processUrl()` detects `series/SERIES_ID` pattern (`main.go:65-89`)
2. Seasons: `getSeasons(id, audio, sub)` lists all seasons (`season.go:66-96`)
3. Filter: If `--season` flag set, filter to that season number (`main.go:67-78`)
4. Episodes: `getSeasonEpisodes(seasonId, audio, sub)` lists episodes (`season.go:25-55`)
5. Download each: `downloadSeason()` → `downloadEpisode()` per episode (`download.go:375-393`)
6. Each episode follows the single-episode path above

**State Management:**
- **Global mutable state:** `token` (string, set once in `main()`) and `keys` ([]*widevine.Key, overwritten per audio version in `getLicense()`) are package-level globals
- **Stream tracking:** `activeStreams` (map[string]string) in `downloadEpisode()` tracks open playback tokens for deferred cleanup via deferred closure
- **Device ID:** `deviceId` (uuid, generated at init time in `token.go:14`) — persists for the process lifetime

## Key Abstractions

**Episode / Season Data Types:**
- Purpose: Model Crunchyroll API responses
- Examples: `Episode` (`episode.go:11-20`), `SeasonEpisode` (`season.go:14-23`), `Season` (`season.go:61-64`), `EpisodeInfo` (`episode.go:66-70`)
- Pattern: Struct tags for JSON deserialization from Crunchyroll CMS and playback APIs

**DRM / Widevine:**
- Purpose: Decrypt protected DASH segments
- Examples: `getPssh()` (`drm.go:21-34`), `getLicense()` (`drm.go:115-146`), `sendChallenge()` (`drm.go:40-74`), `getWidevineDevice()` (`drm.go:76-113`)
- Pattern: PSSH extraction from MPD → Widevine CDM challenge → license response → keys stored in global

**Segment Download:**
- Purpose: Concurrent download of DASH segments with retry and decryption
- Examples: `downloadParts()` (`download.go:94-158`), `downloadPart()` (`download.go:30-70`), `segmentJob` (`download.go:89-92`)
- Pattern: Producer-consumer worker pool (10 workers), channel-based job dispatch, `sync.WaitGroup` coordination, `atomic.Int64` progress counter

**Media Multiplexing:**
- Purpose: Combine video, audio, and subtitle tracks into a single MKV
- Examples: `mergeEverything()` (`output.go:27-109`), `mediaTrack` (`output.go:11-14`)
- Pattern: ffmpeg subprocess with explicit stream mapping, language metadata, and disposition flags

## Entry Points

**CLI Binary:**
- Location: `main.go:93-136`
- Triggers: Terminal invocation: `./crunchyroll-downloader --url <URL> --etp-rt <COOKIE>`
- Responsibilities: Parse flags, validate inputs, call `GetAccessToken`, dispatch `processUrl` for single URL or batch from file

## Architectural Constraints

- **Threading:** Single-threaded orchestration with a concurrent worker pool (10 goroutines) in `downloadParts()` for parallel segment downloading. No mutexes, synchronization via channels + `sync.WaitGroup`. No goroutine lifecycle management beyond `wg.Wait`.
- **Global state:** `token` (string, `main.go:12`) and `keys` (`drm.go:18`) are package-level globals shared across multiple files. `token` is read by all HTTP request construction. `keys` is written in `getLicense()` and read in `downloadParts()` — the write/read dependency across audio versions is managed by sequential `getLicense()` → `downloadParts()` calls (no races but fragile).
- **Circular imports:** Not possible — single `package main` with no import graph.
- **Error handling:** `panic(err)` used throughout as the primary error strategy (no error return propagation except in `http_request.go`). This means any failure terminates the entire process.
- **External dependency:** `ffmpeg` binary must be on `$PATH` — no fallback or detection logic.

## Anti-Patterns

### Global mutable keys slice

**What happens:** `var keys []*widevine.Key` (`drm.go:18`) is overwritten for each audio version during multi-dub downloads. The video downloader relies on `keys` being set for the first version's license.
**Why it's wrong:** If `getLicense()` fails after `downloadParts()` starts, stale keys could be used silently.
**Do this instead:** Pass keys explicitly as parameters to `downloadParts()` (`download.go:94`).

### panic(err) as primary error handling

**What happens:** Nearly every API call and data transformation uses `panic(err)` on failure (`episode.go:36`, `season.go:36-42`, `drm.go:128-130`, etc.).
**Why it's wrong:** `panic` is not recoverable by callers in standard practice (the deferred recovery in `downloadEpisode` only covers that function's scope). A single network failure kills the entire batch.
**Do this instead:** Return `(result, error)` from all functions and propagate errors up to the caller (`main.go`), logging and continuing to the next episode in batch mode.

### init-time deviceId generation

**What happens:** `var deviceId = uuid.NewString()` (`token.go:14`) generates a new device ID every time the program runs.
**Why it's wrong:** Crunchyroll may rate-limit or flag accounts that constantly rotate device IDs. A persistent device ID per user would be more reliable.
**Do this instead:** Persist device ID to disk (e.g., `~/.config/crunchyroll-downloader/device_id`) and reuse it across runs.

## Error Handling

**Strategy:** Predominantly `panic(err)` on failure, with deferred recovery in `downloadEpisode()` (`download.go:274-283`) for cleanup of active streams. The `http_request.go` `DoRequest()` function is the only place that returns errors properly and implements retry logic (401 → re-auth, retry).

**Patterns:**
- `panic(err)` after failed network calls in `season.go`, `episode.go`, `mpd.go`, `drm.go`
- `defer` + `recover()` in `downloadEpisode()` to release Crunchyroll streams on failure
- `fmt.Printf` for user-facing status messages
- `os.Exit(1)` in `main()` for critical validation failures (missing flags, file not found)

## Cross-Cutting Concerns

**Logging:** Uses raw `fmt.Printf` / `print` to stdout for progress messages. No structured logging, no log levels, no log file support.

**Validation:** URL format validated by string splitting in `processUrl()` (`main.go:35-44`). Audio/subtitle locale availability validated at download time in `downloadEpisode()` with early abort.

**Authentication:** Bearer token flow — `GetAccessToken()` exchanges `etp_rt` cookie for a short-lived `access_token` stored in global `token`. Auto-refresh on 401 in `DoRequest()` (`http_request.go:13-19`).

---

*Architecture analysis: 2026-07-08*
