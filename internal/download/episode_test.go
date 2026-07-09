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

	err := Episode(context.Background(), nil, "base-content-id", info, []string{"en-US"}, nil, &videoQuality, &audioQuality, 2, "")
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

func TestEpisodeSingleVersion(t *testing.T) {
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

	// Use a cancelled context so GetEpisode fails fast (no real HTTP call).
	// With only one version (no Versions slice), the function takes the
	// sequential Phase A path — no errgroup is created.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := api.NewTestClient(nil, "https://example.com", "test-token")

	err := Episode(ctx, client, "content-id", info, []string{"ja-JP"}, nil, &videoQuality, &audioQuality, 2, "")
	if err == nil {
		t.Fatal("Episode() error = nil, want error from single-version sequential path")
	}
}

func TestEpisodeParallelAudio(t *testing.T) {
	// NOTE: Full parallel audio integration test requires a mock HTTP server.
	// See Phase 5 for comprehensive test coverage.
	t.Chdir(t.TempDir())

	videoQuality := "1080p"
	audioQuality := "192k"
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
			Versions: []*api.DubVersion{
				{AudioLocale: "en-US", GUID: "en-us-guid"},
			},
		},
		Title: "Test Episode",
	}

	// Cancelled context causes GetEpisode to fail fast before HTTP calls.
	// The error should propagate from the sequential path (Phase A) before
	// the errgroup section (Phase B) is even reached.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := api.NewTestClient(nil, "https://example.com", "test-token")

	err := Episode(ctx, client, "content-id", info, []string{"ja-JP", "en-US"}, nil, &videoQuality, &audioQuality, 2, "")
	if err == nil {
		t.Fatal("Episode() error = nil, want error from parallel or sequential path")
	}
}

func TestEpisodeParallelAudioZeroVersions(t *testing.T) {
	// Verify that requesting a non-existent audio locale returns an error
	// before any parallel work begins.
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

	err := Episode(context.Background(), nil, "content-id", info, []string{"fr-FR"}, nil, &videoQuality, &audioQuality, 2, "")
	if err == nil {
		t.Fatal("Episode() error = nil, want audio locale unavailable error")
	}
	if !strings.Contains(err.Error(), "audio locale fr-FR is not available") {
		t.Fatalf("Episode() error = %q, want unavailable audio locale message", err)
	}
}

func TestOutputDirCreatesSeriesSubfolderInOutputDir(t *testing.T) {
	outputDir := t.TempDir()

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

	// Use a cancelled context so GetEpisode fails fast (no real HTTP call).
	// The outputDir's series subfolder should be created before the API call.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := api.NewTestClient(nil, "https://example.com", "test-token")

	err := Episode(ctx, client, "content-id", info, []string{"ja-JP"}, nil, &videoQuality, &audioQuality, 2, outputDir)
	if err == nil {
		t.Fatal("Episode() error = nil, want error from cancelled context (GetEpisode)")
	}

	// Verify the series subfolder was created inside outputDir
	expectedDir := filepath.Join(outputDir, sanitizeFilename("Test Series"))
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Fatalf("Episode() did not create series subfolder at %s", expectedDir)
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
