# Codebase Concerns

**Analysis Date:** 2026-07-08

## Tech Debt

### Widespread use of `panic()` as Primary Error Handler

- **Issue:** Almost every function across the codebase uses `panic(err)` instead of returning errors. 35 instances of `panic()` exist across 8 source files. This makes the program crash on any unexpected condition — network timeouts, malformed API responses, disk I/O failures, etc.
- **Files:**
  - `episode.go` (lines 32, 38, 45, 48, 90, 96, 103, 106, 116, 122)
  - `season.go` (lines 36, 42, 48, 51, 77, 83, 89, 92)
  - `download.go` (lines 163, 170, 176, 182, 331, 336, 343, 347, 358, 362)
  - `drm.go` (line 29)
  - `mpd.go` (lines 16, 22, 28)
  - `token.go` (lines 29, 39, 47)
  - `output.go` (line 96)
- **Impact:** Any transient error (network blip, API rate-limit, parsing issue) crashes the entire download, potentially losing progress on a multi-episode batch download. Users must restart from scratch.
- **Fix approach:** Replace `panic()` with proper error propagation using Go's idiomatic `return err` pattern. Use `recover()` only at the top-level goroutine boundary. Implement retry logic for network-related API calls instead of hard-crashing.

### Global Mutable State

- **Issue:** Three package-level variables carry mutable state across the entire program: `token` (string), `keys` ([]*widevine.Key), and `deviceId` (string). These are written and read across multiple goroutines and functions with no synchronization.
  - `token` is set once in `main()` but also mutated in `DoRequest()` (`http_request.go:16`) during HTTP retries.
  - `keys` is overwritten on every `getLicense()` call (`drm.go:140`) and consumed in `downloadParts()` (`download.go:153`) — if two audio tracks are being processed concurrently, the keys can be overwritten between the license fetch and the decryption.
  - `deviceId` is initialized at package load time via `uuid.NewString()` (`token.go:14`), making it effectively random per invocation but global.
- **Files:**
  - `main.go:12` — `var token = ""`
  - `http_request.go:16` — token mutation during retry
  - `drm.go:18` — `var keys []*widevine.Key`
  - `drm.go:140` — `keys, err = parseLicense(resp)`
  - `download.go:153` — `widevine.DecryptMP4Auto(..., keys, ...)`
  - `token.go:14` — `var deviceId = uuid.NewString()`
- **Impact:** Race conditions in concurrent code paths. The `keys` variable is especially dangerous — audio and video could be decrypted with the wrong keys if another version's license arrives first.
- **Fix approach:** Pass `token` as a function parameter or through a context. Return `keys` from `getLicense()` instead of storing globally. Remove all package-level mutable state.

### No Test Coverage

- **Issue:** Zero test files exist in the entire repository. No `*_test.go` files, no test infrastructure configured.
- **Files:** Entire repo
- **Impact:** Every refactor or feature addition risks regressions. The URL parsing bug in `main.go` (see Known Bugs section) would have been caught by a simple unit test.
- **Fix approach:** Add `go test` suite with unit tests for URL parsing, language parsing, manifest timeline expansion, and filename sanitization. Add integration test harness that runs against mock HTTP servers.

### File Descriptor Leak in `getFilename()`

- **Issue:** `getFilename()` in `download.go:72-87` calls `os.CreateTemp()` which opens a file and returns an `*os.File`. The function only returns the file's name string but never closes the file handle. The file descriptor remains open until garbage collection or process exit.
- **Files:** `download.go:72-87`
- **Impact:** Temporary files are created but not closed, leaking file descriptors. Over a long batch download this could exhaust the process's file descriptor limit.
- **Fix approach:** Create the temp file, close it, then return the name — or use `os.CreateTemp()` and immediately `file.Close()` before returning.

### Silent Error Discard

- **Issue:** Multiple calls silently discard their error return values using `_`:
  - `download.go:74` — `f, _ := os.CreateTemp(...)`
  - `download.go:79` — `f, _ := os.CreateTemp(...)`
  - `download.go:82` — `f, _ := os.CreateTemp(...)`
  - `download.go:211-212` — `_ = os.MkdirAll(..., 0777)`
  - `output.go:95, 100, 102, 105` — `_ = os.Remove(...)` (four instances)
  - `drm.go:79` — `files, _ := os.ReadDir(".")`
- **Files:** `download.go`, `output.go`, `drm.go`
- **Impact:** Silent failures — if a directory can't be created, a temp file can't be written, or a cleanup removal fails, the program continues as if nothing happened. This leads to corrupted output or mysterious downstream errors.
- **Fix approach:** Check and log errors at minimum. For cleanup removals, log warnings. For critical paths like file creation and directory creation, return errors to the caller.

## Known Bugs

### URL Content ID Validation is a No-Op

- **Symptoms:** The validation `if len(contentId) < 9 && len(contentId) > 14` in `processUrl()` is logically impossible — a string cannot be simultaneously shorter than 9 and longer than 14 characters. This means no invalid URL is ever rejected by this check.
- **Files:** `main.go:37`
- **Trigger:** Enter any URL with `/watch/` or `/series/` structure, regardless of the actual content ID format.
- **Fix:** Change `&&` to `||` — `if len(contentId) < 9 || len(contentId) > 14`.
- **Workaround:** Invalid IDs will produce a downstream API error, but the error message is a `panic()` crash with no guidance.

### `/watch/` vs `/series/` Path Parsing is Brittle

- **Symptoms:** The URL parser at `main.go:35-36` splits on `"/"` and blindly takes index `[3]` and `[4]`. This breaks on URLs with trailing slashes, different path structures, or query parameters before the path.
- **Files:** `main.go:35-37`
- **Trigger:** A URL like `https://www.crunchyroll.com/watch/GE00198973JAJP/` (trailing slash) or `https://www.crunchyroll.com/watch/GE00198973JAJP/dawn-and-confusion?param=value`.
- **Fix:** Use `url.Parse()` from the standard library and parse the path properly.

## Security Considerations

### Hardcoded API Credential in Source

- **Risk:** `token.go:31` contains a hardcoded Basic Authorization token: `Authorization: Basic bm9haWhkZXZtXzZpeWcwYThsMHE6`. This is a static credential for the Crunchyroll auth endpoint. Anyone who can read the source can use this credential to hit the Crunchyroll API.
- **Files:** `token.go:31`
- **Current mitigation:** The credential is for the OAuth token endpoint only and is likely a public client ID registered with Crunchyroll. However, it is opaque and undocumented.
- **Recommendation:** Document what this credential is (public client ID vs secret). Consider reading it from an environment variable as a fallback in case Crunchyroll rotates it. Add a comment explaining its purpose.

### `etp_rt` Cookie Handled as CLI Argument

- **Risk:** The Crunchyroll session cookie (`etp_rt`) is passed as a `-etp-rt` CLI flag. On multi-user systems this is visible in process listings (`/proc/*/cmdline`) and shell history. This is a persistent session token that grants full account access.
- **Files:** `main.go:19`
- **Current mitigation:** None. The README instructs users to paste the cookie directly as a CLI argument.
- **Recommendation:** Support reading from an environment variable (`CRUNCHYROLL_ETP_RT`) or from a file with restricted permissions (`chmod 600`).

### User-Agent Spoofing

- **Risk:** Every HTTP request uses a hardcoded User-Agent string impersonating Firefox on Linux (`Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0`). This violates Crunchyroll's Terms of Service, which the tool already exists to circumvent.
- **Files:** Every `http.NewRequest` call across `download.go`, `episode.go`, `season.go`, `drm.go`, `mpd.go`, `token.go`
- **Current mitigation:** None — spoofing is intentional.
- **Recommendation:** Make User-Agent configurable or rotate it periodically to avoid fingerprinting.

### Widevine Device Files Searched in Current Directory

- **Risk:** `getWidevineDevice()` in `drm.go:76-113` scans the current working directory for `*.wvd`, `client_id.bin`, and `private_key.pem` files. If the user runs the tool from an unintended directory, sensitive Widevine device files could be loaded from unexpected locations.
- **Files:** `drm.go:79-106`
- **Current mitigation:** Files are listed via `.gitignore`, but there's no validation the files belong to the user.
- **Recommendation:** Accept explicit paths via CLI flags for the Widevine device files rather than scanning the working directory.

## Performance Bottlenecks

### MPD Manifest Re-Parsing for Each Audio Version

- **Problem:** For each audio version/locale requested, `downloadEpisode()` fetches a separate playback manifest (`getEpisode()` → `parseManifest()`), issues a Widevine license challenge (`getLicense()`), and downloads the audio segments `downloadParts()`). The video is downloaded only once (as an optimization at `download.go:353`), but the manifest is fetched and parsed N times.
- **Files:** `download.go:321-370`
- **Cause:** Each audio dub has a separate content ID and requires its own license. This is architecturally necessary but the manifests could be fetched in parallel rather than sequentially.
- **Improvement path:** Fetch manifests for all audio versions concurrently using goroutines. This is a natural parallelization opportunity since each audio version is independent.

### Sequential Audio Download

- **Problem:** Audio tracks are downloaded one after another in the loop at `download.go:322`. Since each audio version has its own Widevine license (which overwrites the global `keys`), they must be sequential in the current architecture — but this is an artificial constraint imposed by the global `keys` variable.
- **Files:** `download.go:321-370`
- **Cause:** The global `keys` variable (`drm.go:18`) forces sequential processing.
- **Improvement path:** Make `getLicense()` return keys instead of storing globally, then download audio tracks concurrently.

## Fragile Areas

### `DoRequest()` Recursive Token Refresh

- **Files:** `http_request.go:7-23`
- **Why fragile:** When a 401 Unauthorized response is received, `DoRequest()` refetches the access token by calling `GetAccessToken()` (which makes an HTTP POST to Crunchyroll's auth endpoint), mutates the global `token`, then **recursively calls itself** with the new token. If the token refresh also returns 401 (e.g., the `etp_rt` cookie expired), this creates infinite recursion until stack overflow.
- **Safe modification:** Add a recursion-depth guard or a `refreshed` boolean parameter. Max one re-auth attempt, then return the 401 error to the caller.
- **Test coverage:** Zero — no tests exist for this retry logic.

### `downloadParts()` Goroutine Error Handling

- **Files:** `download.go:94-158`
- **Why fragile:** When any download worker encounters an error, `errOnce.Do` captures it and the worker returns (exiting the `range jobs` loop). However, remaining workers continue consuming jobs from the channel. The main goroutine still blocks on `wg.Wait()`. The function returns `downloadErr` which may or may not be set yet due to the race between `wg.Wait()` completing and `errOnce` firing. Additionally, partially-downloaded segment data in `results` is silently discarded.
- **Safe modification:** Close the `jobs` channel on first error so all workers stop. Or use `context.Context` with cancellation.
- **Test coverage:** Zero.

### `getWidevineDevice()` Fallback Logic

- **Files:** `drm.go:76-113`
- **Why fragile:** This function scans the current working directory with `os.ReadDir(".")` and tries to load either a `.wvd` file OR separate `client_id.bin` + `private_key.pem` files. If only one of the two separate files exists (e.g., `client_id.bin` but no `private_key.pem`), it silently returns `nil, nil`. If both exist but `private_key.pem` is listed before `client_id.bin` alphabetically, the `break` at line 104 causes `clientID` to never be read from the subsequent iteration.
- **Safe modification:** Use explicit paths via CLI flags. Validate that both `client_id.bin` and `private_key.pem` exist together before attempting to load. Don't `break` after finding `private_key.pem` — continue scanning for both.

### `getBaseUrl()` Bandwidth Matching

- **Files:** `mpd.go:40-72`
- **Why fragile:** Audio quality matching uses bandwidth thresholds (`>= 192000`, `>= 128000`, `>= 96000`). If Crunchyroll changes their MPD bandwidth values (e.g., switching from 192002 to 192000 flat), or if the user specifies a non-standard quality like `320k`, the function falls through to returning the first available representation silently. The fallback prints a message but doesn't error.
- **Safe modification:** Add explicit quality-bandwidth mapping. Return an error for unrecognized quality values instead of silently downgrading.
- **Test coverage:** Zero.

## Scaling Limits

### No Rate Limiting

- **Current capacity:** The tool fires HTTP requests as fast as the 10-worker pool can process them, with no delay between API calls to Crunchyroll.
- **Limit:** If Crunchyroll enforces rate limits (e.g., 429 responses), the only handling is in `downloadPart()` which has retry with backoff, but only for segment downloads. The Crunchyroll API calls (episode info, season listing, playback tokens, license challenges) have zero rate-limit handling.
- **Scaling path:** Add HTTP response code handling for 429 status codes across all API calls. Implement exponential backoff with jitter.

### Single-Process Architecture

- **Current capacity:** All work is done in a single process — sequential URL processing in batch mode (`main.go:128-132`), sequential audio version processing (`download.go:322`), and sequential episode downloading in season mode (`download.go:378-392`).
- **Limit:** Cannot utilize multiple CPU cores for independent downloads (different episodes in a season, different URLs in batch file).
- **Scaling path:** Use a worker pool for batch URLs. Download multiple episodes concurrently when processing a season.

## Dependencies at Risk

### `github.com/iyear/gowidevine` (v0.1.3)

- **Risk:** Pre-v1.0 dependency with no stability guarantees. The API may change without notice. v0.1.3 is already fixed in `go.mod`.
- **Impact:** If this library breaks or is abandoned, the entire DRM decryption pipeline breaks with no alternative. The project has no fallback for Widevine decryption.
- **Migration plan:** Pin to a specific commit. Consider contributing to the upstream to encourage stability. Evaluate `github.com/iyear/gowidevine/ext` for any newer patterns.

### `github.com/unki2aut/go-mpd` (v0.0.0-20250515065241-e261b43d6523)

- **Risk:** Pseudo-version pinned to a specific commit, not a release. This is a fork of a DASH MPD parser with unknown maintenance status. If the forked API changes, the MPD parsing breaks.
- **Impact:** The entire download pipeline depends on MPD parsing — no MPD, no segments, no download.
- **Migration plan:** Evaluate upstream `github.com/Eyevinn/mp4ff` (already an indirect dependency at v0.48.0) for MPD parsing capabilities. Consolidate to fewer forks.

## Missing Critical Features

### No Resume/Partial Download Support

- **Problem:** If a download is interrupted (network failure, crash, user Ctrl+C), partially downloaded segments and tracks are discarded. The download must restart from the beginning.
- **Blocks:** Large season downloads on unstable connections.
- **Priority:** Medium — impactful but mitigated by the segment retry logic.

### No Download Progress Persistence

- **Problem:** In batch mode (file with multiple URLs), if the process crashes or is interrupted after completing URL #5 of #15, all progress for URLs #1-#5 is lost (the output .mkv files exist, but the program state tracking which episodes completed is lost).
- **Blocks:** Long batch download sessions.
- **Priority:** Low — the user can re-run and already-downloaded episodes are skipped via the `os.Stat` check at `download.go:223-226`.

### No FFmpeg Validation at Startup

- **Problem:** The program doesn't check if `ffmpeg` is available on `$PATH` until the merge step at `output.go:91`. If ffmpeg is missing, the user downloads all segments and subtitles, then the merge panics. All temporary work is wasted.
- **Blocks:** Better UX for first-time users who missed the README requirement.
- **Priority:** Medium — the README documents the requirement, but the program should validate at startup.

### No Disk Space Check

- **Problem:** The program does not check available disk space before starting a download. An episode can be 500MB-2GB; a season can be 10GB+. If disk fills up mid-download, the merge step fails with incomplete files.
- **Priority:** Low — hidden by OS `ENOSPC` errors in the merge step.

## Test Coverage Gaps

- **What's not tested:** Everything — the entire codebase has zero test files.
- **Files:** All `.go` files
- **Risk:** Every change is a blind refactor. The URL parsing bug (`main.go:37`) is the clearest example of a defect that would have been caught immediately by a unit test.
- **Priority:** High

---

*Concerns audit: 2026-07-08*
