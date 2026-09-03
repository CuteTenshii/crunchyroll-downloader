package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// progressWidth is the number of columns the bar itself occupies, excluding the
// surrounding brackets and the trailing percentage.
const progressWidth = 40

// esc is the ANSI escape prefix used to move the cursor and clear lines.
const esc = "\x1b"

const (
	// rateWindow is the sliding window over which the download rate is averaged.
	rateWindow = 2 * time.Second
	// sampleInterval bounds how often a rate sample is recorded, so a fast
	// download does not accumulate an unbounded number of samples.
	sampleInterval = 200 * time.Millisecond
)

// progressRenderer draws a set of progress bars, one per line, and rewrites them
// in place as they advance. It exists so parallel video/audio downloads can both
// report progress without clobbering each other's terminal lines.
var progressRenderer = struct {
	mu    sync.Mutex
	bars  []*progressBar
	lines int
}{}

// byteSample pairs a cumulative byte count with the time it was observed, for
// computing the download rate.
type byteSample struct {
	at    time.Time
	bytes int64
}

// progressBar tracks the completion of a single download and renders one line
// of the progress display.
type progressBar struct {
	title   string // what is being downloaded, e.g. "video" or "audio 日本語"
	label   string // unit shown after the count, e.g. "segments"
	total   int64
	current int64
	samples []byteSample // recent (time, bytes) observations for the rate
}

// newProgressBar registers a bar for a download. Callers that do not know the
// total until after the first response may pass 0 and use setTotal later.
func newProgressBar(title string, total int64, label string) *progressBar {
	progressRenderer.mu.Lock()
	defer progressRenderer.mu.Unlock()
	b := &progressBar{title: title, total: total, label: label}
	progressRenderer.bars = append(progressRenderer.bars, b)
	renderLocked()
	return b
}

// setTotal records the total once it is known, ignoring later calls so a
// partially-streamed response cannot shrink it.
func (b *progressBar) setTotal(total int64) {
	progressRenderer.mu.Lock()
	defer progressRenderer.mu.Unlock()
	if b.total == 0 && total > 0 {
		b.total = total
	}
}

// update advances the bar to the given value and records the cumulative byte
// count for the rate display, then redraws the display. Updates from concurrent
// downloads can arrive out of order, so the bar never moves backwards.
func (b *progressBar) update(current, bytes int64) {
	progressRenderer.mu.Lock()
	defer progressRenderer.mu.Unlock()

	if current < 0 {
		current = 0
	}
	if current <= b.current {
		return
	}
	if b.total > 0 && current > b.total {
		current = b.total
	}
	b.current = current
	b.recordBytes(bytes)
	renderLocked()
}

// finish removes the bar from the display, leaving the remaining bars redrawn.
func (b *progressBar) finish() {
	progressRenderer.mu.Lock()
	defer progressRenderer.mu.Unlock()
	for i, bar := range progressRenderer.bars {
		if bar == b {
			progressRenderer.bars = append(progressRenderer.bars[:i], progressRenderer.bars[i+1:]...)
			break
		}
	}
	renderLocked()
}

// recordBytes appends a rate sample, pruning samples older than rateWindow. It
// must be called with progressRenderer.mu held.
func (b *progressBar) recordBytes(bytes int64) {
	now := time.Now()
	if len(b.samples) > 0 && now.Sub(b.samples[len(b.samples)-1].at) < sampleInterval {
		b.samples[len(b.samples)-1].bytes = bytes
		return
	}
	b.samples = append(b.samples, byteSample{at: now, bytes: bytes})
	cutoff := now.Add(-rateWindow)
	for len(b.samples) > 2 && b.samples[1].at.Before(cutoff) {
		b.samples = b.samples[1:]
	}
}

// rateMbps returns the download rate in megabits per second, averaged over the
// rateWindow. It returns 0 until enough samples have accumulated.
func (b *progressBar) rateMbps() float64 {
	if len(b.samples) < 2 {
		return 0
	}
	first := b.samples[0]
	last := b.samples[len(b.samples)-1]
	elapsed := last.at.Sub(first.at).Seconds()
	if elapsed <= 0 {
		return 0
	}
	bytesPerSec := float64(last.bytes-first.bytes) / elapsed
	return bytesPerSec * 8 / 1e6
}

// renderLine formats this bar as a single line of text, padding the title to
// titleWidth columns so the bars on every line start at the same column.
func (b *progressBar) renderLine(titleWidth int) string {
	current := b.current
	total := b.total
	percent := int64(0)
	filled := 0
	if total > 0 {
		if current > total {
			current = total
		}
		percent = current * 100 / total
		filled = int(current * progressWidth / total)
	}

	bar := ""
	if filled > 0 {
		bar = strings.Repeat("=", filled-1) + ">"
	}
	line := fmt.Sprintf("[%s%s] %3d%%", bar, strings.Repeat(" ", progressWidth-filled), percent)
	if b.label != "" {
		line += fmt.Sprintf(" (%d/%d %s)", current, total, b.label)
	}
	if rate := b.rateMbps(); rate > 0 {
		line += fmt.Sprintf(" %.1f Mbps", rate)
	}
	if b.title != "" {
		pad := titleWidth - displayWidth(b.title)
		if pad < 0 {
			pad = 0
		}
		line = b.title + strings.Repeat(" ", pad) + ": " + line
	}
	return line
}

// renderLocked rewrites every bar on its own line, moving the cursor back over
// the previous frame first. It must be called with progressRenderer.mu held.
func renderLocked() {
	titleWidth := 0
	for _, b := range progressRenderer.bars {
		if w := displayWidth(b.title); w > titleWidth {
			titleWidth = w
		}
	}

	prev := progressRenderer.lines
	if prev > 0 {
		fmt.Printf("%s[%dA", esc, prev)
	}
	for _, b := range progressRenderer.bars {
		fmt.Printf("%s[2K\r%s\n", esc, b.renderLine(titleWidth))
	}
	now := len(progressRenderer.bars)
	for i := now; i < prev; i++ {
		fmt.Printf("%s[2K\r\n", esc)
	}
	if prev > now {
		fmt.Printf("%s[%dA", esc, prev-now)
	}
	progressRenderer.lines = now
}

// displayWidth returns the number of terminal columns s occupies, counting CJK
// and other double-width runes as two columns so the progress bars line up
// regardless of the locale names used in the titles.
func displayWidth(s string) int {
	width := 0
	for _, r := range s {
		if doubleWidth(r) {
			width += 2
		} else {
			width++
		}
	}
	return width
}

// doubleWidth reports whether r occupies two terminal columns.
func doubleWidth(r rune) bool {
	return r >= 0x1100 && (r <= 0x115F ||
		r == 0x2329 || r == 0x232A ||
		(r >= 0x2E80 && r <= 0xA4CF && r != 0x303F) ||
		(r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE10 && r <= 0xFE19) ||
		(r >= 0xFE30 && r <= 0xFE6F) ||
		(r >= 0xFF00 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x20000 && r <= 0x2FFFD) ||
		(r >= 0x30000 && r <= 0x3FFFD))
}
