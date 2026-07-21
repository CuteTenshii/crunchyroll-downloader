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
        (or set the CRUNCHYROLL_ETP_RT environment variable — the raw value never
        goes on the command line, where any local process could read it from argv)
  -season int
        Season number. Not used if an episode link is entered
  -subs-lang string
        Subtitle language(s), comma-separated for multiple (e.g. "en-US,es-419"). First is the default track (default "en-US")
  -url string
        URL of the episode/season to download
  -file string
        Path to a text file with one URL per line
  -video-quality string
        Video quality (default "1080p")
```

Ex: to download the first season of *Hell's Paradise*:
```shell
./crunchyroll-downloader --url https://www.crunchyroll.com/series/GJ0H7Q5ZJ/hells-paradise --season 1 --etp-rt-file ~/.config/crunchyroll/etp_rt.txt
```

To download a specific episode:
```shell
./crunchyroll-downloader --url https://www.crunchyroll.com/watch/GE00198973JAJP/dawn-and-confusion --etp-rt-file ~/.config/crunchyroll/etp_rt.txt
```

To batch download from a file (one URL per line):
```shell
./crunchyroll-downloader --urls list.txt --etp-rt-file ~/.config/crunchyroll/etp_rt.txt --subs-lang pt-BR
```

To download multiple audio tracks and subtitles into a single file (the first of each is set as the default track). If any requested language is missing for an episode, that episode is skipped:
```shell
./crunchyroll-downloader --url https://www.crunchyroll.com/watch/GE00198973JAJP/dawn-and-confusion --etp-rt-file ~/.config/crunchyroll/etp_rt.txt --audio-lang ja-JP,en-US --subs-lang en-US,es-419,de-DE
```

## Building

### Requirements

- [Go](https://go.dev/dl/)

### Guide

- Clone this repository
- Open a Terminal/Command prompt, and go to the folder where you cloned the repo
- Run `go build .`

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
