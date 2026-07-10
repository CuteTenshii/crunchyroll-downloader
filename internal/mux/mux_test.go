package mux

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"crunchyroll-downloader/internal/api"
)

func TestMergeEverythingReturnsErrorAndRemovesPartialOutputOnFFmpegFailure(t *testing.T) {
	restoreFFmpegCommand(t, "1", "bad mux")

	dir := t.TempDir()
	videoFile := writeTempFile(t, dir, "video.mp4")
	audioFile := writeTempFile(t, dir, "audio.mp3")
	outputFile := writeTempFile(t, dir, "partial.mkv")

	err := MergeEverything(context.Background(), videoFile, []MediaTrack{{File: audioFile, Locale: "ja-JP"}}, nil, outputFile, testEpisodeInfo())
	if err == nil {
		t.Fatal("MergeEverything() error = nil, want ffmpeg error")
	}
	if !strings.Contains(err.Error(), "ffmpeg failed") {
		t.Fatalf("MergeEverything() error = %q, want ffmpeg failure", err)
	}
	if _, statErr := os.Stat(outputFile); !os.IsNotExist(statErr) {
		t.Fatalf("partial output still exists after ffmpeg failure; stat error = %v", statErr)
	}
}

func TestMergeEverythingWarnsButSucceedsWhenCleanupFails(t *testing.T) {
	restoreFFmpegCommand(t, "0", "")

	dir := t.TempDir()
	missingVideoFile := filepath.Join(dir, "missing-video.mp4")
	audioFile := writeTempFile(t, dir, "audio.mp3")
	outputFile := filepath.Join(dir, "output.mkv")

	stdout := captureStdout(t, func() {
		err := MergeEverything(context.Background(), missingVideoFile, []MediaTrack{{File: audioFile, Locale: "ja-JP"}}, nil, outputFile, testEpisodeInfo())
		if err != nil {
			t.Fatalf("MergeEverything() error = %v, want nil despite cleanup warning", err)
		}
	})

	if !strings.Contains(stdout, "Failed to remove temporary file") {
		t.Fatalf("MergeEverything() stdout = %q, want cleanup warning", stdout)
	}
}

func TestMergeEverythingKillsFFmpegAndRemovesPartialOutputOnCancellation(t *testing.T) {
	restoreFFmpegCommand(t, "0", "")

	dir := t.TempDir()
	videoFile := writeTempFile(t, dir, "video.mp4")
	audioFile := writeTempFile(t, dir, "audio.mp3")
	outputFile := writeTempFile(t, dir, "partial.mkv")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := MergeEverything(ctx, videoFile, []MediaTrack{{File: audioFile, Locale: "ja-JP"}}, nil, outputFile, testEpisodeInfo())
	if err == nil {
		t.Fatal("MergeEverything() error = nil, want cancellation error")
	}
	if _, statErr := os.Stat(outputFile); !os.IsNotExist(statErr) {
		t.Fatalf("partial output still exists after cancellation; stat error = %v", statErr)
	}
}

func restoreFFmpegCommand(t *testing.T, exitCode, stderr string) {
	t.Helper()
	original := ffmpegCommand
	ffmpegCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			"GO_HELPER_EXIT_CODE="+exitCode,
			"GO_HELPER_STDERR="+stderr,
		)
		return cmd
	}
	t.Cleanup(func() {
		ffmpegCommand = original
	})
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if msg := os.Getenv("GO_HELPER_STDERR"); msg != "" {
		_, _ = os.Stderr.WriteString(msg)
	}
	if sleep := os.Getenv("GO_HELPER_SLEEP"); sleep != "" {
		duration, err := time.ParseDuration(sleep)
		if err != nil {
			os.Exit(2)
		}
		time.Sleep(duration)
	}
	if os.Getenv("GO_HELPER_EXIT_CODE") == "0" {
		os.Exit(0)
	}
	os.Exit(1)
}

func writeTempFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func testEpisodeInfo() *api.EpisodeInfo {
	return &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
		},
		Title: "Test Episode",
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
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
