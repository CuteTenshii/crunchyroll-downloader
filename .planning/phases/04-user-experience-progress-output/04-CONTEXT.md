# Phase 4: User Experience — Progress & Output - Context

**Gathered:** 2026-07-09
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase replaces all raw `fmt.Printf` output across the codebase with a structured output abstraction, adds season download progress with per-episode status, download speed/ETA estimation during segment downloads, `--quiet`/`--json` output modes, structured logging with levels, and season/batch error accumulation with detailed reporting.

</domain>

<decisions>
## Implementation Decisions

### Output Abstraction
- **D-01:** Custom output interface, not slog or raw `fmt.Printf`. Eliminates all 43 `fmt.Printf` calls across 8 files.
- **D-02:** Interface methods: `Info`, `Warn`, `Error`, `Debug` (leveled log methods) + `Progress` (for segment/season progress updates).
- **D-03:** Wired as a package-level global set once in `main()` after flag parsing — same pattern as Phase 3's config resolution. No function signature changes across internal packages.
- **D-04:** Default mode (no `--quiet`/`--json`): human-readable output with colors (green ✓, red ✗). ANSI color codes in terminal output.

### Season Progress (UX-02, UX-06)
- **D-05:** Per-episode output is multi-line: a progress block during download, then a result line on completion.
- **D-06:** Success character: `✓` (green), Failure character: `✗` (red).
- **D-07:** Errors shown inline as they occur (per-episode ✗ visible immediately, plus error message) AND accumulated for final summary.
- **D-08:** Season summary at end: detailed error listing — each failed episode by title/number with the specific error message.

### Speed/ETA (UX-03)
- **D-09:** Update frequency: time-based, once per second (not every segment).
- **D-10:** ETA calculation: rolling average over last 5-10 seconds.
- **D-11:** ETA scope: per-episode total (sum of remaining time across video + all audio versions + subtitles — single countdown).
- **D-12:** Display format: inline with segment progress on the same `\r` line: `Downloaded 145/212 segments (68%) — 12.4 MB/s, ETA 23s`.

### JSON Output (UX-04)
- **D-13:** Format: NDJSON — one JSON object per line, streaming-friendly.
- **D-14:** Event types: `episode_start`, `segment_progress`, `error`, `episode_complete`, `season_summary`. Granular events including per-segment details.
- **D-15:** Field naming: snake_case (matches existing codebase JSON convention).

### Quiet Mode (UX-04)
- **D-16:** `--quiet` suppresses progress output only (segment progress, "Downloading...", speed/ETA). Errors and warnings still print. If `--json` is also set, errors emit as JSON events.

### the agent's Discretion
None — all decisions were made explicitly during discussion.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition and constraints
- `.planning/ROADMAP.md` — Phase 4 scope (plans 4.1/4.2/4.3), plan grouping, and requirement mapping.
- `.planning/REQUIREMENTS.md` — Full definitions for UX-02, UX-03, UX-04, UX-06, UX-08 with acceptance criteria.
- `.planning/PROJECT.md` — Project context, validated requirements, and key decisions.

### Codebase maps
- `.planning/codebase/STACK.md` — Go 1.25 stack, no logging library, ffmpeg dependency.
- `.planning/codebase/ARCHITECTURE.md` — Current data flow, `fmt.Printf` scatter across 8 files, global state patterns.
- `.planning/codebase/CONVENTIONS.md` — Current output patterns (carriage return progress, `fmt.Printf`), naming conventions.

### Prior phase context
- `.planning/phases/03-usability-configuration-validation/03-CONTEXT.md` — Config file structure, precedence hierarchy (D-06), relevant for `--quiet`/`--json` flag handling.
- `.planning/phases/01-foundation-error-handling-http-memory/01-CONTEXT.md` — Error contract (D-02: clean messages, no stack traces), signal handling (D-18).
- `.planning/phases/02-performance-caching-parallelism/02-CONTEXT.md` — Parallel audio fan-out, relevant for per-episode total ETA calculation.

### Current implementation touchpoints
- `main.go` — All CLI output, URL validation messages, batch progress, config creation messages.
- `internal/download/episode.go` — Episode download output ("Downloading:", audio/sub progress, cleanup messages) — 10+ fmt.Printf calls.
- `internal/download/season.go` — Season header and per-episode error output — 3 fmt.Printf calls.
- `internal/media/segment.go` — Segment progress via `\r` carriage return — 2 fmt.Printf calls.
- `internal/mux/mux.go` — Final success message, temp file cleanup warnings — 2 fmt.Printf calls.
- `internal/api/client.go` — Token refresh message — 1 fmt.Printf.
- `internal/api/episode.go` — Debug manifest dump — 1 fmt.Printf.
- `internal/api/manifest.go` — Debug manifest dump — 1 fmt.Printf.

### No external specs
No external specs — requirements are fully captured in the decisions above and the roadmap.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config` package (Phase 3) — Package-level config struct set once in `main()` — proven pattern for D-03 (package-level output global).
- `\r` carriage return pattern in `internal/media/segment.go:173` — Already uses in-place line updates for segment progress. D-12 extends this with speed/ETA.
- `atomic.Int64` progress counter in `internal/media/segment.go` — Already tracks segment count. Can drive bytes-per-second calculation.

### Established Patterns
- Package-level global set in `main()` — Phase 3 config uses this exactly (D-03 mirrors it). No plumbing through function signatures.
- `internal/` package structure — New output abstraction goes in a new `internal/output/` package (similar to `internal/config/`).
- `sync.Mutex` for shared state — Use for the rolling speed window (D-10) and accumulated error list (D-08).

### Integration Points
- Every `fmt.Printf` call across all 8 files — Replaced with `output.Info(...)` / `output.Warn(...)` / `output.Error(...)` / `output.Debug(...)` / `output.Progress(...)`.
- `internal/media/segment.go:173` — Segment progress line extended with speed/ETA (D-12) and update throttled to 1-second intervals (D-09).
- `internal/download/season.go` — Needs season-level orchestration: per-episode result tracking (D-06/D-07) and final summary (D-08).
- `main.go:flag.Parse()` — Add `--quiet` and `--json` flags. Set output mode before any API calls or downloads.

</code_context>

<specifics>
## Specific Ideas

- The output interface should live in `internal/output/` package with constructor `New(mode OutputMode)`. Mode enum: `ModeHuman` (default, colored), `ModeJSON`, `ModeQuiet`.
- Rolling speed window: maintain a ring buffer of (timestamp, bytes) pairs covering the last 5-10 seconds. ETA = remaining_bytes / rolling_speed.
- NDJSON event schema: `{"type":"segment_progress","timestamp":"...","downloaded":145,"total":212,"bytes_per_sec":12400000,"eta_secs":23,"stream":"video"}`.
- Error accumulation: collect `[]episodeError{number, title, err}` in `Season()` and print on completion. Same pattern for batch mode in `main.go`.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within Phase 4 scope.

</deferred>

---

*Phase: 4-User Experience — Progress & Output*
*Context gathered: 2026-07-09*
