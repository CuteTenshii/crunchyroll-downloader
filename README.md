# Crunchyroll Downloader

Downloads anime from Crunchyroll and outputs them in a MKV file.

You won't be banned or anything, I downloaded all Kaguya-Sama seasons to test during 30 mins and everything went fine

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
- Either a `.wvd` file, or a `client_id.bin` and `private_key.pem`

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
  -etp-rt string
        The "etp_rt" cookie value of your account
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
./crunchyroll-downloader --url https://www.crunchyroll.com/series/GJ0H7Q5ZJ/hells-paradise --season 1 --etp-rt replace_this
```

To download a specific episode:
```shell
./crunchyroll-downloader --url https://www.crunchyroll.com/watch/GE00198973JAJP/dawn-and-confusion --etp-rt replace_this
```

To batch download from a file (one URL per line):
```shell
./crunchyroll-downloader --urls list.txt --etp-rt replace_this --subs-lang pt-BR
```

To download multiple audio tracks and subtitles into a single file (the first of each is set as the default track). If any requested language is missing for an episode, that episode is skipped:
```shell
./crunchyroll-downloader --url https://www.crunchyroll.com/watch/GE00198973JAJP/dawn-and-confusion --etp-rt replace_this --audio-lang ja-JP,en-US --subs-lang en-US,es-419,de-DE
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

Yes, Crunchyroll uses DRM-only content. This file is used to get a Widevine license, which gives the keys to decrypt the media.

If you don't have a rooted Android device or are just lazy, search "ready to use cdms" and you'll find plenty of websites providing those files.

## Badges

[![Go Version](https://img.shields.io/badge/Go-1.25.0-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE.txt)

## Installation

### Install with Go

If you have Go 1.25+ installed, you can install the binary directly:

```bash
go install github.com/CuteTenshii/crunchyroll-downloader@latest
```

### Pre-built binaries

Pre-built binaries for Linux, macOS, and Windows are available on the
[releases page](https://github.com/CuteTenshii/crunchyroll-downloader/releases/latest).
See the **Download** section above for details.

### Build from source

Clone the repository and run `go build .` in the project root. See the
**Building** section above for detailed instructions.

## Quick Start

1. **Get your `etp_rt` cookie** &mdash; Follow the instructions in the
   **Help** section below. This cookie authenticates you with Crunchyroll.

2. **Install the binary** &mdash; Use one of the methods in the
   **Installation** or **Download** sections above.

3. **Run the downloader** with your cookie and a Crunchyroll URL:

   ```bash
   ./crunchyroll-downloader \
     --url https://www.crunchyroll.com/series/GJ0H7Q5ZJ/hells-paradise \
     --season 1 \
     --etp-rt your_cookie_here
   ```

   The episodes will be saved as MKV files in the current directory.

   For a single episode:

   ```bash
   ./crunchyroll-downloader \
     --url https://www.crunchyroll.com/watch/GE00198973JAJP/dawn-and-confusion \
     --etp-rt your_cookie_here
   ```

Alternatively, set your credentials in a `.env` file (see `.env.example`) to
avoid passing `--etp-rt` on every invocation:

```bash
echo "CRUNCHYROLL_ETP_RT=your_cookie_here" > .env
./crunchyroll-downloader --url <url>
```

## License

This project is licensed under the MIT License. See [LICENSE.txt](LICENSE.txt)