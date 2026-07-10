<!-- generated-by: gsd-doc-writer -->
# Getting Started

This guide walks you through setting up and running Crunchyroll Downloader for the first time.

---

## Prerequisites

Before you begin, ensure the following are installed and configured on your system:

| Dependency | Minimum Version | Notes |
|------------|-----------------|-------|
| **Go**     | `1.25.0`        | Required to build from source. See [go.dev/dl](https://go.dev/dl/). |
| **FFmpeg** | Recent build    | Required for muxing video, audio, and subtitles into MKV files. Install via your system package manager or from [ffmpeg.org](https://ffmpeg.org/download.html). |
| **Crunchyroll account** | — | A **Premium** account is required to download premium-only content. A free trial is sufficient. |
| **Widevine device** | — | A `.wvd` file or a directory containing `client_id.bin` + `private_key.pem`. See [CONFIGURATION.md](CONFIGURATION.md#widevine-device-configuration). |

### Go Version

The project requires **Go 1.25.0** or later. Check your version:

```bash
go version
```

If you need to upgrade, download the latest release from [go.dev/dl](https://go.dev/dl/).

### FFmpeg

FFmpeg must be on your system `$PATH`. Verify it is available:

```bash
ffmpeg -version
```

Installation commands by platform:

- **Ubuntu/Debian**: `sudo apt install ffmpeg`
- **macOS (Homebrew)**: `brew install ffmpeg`
- **Windows (Scoop)**: `scoop install ffmpeg`
- **Windows (Chocolatey)**: `choco install ffmpeg`

---

## Installation Steps

### 1. Clone the Repository

```bash
git clone https://github.com/FM0Ura/crunchyroll-downloader.git
cd crunchyroll-downloader
```

### 2. Verify Dependencies

Run the dependency check to confirm Go and FFmpeg are properly installed:

```bash
make deps
```

This checks for Go, downloads Go module dependencies, and verifies that `ffmpeg` and `ffprobe` are available.

### 3. Build the Binary

```bash
make build
```

The compiled binary will be placed at `dist/crunchyroll-downloader`.

Alternatively, build directly with Go:

```bash
go build -o dist/crunchyroll-downloader .
```

### 4. Configure Authentication

You need your Crunchyroll `etp_rt` session cookie. Provide it via one of these methods:

**Option A — Environment variable (recommended):**

Copy the example `.env` file and fill in your cookie:

```bash
cp .env.example .env
```

Then edit `.env` and set `CRUNCHYROLL_ETP_RT` to your cookie value:

```
CRUNCHYROLL_ETP_RT=your_etp_rt_cookie_value_here
```

**Option B — CLI flag:**

Pass `--etp-rt` on every invocation (see [First Run](#first-run) below).

> **How to get your `etp_rt` cookie:**
> 1. Go to [crunchyroll.com](https://crunchyroll.com) and log in.
> 2. Open Developer Tools (F12).
> 3. Go to the **Storage** (Firefox) or **Application** (Chrome) tab, then **Cookies**.
> 4. Select the Crunchyroll domain and copy the value of the `etp_rt` cookie.

### 5. Configure a Widevine Device

Crunchyroll uses DRM-protected streams. You need a Widevine device file to decrypt them.

Set the device path via one of these methods:

**In `.env`:**

```
WIDEVINE_DEVICE_PATH=/path/to/device.wvd
```

**Or via CLI flag:**

```bash
--widevine-device /path/to/device.wvd
```

See [CONFIGURATION.md](CONFIGURATION.md#widevine-device-configuration) for detailed instructions on both `.wvd` and raw file formats.

---

## First Run

Once you have configured your `etp_rt` cookie and Widevine device, download a single episode:

```bash
./dist/crunchyroll-downloader \
  --url https://www.crunchyroll.com/watch/GE00198973JAJP/dawn-and-confusion \
  --etp-rt your_cookie_here
```

Or if you are using a `.env` file, omit the `--etp-rt` flag:

```bash
./dist/crunchyroll-downloader \
  --url https://www.crunchyroll.com/series/GJ0H7Q5ZJ/hells-paradise \
  --season 1
```

The output MKV files will be saved in the current directory.

Use the Makefile convenience target for a build + run:

```bash
make run ARGS="--url https://www.crunchyroll.com/watch/GE00198973JAJP/dawn-and-confusion"
```

---

## Common Setup Issues

### FFmpeg Not Found

**Error message:** `FFmpeg not found: install FFmpeg and ensure it is on $PATH`

**Solution:** Install FFmpeg using your system package manager (see [Prerequisites](#prerequisites) above) and ensure the `ffmpeg` binary is available in your `$PATH`.

### Authentication Failure

**Symptom:** Downloads fail with HTTP 401 or similar authentication errors.

**Cause:** The `etp_rt` cookie is missing, expired, or incorrect.

**Solution:** Generate a fresh cookie by logging out and back in on crunchyroll.com, then copy the new `etp_rt` value. Verify the cookie is being loaded correctly — if using `.env`, confirm the file is in the current working directory and contains the correct value.

### No Widevine Device Configured

**Error message:** `no Widevine device configured`

**Solution:** Provide a valid `.wvd` file or a directory containing `client_id.bin` and `private_key.pem`. See [CONFIGURATION.md](CONFIGURATION.md#widevine-device-configuration).

### Wrong Go Version

**Symptom:** `go build` fails with a syntax or compatibility error.

**Solution:** Verify your Go version with `go version`. The project requires **Go 1.25.0** or later. Upgrade if needed.

### Output Directory Does Not Exist

**Error message:** `Output directory /path/to/dir does not exist`

**Solution:** Create the directory first, or omit `--output-dir` to use the current working directory.

---

## Next Steps

- [Architecture](ARCHITECTURE.md) — Understand the system design and component pipeline.
- [Configuration](CONFIGURATION.md) — Detailed reference for all CLI flags, environment variables, and config file options.
- [Development](../README.md#building) — Building from source and contributing.
