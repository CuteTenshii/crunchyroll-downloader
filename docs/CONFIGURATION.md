<!-- generated-by: gsd-doc-writer -->
# Configuration

Crunchyroll Downloader supports three configuration layers, resolved in the
following precedence order (highest priority first):

1. **CLI flags** — passed at runtime
2. **Environment variables** — loaded from a `.env` file or set in the shell
3. **Config file** — `config.json` in the project root directory
4. **Built-in defaults** — compiled into the binary

This document covers all configuration mechanisms and their available settings.

---

## Configuration Precedence

Every configurable value follows a consistent resolution chain:

```
CLI flag  →  environment variable  →  config.json field  →  compiled default
```

For example, `--audio-lang` overrides the `AUDIO_LANG` environment variable,
which overrides the `audio_lang` field in `config.json`, which falls back to
`"ja-JP"`.

The application loads `.env` automatically on startup by walking up from the
current working directory. It then loads `config.json` from the project root.
After that, all CLI flags are parsed and the precedence resolution runs for
each value.

---

## CLI Flags

All flags are defined in [`main.go`](../main.go) using Go's `flag` package.
Pass them after the binary name:

```bash
./crunchyroll-downloader --url <URL> [flags]
```

| Flag                  | Type   | Default     | Description                                                              |
|-----------------------|--------|-------------|--------------------------------------------------------------------------|
| `--url`               | string | —           | URL of the episode or series to download (required)                      |
| `--file`              | string | —           | Path to a text file with one URL per line (alternative to `--url`)       |
| `--audio-lang`        | string | `"ja-JP"`   | Audio language(s), comma-separated for multiple. First is the default.   |
| `--subs-lang`         | string | `"en-US"`   | Subtitle language(s), comma-separated for multiple. First is the default.|
| `--video-quality`     | string | `"1080p"`   | Desired video quality                                                    |
| `--audio-quality`     | string | `"192k"`    | Desired audio bitrate                                                    |
| `--season`            | int    | `0`         | Season number (only used with a series URL; `0` downloads all seasons)   |
| `--etp-rt`            | string | `""`        | Crunchyroll `etp_rt` session cookie value for authentication             |
| `--output-dir`        | string | `""`        | Custom output directory (uses current directory if empty)                |
| `--workers`           | int    | `10`        | Number of concurrent segment download workers                            |
| `--widevine-device`   | string | `""`        | Path to a `.wvd` file or a directory with `client_id.bin` + `private_key.pem` |
| `--debug-manifest`    | bool   | `false`     | Log raw episode playback JSON and manifest XML                           |
| `--json`              | bool   | `false`     | Output progress as NDJSON                                                |
| `--quiet`             | bool   | `false`     | Suppress progress output (errors still print)                            |

---

## Environment Variables

### `.env` File

The application automatically loads a `.env` file on startup. It searches
upward from the current working directory until it finds a file named `.env`.

The file uses standard `KEY=VALUE` syntax:

```bash
# .env
CRUNCHYROLL_ETP_RT=your_cookie_here
OUTPUT_DIR=/path/to/downloads
```

- Lines starting with `#` are comments
- Blank lines are ignored
- Single and double quotes around values are stripped
- Variables are set with `os.Setenv` and become available to the running process

### Variable Reference

| Variable                     | Required | Default | Description                                                             |
|------------------------------|----------|---------|-------------------------------------------------------------------------|
| `CRUNCHYROLL_ETP_RT`        | Required | —       | Crunchyroll `etp_rt` session cookie value. Used for authentication.     |
| `WIDEVINE_DEVICE_PATH`       | Optional | —       | Path to a Widevine device file (`.wvd`).                                |
| `WIDEVINE_CLIENT_ID_PATH`    | Optional | —       | Path to the Widevine client ID binary (must be paired with private key).|
| `WIDEVINE_PRIVATE_KEY_PATH`  | Optional | —       | Path to the Widevine private key (must be paired with client ID).       |
| `OUTPUT_DIR`                 | Optional | `""`    | Custom output directory for downloaded files.                           |
| `CRUNCHYROLL_CLIENT_AUTH`    | Optional | See note | Override for the Crunchyroll Basic Auth credential used during token requests. |

<!-- VERIFY: CRUNCHYROLL_CLIENT_AUTH default value is a compiled-in constant. The current default is "Basic bm9haWhkZXZtXzZpeWcwYThsMHE6". If Crunchyroll rotates this credential, set this env var to the new value. -->

> **Required** variables cause a runtime error or authentication failure if
> absent. The application will start, but downloads will fail without a valid
> `CRUNCHYROLL_ETP_RT` and a configured Widevine device.

---

## Config File (`config.json`)

The configuration file lives at `config.json` in the project root directory
(the current working directory at startup).

If no `config.json` exists, the application creates a skeleton file with
default values automatically on first run.

### Fields

```json
{
  "audio_lang": "ja-JP",
  "audio_quality": "192k",
  "subs_lang": "en-US",
  "video_quality": "1080p",
  "workers": 10
}
```

| Field             | Type   | Default                      | Description                                        |
|-------------------|--------|------------------------------|----------------------------------------------------|
| `audio_lang`      | string | `"ja-JP"`                    | Default audio language                             |
| `audio_quality`   | string | `"192k"`                     | Default audio bitrate                              |
| `subs_lang`       | string | `"en-US"`                    | Default subtitle language                          |
| `video_quality`   | string | `"1080p"`                    | Default video quality                              |
| `workers`         | number | `10`                         | Number of concurrent download workers              |
| `output_dir`      | string | `""` (uses CWD)              | Custom output directory                            |
| `etp_rt`          | string | `""`                         | Crunchyroll session cookie (`etp_rt`)              |
| `widevine_device` | string | `""`                         | Path to `.wvd` file or directory with device keys  |

> **Note:** The config file uses JSON `null` (omitted field) to represent
> "not set". Only explicitly written fields are used. Fields absent from the
> file are ignored and the next priority layer (env var or default) applies.

### Skeleton Defaults

When created automatically, the skeleton config includes only the media
preference fields:

```json
{
  "audio_lang": "ja-JP",
  "subs_lang": "en-US",
  "video_quality": "1080p",
  "audio_quality": "192k",
  "workers": 10
}
```

The fields `output_dir`, `etp_rt`, and `widevine_device` are **not** written
to the skeleton because they have no meaningful built-in default and must be
configured explicitly via CLI flag or environment variable.

---

## Required vs Optional Settings

### Required for Download to Work

These settings **must** be configured before a download can succeed. The
application will start without them but will fail when attempting to
authenticate or decrypt content:

| Setting                | How to Provide                          | Error if Missing                                      |
|------------------------|-----------------------------------------|-------------------------------------------------------|
| `etp_rt` / `CRUNCHYROLL_ETP_RT` | `--etp-rt` flag, `.env`, or `config.json` | Authentication fails (Crunchyroll rejects the request) |
| Widevine device path   | `--widevine-device` flag, `.env`, or `config.json` | `"no Widevine device configured"` error               |

### Settings with Built-in Defaults

These settings are optional and fall back to the compiled default if not
provided through any configuration layer:

| Setting          | Default     | CLI Flag           | Environment Variable |
|------------------|-------------|--------------------|----------------------|
| Audio language   | `"ja-JP"`   | `--audio-lang`     | — (config file only) |
| Subtitle language| `"en-US"`   | `--subs-lang`      | — (config file only) |
| Video quality    | `"1080p"`   | `--video-quality`  | — (config file only) |
| Audio quality    | `"192k"`    | `--audio-quality`  | — (config file only) |
| Workers          | `10`        | `--workers`        | — (config file only) |
| Output directory | `""` (CWD)  | `--output-dir`     | `OUTPUT_DIR`         |
| Debug manifest   | `false`     | `--debug-manifest` | —                    |
| JSON output      | `false`     | `--json`           | —                    |
| Quiet mode       | `false`     | `--quiet`          | —                    |

---

## Per-Environment Overrides

The project does **not** include environment-specific config files (e.g.,
`.env.development`, `.env.production`). All configuration is resolved at
runtime through the precedence chain.

To use different configurations for different environments:

1. **Create separate `.env` files** and symlink or copy the appropriate one
   to `.env` before running.
2. **Pass CLI flags** in a wrapper script or Makefile target for each
   environment.
3. **Use shell wrappers** that export environment variables before launching
   the binary:

```bash
# production download
CRUNCHYROLL_ETP_RT=prod_cookie \
  WIDEVINE_DEVICE_PATH=/etc/widevine/prod_device.wvd \
  ./crunchyroll-downloader --url <URL>

# development/test download
CRUNCHYROLL_ETP_RT=dev_cookie \
  WIDEVINE_DEVICE_PATH=/home/user/dev/device.wvd \
  OUTPUT_DIR=/tmp/test-downloads \
  ./crunchyroll-downloader --url <URL>
```

---

## Widevine Device Configuration

The Widevine device can be provided in two formats:

### Single `.wvd` File

```bash
./crunchyroll-downloader --widevine-device /path/to/device.wvd --url <URL>
```

### Directory with Raw Files

A directory containing `client_id.bin` and `private_key.pem`:

```bash
# Directory
./crunchyroll-downloader --widevine-device /path/to/device-dir/ --url <URL>

# Or via legacy environment variables
export WIDEVINE_CLIENT_ID_PATH=/path/to/client_id.bin
export WIDEVINE_PRIVATE_KEY_PATH=/path/to/private_key.pem
./crunchyroll-downloader --url <URL>
```

The resolution order for the Widevine device path is:

1. `--widevine-device` CLI flag
2. `WIDEVINE_DEVICE_PATH` environment variable
3. `widevine_device` field in `config.json`
4. Legacy `WIDEVINE_CLIENT_ID_PATH` + `WIDEVINE_PRIVATE_KEY_PATH` env vars
5. Error — no device configured
