package download

import (
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

	err := Episode(nil, "base-content-id", info, []string{"en-US"}, nil, &videoQuality, &audioQuality)
	if err == nil {
		t.Fatal("Episode() error = nil, want unavailable audio locale error")
	}
	if !strings.Contains(err.Error(), "audio locale en-US is not available") {
		t.Fatalf("Episode() error = %q, want unavailable audio locale message", err)
	}
}
