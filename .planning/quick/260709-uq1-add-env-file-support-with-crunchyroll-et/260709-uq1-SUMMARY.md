---
quick_id: 260709-uq1
status: complete
date: 2026-07-10
---

# Quick Task 260709-uq1: Add .env file support

## Changes

- **internal/config/dotenv.go** — New file. Simple .env file loader that walks up from CWD, parses KEY=VALUE lines (with quoted value support), calls os.Setenv for each.
- **main.go** — Calls config.LoadDotenv() at startup before flag parsing. Changed resolveString for output-dir to use "OUTPUT_DIR" env var name.
- **.env.example** — Documents all 6 supported env vars: CRUNCHYROLL_ETP_RT, WIDEVINE_DEVICE_PATH, WIDEVINE_CLIENT_ID_PATH, WIDEVINE_PRIVATE_KEY_PATH, XDG_CONFIG_HOME, OUTPUT_DIR.

## Verification

- `go build ./...` — passes
- `go vet ./...` — passes
- `go test ./...` — all tests pass
- Precedence preserved: CLI flag > os env > .env > config > default
