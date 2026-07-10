package output

import (
	"sync"
	"time"
)

const windowSize = 10 // seconds — matches D-10

// sample holds a download speed measurement.
type sample struct {
	at    time.Time
	bytes int64
}

// SpeedTracker tracks download speed using a rolling ring buffer.
type SpeedTracker struct {
	mu      sync.Mutex
	buf     [windowSize]sample
	pos     int
	count   int
	lastETA time.Duration
}

// NewSpeedTracker creates a new SpeedTracker.
func NewSpeedTracker() *SpeedTracker {
	return &SpeedTracker{}
}

// Record records a download of n bytes at the current time.
func (st *SpeedTracker) Record(bytes int64) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.buf[st.pos] = sample{at: time.Now(), bytes: bytes}
	st.pos = (st.pos + 1) % windowSize
	if st.count < windowSize {
		st.count++
	}
}

// Bps calculates the rolling average speed in bytes per second.
// Returns 0 if fewer than 2 samples are available.
func (st *SpeedTracker) Bps() float64 {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.count < 2 {
		return 0
	}

	cutoff := time.Now().Add(-10 * time.Second)
	var totalBytes int64
	var oldestTime time.Time
	validSamples := 0

	for i := 0; i < windowSize; i++ {
		idx := (st.pos - 1 - i + windowSize*2) % windowSize
		if i == 0 {
			oldestTime = st.buf[idx].at
		}
		if st.buf[idx].at.After(cutoff) {
			totalBytes += st.buf[idx].bytes
			validSamples++
		}
	}

	if validSamples < 2 {
		return 0
	}

	elapsed := time.Since(oldestTime).Seconds()
	if elapsed <= 0 {
		return 0
	}

	return float64(totalBytes) / elapsed
}

// ETA calculates the estimated time to download remaining bytes.
// Implements Pitfall 5 mitigation: ETA never goes up.
func (st *SpeedTracker) ETA(remaining int64) time.Duration {
	bps := st.Bps()
	if bps <= 0 {
		return 0
	}

	currentETA := time.Duration(float64(remaining)/bps) * time.Second

	// Pitfall 5: ETA clamping so it never goes up
	if st.lastETA > 0 && currentETA > st.lastETA {
		currentETA = st.lastETA
	}

	// If remaining <= 2 segments worth (approximated as small remaining count)
	if remaining <= 2*1024*1024 { // 2 MB threshold
		return 0
	}

	st.lastETA = currentETA
	return currentETA
}

// clampETA provides a non-increasing ETA by taking the minimum.
// Only exported for testability.
func clampETA(current, last time.Duration) time.Duration {
	if last > 0 && current > last {
		return last
	}
	return current
}

// min returns the smaller of two time.Durations.
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
