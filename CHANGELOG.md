# Changelog

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
