package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadETPRTFileRequiresPrivateRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "etp_rt")
	if err := os.WriteFile(path, []byte(" secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readETPRTFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "secret" {
		t.Fatalf("got %q", got)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	_, err = readETPRTFile(path)
	var unsafe *CredentialFileError
	if !errors.As(err, &unsafe) {
		t.Fatalf("expected CredentialFileError, got %v", err)
	}
}

func TestLoadETPRTRequiresFileEvenWhenLegacyEnvironmentValueExists(t *testing.T) {
	t.Setenv("CRUNCHYROLL_ETP_RT", "legacy-value-must-not-be-used")
	if _, err := loadETPRT(""); err == nil || err.Error() != "provide --etp-rt-file with a 0600 regular file" {
		t.Fatalf("expected file-only credential error, got %v", err)
	}
}

func TestValidatePlaybackRetryConfig(t *testing.T) {
	for _, test := range []struct {
		name    string
		retries int
		backoff time.Duration
	}{
		{name: "negative retries", retries: -1, backoff: time.Second},
		{name: "too many retries", retries: maxPlayback4294Retries + 1, backoff: time.Second},
		{name: "zero backoff", retries: 1, backoff: 0},
		{name: "excessive backoff", retries: 1, backoff: maxPlayback4294Backoff + time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validatePlaybackRetryConfig(test.retries, test.backoff); err == nil {
				t.Fatal("expected invalid playback retry configuration")
			}
		})
	}
	if err := validatePlaybackRetryConfig(defaultPlayback4294Retries, defaultPlayback4294Backoff); err != nil {
		t.Fatalf("defaults are invalid: %v", err)
	}
}

func TestReadETPRTFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "etp_rt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	_, err := readETPRTFile(link)
	var unsafe *CredentialFileError
	if !errors.As(err, &unsafe) || unsafe.Problem != "must not be a symlink" {
		t.Fatalf("expected symlink CredentialFileError, got %v", err)
	}
}

func TestValidateOpenedCredentialFileRejectsReplacement(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first")
	second := filepath.Join(dir, "second")
	if err := os.WriteFile(first, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	entryInfo, err := os.Lstat(first)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(second)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	err = validateOpenedCredentialFile(first, entryInfo, openedInfo)
	var unsafe *CredentialFileError
	if !errors.As(err, &unsafe) || unsafe.Problem != "changed after validation" {
		t.Fatalf("expected replacement CredentialFileError, got %v", err)
	}
}
