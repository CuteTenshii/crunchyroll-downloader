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

```shell
# Windows (plain go build; requires WebView2)
go build -o crunchyroll-downloader-gui.exe ./cmd/gui

# Or package with Wails from the GUI module
cd cmd/gui && wails build
```

On macOS/Linux the same `go build -o crunchyroll-downloader-gui ./cmd/gui` pattern applies (platform webview dependencies apply).

## GUI usage

1. Paste a series or episode **URL**
2. Choose your **cookie file** (`etp_rt` path; the raw value is never stored)
3. Press **Inspect** to load seasons, episodes, and language options
4. **Select** what you want (episodes, audio, subs, quality)
5. Press **Download**

### GUI defaults (first run / after Inspect)

| Setting | Default |
|---------|---------|
| Episodes | **S1E1** only (or the single `/watch/…` title) |
| Audio | **Original** (usually `ja-JP`) |
| Subtitles | **None** |
| Video / audio quality | **Max available** after probe |

Preferences (URL, cookie path, output folder, last selections — never raw secrets) are saved under:

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
