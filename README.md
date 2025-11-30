# Crunchyroll Downloader

Downloads anime from Crunchyroll and outputs them in a MKV file.

## Features

- Supports choosing the audio and subtitles language
- Supports choosing the audio and video quality
- Decrypts Widevine DRM (requires a .wvd file, which can't be provided here for legal reasons. Search on Google to create/get one)
- Adds metadata (like episode name) to the MKV container

## Download

Check the [latest release](https://github.com/CuteTenshii/crunchyroll-downloader/releases/latest) and download the file associated that corresponds to your OS.

## Requirements

- [Go](https://go.dev/dl/)
- To download Premium-only content, a Crunchyroll Premium account. No, this can't be bypassed and a free trial should be enough
- A `.wvd` file.

## Usage

- Clone/Download this repository. You can use the green "Code" button, then click on "Download ZIP"
- Open a Terminal/Command prompt in the folder of the repository.
- Run `go build .`

```shell
Usage of ./crunchyroll-downloader:
  -audio-lang string
        Audio language (default "jp-JP")
  -audio-quality string
        Audio quality (default "192k")
  -etp-rt string
        Idk what this means. This is the cookie value on your browser tho
  -subtitles-lang string
        Subtitles language (default "en-US")
  -url string
        URL of the episode/season to download
  -video-quality string
        Video quality (default "1080p")
```

## Help

### How do I get the `etp_rt` cookie value?

This cookie is used to get an access token for your account.

![](.github/screenshots/etp-rt-cookie.png)

### What is a `.wvd` file?? Do I really need one?

Yes, Crunchyroll uses DRM-only content. This file is used to get a Widevine license, which gives the keys to decrypt the media.
