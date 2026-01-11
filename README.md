# Crunchyroll Downloader

Downloads anime from Crunchyroll and outputs them in a MKV file.

## Features

- Supports choosing the audio and subtitles language
- Supports choosing the audio and video quality
- Decrypts Widevine DRM (requires a .wvd file, which can't be provided here for legal reasons. Search on Google to create/get one)
- Adds metadata (like episode name) to the MKV container

## Download

Check the [latest release](https://github.com/CuteTenshii/crunchyroll-downloader/releases/latest) and download the file that corresponds to your OS.

## Requirements

- [Go](https://go.dev/dl/)
- To download Premium-only content, a Crunchyroll Premium account. No, this can't be bypassed and a free trial should be enough
- A `.wvd` file.

## Usage

- Clone this repository
- Open a Terminal/Command prompt, and go to the folder where you cloned the repo
- Run `go build .`
- Run the program with the executable built

```shell
Usage of ./crunchyroll-downloader:
  -audio-lang string
        Audio language (default "jp-JP")
  -audio-quality string
        Audio quality (default "192k")
  -etp-rt string
        The "etp_rt" cookie value of your account
  -subtitles-lang string
        Subtitles language (default "en-US")
  -url string
        URL of the episode/season to download
  -video-quality string
        Video quality (default "1080p")
```

## Help

### How do I get my `etp_rt` cookie?

- Go to https://crunchyroll.com
- Open Developer Tools
- Firefox: Go to *Storage* then *Cookies*<br />Chrome: Go to *Application* then *Cookies*
- Select the Crunchyroll domain, then copy the `etp_rt` cookie value

![](.github/screenshots/etp-rt-cookie.png)

### What is a `.wvd` file and do I really need one?

Yes, Crunchyroll uses DRM-only content. This file is used to get a Widevine license, which gives the keys to decrypt the media.
