# Technology Stack

**Analysis Date:** 2026-07-08

## Languages

**Primary:**
- Go (go1.25) - All application source code is in a single `main` package

## Runtime

**Environment:**
- Go runtime (go1.25, module `crunchyroll-downloader`)

**Package Manager:**
- Go modules (`go.mod` + `go.sum`)
- Lockfile: `go.sum` present

## Frameworks

**Core:**
- None (no web framework — the project is a CLI tool)

**DASH/MPD Parsing:**
- `github.com/unki2aut/go-mpd v0.0.0-20250515065241-e261b43d6523` - Parses MPEG-DASH MPD manifests from Crunchyroll

**DRM / Widevine:**
- `github.com/iyear/gowidevine v0.1.3` - Widevine CDM integration for decrypting DRM-protected media
- `google.golang.org/protobuf v1.36.2` (indirect) - Protobuf dependency for gowidevine
- `github.com/chmike/cmac-go v1.1.0` (indirect) - CMAC crypto for Widevine

**Media Processing:**
- `github.com/Eyevinn/mp4ff v0.48.0` (indirect) - MP4 parsing (dependency of gowidevine)

**Build/Dev:**
- None (no build system beyond `go build`)

## Key Dependencies

**Critical:**
- `github.com/google/uuid v1.6.0` - UUID generation for device ID used in Crunchyroll auth
- `github.com/iyear/gowidevine v0.1.3` - All DRM decryption flows depend on this library
- `github.com/unki2aut/go-mpd` - All manifest parsing depends on this library

**System Dependencies (external, not Go modules):**
- `ffmpeg` - Required at runtime for muxing video, audio, and subtitle streams into MKV container (`output.go:91`)

**Infrastructure:**
- Standard library only for HTTP, JSON, crypto, OS operations, concurrency

## Configuration

**Environment:**
- No `.env` file. All configuration is via CLI flags:
  - `-url` / `-file` - target URL(s) for download
  - `-etp-rt` - Crunchyroll session cookie (required, passed as string)
  - `-audio-lang` - audio language(s), comma-separated (default: `ja-JP`)
  - `-subs-lang` - subtitle language(s), comma-separated (default: `en-US`)
  - `-video-quality` - video resolution (default: `1080p`)
  - `-audio-quality` - audio bitrate (default: `192k`)
  - `-season` - season number for series downloads (default: 0 = all seasons)
  - `-debug-manifest` - boolean flag to dump raw manifest XML/JSON

**Build:**
- No build config files. Standard `go build .`

## Platform Requirements

**Development:**
- Go 1.25+
- FFmpeg (for testing MKV output)

**Production:**
- Linux, macOS, or Windows binary (Go cross-compiles to all three)
- Runtime dependencies:
  - FFmpeg installed and on `PATH` (`exec.Command("ffmpeg", ...)` in `output.go`)
  - Widevine device file (`.wvd` or `client_id.bin` + `private_key.pem`) placed in working directory
  - Crunchyroll Premium account (for Premium-only content)
  - Network access to `crunchyroll.com` and CDN endpoints

---

*Stack analysis: 2026-07-08*