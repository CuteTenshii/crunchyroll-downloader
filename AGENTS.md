# Agent Guide — Crunchyroll Downloader

This document describes the subtitle-indexing and full-media workflows. Subtitle-only indexing does not require a Widevine device. Full-media downloads require an operator-owned, lawfully provisioned Widevine credential. Written for macOS (Apple Silicon), but the steps generalize to Linux/Windows.

---

## 1. What this tool does

Downloads DRM-protected anime from Crunchyroll and outputs a decrypted `.mkv` file containing video (H.264), audio (AAC), and subtitles (ASS). The Widevine CDM is used **only at download time** to decrypt the stream — the output MKV is a plain, DRM-free file that plays in any player and can be clipped freely with no keys or CDM needed afterward.

**Pipeline** (see `download.go:190`–`downloadEpisode`):
1. Authenticate with the `etp_rt` cookie → get an access token (`token.go`)
2. Fetch episode metadata + playback info (`episode.go`)
3. Parse the DASH manifest (`mpd.go`)
4. Load the `.wvd` CDM and obtain a Widevine license (`drm.go`)
5. Download encrypted audio + video segments in parallel (10 workers)
6. Decrypt segments using the license keys (`download.go:153`)
7. Merge video + audio + subtitles into a single MKV via ffmpeg (`output.go`)
8. Clean up temp files

## 2. Requirements

| Requirement | How to install (macOS) |
|---|---|
| [Go](https://go.dev/dl/) 1.25+ | `brew install go` |
| [FFmpeg](https://ffmpeg.org) | `brew install ffmpeg` |
| Crunchyroll Premium account | — (needed for premium-only content) |
| Widevine CDM (`.wvd` file) | Full-media workflow only; see §4 below |

Verify:
```bash
go version      # expect go1.25+
ffmpeg -version # expect 5.x+
```

## 3. Setup

```bash
# Clone the repo
git clone https://github.com/CuteTenshii/crunchyroll-downloader.git
cd crunchyroll-downloader

# Build the binary
go build .
# → produces ./crunchyroll-downloader
```

## 4. Widevine CDM (`.wvd` file)

Crunchyroll streams are DRM-protected. Full-media operation needs a Widevine L3 credential to obtain decryption keys. The downloader reads credentials only from explicit private-file environment variables (`drm.go`–`getWidevineDevice`):

- `CRUNCHYROLL_WIDEVINE_DEVICE_FILE=/private/path/device.wvd` — **preferred, single file**
- Or both `CRUNCHYROLL_WIDEVINE_CLIENT_ID_FILE` and `CRUNCHYROLL_WIDEVINE_PRIVATE_KEY_FILE`

Each path must resolve directly to a regular mode-`0600` file outside the repository; symlinks and broader modes fail closed. The downloader no longer scans the working tree for device credentials.

### Credential provenance

Use only a Widevine credential that the operator lawfully owns or is authorized to use. Never obtain, recommend, or use leaked, extracted, shared, or third-party device credentials. Subtitle-only indexing does not load a CDM and must not be blocked on one.

### How to place it

Keep the credential outside the repository in operator-private storage with mode `0600`, then select it by path without copying it into the working tree:
```bash
install -m 0600 /path/to/operator-owned.wvd ~/.config/crunchyroll-downloader/device.wvd
export CRUNCHYROLL_WIDEVINE_DEVICE_FILE="$HOME/.config/crunchyroll-downloader/device.wvd"
```

The downloader never scans its working directory for a device. Keep the configured path in operator-owned runtime configuration and do not copy the credential into the repository.

### CDMs get revoked

An operator-owned device can become unusable or revoked. A device may still **load** but fail at license time. If that happens, stop the full-media workflow and have the operator provision another authorized device; do not search for shared credentials.

### Verifying a CDM (optional)

A test file (`cdm_test.go`) is included that checks whether a `.wvd` parses correctly through the `gowidevine` library — verifying the RSA private key and DRM certificate are valid. This does **not** check revocation (only a live license request can do that):

```bash
go test -run TestLoadWVD -v
```

### Creating a `.wvd` from raw files (pywidevine)

If you have `client_id.bin` and `private_key.pem` but want a single `.wvd`:
```bash
pip install pywidevine
pywidevine create-device -k private_key.pem -c client_id.bin -t CHROME -l 3 -o .
```
However, **this is optional** — the Go downloader accepts the raw files directly.

## 5. Authentication (`etp_rt` cookie)

The downloader needs your Crunchyroll `etp_rt` cookie to authenticate.

### How to get it

1. Go to https://crunchyroll.com and log in
2. Open Developer Tools (F12 or Ctrl+Shift+I)
3. **Chrome/Edge**: Application → Cookies → crunchyroll.com
   **Firefox**: Storage → Cookies → crunchyroll.com
4. Find the cookie named `etp_rt` and copy its value

### How to store it safely

The `etp_rt` value is a secret. Save it outside the repository in an operator-private `0600` file:

```bash
install -d -m 0700 ~/.config/crunchyroll-downloader
install -m 0600 /path/to/newly-provisioned-etp-rt ~/.config/crunchyroll-downloader/etp_rt.txt
```

Pass only the file path. Never put the value in argv, shell substitution, logs, receipts, prompts, fixtures, or Git:
```bash
./crunchyroll-downloader --url "..." --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt
```

### Token expiry

Access tokens expire. The downloader handles this automatically — `http_request.go:13`–`19` detects a 401, refetches a token using the stored `etp_rt`, and retries. You don't need to do anything.

## 6. Usage

### Flags

| Flag | Default | Description |
|---|---|---|
| `-url` | — | URL of an episode (`/watch/...`) or series (`/series/...`) |
| `-file` | — | Path to a text file with one URL per line (batch) |
| `-etp-rt-file` | — | Path to a regular `0600` file containing the `etp_rt` cookie (**required**) |
| `-season` | `0` | Season number (only for `/series/` URLs; `0` = all seasons) |
| `-audio-lang` | `ja-JP` | Audio language(s), comma-separated. First = default track. Use `all` for every available dub. |
| `-subs-lang` | `en-US` | Subtitle language(s), comma-separated. First = default track. Use `all` for every available sub. |
| `-video-quality` | `1080p` | Video quality (see §7) |
| `-audio-quality` | `192k` | Audio quality (see §7) |
| `-debug-manifest` | `false` | Print raw playback JSON and manifest XML |

### Examples

```bash
# Single episode (Japanese audio + English subs)
./crunchyroll-downloader \
  --url "https://www.crunchyroll.com/watch/GE00350806JAJP/the-long-sought-elbaph-the-big-reunion-banquet" \
  --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt

# Entire season
./crunchyroll-downloader \
  --url "https://www.crunchyroll.com/series/GJ0H7Q5ZJ/hells-paradise" \
  --season 1 \
  --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt

# All seasons of a series
./crunchyroll-downloader \
  --url "https://www.crunchyroll.com/series/GJ0H7Q5ZJ/hells-paradise" \
  --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt

# English dub audio
./crunchyroll-downloader \
  --url "https://www.crunchyroll.com/watch/GE00350806JAJP/..." \
  --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt \
  --audio-lang en-US

# Multiple audio + subtitle tracks in one file (first of each = default)
./crunchyroll-downloader \
  --url "https://www.crunchyroll.com/watch/GE00350806JAJP/..." \
  --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt \
  --audio-lang ja-JP,en-US \
  --subs-lang en-US,es-419,de-DE

# Download ALL available dubs + ALL subtitles
./crunchyroll-downloader \
  --url "https://www.crunchyroll.com/watch/GE00350806JAJP/..." \
  --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt \
  --audio-lang all \
  --subs-lang all

# Batch download from a file (one URL per line)
./crunchyroll-downloader \
  --file list.txt \
  --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt \
  --subs-lang en-US
```

### Output location

Files are saved to `<SeriesTitle>/<SeriesTitle> S<SS>E<EE> - <EpisodeTitle> [<quality>].mkv` relative to the working directory. If the file already exists, the episode is skipped (`download.go:223`).

### Debugging

If a download fails or behaves unexpectedly, add `--debug-manifest` to see the raw API responses:
```bash
./crunchyroll-downloader --url "..." --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt --debug-manifest
```

## 7. Available qualities

### Video quality (`-video-quality`)

Set via the `-video-quality` flag as a height string. The code (`mpd.go:42`–`46`) matches the flag against the `Height` field of each representation in the manifest. Common values Crunchyroll serves:

| Flag value | Resolution |
|---|---|
| `1080p` | 1920×1080 (default; requires Premium) |
| `720p` | 1280×720 |
| `480p` | 854×480 |
| `360p` | 640×360 |

**Notes:**
- Not all resolutions are available for every episode — depends on what's in the manifest.
- If the requested quality isn't found, the code falls back to the first representation and prints a warning (`mpd.go:69`–`71`). The download will still succeed, just at a different quality.
- Premium (1080p) content requires a Premium account. The `etp_rt` cookie from a free account will fail on premium-only streams.

### Audio quality (`-audio-quality`)

Set via the `-audio-quality` flag. The code (`mpd.go:52`–`63`) matches by bandwidth. Supported values:

| Flag value | Bitrate |
|---|---|
| `192k` | ~192 kbps (default) |
| `128k` | ~128 kbps |
| `96k` | ~96 kbps |

If the requested quality isn't found, it falls back to the first available audio representation (`mpd.go:69`–`71`).

## 8. Supported languages

The downloader recognizes these locale codes (defined in `utils.go`). Use them with `-audio-lang` and `-subs-lang`:

| Code | Language |
|---|---|
| `ja-JP` | 日本語 (Japanese) — default audio |
| `en-US` | English — default subs |
| `en-IN` | English (India) |
| `es-419` | Español (Latin America) |
| `es-ES` | Español (España) |
| `pt-BR` | Português (Brasil) |
| `pt-PT` | Português (Portugal) |
| `fr-FR` | Français |
| `de-DE` | Deutsch |
| `it-IT` | Italiano |
| `ca-ES` | Català |
| `ru-RU` | Русский |
| `ar-SA` | العربية |
| `hi-IN` | हिंदी |
| `ta-IN` | தமிழ் |
| `te-IN` | తెలుగు |
| `ko-KR` | 한국어 |
| `zh-CN` | 中文 (普通话) |
| `zh-HK` | 中文 (粵語) |
| `zh-TW` | 中文 (國語) |
| `id-ID` | Bahasa Indonesia |
| `ms-MY` | Bahasa Melayu |
| `vi-VN` | Tiếng Việt |
| `th-TH` | ไทย |
| `pl-PL` | Polski |
| `tr-TR` | Türkçe |

**Important:** Not every episode has every language. Available dubs depend on the episode. If a requested audio language is missing, the episode is **skipped** (`download.go:263`). If a requested subtitle language is missing, the episode is **skipped** (`download.go:304`). Use `all` to get only what's actually available.

## 9. Subtitle format (ASS → SRT conversion)

**The downloader fetches subtitles in ASS format** (Advanced SubStation Alpha) — this is hardcoded in the Crunchyroll API (`episode.go:14`–`27`). Subtitles are embedded in the MKV as-is using stream copy (`output.go:48`).

ASS is a rich subtitle format that supports styling, positioning, and effects. Most modern players (VLC, mpv, MPC-HC, Plex, Jellyfin) support ASS natively, so the embedded subs will display correctly.

### Converting to SRT

If you specifically need SRT format (e.g., for a player or tool that doesn't support ASS), convert after download using ffmpeg. SRT is plain text only — styling, positioning, and effects are lost:

```bash
# Extract and convert the English subtitle track to SRT
ffmpeg -i "Series Title S01E01 - Episode [1080p].mkv" \
  -map 0:s:0 -c:s srt \
  "episode.srt"
```

If the MKV has multiple subtitle tracks, select by index:
```bash
# List all streams to find the right subtitle index
ffprobe "Series Title S01E01 - Episode [1080p].mkv"

# Extract the 2nd subtitle track (0:s:1)
ffmpeg -i "input.mkv" -map 0:s:1 -c:s srt "episode_es.srt"
```

### Batch-convert all subtitles in an MKV to SRT

```bash
# Get subtitle stream count
SUB_COUNT=$(ffprobe -v error -select_streams s -show_entries stream=index -of csv=p=0 "input.mkv" | wc -l)

for i in $(seq 0 $((SUB_COUNT - 1))); do
  ffmpeg -i "input.mkv" -map "0:s:$i" -c:s srt "subtitle_$i.srt"
done
```

## 10. Clipping the output

The output MKV is fully decrypted and DRM-free. You can clip it with ffmpeg without any CDM or keys:

```bash
# Lossless clip (no re-encode) — fast, preserves quality
ffmpeg -i "input.mkv" \
  -ss 00:10:00 -to 00:12:30 \
  -c copy \
  "clip.mkv"

# Clip with re-encode (use if -c copy produces A/V sync issues at cut points)
ffmpeg -i "input.mkv" \
  -ss 00:10:00 -to 00:12:30 \
  -c:v libx264 -c:a aac \
  "clip.mp4"
```

The clips play in any player — no Widevine, no license server, no CDM.

## 11. File layout

```
crunchyroll-downloader/
├── main.go              # CLI entry point, flag parsing, URL dispatch
├── token.go             # Crunchyroll OAuth token exchange (etp_rt → access_token)
├── http_request.go      # HTTP wrapper with auto token-refresh on 401
├── episode.go           # Episode metadata + playback API, subtitle URLs
├── season.go            # Season/series listing API
├── mpd.go               # DASH manifest parsing, quality selection
├── download.go          # Segment downloading (parallel), decryption, episode orchestration
├── drm.go               # Widevine CDM loading, license challenge/response
├── output.go            # ffmpeg merge → MKV, metadata tagging
├── utils.go             # Language name/code maps
├── go.mod / go.sum      # Go dependencies
├── .gitignore           # Ignores *.wvd, *.mkv, etp_rt.txt, .env
├── cdm_test.go          # Optional test: verifies .wvd loads correctly
└── crunchyroll-downloader  # Built binary
```

Authentication and device files belong in private operator storage outside this tree; only their paths are configured at runtime.

### Key dependencies

| Package | Purpose |
|---|---|
| `github.com/iyear/gowidevine` | Widevine CDM, license challenge, MP4 decryption |
| `github.com/unki2aut/go-mpd` | DASH manifest (MPD) parsing |
| `github.com/google/uuid` | Random device ID for auth |
| `github.com/Eyevink/mp4ff` | MP4 box parsing (indirect, for decryption) |

## 12. Troubleshooting

| Problem | Cause | Fix |
|---|---|---|
| `no widevine device provided` | A full-media run has no explicit authorized device-file environment configuration | Provision an authorized device in private `0600` storage and set the documented file variable; subtitle-only indexing needs none |
| `getLicense` error / no keys | The operator-owned device may be invalid or revoked | Stop and have the operator provision another authorized device |
| `Access token expired` message | Normal — token auto-refreshes | Nothing; it retries automatically |
| `Audio locale X is not available` | Episode doesn't have that dub | Remove it from `-audio-lang` or use `all` |
| `Subtitle locale X is not available` | Episode doesn't have that sub | Remove it from `-subs-lang` or use `all` |
| `failed to get the video base URL` | Wrong `-video-quality` value | Check available qualities; use `1080p`, `720p`, etc. |
| `failed to get the audio base URL` | Wrong `-audio-quality` value | Use `192k`, `128k`, or `96k` |
| `ffmpeg failed` | ffmpeg not installed or path issue | `brew install ffmpeg`, verify `which ffmpeg` |
| Episode skipped silently | Output file already exists | Delete or rename the existing `.mkv` |
| 401 on premium content | Free account, not Premium | Use a Premium account's `etp_rt` |

## 13. Security notes

- Keep `etp_rt` and device credentials outside the repository in `0700` directories and `0600` regular files; ignore rules are defense in depth, not storage policy.
- The `etp_rt` cookie grants access to your Crunchyroll account. Treat it like a password.
- Never pass a raw credential in argv or record it in logs, prompts, fixtures, receipts, or Git.
- Use only operator-owned or lawfully provisioned Widevine credentials. Never use leaked, extracted, shared, or third-party device credentials.
- The downloader authenticates as your account. Download only content you have legitimate access to.
- Decrypted output files are DRM-free. Respect Crunchyroll's terms of service regarding offline copies.
