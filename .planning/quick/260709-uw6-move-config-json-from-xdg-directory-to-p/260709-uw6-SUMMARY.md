---
quick_id: 260709-uw6
status: complete
date: 2026-07-10
---

# Quick Task 260709-uw6: Move config JSON from XDG directory to project root

## Changes

- **internal/config/config.go** — `ConfigDir()` now returns `os.Getwd()` instead of `$XDG_CONFIG_HOME/animeheaven` or `os.UserConfigDir()/animeheaven`. Config file is now `./config.json` (CWD).
- **internal/config/config_test.go** — Replaced XDG-specific tests with `TestConfigDirReturnsCwd` and updated `TestConfigPath` for CWD-based path.

## Verification

- `go build ./...` — passes
- `go vet ./...` — passes
- `go test ./...` — all tests pass
