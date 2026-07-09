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

func TestResolveEtpRtPrefersFlag(t *testing.T) {
	flags := map[string]bool{"etp-rt": true}
	result := resolveEtpRt(flags, "my-cookie", nil)
	if result != "my-cookie" {
		t.Fatalf("resolveEtpRt() = %q, want 'my-cookie'", result)
	}
}

func TestResolveEtpRtPrefersFlagOverEnv(t *testing.T) {
	t.Setenv("CRUNCHYROLL_ETP_RT", "env-cookie")
	flags := map[string]bool{"etp-rt": true}
	result := resolveEtpRt(flags, "flag-cookie", nil)
	if result != "flag-cookie" {
		t.Fatalf("resolveEtpRt() = %q, want 'flag-cookie'", result)
	}
}

func TestResolveEtpRtFallsBackToEnv(t *testing.T) {
	t.Setenv("CRUNCHYROLL_ETP_RT", "env-cookie")
	flags := map[string]bool{} // etp-rt not explicitly set
	result := resolveEtpRt(flags, "", nil)
	if result != "env-cookie" {
		t.Fatalf("resolveEtpRt() = %q, want 'env-cookie'", result)
	}
}

func TestResolveEtpRtFallsBackToConfig(t *testing.T) {
	configVal := "config-cookie"
	flags := map[string]bool{} // etp-rt not explicitly set
	result := resolveEtpRt(flags, "", &configVal)
	if result != "config-cookie" {
		t.Fatalf("resolveEtpRt() = %q, want 'config-cookie'", result)
	}
}

func TestResolveEtpRtReturnsEmptyWhenUnset(t *testing.T) {
	flags := map[string]bool{}
	result := resolveEtpRt(flags, "", nil)
	if result != "" {
		t.Fatalf("resolveEtpRt() = %q, want ''", result)
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
