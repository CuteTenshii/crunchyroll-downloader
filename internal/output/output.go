package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"golang.org/x/term"
)

// Mode represents the output mode.
type Mode int

const (
	ModeHuman Mode = iota
	ModeJSON
	ModeQuiet
)

// ANSI color escape constants.
var (
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

// Outputter defines the output interface with five methods.
type Outputter interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
	Debug(format string, args ...any)
	Progress(format string, args ...any)
}

// package-level speed tracker state
var (
	speedTrackerMu    sync.Mutex
	globalSpeedTracker *SpeedTracker
)

// RecordBytes records n bytes downloaded via the global speed tracker.
func RecordBytes(n int64) {
	speedTrackerMu.Lock()
	defer speedTrackerMu.Unlock()
	if globalSpeedTracker == nil {
		globalSpeedTracker = NewSpeedTracker()
	}
	globalSpeedTracker.Record(n)
}

// SpeedBps returns the current rolling speed in bytes per second.
func SpeedBps() float64 {
	speedTrackerMu.Lock()
	defer speedTrackerMu.Unlock()
	if globalSpeedTracker == nil {
		return 0
	}
	return globalSpeedTracker.Bps()
}

// ETASeconds returns the estimated seconds remaining for the given bytes.
func ETASeconds(remaining int64) int {
	speedTrackerMu.Lock()
	defer speedTrackerMu.Unlock()
	if globalSpeedTracker == nil {
		return 0
	}
	return int(globalSpeedTracker.ETA(remaining).Seconds())
}

// Init sets Global to the correct implementation based on mode.
// It also performs TTY detection to disable ANSI colors when not connected
// to a terminal.
func Init(m Mode) {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if !isTTY {
		ANSIReset = ""
		ANSIBold = ""
		ANSIDim = ""
		ANSIRed = ""
		ANSIGreen = ""
		ANSIYellow = ""
		ANSICyan = ""
	}

	switch m {
	case ModeHuman:
		Global = &humanOutput{speedTracker: NewSpeedTracker()}
	case ModeJSON:
		Global = &jsonOutput{enc: json.NewEncoder(os.Stdout), speedTracker: NewSpeedTracker()}
	case ModeQuiet:
		Global = &quietOutput{}
	}
}

// humanOutput implements Outputter for colored terminal output.
type humanOutput struct {
	mu           sync.Mutex
	speedTracker *SpeedTracker
}

func (h *humanOutput) Info(format string, args ...any) {
	h.mu.Lock()
	fmt.Fprintf(os.Stdout, "\033[K") // clear line remnants
	fmt.Fprintf(os.Stdout, format, args...)
	fmt.Fprintln(os.Stdout)
	h.mu.Unlock()
}

func (h *humanOutput) Warn(format string, args ...any) {
	h.mu.Lock()
	fmt.Fprintf(os.Stdout, "\033[K%s⚠%s %s\n", ANSIYellow, ANSIReset, fmt.Sprintf(format, args...))
	h.mu.Unlock()
}

func (h *humanOutput) Error(format string, args ...any) {
	h.mu.Lock()
	fmt.Fprintf(os.Stderr, "%s✗%s %s\n", ANSIRed, ANSIReset, fmt.Sprintf(format, args...))
	h.mu.Unlock()
}

func (h *humanOutput) Debug(format string, args ...any) {
	h.mu.Lock()
	fmt.Fprintf(os.Stderr, "%s[debug]%s %s\n", ANSIDim, ANSIReset, fmt.Sprintf(format, args...))
	h.mu.Unlock()
}

func (h *humanOutput) Progress(format string, args ...any) {
	h.mu.Lock()
	fmt.Fprintf(os.Stdout, "\r\033[K") // CR + clear line
	fmt.Fprintf(os.Stdout, format, args...)
	h.mu.Unlock()
}

// jsonOutput implements Outputter for NDJSON streaming.
type jsonOutput struct {
	mu           sync.Mutex
	enc          *json.Encoder
	speedTracker *SpeedTracker
}

func (j *jsonOutput) Info(format string, args ...any) {
	j.mu.Lock()
	emitEvent(event{Type: "info", Message: fmt.Sprintf(format, args...)})
	j.mu.Unlock()
}

func (j *jsonOutput) Warn(format string, args ...any) {
	j.mu.Lock()
	emitEvent(event{Type: "warn", Message: fmt.Sprintf(format, args...)})
	j.mu.Unlock()
}

func (j *jsonOutput) Error(format string, args ...any) {
	j.mu.Lock()
	emitEvent(event{Type: "error", Message: fmt.Sprintf(format, args...)})
	j.mu.Unlock()
}

func (j *jsonOutput) Debug(format string, args ...any) {
	j.mu.Lock()
	emitEvent(event{Type: "debug", Message: fmt.Sprintf(format, args...)})
	j.mu.Unlock()
}

func (j *jsonOutput) Progress(format string, args ...any) {
	j.mu.Lock()
	emitEvent(event{Type: "segment_progress", Message: fmt.Sprintf(format, args...)})
	j.mu.Unlock()
}

// quietOutput implements Outputter suppressing progress/info but showing errors and warnings.
type quietOutput struct{}

func (q *quietOutput) Info(format string, args ...any) {}

func (q *quietOutput) Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\033[K⚠ %s\n", fmt.Sprintf(format, args...))
}

func (q *quietOutput) Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\033[K✗ %s\n", fmt.Sprintf(format, args...))
}

func (q *quietOutput) Debug(format string, args ...any) {}

func (q *quietOutput) Progress(format string, args ...any) {}
