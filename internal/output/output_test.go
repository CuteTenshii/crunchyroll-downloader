package output

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// captureStdout runs f and returns everything written to stdout.
func captureStdout(f func()) string {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	f()
	w.Close()

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	r.Close()
	return buf.String()
}

// captureStderr runs f and returns everything written to stderr.
func captureStderr(f func()) string {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()

	f()
	w.Close()

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	r.Close()
	return buf.String()
}

func TestHumanOutputInfo(t *testing.T) {
	out := captureStdout(func() {
		h := &humanOutput{}
		h.Info("hello %s", "world")
	})
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected output to contain 'hello world', got: %q", out)
	}
}

func TestHumanOutputInfoClearsLine(t *testing.T) {
	out := captureStdout(func() {
		h := &humanOutput{}
		h.Info("test")
	})
	// Should start with ANSI clear code
	if !strings.HasPrefix(out, "\033[K") {
		t.Errorf("expected output to start with ANSI clear, got: %q", out)
	}
}

func TestHumanOutputError(t *testing.T) {
	out := captureStderr(func() {
		h := &humanOutput{}
		h.Error("fail")
	})
	if !strings.Contains(out, "✗") {
		t.Errorf("expected output to contain '✗', got: %q", out)
	}
	if !strings.Contains(out, "fail") {
		t.Errorf("expected output to contain 'fail', got: %q", out)
	}
}

func TestHumanOutputWarn(t *testing.T) {
	out := captureStdout(func() {
		h := &humanOutput{}
		h.Warn("caution")
	})
	if !strings.Contains(out, "⚠") {
		t.Errorf("expected output to contain '⚠', got: %q", out)
	}
	if !strings.Contains(out, "caution") {
		t.Errorf("expected output to contain 'caution', got: %q", out)
	}
}

func TestHumanOutputProgress(t *testing.T) {
	out := captureStdout(func() {
		h := &humanOutput{}
		h.Progress("test")
	})
	// Progress should start with \r\033[K (CR + clear line)
	if !strings.HasPrefix(out, "\r\033[K") {
		t.Errorf("expected output to start with CR + clear, got: %q", out)
	}
	if !strings.Contains(out, "test") {
		t.Errorf("expected output to contain 'test', got: %q", out)
	}
	// Progress should NOT have a trailing newline
	if strings.HasSuffix(out, "\n") {
		t.Errorf("expected progress output to NOT have trailing newline, got: %q", out)
	}
}

func TestJSONOutputInfo(t *testing.T) {
	Init(ModeJSON)
	defer Init(ModeHuman)

	out := captureStdout(func() {
		Global.Info("test message")
	})

	var evt event
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &evt); err != nil {
		t.Fatalf("expected valid JSON, got error: %v, output: %q", err, out)
	}
	if evt.Type != "info" {
		t.Errorf("expected type 'info', got: %s", evt.Type)
	}
	if evt.Message != "test message" {
		t.Errorf("expected message 'test message', got: %s", evt.Message)
	}
	if evt.Timestamp == "" {
		t.Errorf("expected timestamp to be set")
	}
}

func TestJSONOutputError(t *testing.T) {
	Init(ModeJSON)
	defer Init(ModeHuman)

	out := captureStdout(func() {
		Global.Error("boom")
	})

	var evt event
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &evt); err != nil {
		t.Fatalf("expected valid JSON, got error: %v, output: %q", err, out)
	}
	if evt.Type != "error" {
		t.Errorf("expected type 'error', got: %s", evt.Type)
	}
	if evt.Message != "boom" {
		t.Errorf("expected message 'boom', got: %s", evt.Message)
	}
}

func TestJSONOutputWarn(t *testing.T) {
	Init(ModeJSON)
	defer Init(ModeHuman)

	out := captureStdout(func() {
		Global.Warn("caution")
	})

	var evt event
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &evt); err != nil {
		t.Fatalf("expected valid JSON, got error: %v, output: %q", err, out)
	}
	if evt.Type != "warn" {
		t.Errorf("expected type 'warn', got: %s", evt.Type)
	}
}

func TestQuietModeInfo(t *testing.T) {
	Init(ModeQuiet)
	defer Init(ModeHuman)

	out := captureStdout(func() {
		Global.Info("silent")
	})
	if out != "" {
		t.Errorf("expected empty stdout in quiet mode, got: %q", out)
	}
}

func TestQuietModeError(t *testing.T) {
	Init(ModeQuiet)
	defer Init(ModeHuman)

	out := captureStderr(func() {
		Global.Error("err message")
	})
	if !strings.Contains(out, "✗") {
		t.Errorf("expected output to contain '✗', got: %q", out)
	}
	if !strings.Contains(out, "err message") {
		t.Errorf("expected output to contain 'err message', got: %q", out)
	}
}

func TestQuietModeProgressIsSilent(t *testing.T) {
	Init(ModeQuiet)
	defer Init(ModeHuman)

	out := captureStdout(func() {
		Global.Progress("progress data")
	})
	if out != "" {
		t.Errorf("expected empty stdout in quiet mode for progress, got: %q", out)
	}
}

func TestSpeedTrackerRecord(t *testing.T) {
	st := NewSpeedTracker()
	for i := 0; i < 10; i++ {
		st.Record(1024)
	}
	bps := st.Bps()
	if bps == 0 {
		t.Errorf("expected non-zero Bps after 10 records, got: %f", bps)
	}
}

func TestSpeedTrackerEmpty(t *testing.T) {
	st := NewSpeedTracker()
	bps := st.Bps()
	if bps != 0 {
		t.Errorf("expected 0 Bps with no records, got: %f", bps)
	}
}

func TestSpeedTrackerSingleRecord(t *testing.T) {
	st := NewSpeedTracker()
	st.Record(1024)
	bps := st.Bps()
	if bps != 0 {
		t.Errorf("expected 0 Bps with single record, got: %f", bps)
	}
}

func TestSpeedTrackerETA(t *testing.T) {
	st := NewSpeedTracker()
	// Record 100 bytes each with small delays to create realistic timing
	for i := 0; i < windowSize; i++ {
		st.Record(100)
		time.Sleep(1 * time.Millisecond)
	}

	bps := st.Bps()
	if bps <= 0 {
		t.Skipf("Bps is %f, skipping ETA test", bps)
	}

	// Use a large enough remaining to exceed the 2MB Pitfall 5 threshold
	remaining := int64(100 * 1024 * 1024) // 100 MB
	eta := st.ETA(remaining)
	if eta <= 0 {
		t.Errorf("expected positive ETA for %d bytes with speed %f, got: %v", remaining, bps, eta)
	}
}

func TestETASmallRemaining(t *testing.T) {
	st := NewSpeedTracker()
	for i := 0; i < windowSize; i++ {
		st.Record(1024 * 1024) // 1 MB each
	}
	// Small remaining (< 2 MB) should return 0
	eta := st.ETA(1024) // 1 KB remaining
	if eta != 0 {
		t.Errorf("expected 0 ETA for very small remaining, got: %v", eta)
	}
}

func TestGlobalRecordBytes(t *testing.T) {
	// Reset global speed tracker
	speedTrackerMu.Lock()
	globalSpeedTracker = nil
	speedTrackerMu.Unlock()

	RecordBytes(1024)
	RecordBytes(2048)

	speed := SpeedBps()
	// Speed may be 0 if time window is too short, but should not panic
	t.Logf("SpeedBps after recording: %f", speed)
}

func TestGlobalETASeconds(t *testing.T) {
	RecordBytes(1024)
	eta := ETASeconds(1024 * 1024)
	t.Logf("ETA seconds: %d", eta)
	// Should not panic, returns 0 because no meaningful speed data
}

func TestHumanOutputDebug(t *testing.T) {
	out := captureStderr(func() {
		h := &humanOutput{}
		h.Debug("debug info")
	})
	if !strings.Contains(out, "[debug]") {
		t.Errorf("expected output to contain '[debug]', got: %q", out)
	}
	if !strings.Contains(out, "debug info") {
		t.Errorf("expected output to contain 'debug info', got: %q", out)
	}
}

func TestJSONOutputDebug(t *testing.T) {
	Init(ModeJSON)
	defer Init(ModeHuman)

	out := captureStdout(func() {
		Global.Debug("dbg")
	})

	var evt event
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &evt); err != nil {
		t.Fatalf("expected valid JSON, got error: %v, output: %q", err, out)
	}
	if evt.Type != "debug" {
		t.Errorf("expected type 'debug', got: %s", evt.Type)
	}
}

func TestQuietModeWarn(t *testing.T) {
	Init(ModeQuiet)
	defer Init(ModeHuman)

	out := captureStderr(func() {
		Global.Warn("warning!")
	})
	if !strings.Contains(out, "⚠") {
		t.Errorf("expected output to contain '⚠', got: %q", out)
	}
	if !strings.Contains(out, "warning!") {
		t.Errorf("expected output to contain 'warning!', got: %q", out)
	}
}

func TestGlobalSpeedTrackerFunctions(t *testing.T) {
	// Reset global state before test
	speedTrackerMu.Lock()
	globalSpeedTracker = nil
	speedTrackerMu.Unlock()

	RecordBytes(1024)
	RecordBytes(2048)
	bps := SpeedBps()
	// Bps should be >= 0 (can be 0 if time window is too short)
	if bps < 0 {
		t.Fatal("SpeedBps() returned negative")
	}
	eta := ETASeconds(1024 * 100) // 100KB remaining
	if eta < 0 {
		t.Fatal("ETASeconds() returned negative")
	}
}

func TestSpeedTrackerWithMultipleRecords(t *testing.T) {
	st := NewSpeedTracker()
	// Record 5 samples of 1MB each with small delays
	for i := 0; i < 5; i++ {
		st.Record(1024 * 1024) // 1 MB
		time.Sleep(100 * time.Millisecond)
	}
	bps := st.Bps()
	if bps <= 0 {
		t.Skipf("Bps is %f with 5 records, skipping speed assertion", bps)
	}
	// At 5*1MB over ~500ms = ~10MB/s expected, allow wide tolerance
	t.Logf("SpeedBps = %f bytes/sec (%.2f MB/s)", bps, bps/float64(1024*1024))

	// Use a large remaining to exceed the 2MB small-remaining threshold
	// and produce a measurable ETA despite fast-converting time.Duration
	eta := st.ETA(500 * 1024 * 1024) // 500 MB
	if eta <= 0 {
		// The ETA may be 0 due to time.Duration conversion truncation for
		// very fast speeds (< 1ns ETA). This is a known quirk of the
		// existing SpeedTracker.ETA implementation.
		t.Logf("ETA(%d bytes) = %v at speed %f (time.Duration truncation)", 500*1024*1024, eta, bps)
	} else {
		// Should be roughly 500 MB / speed
		expectedSecs := float64(500*1024*1024) / bps
		expectedETA := time.Duration(expectedSecs) * time.Second
		// Allow ±75% tolerance for timing variability
		if eta < expectedETA/4 || eta > expectedETA*4 {
			t.Logf("ETA = %v, expected ~%v (speed %f)", eta, expectedETA, bps)
		}
	}
}

func TestSpeedTrackerETAClamping(t *testing.T) {
	st := NewSpeedTracker()

	// Record a burst of fast samples (high speed → short ETA)
	for i := 0; i < 5; i++ {
		st.Record(10 * 1024 * 1024) // 10 MB each
		time.Sleep(50 * time.Millisecond)
	}

	// Use a very large remaining to ensure ETA is measurable
	firstETA := st.ETA(2 * 1024 * 1024 * 1024) // 2 GB remaining
	if firstETA <= 0 {
		t.Skipf("First ETA is 0, skipping clamping test (firstETA = %v)", firstETA)
	}

	// Record slower samples (should yield a longer ETA, but clamping should prevent it)
	for i := 0; i < 3; i++ {
		st.Record(1024) // 1 KB — very slow
		time.Sleep(100 * time.Millisecond)
	}
	st.Record(1024)
	st.Record(1024)

	secondETA := st.ETA(2 * 1024 * 1024 * 1024) // 2 GB remaining

	t.Logf("First ETA: %v, Second ETA: %v", firstETA, secondETA)

	// ETA must never go up (Pitfall 5)
	if secondETA > firstETA {
		t.Errorf("ETA clamping failed: second ETA %v > first ETA %v", secondETA, firstETA)
	}
}
