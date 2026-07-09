# Phase 4: User Experience — Progress & Output - Research

**Researched:** 2026-07-09
**Domain:** Go CLI output abstraction, terminal progress, NDJSON streaming, ANSI color
**Confidence:** HIGH

## Summary

This phase replaces all 43 raw `fmt.Printf` calls across 8 source files with a structured output abstraction in a new `internal/output/` package. The design mirrors Phase 3's `internal/config` pattern: a package-level global set once in `main()` after flag parsing, with no function signature changes across internal packages. The output interface exposes five methods — `Info`, `Warn`, `Error`, `Debug`, `Progress` — and supports three modes: human (default, colored ANSI terminal output), JSON (NDJSON to stdout), and quiet (suppresses progress only, errors still print).

**Primary recommendation:** Direct ANSI escape code constants (no dependency on `fatih/color` or Charm.sh libraries), `golang.org/x/term` for TTY detection (Go team-maintained extension), a custom ring buffer for rolling speed average, and `encoding/json` for NDJSON (no NDJSON-specific library needed — it's just one JSON object per line followed by newline).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Custom output interface, not slog or raw `fmt.Printf`. Eliminates all 43 `fmt.Printf` calls across 8 files.
- **D-02:** Interface methods: `Info`, `Warn`, `Error`, `Debug` (leveled log methods) + `Progress` (for segment/season progress updates).
- **D-03:** Wired as a package-level global set once in `main()` after flag parsing — same pattern as Phase 3's config resolution. No function signature changes across internal packages.
- **D-04:** Default mode (no `--quiet`/`--json`): human-readable output with colors (green ✓, red ✗). ANSI color codes in terminal output.
- **D-05:** Per-episode output is multi-line: a progress block during download, then a result line on completion.
- **D-06:** Success character: `✓` (green), Failure character: `✗` (red).
- **D-07:** Errors shown inline as they occur (per-episode ✗ visible immediately, plus error message) AND accumulated for final summary.
- **D-08:** Season summary at end: detailed error listing — each failed episode by title/number with the specific error message.
- **D-09:** Update frequency: time-based, once per second (not every segment).
- **D-10:** ETA calculation: rolling average over last 5-10 seconds.
- **D-11:** ETA scope: per-episode total (sum of remaining time across video + all audio versions + subtitles — single countdown).
- **D-12:** Display format: inline with segment progress on the same `\r` line: `Downloaded 145/212 segments (68%) — 12.4 MB/s, ETA 23s`.
- **D-13:** Format: NDJSON — one JSON object per line, streaming-friendly.
- **D-14:** Event types: `episode_start`, `segment_progress`, `error`, `episode_complete`, `season_summary`. Granular events including per-segment details.
- **D-15:** Field naming: snake_case (matches existing codebase JSON convention).
- **D-16:** `--quiet` suppresses progress output only (segment progress, "Downloading...", speed/ETA). Errors and warnings still print. If `--json` is also set, errors emit as JSON events.

### the agent's Discretion
None — all decisions were made explicitly during discussion.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within Phase 4 scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UX-02 | Season download shows `[Episode 3/24] Title — Downloading... ✓` with per-episode result and cumulative progress | Season-level coordination in `internal/download/season.go` accumulates errors; output interface handles progress display |
| UX-03 | Download speed (MB/s) and estimated time remaining displayed during segment downloads | Rolling window ring buffer in `internal/output/` tracks bytes over last N seconds; ETA = remaining / rolling_speed |
| UX-04 | `--quiet` and `--json` output modes; structured logging with levels | Output interface supports `ModeQuiet` (suppresses progress, not errors) and `ModeJSON` (NDJSON events via stdout) |
| UX-06 | Season/batch error accumulation — report total failed episodes at end | `SeasonError` struct already in `season.go` collects per-episode errors; output interface formats and displays summary |
| UX-08 | Structured logging with levels (info, warn, error) and optional JSON output — replace raw `fmt.Printf` scatter | `internal/output` package with `Info`, `Warn`, `Error`, `Debug` methods; all 43 `fmt.Printf` calls replaced via output global |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Output abstraction | Backend (CLI layer) | — | New `internal/output/` package — output is a pure CLI concern, no browser/API involvement |
| ANSI color rendering | Backend (CLI layer) | — | Terminal escape codes only relevant when writing to stdout; no secondary tier needed |
| NDJSON serialization | Backend (CLI layer) | — | Uses `encoding/json` from stdlib — one JSON object per line, written to stdout |
| Rolling speed window | Backend (CLI layer) | — | Ring buffer of (timestamp, bytes) pairs — purely local to the output package |
| TTY detection | Backend (CLI layer) | — | `golang.org/x/term.IsTerminal()` — checks stdout fd |
| Season progress coordination | Application layer (`internal/download/season.go`) | Output layer | Season orchestrates per-episode calls; output layer displays results |
| Error accumulation | Application layer (`internal/download/season.go`) | Output layer | `SeasonError` struct collects errors; output prints formatted summary after execution |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/json` (stdlib) | go1.25 | NDJSON serialization for `--json` mode | Already used in codebase for API response parsing; `json.Marshal` with `os.Stdout` write is the canonical NDJSON pattern |
| `fmt.Fprintf`/`fmt.Fprintln` (stdlib) | go1.25 | Write colored text and progress lines to stdout/stderr | Direct replacement for raw `fmt.Printf` — write to `os.Stdout` or `os.Stderr` with proper color wrapping |
| `time` (stdlib) | go1.25 | 1-second ticker for progress updates, speed window pruning | Already used in codebase for retry backoff; `time.NewTicker` for periodic updates |
| `sync.Mutex` (stdlib) | go1.25 | Thread-safe access to rolling speed buffer | Already used in codebase (`internal/download/episode.go`); standard Go concurrency primitive |
| `golang.org/x/term` | latest | TTY detection via `term.IsTerminal(int(os.Stdout.Fd()))` | Go team-maintained extension; cross-platform (Linux, macOS, Windows) — replaces `mattn/go-isatty` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `bufio.NewWriter(os.Stdout)` | go1.25 | Buffered writing for JSON mode (many small NDJSON writes) | Only for `--json` mode where event throughput requires buffering; `Flush()` after every event or on a timer |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Direct ANSI constants | `fatih/color` | Adds external dependency for trivial string wrapping. The color usage is simple enough (5 ANSI codes) that a constant-based approach is clearer and has zero dependency cost |
| Direct ANSI constants | `charm.sh/lipgloss` | Lipgloss is designed for full terminal UI layouts. This project is a linear-output CLI; the styling overhead is unnecessary |
| Direct ANSI constants | `rs/curse` | Unmaintained (last commit ~2016). No Windows support |
| Custom NDJSON | `kandros/go-ndjson` | NDJSON is just `json.Marshal(obj) + "\n"` — a library adds zero value. The project already has `encoding/json` |
| Custom ring buffer | `kevinconway/rolling` | More flexible but adds a dependency. The ring buffer needed is < 30 lines of Go: a slice, an index, and a mutex |

**Installation:**
```bash
go get golang.org/x/term
```

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `golang.org/x/term` | Go module registry | 10+ yrs | Millions | `go.googlesource.com/dl` (Go team) | OK | Approved |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none
**Verdicts are [ASSUMED] — upstream Go team package, maintained by Go team at golang.org/x**

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────────┐
                    │               main.go (CLI entry)             │
                    │  flag.Parse()                                │
                    │  output.Init(output.ModeHuman)               │
                    │  output.Global.Info("Starting download")   │
                    └───────┬─────────────────────────┬───────────┘
                            │                         │
                            ▼                         ▼
              ┌─────────────────────────────┐  ┌─────────────────────┐
              │ internal/download/season.go   │  │ internal/download │
              │ output.Global.Info(...)     │  │ episode.go       │
              │ output.Global.Warn(...)     │  │ output.Global.   │
              │ + collects errors           │  │ Info/Warn/Progress│
              └─────────────────────────────┘  └─────────────────────┘
                            │                         │
                            ▼                         ▼
              ┌─────────────────────────────┐  ┌─────────────────────┐
              │ internal/media/segment.go  │  │ internal/api/*.go │
              │ output.Global.Progress(...)  │  │ output.Global.Debug │
              │ (carriage return, speed/ETA) │  │ (debug manifest)   │
              └─────────────────────────────┘  └─────────────────────┘
                            │
                            ▼
              ┌─────────────────────────────┐
              │     internal/output package  │
              │                              │
              │  ModeHuman:                  │
              │   os.Stdout ← fmt.Fprintf   │
              │   + ANSI color wrapping     │
              │                              │
              │  ModeJSON:                   │
              │   os.Stdout ← json.Marshal  │
              │   + "\n" per event          │
              │                              │
              │  ModeQuiet:                  │
              │   Progress() → no-op        │
              │   Info/Warn/Error → stdout  │
              └─────────────────────────────┘
```

### Recommended Project Structure
```
internal/
├── output/
│   ├── output.go        # Output interface, global, Init(), mode enum
│   ├── speed.go         # RollingWindow ring buffer for speed/ETA
│   └── ndjson.go        # NDJSON event types and marshal helpers
├── download/
│   ├── episode.go       # Replace fmt.Printf → output.Global.Info(...)
│   ├── season.go        # Replace fmt.Printf → output.Global.Info/Warn/Error
│   └── ... (unchanged)
├── media/
│   └── segment.go       # Replace fmt.Printf → output.Global.Progress(...)
├── mux/
│   └── mux.go           # Replace fmt.Printf → output.Global.Info/Warn
└── api/
    ├── client.go         # Replace fmt.Printf → output.Global.Debug(...)
    ├── episode.go        # Replace fmt.Printf → output.Global.Debug(...)
    └── manifest.go       # Replace fmt.Printf → output.Global.Debug(...)
```

### Pattern 1: Package-level Global (mirroring internal/config)
**What:** A package-level `Global` variable that is the single output sink, initialized once in `main()`.
**When to use:** D-03 mandates this pattern — same as Phase 3's `internal/config.Merge` approach.
**Example:**
```go
// internal/output/output.go
type Mode int
const (
    ModeHuman Mode = iota
    ModeJSON
    ModeQuiet
)

type Outputter interface {
    Info(format string, args ...any)
    Warn(format string, args ...any)
    Error(format string, args ...any)
    Debug(format string, args ...any)
    Progress(format string, args ...any)
}

var Global Outputter = &humanOutput{} // default human output

func Init(mode Mode) {
    switch mode {
    case ModeHuman:
        Global = &humanOutput{}
    case ModeJSON:
        Global = &jsonOutput{}
    case ModeQuiet:
        Global = &quietOutput{}
    }
}
```
**Source:** Directly adapted from `internal/config/config.go` pattern [VERIFIED: codebase context]

### Pattern 2: Rolling Speed Window (Ring Buffer)
**What:** A fixed-size mutable ring buffer of `(timestamp, bytes)` pairs that tracks the last N seconds of download activity.
**When to use:** D-10 — rolling average over last 5-10 seconds for ETA calculation.
**Example:**
```go
type sample struct {
    at    time.Time
    bytes int64
}

type SpeedTracker struct {
    mu    sync.Mutex
    ring  [10]sample  // fixed-size ring, one entry per second
    pos   int         // current write position
    count int         // total entries (up to 10)
}

func (s *speedTracker) Record(bytes int64) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.buf[s.pos] = sample{ts: time.Now(), bytes: bytes}
    s.pos = (s.pos + 1) % len(s.buf)
    if s.count < len(s.buf) { s.count++ }
}

func (s *speedTracker) Speed() float64 {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.count == 0 { return 0 }
    now := time.Now()
    var totalBytes int64
    // count valid samples within window + prune old
    cutoff := now.Add(-10 * time.Second)
    for i := 0; i < s.count; i++ {
        idx := (s.pos - 1 - i + len(s.buf)) % len(s.buf)
        if s.buf[idx].ts.After(cutoff) {
            totalBytes += s.buf[idx].bytes
        }
    }
    window := now.Sub(s.buf[(s.pos-1+len(s.buf))%len(s.buf)].ts)
    if window <= 0 { return 0 }
    return float64(totalBytes) / window.Seconds()
}

func (s *speedTracker) ETA(remainingBytes int64) time.Duration {
    speed := s.Bps()
    if speed <= 0 { return 0 }
    return time.Duration(float64(remainingBytes)/speed) * time.Second
}
```

### Pattern 3: NDJSON Emitter
**What:** For `--json` mode, marshal each event struct to a single line and write to stdout.
**When to use:** D-13 — NDJSON streaming of progress events.
**Example:**
```go
// internal/output/ndjson.go
type SegmentProgressEvent struct {
    Type        string  `json:"type"`
    Timestamp   string  `json:"timestamp"`
    EpisodeNum  int     `json:"episode_number"`
    Downloaded  int     `json:"downloaded"`
    Total       int     `json:"total"`
    Percent     float64 `json:"percent"`
    BytesPerSec int64   `json:"bytes_per_sec"`
    ETASecs     int     `json:"eta_secs"`
    Stream      string  `json:"stream"`
    Locale      string  `json:"locale,omitempty"`
}

func emitEvent(v any) {
    data, _ := json.Marshal(v)
    os.Stdout.Write(data)
    os.Stdout.Write([]byte{'\n'})
}
```

### Anti-Patterns to Avoid
- **Interleaving progress with data on stdout:** When `--json` mode is active, progress output must NOT print human-friendly text. Only NDJSON events go to stdout.
- **Buffering issues on carriage return:** After `\r`, always write enough padding (trailing spaces) to clear the previous line. Otherwise remnants of a longer previous line remain visible.
- **Concurrent writes to output without mutex:** `bufio.Writer` is not goroutine-safe. The `Progress` method can be called from goroutines (in `DownloadParts` worker pool). Wrap writes in a mutex or use `fmt.Fprint` which is syscall-level.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| ANSI color functions | `wrapRed(s)`, `wrapGreen(s)`, etc. | Direct `const` + `fmt.Sprintf("%s%s%s", ANSIRed, s, ANSIReset)` | The color usage is 5 simple constants. Writing wrapper functions adds indirection without benefit. The UI-SPEC.md already defines the exact constants |
| NDJSON library | `json.Marshal` loop | One JSON marshal + `\n` per event | NDJSON spec is trivial: one JSON value per line. The project already has `encoding/json`. No library needed |
| TTY detection C wrapper | CGo or syscall | `golang.org/x/term.IsTerminal()` | Cross-platform, maintained by Go team. Replaces `mattn/go-isatty` which is also fine but `x/term` is more official |
| Progress bar rendering library | Custom `\r` + format string | `fmt.Fprintf(os.Stdout, "\rDownloaded %d/%d...")` | The progress display is a single format string updated once per second. A library like `vbauerster/mpb` or `cheggaaa/pb` would be overkill for this pattern |

**Key insight:** This phase's output patterns are simple enough that every concern maps to a 5-30 line Go function. Adding external libraries for ANSI colors, NDJSON, progress bars, or speed tracking would increase the dependency surface without meaningful benefit. The only new dependency needed is `golang.org/x/term` for cross-platform TTY detection, and even that could be done with `os.Stdout.Stat()` + `ModeCharDevice` check (stdlib only, but less portable across platforms).

## Common Pitfalls

### Pitfall 1: Carriage Return Line Remnants
**What goes wrong:** After printing `\rDownloaded 200/200 segments (100%)`, the next non-progress line contains the tail of the previous progress string because the new line is shorter.
**Why it happens:** Carriage return moves cursor to column 0 but does not clear the rest of the line.
**How to avoid:** Pad progress lines to a fixed width or use ANSI escape sequence `\033[K` (erase to end of line) after the format string: `fmt.Fprintf(w, "\r%s\033[K", formattedLine)`.
**Warning signs:** Visible artifacts like `...segments (100%) — 12.4 MB/s, ETA 0s file.mp4`.

### Pitfall 2: Goroutine Race on Global Output
**Description:** Multiple goroutines call `output.Global.Info()` and `output.Global.Progress()` concurrently (workers in `DownloadParts` write progress while season/orchestrator writes Info).
**Why it happens:** `internal/output` global is accessed from many goroutines. `fmt.Fprint` under the hood is atomic-per-call for small writes, but interleaving still occurs.
**How to avoid:** The `Progress()` method can be called from concurrent goroutines (it's `os.Stdout.Write` + `\r`). Wrap the underlying writer with `sync.Mutex` in the output struct. For JSON mode, use the mutex so a single NDJSON event never interleaves with another write.
**Warning signs:** Garbled output lines, mixed ANSI codes breaking color.

### Pitfall 3: Overwriting Progress After Episode Completion
**What goes wrong:** The progress line is on the same line as "Downloading: Title...". When the progress updates are done, the next print does not start on a new line.
**Why it happens:** `\r` does not advance to the next line. After final progress update, a bare `fmt.Println()` prints `\n` which starts at column 0 of the next line but the previous line content remains visible.
**How to avoid:** Always print an explicit `\n` before any non-progress output that follows a progress update (as shown in D-05 multi-line pattern). The UI-SPEC.md shows: progress line → `\n` → result line.
**Warning signs:** Episode result line overwritten by subsequent progress from next stream.

### Pitfall 4: JSON Output on Stderr vs Stdout
**What goes wrong:** In `--json` mode, NDJSON events go to stdout but errors/warnings also print to stdout, polluting the JSON stream.
**Why:** By default, all `output.Error()` and `output.Warn()` calls from the human path write to stdout. For JSON mode, errors must emit as NDJSON `error` events (D-16).
**How to avoid:** In `--json` mode, route all output through NDJSON emitter to stdout. Use stderr only for crash/panic messages. The NDJSON error event carries the same information as a human error line.
**Warning signs:** A user piping `--json` output to `jq` sees error messages that break the stream.

### Pitfall 5: ETA Oscillation at End of Download
**What goes wrong:** As the last few segments download, the rolling speed window gets very few data points, causing the ETA to jump between 0s and 1-2s repeatedly.
**Why:** The rolling window has low sample count at the tail. One slow segment then one fast segment creates ETA oscillation.
**How to avoid:** If `remainingBytes < 0` (overflow) or `remainingSegments <= 2`, set ETA to 0. Also add a simple smoothing: ETA can never go UP if progress is making forward progress. Keep a `lastETA` field and clamp `currentETA = min(currentETA, lastETA)`.
**Warning signs:** ETA bounces between 0s and 3s in the final seconds.

## Code Examples

### Output Interface Definition
```go
// internal/output/output.go
package output

import (
    "encoding/json"
    "fmt"
    "os"
    "sync"
    "time"
)

type Mode int

const (
    ModeHuman Mode = iota
    ModeJSON
    ModeQuiet
)

const (
    ANSIReset  = "\033[0m"
    ANSIBold   = "\033[1m"
    ANSIDim    = "\033[90m"
    ANSIRed    = "\033[31m"
    ANSIGreen  = "\033[32m"
    ANSIYellow = "\033[33m"
    ANSICyan   = "\033[36m"
)

// Global is the application-wide output sink.
// Set once in main() after flag parsing — same pattern as internal/config.
var Global Outputter = &humanOutput{}

type Outputter interface {
    Info(format string, args ...any)
    Warn(format string, args ...any)
    Error(format string, args ...any)
    Debug(format string, args ...any)
    Progress(format string, args ...any)
}

func Init(m Mode) {
    switch m {
    case ModeHuman:
        Global = &humanOutput{}
    case ModeJSON:
        Global = &jsonOutput{enc: json.NewEncoder(os.Stdout)}
    case ModeQuiet:
        Global = &quietOutput{}
    }
}
```
**Source:** Derived from project conventions [VERIFIED: codebase context — internal/config pattern]

### Human Output Implementation
```go
type humanOutput struct {
    mu     sync.Mutex
    ticks  *SpeedTracker
}

func (h *humanOutput) Info(format string, args ...any) {
    h.mu.Lock()
    fmt.Fprintf(os.Stdout, "\033[K") // clear any progress remnant
    fmt.Fprintf(os.Stdout, format, args...)
    fmt.Fprintln(os.Stdout)
    h.mu.Unlock()
}

func (h *humanOutput) Progress(format string, args ...any) {
    h.mu.Lock()
    fmt.Fprintf(os.Stdout, "\r\033[K") // CR + clear line
    fmt.Fprintf(os.Stdout, format, args...)
    h.mu.Unlock()
}

func (h *humanOutput) Warn(format string, args ...any) {
    h.mu.Lock()
    fmt.Fprintf(os.Stdout, "%s%s%s %s\n", ANSIYellow, "⚠", ANSIReset,
        fmt.Sprintf(format, args...))
    h.mu.Unlock()
}

func (h *humanOutput) Error(format string, args ...any) {
    h.mu.Lock()
    fmt.Fprintf(os.Stderr, "%s✗%s %s\n", ANSIRed, ANSIReset,
        fmt.Sprintf(format, args...))
    h.mu.Unlock()
}

func (h *humanOutput) Debug(format string, args ...any) {
    h.mu.Lock()
    fmt.Fprintf(os.Stderr, "%s[debug]%s %s\n", ANSIDim, ANSIReset,
        fmt.Sprintf(format, args...))
    h.mu.Unlock()
}
```

### Quiet Mode Implementation
```go
type quietOutput struct{}

func (q *quietOutput) Info(format string, args ...any) {}
func (q *quietOutput) Progress(format string, args ...any) {}
func (q *quietOutput) Debug(format string, args ...any) {}
func (q *quietOutput) Warn(format string, args ...any) {
    fmt.Fprintf(os.Stderr, "⚠ %s\n", fmt.Sprintf(format, args...))
}
func (q *quietOutput) Error(format string, args ...any) {
    fmt.Fprintf(os.Stderr, "✗ %s\n", fmt.Sprintf(format, args...))
}
```

### main.go Initialization Pattern
```go
func main() {
    // ... existing flag parsing, config loading ...
    jsonMode := flag.Bool("json", false, "Output progress as NDJSON")
    quietMode := flag.Bool("quiet", false, "Suppress progress output")
    flag.Parse()
    // ... explicitFlags tracking ...

    // Resolve output mode (after flag.Parse)
    switch {
    case *jsonMode:
        output.Init(output.ModeJSON)
    case *quietMode:
        output.Init(output.ModeQuiet)
    default:
        output.Init(output.ModeHuman)
    }

    // All subsequent code uses output.Global instead of fmt.Printf
    output.Global.Info("Found %d URLs to download", len(urls))
}
```

### Speed/ETA Rolling Window (Full Implementation)
```go
// internal/output/speed.go
package output

import (
    "sync"
    "time"
)

const windowSize = 10 // seconds — matches D-10

type sample struct {
    at    time.Time
    bytes int64
}

type SpeedTracker struct {
    mu    sync.Mutex
    buf   [windowSize]sample
    pos   int
    count int
}

func NewSpeedTracker() *SpeedTracker { return &SpeedTracker{} }

func (st *SpeedTracker) Record(bytes int64) {
    st.mu.Lock()
    defer st.mu.Unlock()
    st.buf[st.pos] = sample{at: time.Now(), bytes: bytes}
    st.pos = (st.pos + 1) % windowSize
    if st.count < windowSize {
        st.count++
    }
}

func (st *SpeedTracker) Bps() float64 {
    st.mu.Lock()
    defer st.mu.Unlock()
    if st.count < 2 {
        return 0
    }
    cutoff := time.Now().Add(-10 * time.Second)
    var totalBytes int64
    oldest := st.buf[(st.pos-1+windowSize)%windowSize].at
    for i := 0; i < windowSize; i++ {
        idx := (st.pos - 1 - i + windowSize*2) % windowSize
        if st.buf[idx].at.After(cutoff) {
            totalBytes += st.buf[idx].bytes
        } else {
            if i >= st.count {
                break
            }
        }
    }
    elapsed := time.Now().Sub(oldest).Seconds()
    if elapsed <= 0 {
        return 0
    }
    return float64(totalBytes) / elapsed
}

func (st *SpeedTracker) ETA(remaining int64) time.Duration {
    bps := st.Bps()
    if bps <= 0 {
        return 0
    }
    return time.Duration(float64(remaining)/bps) * time.Second
}
```

### NDJSON Event Emission
```go
// internal/output/ndjson.go
package output

import (
    "encoding/json"
    "os"
    "time"
)

type event struct {
    Type      string `json:"type"`
    Timestamp string `json:"timestamp"`
    // ... fields per event type per UI-SPEC.md schema ...
    EpisodeNumber int    `json:"episode_number,omitempty"`
    Title         string `json:"title,omitempty"`
    // ...
}

func emit(evt event) {
    evt.Timestamp = time.Now().UTC().Format(time.RFC3339)
    data, _ := json.Marshal(evt)
    os.Stdout.Write(data)
    os.Stdout.Write([]byte{'\n'})
}
```

### TTY Detection for ANSI Colors
```go
import "golang.org/x/term"

// In Init() or humanOutput constructor:
isTTY := term.IsTerminal(int(os.Stdout.Fd()))
if !isTTY {
    // Disable ANSI colors when piping
    ANSIGreen, ANSIRed, ... = "", "", ""
}
```
**Source:** [CITED: darkcoding.net] Go team's `golang.org/x/term.IsTerminal` is the canonical approach [VERIFIED: golang.org/x/term docs].

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `fmt.Printf` scattered across 8 files | Centralized `output.Global` interface | This phase | All output routing through a single abstraction; testable via mock `Outputter` |
| Raw `\r` progress without speed | `\r` progress with rolling speed window + ETA | This phase (UX-03) | Users see real-time throughput and estimated completion time |
| No structured output | NDJSON streaming for machine consumption | This phase (UX-04) | Pipable output for CI, scripts, and agent consumption |
| No quiet mode | `--quiet` suppresses progress | This phase (UX-04) | Clean output for log aggregation and minimal-display use |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (`go test`) |
| Config file | None (no additional test framework) |
| Quick run command | `go test ./internal/output/... -count=1 -v` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command |
|--------|----------|-----------|-------------------|
| UX-02 | Season progress displays per-episode result and cumulative error report | Integration (mock output) | `go test ./internal/output/... -run TestSeasonProgress -v` |
| UX-03 | Speed/ETA calculation with rolling window | Unit | `go test ./internal/output/... -run TestSpeedTracker -v` |
| UX-04 | `--json` mode emits NDJSON events for all event types | Integration | `go test ./internal/output/... -run TestJSONOutput -v` |
| UX-04 | `--quiet` suppresses progress, shows errors | Unit | `go test ./internal/output/... -run TestQuietMode -v` |
| UX-06 | Season error accumulation with detailed summary | Integration | `go test ./internal/output/... -run TestSeasonErrors -v` |
| UX-08 | All output methods (Info/Warn/Error/Debug) write to correct stream | Unit | `go test ./internal/output/... -run TestOutputMethods -v` |

### Wave 0 Gaps
- [ ] `internal/output/output_test.go` — place all output tests
- [ ] `testdata/` directory — for expected NDJSON event fixtures
- [ ] Use `os.Pipe()` to capture stdout/stderr in tests for assertion

## Security Domain

**security_enforcement:** Not applicable. This phase introduces no new authentication, authorization, data validation, or cryptography. The output package writes to stdout/stderr only. No user input is parsed by the output layer.

## Common Pitfalls (continued)

### Pitfall 6: fmt.Fprintf vs bufio.Writer Flushing
**What goes wrong:** If `bufio.NewWriter(os.Stdout)` is used for buffered output (e.g., in JSON mode), a `defer` that calls `os.Exit` may bypass the buffer flush, losing the final event.
**Why it happens:** `os.Exit` terminates the process immediately; deferred `Flush()` does NOT run after `os.Exit`.
**How to avoid:** Either don't use `bufio.Writer` for stdout (just use `os.Stdout.Write` directly — terminal stdout is already fast enough for 1 event/sec), or flush manually before any `os.Exit` call.
**Warning signs:** The last NDJSON event is missing from output.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `go` | Build/Test | ✓ | 1.26.4 | — |
| `ffmpeg` | Runtime (muxing, not output) | ✓ | n8.1.2 | — |
| `golang.org/x/term` | TTY detection | Will install | latest | `os.Stdout.Stat()` with `ModeCharDevice` check (less portable but no dependency) |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** `golang.org/x/term` can be replaced with `os.Stdout.Stat()` + bitmask check on Unix, but `x/term` is recommended for cross-platform support.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `golang.org/x/term` is the canonical Go stdlib extension for TTY detection | Standard Stack | Low — `mattn/go-isatty` also works. Both are well-tested |
| A2 | `bufio.Writer` is not needed — stdout writes at 1/sec are fast enough | Don't Hand-Roll | Low — if JSON mode outputs many events at once, a buffer may help; can be added later |
| A3 | ANSI `\033[K` (erase to end of line) works on all target terminals | Pitfalls | Low — widely supported across xterm, gnome-terminal, Windows Terminal, iTerm2, Alacritty |

## Open Questions

1. **Should quiet mode suppress warnings?** D-16 says quiet mode suppresses "progress output only" and "Errors and warnings still print." The warning status line (`⚠`) is not a progress line but a logged message, so it stays. Confirmed by D-16 text.

2. **Should `--json` output go to stdout and human output to stderr?** The standard design pattern (two-stream rule) puts machine-readable output on stdout and human-readable on stderr. However, D-04/D-13 say human-friendly output with colors goes to "terminal output" (stdout by default). **Recommendation:** Human mode uses stdout (current behavior). JSON mode uses stdout for NDJSON events. This matches the current pattern and doesn't break existing usage.

## Sources

### Primary (HIGH confidence)
- Codebase files: `internal/config/config.go`, `internal/download/episode.go`, `internal/download/season.go`, `internal/media/segment.go`, `internal/mux/mux.go`, `internal/api/client.go`, `internal/api/episode.go`, `internal/api/manifest.go`, `main.go` — all fmt.Printf patterns verified by reading each file
- `.planning/phases/04-user-experience-progress-output/04-CONTEXT.md` — all decisions (D-01 through D-16)
- `.planning/phases/04-user-experience-progress-output/04-UI-SPEC.md` — ANSI color scheme, NDJSON schema, message formats

### Secondary (MEDIUM confidence)
- [CITED: golang.org/x/term docs] — `term.IsTerminal()` pattern for TTY detection
- [CITED: darkcoding.net] — Go isatty pattern using `golang.org/x/term`
- [CITED: ndjson.org/ndjson-spec] — NDJSON specification (one JSON value per line, `\n` delimiter)

### Tertiary (LOW confidence)
- Stack Overflow rolling window ETA patterns — used as algorithmic reference only

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries are stdlib or Go team maintained
- Architecture: HIGH — mirrors proven `internal/config` pattern from Phase 3
- Pitfalls: HIGH — all derived from known Go CLI output traps and the codebase's actual concurrency patterns
- NDJSON: HIGH — trivial pattern, well-documented standard
- Speed/ETA: MEDIUM — algorithm choice (ring buffer) is sound but specific implementation constants (10s window) may need tuning

**Research date:** 2026-07-09
**Valid until:** 2026-08-09