# External Integrations

**Analysis Date:** 2026-07-08

## APIs & External Services

**Crunchyroll API (primary):**
- **Auth Token** (`token.go:21`) — `POST https://www.crunchyroll.com/auth/v1/token`
  - Grant type: `etp_rt_cookie` (session cookie exchange)
  - Auth header: `Authorization: Basic bm9haWhkZXZtXzZpeWcwYThsMHE6` (hardcoded Basic auth)
  - Cookies: `device_id` (generated UUID), `etp_rt` (user-provided cookie)
  - Returns: JSON `{"access_token": "..."}` — short-lived Bearer token
  - SDK/Client: Go `net/http` (no SDK)

- **CMS (Content Management System) — Season/Episode Metadata** (`season.go`, `episode.go`):
  - `GET https://www.crunchyroll.com/content/v2/cms/series/{id}/seasons?force_locale=&preferred_audio_language={lang}&locale={locale}`
    - Returns list of seasons for a series (`season.go:75`)
  - `GET https://www.crunchyroll.com/content/v2/cms/seasons/{id}/episodes?preferred_audio_language={lang}&locale={locale}`
    - Returns list of episodes for a season (`season.go:34`)
  - `GET https://www.crunchyroll.com/content/v2/cms/objects/{id}?ratings=true&preferred_audio_language=ja-JP&locale=en-US`
    - Returns episode metadata including title, series info, dub versions (`episode.go:88`)
  - Auth: Bearer token (from auth step above) in `Authorization` header
  - User-Agent: Spoofed Firefox/Linux browser string

- **Playback API** (`episode.go:30`):
  - `GET https://www.crunchyroll.com/playback/v3/{id}/web/firefox/play`
    - Returns DASH manifest URL, subtitle URLs, playback token, and stream error
  - Auth: Bearer token
  - SDK/Client: Go `net/http`

- **Stream Token Management** (`episode.go:113`):
  - `DELETE https://www.crunchyroll.com/playback/v1/token/{contentId}/{sToken}`
    - Releases a playback stream after download (prevents stream limit issues)

- **Widevine License** (`drm.go:41`):
  - `POST https://www.crunchyroll.com/license/v1/license/widevine`
    - Headers: `X-Cr-Content-Id`, `X-Cr-Video-Token`, `Authorization: Bearer ...`
    - Body: Widevine challenge (binary/`application/octet-stream`)
    - Returns: JSON `{"license": "<base64-encoded-Widevine-license>"}`
  - SDK/Client: Go `net/http`

**DASH Media CDN:**
- Media segments are downloaded from Crunchyroll's CDN (base URLs extracted from MPD manifest)
  - Origin header: `https://static.crunchyroll.com`
  - Referer header: `https://static.crunchyroll.com/`
  - User-Agent: Spoofed Firefox
  - Retry logic: 5 attempts with exponential backoff (2s × attempt number)

## Data Storage

**Databases:**
- None (no database — the tool downloads files to local disk)

**File Storage:**
- Local filesystem only. Output is MKV files in subdirectories named after the series title.
- Temporary files go to OS temp directory via `os.CreateTemp()` (e.g., `crdl-video-*.mp4`, `crdl-audio-*.mp3`, `crdl-subs-*.ass`).

**Caching:**
- None

## Authentication & Identity

**Auth Provider:**
- **Crunchyroll (custom):** Session-token exchange flow
  - User provides `etp_rt` cookie value (obtained from browser dev tools)
  - Tool exchanges `etp_rt` + generated `device_id` for a Bearer access token via `POST https://www.crunchyroll.com/auth/v1/token`
  - Bearer token is used for all subsequent API calls
  - Auto-refresh on 401: `http_request.go:13` — if any request returns 401 Unauthorized, the token is automatically refetched and the request retried
  - Implementation: `token.go:21` (`GetAccessToken` function)

## Monitoring & Observability

**Error Tracking:**
- None (no Sentry, no error reporting service)

**Logs:**
- Console-based logging via `fmt.Printf` / `fmt.Println` throughout all files
- Debug flag `-debug-manifest` in `main.go:20` enables raw JSON/XML dump of manifests to stdout
- Panic-based error handling used extensively (calls to `panic()` in HTTP, DRM, and file operations)

## CI/CD & Deployment

**Hosting:**
- Not applicable (this is a CLI tool, not a hosted service)

**CI Pipeline:**
- None detected (no `.github/workflows/` directory)

**Build:**
- Standard `go build .` from project root
- Binary output: `crunchyroll-downloader`
- Cross-compilation possible via standard Go tooling (`GOOS= GOARCH= go build`)

## Environment Configuration

**Required env vars:**
- None. All configuration is via CLI flags.

**Secrets location:**
- The `etp_rt` cookie value is passed as a CLI argument (`--etp-rt <value>`). It is NOT read from a file or env var.
- The `.gitignore` excludes `*.wvd`, `client_id.bin`, and `private_key.pem` (Widevine device files).

## Webhooks & Callbacks

**Incoming:**
- None

**Outgoing:**
- `DELETE https://www.crunchyroll.com/playback/v1/token/{contentId}/{sToken}` — stream cleanup after download completes (`episode.go:113`)

---

*Integration audit: 2026-07-08*