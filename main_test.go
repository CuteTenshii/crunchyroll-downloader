package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestProcessURLRejectsInvalidContentIDLength(t *testing.T) {
	output := captureMainStdout(t, func() {
		processURL(context.Background(), nil, "https://www.crunchyroll.com/watch/short/title")
	})

	if !strings.Contains(output, "Invalid URL format") {
		t.Fatalf("processURL() output = %q, want invalid URL format message", output)
	}
}

func TestProcessURLRejectsUnsupportedContentType(t *testing.T) {
	output := captureMainStdout(t, func() {
		processURL(context.Background(), nil, "https://www.crunchyroll.com/browse/G123456789/title")
	})

	if !strings.Contains(output, "Invalid URL (must be /watch/ or /series/)") {
		t.Fatalf("processURL() output = %q, want unsupported content type message", output)
	}
}

func captureMainStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe(): %v", err)
	}
	os.Stdout = writePipe

	fn()

	if err := writePipe.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	os.Stdout = original

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, readPipe); err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	return buf.String()
}
