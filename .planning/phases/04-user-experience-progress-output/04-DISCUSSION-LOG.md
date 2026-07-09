# Phase 4: User Experience — Progress & Output - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-09
**Phase:** 4-User Experience — Progress & Output
**Areas discussed:** Output abstraction, Season progress format, Speed/ETA display, JSON output schema

---

## Output Abstraction

| Option | Description | Selected |
|--------|-------------|----------|
| slog.Logger via context | Use Go 1.21+ stdlib slog, pass through context/params | |
| Custom output interface | Define interface with Info/Warn/Error/Debug + Progress methods | ✓ |
| Package-level output state | Global output struct set in main() | |

**User's choice:** Custom output interface

| Option | Description | Selected |
|--------|-------------|----------|
| Info/Warn/Error/Debug + Progress | Standard log levels + Progress method | ✓ |
| Leveled methods only, progress separate | Just Info/Warn/Error/Debug, progress through separate writer | |

**User's choice:** Info/Warn/Error/Debug + Progress

| Option | Description | Selected |
|--------|-------------|----------|
| Pass as parameter | Add output.Logger param to internal functions | |
| Package-level global set in main() | Set var Log once in main() | ✓ |

**User's choice:** Package-level global (same pattern as Phase 3 config)

| Option | Description | Selected |
|--------|-------------|----------|
| Human-readable colored output | Default mode with colored text (green ✓, red ✗) | ✓ |
| Plain text, no colors | Keep current plain text, no ANSI codes | |

**User's choice:** Human-readable colored output

---

## Season Progress Format

| Option | Description | Selected |
|--------|-------------|----------|
| ✗ (red/green coloring) | Green ✓ for success, red ✗ for failure | ✓ |
| ! or ⚠ | Avoids Unicode issues, less distinct | |

**User's choice:** ✗ with red/green coloring

| Option | Description | Selected |
|--------|-------------|----------|
| Single line per episode, updated in place | Like segment \\r pattern, replaces on complete | |
| Multi-line: progress block then result | Separate lines for progress and result | ✓ |

**User's choice:** Multi-line (progress block then result)

| Option | Description | Selected |
|--------|-------------|----------|
| Compact summary line | Single line: "Season completed: 22/24 succeeded" | |
| Detailed error listing | Lists each failed episode with error | ✓ |

**User's choice:** Detailed error listing

| Option | Description | Selected |
|--------|-------------|----------|
| Show inline + accumulate for summary | Error visible immediately AND in final summary | ✓ |
| Accumulate only, show in summary | Error details deferred to end | |

**User's choice:** Show inline + accumulate for summary

---

## Speed/ETA Display

| Option | Description | Selected |
|--------|-------------|----------|
| Time-based: every 1 second | Update at most once per second | ✓ |
| Every N segments | Update every 5-10 segments | |

**User's choice:** Time-based, every 1 second

| Option | Description | Selected |
|--------|-------------|----------|
| Rolling average (last 5-10 seconds) | Smooth speed, converges well | ✓ |
| Simple average (total / elapsed) | Simple but spikes early | |

**User's choice:** Rolling average

| Option | Description | Selected |
|--------|-------------|----------|
| Per-stream | Speed/ETA for current stream only | |
| Per-episode total | Single ETA across all streams | ✓ |

**User's choice:** Per-episode total ETA

| Option | Description | Selected |
|--------|-------------|----------|
| Inline with segment progress | Same line: "145/212 (68%) — 12.4 MB/s, ETA 23s" | ✓ |
| Separate status line | Speed/ETA on dedicated line above | |

**User's choice:** Inline with segment progress

---

## JSON Output Schema

| Option | Description | Selected |
|--------|-------------|----------|
| NDJSON | One JSON object per line | ✓ |
| Single JSON blob | Array of events emitted at end | |

**User's choice:** NDJSON

| Option | Description | Selected |
|--------|-------------|----------|
| episode_start, segment_progress, error, episode_complete, season_summary | Granular events | ✓ |
| episode_start, episode_complete, error, season_summary | Coarse events, no segment details | |

**User's choice:** Granular events (including segment_progress)

| Option | Description | Selected |
|--------|-------------|----------|
| snake_case | Matches existing Go JSON tags | ✓ |
| camelCase | More common in JS tooling | |

**User's choice:** snake_case

| Option | Description | Selected |
|--------|-------------|----------|
| Only progress output (keeps errors) | --quiet suppresses progress, keeps errors | ✓ |
| All output except --json events | Absolute silence unless --json | |

**User's choice:** Only progress suppressed, errors kept

---

## the agent's Discretion

None — all decisions made explicitly.

## Deferred Ideas

None.

