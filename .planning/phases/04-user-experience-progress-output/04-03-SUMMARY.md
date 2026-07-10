---
phase: 04-user-experience-progress-output
plan: 03
subsystem: output
tags: [go, cli, ansi, ndjson, output-abstraction, logging]

requires:
  - phase: 03-usability-configuration-validation
    provides: internal/config package-level global pattern (D-03)
provides:
  - internal/output/ package with Outputter interface and 3 mode implementations
  - --quiet / --json CLI flags for non-interactive output
  - SpeedTracker with 10-second rolling ring buffer for download speed/ETA
  - NDJSON event types for machine-readable output mode
  - All 28 fmt.Printf calls replaced across main.go, mux.go, api/*.go
affects:
  - 04-plan-04-01 (season progress + download/episode output)
  - 04-plan-04-02 (segment progress with speed/ETA)

tech-stack:
  added:
    - golang.org/x/term (TTY detection for ANSI color output)
  patterns:
    - Package-level output global set once in main() (mirrors internal/config)
    - Ring buffer for rolling speed average
    - NDJSON streaming with json.Marshal + newline
    - ANSI escape constants for terminal coloring

key-files:
  created:
    - internal/output/output.go
    - internal/output/speed.go
    - internal/output/ndjson.go
    - internal/output/output_test.go
  modified:
    - main.go
    - internal/mux/mux.go
    - internal/api/client.go
    - internal/api/episode.go
    - internal/api/manifest.go
    - go.mod
    - go.sum
    - main_test.go
    - internal/mux/mux_test.go

key-decisions:
  - "Package-level Global output set once in main() — mirrors Phase 3 internal/config pattern"
  - "ANSI constants with TTY detection via golang.org/x/term — no external color library"
  - "10-second rolling ring buffer for speed/ETA — no external library"
  - "NDJSON via encoding/json + newline write — no external dependency"
  - "Error output routes to stderr in human mode, NDJSON event in JSON mode"
  - "Quiet mode suppresses Info/Debug/Progress, always shows Warn/Error"
  - "Pitfall 5 mitigation: ETA never goes up (clamping), small remaining returns 0"

requirements-completed: [UX-04, UX-08]

coverage:
  - id: D01
    description: "Outputter interface with Info/Warn/Error/Debug/Progress methods"
    verification:
      - kind: unit
        ref: internal/output/output_test.go#OutputterInterface
        status: pass
    human_judgment: false
  - id: D02
    description: "Three mode implementations (human, JSON, quiet) with correct behavior"
    verification:
      - kind: unit
        ref: internal/output/output_test.go#TestHumanOutputInfo
        status: pass
      - kind: unit
        ref: internal/output/output_test.go#TestJSONOutputInfo
        status: pass
      - kind: unit
        ref: internal/output/output_test.go#TestQuietModeInfo
        status: pass
    human_judgment: false
  - id: D03
    description: "SpeedTracker with rolling 10-second ring buffer, ETA with non-increasing clamp"
    verification:
      - kind: unit
        ref: internal/output/output_test.go#TestSpeedTrackerRecord
        status: pass
      - kind: unit
        ref: internal/output/output_test.go#TestSpeedTrackerETA
        status: pass
      - kind: unit
        ref: internal/output/output_test.go#TestETASmallRemaining
        status: pass
    human_judgment: false
  - id: D04
    description: "NDJSON event types with snake_case tags and emitEvent helper"
    verification:
      - kind: unit
        ref: internal/output/output_test.go#TestJSONOutputInfo
        status: pass
    human_judgment: false
  - id: D05
    description: "All 28 fmt.Printf calls replaced across main.go, mux.go, api/*.go"
    verification:
      - kind: unit
        ref: go build ./... (no compile errors)
        status: pass
      - kind: unit
        ref: rg fmt\.(Printf|Println) on modified files returns 0
        status: pass
    human_judgment: false
  - id: D06
    description: "--quiet and --json CLI flags operational in main.go"
    verification:
      - kind: other
        ref: go run . --help shows -json and -quiet flags
        status: pass
    human_judgment: false

duration: 7min
completed: 2026-07-10
status: complete
---

# Phase 04 Plan 03: Output Abstraction and CLI Flag Wiring

**Output infrastructure package (internal/output/) with three modes, SpeedTracker, NDJSON events — all 28 raw fmt.Printf calls replaced across main.go, mux.go, api/client.go, api/episode.go, api/manifest.go**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-10T00:35:17Z
- **Completed:** 2026-07-10T00:42:41Z
- **Tasks:** 3
- **Files modified:** 12

## Accomplishments

- Created `internal/output/` package with `Outputter` interface and three implementations: human (ANSI colors), JSON (NDJSON events), quiet (suppress progress)
- SpeedTracker with 10-second rolling ring buffer, Bps/ETA calculation, and non-increasing ETA clamp (Pitfall 5 mitigation)
- NDJSON event types with snake_case JSON tags and all fields per UI-SPEC.md schema
- Package-level `Global` variable and `Init(mode)` mirroring Phase 3's `internal/config` pattern
- TTY detection via `golang.org/x/term.IsTerminal()` — disables ANSI colors when stdout is piped
- Package-level `RecordBytes`, `SpeedBps`, `ETASeconds` functions for cross-package speed tracking (used by Plan 4.2)
- `--quiet` and `--json` CLI flags in main.go with output.Init() wired after flag.Parse()
- All 28 `fmt.Printf`/`fmt.Println`/`fmt.Print`/`fmt.Fprintf` calls replaced with `output.Global.Info/Warn/Error/Debug`
- 20 unit tests covering all output modes, speed tracker, edge cases
- All pre-existing tests updated for new output format

## Task Commits

Each task was committed atomically:

1. **Task 1: Create internal/output/ package** — `07ee40c` (feat)
2. **Task 2: Add --json/--quiet flags, wire output.Init(), replace main.go fmt.Printf** — `e09321b` (feat)
3. **Task 3: Replace mux.go and api/*.go fmt.Printf calls, fix tests** — `a09d866` (feat)

## Files Created/Modified

- `internal/output/output.go` — Outputter interface, Mode enum, ANSI constants, Global var, Init(), three implementations
- `internal/output/speed.go` — SpeedTracker with 10-sample ring buffer, Record/Bps/ETA, non-increasing ETA clamp
- `internal/output/ndjson.go` — event struct with all event type fields, eventError, emitEvent helper
- `internal/output/output_test.go` — 20 tests covering human/JSON/quiet modes and SpeedTracker
- `main.go` — Added --json/--quiet flags, output.Init() block, replaced 23+ fmt calls with output.Global
- `internal/mux/mux.go` — Replaced 2 fmt.Printf calls, added output import
- `internal/api/client.go` — Replaced 1 fmt.Println, added output import
- `internal/api/episode.go` — Replaced 1 fmt.Printf, added output import
- `internal/api/manifest.go` — Replaced 1 fmt.Printf, removed unused fmt import, added output import
- `go.mod` / `go.sum` — Added golang.org/x/term dependency
- `main_test.go` — Added captureMainStderr helper, updated processURL tests for stderr output
- `internal/mux/mux_test.go` — Updated output string match for new format

## Decisions Made

- **Package-level Global pattern**: Mirrors Phase 3's `internal/config` approach — a single `Global Outputter` var set once in `main()`. No plumbing through function signatures.
- **ANSI escape constants instead of color library**: 5 ANSI codes (reset, bold, red, green, yellow, cyan, dim) as Go consts. Zero dependency cost vs. `fatih/color` or `charm.sh/lipgloss`.
- **TTY detection via golang.org/x/term**: Go team-maintained extension, cross-platform. When stdout is not a terminal, all ANSI constants are set to empty strings.
- **NDJSON via encoding/json + newline**: NDJSON is `json.Marshal(obj) + "\n"`. No external library needed.
- **Rolling ring buffer custom implementation**: Fixed-size 10-sample ring buffer, mutex-guarded. ~30 lines of Go vs. external rolling window library.
- **Pitfall 5 ETA mitigation**: ETA is clamped to never increase, and small remaining bytes (< 2 MB) returns 0 to avoid oscillation at end of download.
- **Quiet mode behavior**: Per D-16, quiet suppresses Info/Debug/Progress (no-ops), but Warn and Error always print to stderr.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

- Existing main_test.go and mux_test.go expected raw stdout output with "Warning:" prefix — updated to match new output.Global format (errors now go to stderr, warning messages use ANSI-wrapped content)
- TestSpeedTrackerETA initially failed because 1000 remaining bytes was below the 2MB Pitfall 5 threshold — fixed test to use 100MB remaining

## Next Phase Readiness

- Output infrastructure ready for Plans 4.1 (season/episode.go output integration) and 4.2 (segment.go progress with speed/ETA)
- `RecordBytes`, `SpeedBps`, `ETASeconds` functions exported for Plan 4.2's segment.go speed tracking
- 13 remaining fmt.Printf calls in `internal/download/episode.go` and `internal/download/season.go` to be replaced in Plans 4.1 and 4.2
- 2 remaining fmt.Printf calls in `internal/media/segment.go` to be replaced in Plan 4.2

## Self-Check: PASSED

All files verified on disk. All 3 commits confirmed in git history. All tests pass. Build and vet pass without errors.

---

*Phase: 04-user-experience-progress-output*
*Completed: 2026-07-10*
