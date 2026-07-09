# Phase 3: Usability — Configuration & Validation - Context

**Gathered:** 2026-07-09
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase adds persistent configuration (config file, env vars), output directory control, explicit Widevine device paths, startup validation (FFmpeg, batch URLs), and four code quality fixes to the CLI tool. Config values follow a strict precedence hierarchy: CLI flag > env var > config file > default.

</domain>

<decisions>
## Implementation Decisions

### Config File Structure (USAB-02)
- **D-01:** Format is plain JSON — `encoding/json` from stdlib, no YAML/TOML.
- **D-02:** All CLI flags are config-file-persistable: `audio_lang`, `subs_lang`, `video_quality`, `audio_quality`, `workers`, `output_dir`, `etp_rt`, `widevine_device`.
- **D-03:** Config file discovery: check `$XDG_CONFIG_HOME/animeheaven/config.json` first, fallback to `~/.config/animeheaven/config.json`.
- **D-04:** If config file doesn't exist, auto-generate a minimal JSON skeleton with defaults and print a message. If it exists but is invalid JSON, warn and continue with defaults (never block startup).
- **D-05:** Config values use explicit-only override — unset/missing JSON fields fall through to defaults. Only fields explicitly present in the config file override the defaults.

### Precedence Hierarchy (USAB-01, USAB-07)
- **D-06:** Precedence: CLI flag > env var > config file > default.
- **D-07:** `CRUNCHYROLL_ETP_RT` env var handled in `main.go` — if `--etp-rt` flag is empty string, fall through to env var check. CLI layer owns the resolution.
- **D-08:** `CRUNCHYROLL_CLIENT_AUTH` env var replaces the hardcoded Basic Auth credential in `auth.go:28` when set. If unset, the hardcoded credential is used as default.
- **D-09:** Phase 1's `.env` file approach is deprecated entirely. Remove `.env` file reading from `internal/drm/drm.go`. The legacy env var names (`WIDEVINE_DEVICE_PATH`, `WIDEVINE_CLIENT_ID_PATH`, `WIDEVINE_PRIVATE_KEY_PATH`) remain as env-var-level fallbacks in the hierarchy.

### Output Directory (USAB-03)
- **D-10:** `--output-dir` flag specifies custom output directory. Series subfolder is created inside it (e.g., `--output-dir /media/anime` produces `/media/anime/Attack_on_Titan/...mkv`).
- **D-11:** If the specified output directory does not exist, print an error and abort — do NOT auto-create it. The series subfolder is created normally via `os.MkdirAll` inside the (existing) output dir.
- **D-12:** Default behavior (no `--output-dir` flag): unchanged — files go to `CWD/<series_title>/<filename>.mkv`, series subfolder created via `os.MkdirAll`.

### Widevine Device Flags (USAB-06)
- **D-13:** Single `--widevine-device` flag accepting either:
  - A `.wvd` file path, or
  - A directory path containing `client_id.bin` + `private_key.pem` as a pair.
- **D-14:** Auto-detect which format the path refers to (`.wvd` extension or directory check).
- **D-15:** Environment variable fallbacks kept: `WIDEVINE_DEVICE_PATH`, `WIDEVINE_CLIENT_ID_PATH`, `WIDEVINE_PRIVATE_KEY_PATH`. Config file can also set the path.
- **D-16:** `.env` file removed — only `os.LookupEnv` for env var reading, no file-based env loading.

### Validation Strategy (USAB-04, USAB-05)
- **D-17:** Batch URL validation: validate structure + content ID length only — must match `/watch/<9-14 char ID>` or `/series/<9-14 char ID>`. No HTTP reachability check. Report ALL invalid URLs upfront before any downloads start.
- **D-18:** FFmpeg check: `exec.LookPath("ffmpeg")` + `exec.Command("ffmpeg", "-version")` at startup to confirm the binary exists and runs.
- **D-19:** FFmpeg missing = hard error with actionable message: "FFmpeg not found. Install FFmpeg and ensure it is on $PATH. See https://ffmpeg.org/download.html". Exit 1.

### QOL Fix Integration (QOL-04, QOL-05, QOL-07, QOL-08)
- **D-20:** QOL-04 (`&&` → `||` in URL validation) and QOL-05 (use `url.Parse()` instead of string split) bundled as one unit — both are refactors of `processURL()`.
- **D-21:** QOL-07 (regex `_{2,}` → `_` replace in `sanitizeFilename`) is a standalone fix in `internal/download/episode.go`.
- **D-22:** QOL-08 (call `parseLangs` once at flag parse time instead of per-URL) must run AFTER config/env resolution completes in `main.go` — depends on the config loading work (D-01 through D-07). Pass the parsed lang slices through to `processURL` and `Episode` instead of re-parsing.

### the agent's Discretion
None — all decisions were made explicitly during discussion.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition and constraints
- `.planning/ROADMAP.md` — Phase 3 scope, plan grouping, and requirement mapping.
- `.planning/REQUIREMENTS.md` — Full definitions for USAB-01 through USAB-07, QOL-04/05/07/08 with acceptance criteria.
- `.planning/PROJECT.md` — Project context, validated requirements, and key decisions.

### Codebase maps
- `.planning/codebase/STACK.md` — Go 1.25 CLI stack, ffmpeg dependency, current config surface (all flags are CLI-only).
- `.planning/codebase/ARCHITECTURE.md` — Current data flow, global state patterns, and configuration discovery gap.
- `.planning/codebase/CONVENTIONS.md` — Naming patterns for CLI flags, file organization.

### Prior phase context
- `.planning/phases/01-foundation-error-handling-http-memory/01-CONTEXT.md` — Prior decisions on error contracts (D-02: clear errors), Widevine device caching (D-13 through D-17: .env-based paths, .wvd wins).
- `.planning/phases/02-performance-caching-parallelism/02-CONTEXT.md` — Prior decisions on MPD cache and parallel audio.

### Current implementation touchpoints
- `main.go:17-26` — Current CLI flag definitions (all 8 flags), `main()` entry point, `processURL()` — config loading and new flags wire in here.
- `internal/api/auth.go:28` — Hardcoded Basic Auth credential (`"Basic bm9haWhkZXZtXzZpeWcwYThsMHE6"`) — USAB-07 target.
- `internal/api/client.go:35-55` — `NewWithContext` — currently requires `etpRt` string; needs env var fallback.
- `internal/drm/drm.go:21-26` — Current Widevine env var constants and `.env` file reading — needs deprecation of `.env`, addition of CLI flag path.
- `internal/drm/drm.go:64-91` — `loadWidevineDevice()` — needs extension to accept explicit path from CLI/config.
- `internal/download/episode.go:21-34` — `sanitizeFilename()` — QOL-07 target (regex replace).
- `internal/download/episode.go:36-55` — `Episode()` output path construction — needs `--output-dir` support.
- `internal/api/manifest.go` — MPD cache location (from Phase 2), config-independent.

### No external specs
No external specs — requirements are fully captured in the decisions above and the roadmap.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `os.MkdirAll` already used in `internal/download/episode.go:40` for series subfolder creation — same pattern works for config directory creation.
- `os.LookupEnv` already used in `internal/drm/drm.go:128` for env var fallback — extends naturally to all env var reads.
- `encoding/json` — already used in API response parsing; `json.MarshalIndent` for writing config file.
- `flag.String` / `flag.Int` pattern in `main.go:17-26` — standard Go flag pattern for new `--output-dir`, `--widevine-device` flags.

### Established Patterns
- `internal/` package pattern from refactoring — config loading function should go in a new `internal/config/` package or live in `main.go`'s package (simpler, avoids circular deps).
- Single `http.Client` with configured transport — config file should not change transport settings.
- CLI flag as default source of truth — precedence hierarchy layers on top without changing how flags work internally.

### Integration Points
- `main.go` — Central wiring point: config file loading, env var resolution, new flag definitions, FFmpeg check, batch URL validation all connect here.
- `internal/api/client.go:35-55` — `NewWithContext` receives `etpRt` from resolved config/env chain instead of raw flag value.
- `internal/api/auth.go:28` — Hardcoded Basic Auth replaced with env-var-checking logic.
- `internal/drm/drm.go:56-62` — `GetWidevineDevice()` uses `sync.Once` — config path needs to be resolved BEFORE this is first called (at startup, before any download).
- `internal/download/episode.go:36-55` — Output path accepts `outputDir` parameter from resolved config.
- `internal/download/episode.go:21-34` — `sanitizeFilename` regex rewrite is self-contained.

</code_context>

<specifics>
## Specific Ideas

- Config file should be a flat JSON object with the same keys as CLI flag names (snake_case): `{"audio_lang": "ja-JP", "subs_lang": "en-US", "video_quality": "1080p", ...}`.
- The config loading function should return a struct that main.go merges with flag values following the precedence hierarchy.
- QOL-08 implies moving `parseLangs` call from inside `processURL` to `main` after config resolution, storing the result in package-level variables or passing as parameters.
- `golang.org/x/sync/errgroup` not needed for config loading — it's synchronous.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within Phase 3 scope.

</deferred>

---

*Phase: 3-Usability — Configuration & Validation*
*Context gathered: 2026-07-09*
