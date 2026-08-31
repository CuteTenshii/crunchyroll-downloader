# Crunchyroll Downloader

Downloads anime from Crunchyroll and outputs them in a MKV file.

Use the downloader only with an authorized account and content you are entitled to access. Provider restrictions and account enforcement are outside this tool's control.

## Features

- Supports choosing the audio and subtitles language, including downloading multiple of each into a single file
- Supports choosing the audio and video quality
- Decrypts Widevine DRM (requires: a `.wvd` file or `client_id.bin` and `private_key.pem` files)
- Adds metadata (like episode name) to the MKV container
- Parallel segment downloads (10 workers) for faster downloads
- Retry with backoff on connection errors
- Batch download from a list of URLs

## Requirements

- [FFmpeg](https://www.ffmpeg.org/download.html#get-packages)
- To download Premium-only content, a Crunchyroll Premium account. No, this can't be bypassed and a free trial should be enough
- For full-media downloads only, an operator-owned and lawfully provisioned `.wvd` file or matching raw device files, stored outside the repository as mode `0600` and selected with `CRUNCHYROLL_WIDEVINE_DEVICE_FILE` (or the paired raw-file environment variables). Subtitle-only indexing does not require a Widevine device.

## Download

Check the [latest release](https://github.com/CuteTenshii/crunchyroll-downloader/releases/latest) and download the file that corresponds to your OS.

## Usage

- Open a Terminal/Command prompt, and go to the folder where you downloaded the binary/cloned the repo
- Run the program with the options you want:
```shell
Usage of ./crunchyroll-downloader:
  -audio-lang string
        Audio language(s), comma-separated for multiple (e.g. "ja-JP,en-US"). First is the default track (default "ja-JP")
  -audio-quality string
        Audio quality (default "192k")
  -etp-rt-file string
        Path to a 0600 regular file containing the "etp_rt" cookie of your account
  -season int
        Season number. Not used if an episode link is entered
  -subs-lang string
        Subtitle language(s), comma-separated for multiple (e.g. "en-US,es-419"). First is the default track (default "en-US")
  -cc-lang string
        Closed caption language(s), comma-separated for multiple. Downloaded in addition to -subs-lang, not instead of it
  -url string
        URL of the episode/season to download
  -file string
        Path to a text file with one URL per line
  -video-quality string
        Video quality (default "1080p")
```

Ex: to download the first season of *Hell's Paradise*:
```shell
./crunchyroll-downloader --url https://www.crunchyroll.com/series/GJ0H7Q5ZJ/hells-paradise --season 1 --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt
```

To download a specific episode:
```shell
./crunchyroll-downloader --url https://www.crunchyroll.com/watch/GE00198973JAJP/dawn-and-confusion --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt
```

To batch download from a file (one URL per line):
```shell
./crunchyroll-downloader --file list.txt --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt --subs-lang pt-BR
```

To download multiple audio tracks and subtitles into a single file (the first of each is set as the default track). If any requested language is missing for an episode, that episode is skipped:
```shell
./crunchyroll-downloader --url https://www.crunchyroll.com/watch/GE00198973JAJP/dawn-and-confusion --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt --audio-lang ja-JP,en-US --subs-lang en-US,es-419,de-DE
```

## Resumable subtitle indexing

`--index-subs` opens playback only to collect the exact requested subtitle
locale. It does not load a Widevine CDM. Authentication remains file-only:
provide `--etp-rt-file` pointing at a regular mode-`0600` operator-private
file; do not put a cookie in an environment variable or command-line value.

```shell
./crunchyroll-downloader \
  --url https://www.crunchyroll.com/series/GJ0H7Q5ZJ/hells-paradise \
  --index-subs \
  --subs-lang en-US \
  --index-window 25 \
  --index-terminal-recheck-window 3 \
  --etp-rt-file ~/.config/crunchyroll-downloader/etp_rt.txt
```

Every attempted provider identity is checkpointed atomically before the next
one. The default bounded window is 25 identities. Verified raw `.ass` bytes
are reused by SHA-256 and are never redownloaded merely to regenerate derived
telemetry. The global playback circuit can pause later attempts with a
cooldown after consecutive provider code `4294` responses.

Code `4294` by itself is recorded as
`subtitle_unknown_provider_playback_restriction`; it is not evidence of a
specific provider cause. `subtitle_provider_rate_limited` is used only when
the HTTP response or direct rate-limit headers provide supporting evidence.
The private run summary uses matching outcomes
`unknown_provider_playback_restriction` and `provider_rate_limited` when the
circuit is open.

Each checksum-verified subtitle cache records `subtitle_cue_quality` with
`healthy` or `degraded` outcome, plus counts for malformed, empty, and skipped
ASS dialogue cues. A cache is marked degraded when it has no usable cues or
when malformed cues exceed 5%, empty cues exceed 20%, or skipped cues exceed
5%. Degraded is a quality observation, not a reason to overwrite or
redownload the original raw ASS bytes.

Rows recorded as `subtitle_missing_locale` or `subtitle_permanent_failed` do
not receive a full historical playback sweep. A stable per-episode source
version and catalog snapshot permit only a deterministic, newest-first sparse
recheck when provider metadata changes, capped by
`--index-terminal-recheck-window` and never above `--index-window`.

For a multi-URL `--file` index run, provider-call counters are reset for each
catalog summary. Omit `--index-run-summary` so each catalog uses its own
default summary path; one explicit summary path is rejected for multiple URLs
instead of being overwritten.

## Building

### Requirements

- [Go](https://go.dev/dl/) 1.25+
- [FFmpeg](https://www.ffmpeg.org/download.html#get-packages) on `PATH` (for downloads)
- **GUI on Windows:** [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) (preinstalled on most Windows 10/11 systems)
- Optional for packaged GUI builds: [Wails CLI](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### CLI

```shell
# Windows
go build -o crunchyroll-downloader.exe ./cmd/cli

# macOS / Linux
go build -o crunchyroll-downloader ./cmd/cli
```

### GUI

Wails apps **require build tags**. A plain `go build ./cmd/gui` without tags will show a dialog and refuse to run.

```shell
# Windows (recommended with Go only — note the tags)
go build -tags "desktop,production" -ldflags "-w -s -H windowsgui" -o crunchyroll-downloader-gui.exe ./cmd/gui

# Or use the Wails CLI (same tags under the hood)
# go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd cmd/gui
wails build

# PowerShell helper from repo root
# .\scripts\build-gui.ps1
```

```shell
# macOS / Linux
go build -tags "desktop,production" -ldflags "-w -s" -o crunchyroll-downloader-gui ./cmd/gui
```

Windows needs [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) (usually already installed with Edge).

## GUI usage

### Home (Discover)

The GUI opens on **Home**, a Crunchyroll-like Discover feed from the same catalog APIs as the website (no Widevine, no playback).

1. Set your **cookie file** (`etp_rt` path) via **Cookie** or the **Account** menu
2. Browse the feed, or **Search** for a series/movie
3. Click a poster/card to open **Download**, fill the series/episode URL, and auto-**Inspect**

**Feed blocks** (provider order, when available):

| Block | What you see |
|-------|----------------|
| **Hero** | Full-bleed featured slides |
| **Continue watching** | Landscape rail with playhead progress |
| **Watchlist** | Poster rail of saved titles |
| **Banners** | Promotional banner rows |
| **Poster / landscape rails** | Horizontal title rails (browse/history/etc.) |
| **Top 10 chrome** | Rank badges (1–10) on rails that look like Top 10 lists |

Discover locale defaults to **`pt-BR`** (stored in preferences as `locale`). If the personalized feed fails, Home falls back to a popular-series browse list.

### Account menu

The header **Account** control manages:

- **Cookie profiles** — named local paths to `etp_rt` files (switch accounts without re-picking a path each time). Add a profile from the menu; only file paths are stored, never the raw cookie.
- **CR multiprofile** — when the active cookie supports Crunchyroll multiprofiles, list and switch the provider profile used for Discover personalization (watchlist, continue watching, etc.).

Use **Cookie** for a one-off path, or save it as a named cookie profile from Account.

### Download

1. Paste a series or episode **URL** (or open a title from Home — that switches to Download and runs **Inspect**)
2. Ensure a **cookie file** / cookie profile is selected (the raw value is never stored)
3. Press **Inspect** to load seasons, episodes, and language options
4. **Select** what you want (episodes, audio, subs, quality)
5. Press **Download**

### GUI Play

Play opens an **in-window cinema overlay** (libmpv) inside this app. It is not a second player window and does not wrap crunchyroll.com.

1. Place `libmpv-2.dll` next to the GUI executable
2. From **Home → Continue Watching**, use **Play** to start the episode (progressive download if no completed MKV is on disk). Clicking the rest of the card still **Inspects** for Download
3. From **Downloads**, **Play** uses the selected episode
4. Quality is **forced** — the same representation pick as download. Never ABR
5. Watch progress is POSTed to Crunchyroll playheads **only on pause, close, or finish** (not while playing)

If the DLL is missing, the overlay shows `player library missing` and does not launch another app.

**Widevine:** Home and subtitle-only indexing need no CDM. Play and full-media download need an operator-owned, lawfully provisioned device selected only via explicit environment variables or GUI prefs (`CRUNCHYROLL_WIDEVINE_DEVICE_FILE`, or the paired raw-file variables). Never use leaked, extracted, shared, or third-party credentials.

### GUI defaults (first run / after Inspect)

| Setting | Default |
|---------|---------|
| Main view | **Home** (Discover feed) |
| Discover locale | **pt-BR** |
| Episodes | **S1E1** only (or the single `/watch/…` title) |
| Audio | **Original** (usually `ja-JP`) |
| Subtitles | **None** |
| Video / audio quality | **Max available** after probe |

Preferences (URL, cookie path, cookie profiles, output folder, last selections — never raw secrets) are saved under:

- Windows: `%AppData%\crunchyroll-downloader\preferences.json`
- macOS/Linux: `~/.config/crunchyroll-downloader/preferences.json`

## Help

### How do I get my `etp_rt` cookie?

- Go to https://crunchyroll.com
- Open Developer Tools
- Firefox: Go to *Storage* then *Cookies*<br />Chrome: Go to *Application* then *Cookies*
- Select the Crunchyroll domain, then copy the `etp_rt` cookie value

![](.github/screenshots/etp-rt-cookie.png)

### What is a `.wvd` file and do I really need one?

Full-media downloads require an operator-owned, lawfully provisioned device credential. Subtitle-only indexing does not require one. Keep device credentials outside the repository in private `0600` storage; never use leaked, extracted, shared, or third-party credentials.

## License

This project is licensed under the MIT License. See [LICENSE.txt](LICENSE.txt)
