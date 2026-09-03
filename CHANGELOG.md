# Changelog

## 1.5.0

- Audio dubs, subtitles, closed captions and the video track now download concurrently instead of one after another, making multi-language downloads much faster
- Added progress bars showing percentage, segment count and download speed (Mbps) for every active download
- Playback errors now surface the API's error reason, and rate-limited (429) responses print a hint to wait or use a different account ([#47](https://github.com/CuteTenshii/crunchyroll-downloader/issues/47))
- Fixed a bug where retrying a request after refreshing an expired access token sent an empty or truncated body, breaking license requests

## 1.4.0

- Added support for on-demand (single-file) manifests and normalized non-Widevine PSSH system IDs, fixing "PSSH not found" and PlayReady errors on some episodes ([#8](https://github.com/CuteTenshii/crunchyroll-downloader/issues/8), [#38](https://github.com/CuteTenshii/crunchyroll-downloader/issues/38), [#39](https://github.com/CuteTenshii/crunchyroll-downloader/issues/39))
- Fixed a crash when the playback response "error" field is a boolean or number instead of a string ([#29](https://github.com/CuteTenshii/crunchyroll-downloader/issues/29))
- Fixed audio language selection so dubs resolve from the versions list instead of silently downloading the original language ([#35](https://github.com/CuteTenshii/crunchyroll-downloader/issues/35))
- Unavailable subtitle and caption locales are now skipped instead of aborting the whole episode ([#27](https://github.com/CuteTenshii/crunchyroll-downloader/issues/27))
- Unavailable audio locales are now skipped, downloading whichever of the requested languages are available ([#28](https://github.com/CuteTenshii/crunchyroll-downloader/issues/28))
- Fixed URL parsing for locale-prefixed links such as /fr/series/...

## 1.3.0

- Large downloads no longer buffer the entire file in memory. Segments are written to disk as they arrive, so memory use no longer scales with the length of the title — movie-length media previously crashed with an out-of-memory error
- The decryption key is now selected by matching each track's key ID, instead of always using the first key in the Widevine license, which could decrypt to garbage
- Added `--cc-lang` to download closed captions (dubtitles) in addition to `--subs-lang`
- `--audio-lang all` and `--subs-lang all` download every available track
- Subtitles no longer depend on which audio track came first
- Added CI: tests, cross-platform builds, vet and gofmt on every push and pull request

## 1.2.0

- Added support for downloading multiple audio tracks and multiple subtitles into a single file (`--audio-lang ja-JP,en-US`, `--subs-lang en-US,es-419`). The first of each is marked as the default track, and each track is tagged with its language so media servers can select them.
- An episode is skipped if any requested audio or subtitle language is unavailable for it.
- Parallel segment downloads (10 workers) for much faster downloads
- Retry with backoff on connection errors instead of crashing
- Added `--urls` flag to batch download from a text file with one URL per line
- Invalid URLs in batch mode are skipped instead of stopping the whole process

## 1.1.1

- Optimized code, tried to handle errors
- Some random fixes
- Added a way to automatically refetch an access token if the current one expires

## 1.1.0

- Added support for downloading entire seasons
- Fixed MPD parsing
- Temporary downloaded files (video, audio segments and subtitles) are now stored in the OS temporary files then deleted
- Fixed FFmpeg merge command
- Docs improvements
- Support for `device_id.bin` and `private_key.pem` files

## 1.0.0

Initial release
