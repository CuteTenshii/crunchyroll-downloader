package download

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"crunchyroll-downloader/internal/api"
)

func TestEpisodeReturnsErrorForUnavailableAudioLocale(t *testing.T) {
	t.Chdir(t.TempDir())

	videoQuality := "1080p"
	audioQuality := "192k"
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
		},
		Title: "Test Episode",
	}

	err := Episode(context.Background(), nil, "base-content-id", info, []string{"en-US"}, nil, &videoQuality, &audioQuality, 2)
	if err == nil {
		t.Fatal("Episode() error = nil, want unavailable audio locale error")
	}
	if !strings.Contains(err.Error(), "audio locale en-US is not available") {
		t.Fatalf("Episode() error = %q, want unavailable audio locale message", err)
	}
}

func TestCleanupEpisodeArtifactsRemovesPartialOutputAndTempFiles(t *testing.T) {
	dir := t.TempDir()
	outputFile := writeEpisodeTestFile(t, dir, "partial.mkv")
	audioFile := writeEpisodeTestFile(t, dir, "audio.mp3")
	videoFile := writeEpisodeTestFile(t, dir, "video.mp4")

	cleanupEpisodeArtifacts(outputFile, []string{audioFile, videoFile, ""})

	for _, path := range []string{outputFile, audioFile, videoFile} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after cleanup; stat error = %v", filepath.Base(path), err)
		}
	}
}

func writeEpisodeTestFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("partial"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	return path
}
