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
		processURL(context.Background(), nil, "https://www.crunchyroll.com/watch/short/title", "")
	})

	if !strings.Contains(output, "Invalid URL format") {
		t.Fatalf("processURL() output = %q, want invalid URL format message", output)
	}
}

func TestProcessURLRejectsUnsupportedContentType(t *testing.T) {
	output := captureMainStdout(t, func() {
		processURL(context.Background(), nil, "https://www.crunchyroll.com/browse/G123456789/title", "")
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

func TestValidateURLValidWatchPath(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/watch/GGGGGGGGG")
	if err != nil {
		t.Fatalf("validateURL() error = %v, want nil", err)
	}
}

func TestValidateURLValidSeriesPath(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/series/GGGGGGGGGGGG")
	if err != nil {
		t.Fatalf("validateURL() error = %v, want nil", err)
	}
}

func TestValidateURLTooShort(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/watch/short")
	if err == nil {
		t.Fatal("validateURL() error = nil, want content ID length error")
	}
	if !strings.Contains(err.Error(), "content ID length") {
		t.Fatalf("validateURL() error = %q, want content ID length error", err)
	}
}

func TestValidateURLTooLong(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/watch/GGGGGGGGGGGGGGG")
	if err == nil {
		t.Fatal("validateURL() error = nil, want content ID length error")
	}
	if !strings.Contains(err.Error(), "content ID length") {
		t.Fatalf("validateURL() error = %q, want content ID length error", err)
	}
}

func TestValidateURLWrongContentType(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/browse/GGGGGGGGG")
	if err == nil {
		t.Fatal("validateURL() error = nil, want watch/series error")
	}
	if !strings.Contains(err.Error(), "must be /watch/ or /series/") {
		t.Fatalf("validateURL() error = %q, want watch/series error", err)
	}
}

func TestValidateURLTrailingSlash(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/watch/GGGGGGGGG/")
	if err != nil {
		t.Fatalf("validateURL() with trailing slash error = %v, want nil", err)
	}
}

func TestValidateURLWithQueryParams(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/watch/GGGGGGGGG?foo=bar")
	if err != nil {
		t.Fatalf("validateURL() with query params error = %v, want nil", err)
	}
}

func TestOutputDirMissingDirErrors(t *testing.T) {
	errMsg := validateOutputDir("/nonexistent/path/that/definitely/does/not/exist")
	if errMsg == "" {
		t.Fatal("validateOutputDir() returned empty, want error message for nonexistent path")
	}
	if !strings.Contains(errMsg, "does not exist") {
		t.Fatalf("validateOutputDir() = %q, want 'does not exist' message", errMsg)
	}
}

func TestOutputDirEmptyIsValid(t *testing.T) {
	errMsg := validateOutputDir("")
	if errMsg != "" {
		t.Fatalf("validateOutputDir('') = %q, want empty string", errMsg)
	}
}

func TestOutputDirFileIsNotValid(t *testing.T) {
	f := t.TempDir() + "/notadir"
	if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	errMsg := validateOutputDir(f)
	if errMsg == "" {
		t.Fatal("validateOutputDir(file) returned empty, want error for file path")
	}
	if !strings.Contains(errMsg, "not a directory") {
		t.Fatalf("validateOutputDir(file) = %q, want 'not a directory' message", errMsg)
	}
}

func TestOutputDirValidDirPasses(t *testing.T) {
	dir := t.TempDir()
	errMsg := validateOutputDir(dir)
	if errMsg != "" {
		t.Fatalf("validateOutputDir(%q) = %q, want empty", dir, errMsg)
	}
}

func TestValidateAllURLsReportsAll(t *testing.T) {
	urls := []string{
		"https://www.crunchyroll.com/watch/GGGGGGGGG",      // valid
		"https://www.crunchyroll.com/series/GGGGGGGGGGGG",   // valid
		"https://www.crunchyroll.com/watch/short",            // too short
		"https://www.crunchyroll.com/browse/GGGGGGGGG",       // wrong type
		"https://www.crunchyroll.com/watch/GGGGGGGGGGGGGGG",  // too long
	}
	invalid := validateAllURLs(urls)
	if len(invalid) != 3 {
		t.Fatalf("validateAllURLs() returned %d invalid URLs, want 3", len(invalid))
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
