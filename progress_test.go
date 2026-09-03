package main

import (
	"strings"
	"testing"
)

func TestProgressBarRenderLine(t *testing.T) {
	bar := &progressBar{title: "video", label: "segments", total: 100, current: 42}
	line := bar.renderLine(displayWidth("video"))
	if !strings.Contains(line, "42%") {
		t.Fatalf("renderLine() = %q, want it to include the percentage", line)
	}
	if !strings.Contains(line, "(42/100 segments)") {
		t.Fatalf("renderLine() = %q, want it to include the segment count", line)
	}
	if !strings.HasPrefix(line, "video: ") {
		t.Fatalf("renderLine() = %q, want it prefixed with the title", line)
	}
}

func TestProgressBarRenderLineClampsAndHidesDetail(t *testing.T) {
	bar := &progressBar{title: "video", label: "segments", total: 100, current: 150}
	line := bar.renderLine(displayWidth("video"))
	if !strings.Contains(line, "100%") || !strings.Contains(line, "(100/100 segments)") {
		t.Fatalf("renderLine() = %q, want it clamped to 100%%", line)
	}

	bar = &progressBar{total: 100, current: 50}
	if line := bar.renderLine(0); !strings.HasSuffix(line, "50%") {
		t.Fatalf("renderLine() = %q, want it to omit label and title when unset", line)
	}
}

func TestProgressBarRenderLineAlignsTitles(t *testing.T) {
	bars := []*progressBar{
		{title: "Downloading video"},
		{title: "Downloading English audio"},
		{title: "Downloading 日本語 audio"},
	}
	titleWidth := 0
	for _, b := range bars {
		if w := displayWidth(b.title); w > titleWidth {
			titleWidth = w
		}
	}

	for _, b := range bars {
		line := b.renderLine(titleWidth)
		idx := strings.Index(line, ": ")
		if idx < 0 {
			t.Fatalf("renderLine(%q) = %q, missing title separator", b.title, line)
		}
		if got := displayWidth(line[:idx]); got != titleWidth {
			t.Fatalf("renderLine(%q) = %q, bar starts at column %d, want %d", b.title, line, got, titleWidth)
		}
	}
}
