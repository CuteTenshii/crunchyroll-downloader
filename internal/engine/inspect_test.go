package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/unki2aut/go-mpd"
)

func TestListVideoHeightsFromMPD(t *testing.T) {
	h720 := uint64(720)
	h1080 := uint64(1080)
	h480 := uint64(480)
	id720, id1080, id480 := "video/avc1/720p", "video/avc1/1080p", "video/avc1/480p"
	manifest := &mpd.MPD{
		Period: []*mpd.Period{{
			AdaptationSets: []*mpd.AdaptationSet{{
				Representations: []mpd.Representation{
					{Height: &h720, ID: &id720, BaseURL: []*mpd.BaseURL{{Value: "https://cdn.example/720"}}},
					{Height: &h1080, ID: &id1080, BaseURL: []*mpd.BaseURL{{Value: "https://cdn.example/1080"}}},
					{Height: &h480, ID: &id480, BaseURL: []*mpd.BaseURL{{Value: "https://cdn.example/480"}}},
					// Duplicate height should be de-duplicated.
					{Height: &h720, ID: &id720, BaseURL: []*mpd.BaseURL{{Value: "https://cdn.example/720-alt"}}},
				},
			}},
		}},
	}
	got := ListVideoQualities(manifest)
	if len(got) != 3 {
		t.Fatalf("got %#v", got)
	}
	// expect labels like "1080p", "720p" sorted descending
	if got[0] != "1080p" {
		t.Fatalf("want max first, got %v", got)
	}
	if got[1] != "720p" || got[2] != "480p" {
		t.Fatalf("want descending heights, got %v", got)
	}
}

func TestListAudioQualitiesFromMPD(t *testing.T) {
	id192, id128, id96 := "audio/ja-JP/192k", "audio/ja-JP/128k", "audio/ja-JP/96k"
	bw192, bw128, bw96 := uint64(192002), uint64(128000), uint64(96000)
	manifest := &mpd.MPD{
		Period: []*mpd.Period{{
			AdaptationSets: []*mpd.AdaptationSet{{
				Representations: []mpd.Representation{
					{ID: &id128, Bandwidth: &bw128, BaseURL: []*mpd.BaseURL{{Value: "https://cdn.example/128"}}},
					{ID: &id192, Bandwidth: &bw192, BaseURL: []*mpd.BaseURL{{Value: "https://cdn.example/192"}}},
					{ID: &id96, Bandwidth: &bw96, BaseURL: []*mpd.BaseURL{{Value: "https://cdn.example/96"}}},
				},
			}},
		}},
	}
	got := ListAudioQualities(manifest)
	if len(got) != 3 {
		t.Fatalf("got %#v", got)
	}
	if got[0] != "192k" || got[1] != "128k" || got[2] != "96k" {
		t.Fatalf("want descending audio qualities, got %v", got)
	}
}

func TestListVideoQualitiesNilSafe(t *testing.T) {
	if got := ListVideoQualities(nil); len(got) != 0 {
		t.Fatalf("nil manifest: got %v", got)
	}
	if got := ListAudioQualities(&mpd.MPD{}); len(got) != 0 {
		t.Fatalf("empty manifest: got %v", got)
	}
}

func writeInspectETPRT(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "etp_rt")
	if err := os.WriteFile(path, []byte("inspect-test-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInspectWatchCatalogWithoutProbe(t *testing.T) {
	originalRefresh := inspectRefreshAccessToken
	originalInfo := inspectGetEpisodeInfo
	t.Cleanup(func() {
		inspectRefreshAccessToken = originalRefresh
		inspectGetEpisodeInfo = originalInfo
	})

	inspectRefreshAccessToken = func() error { return nil }
	inspectGetEpisodeInfo = func(id string) EpisodeInfo {
		if id != "GWDU82Z05" {
			t.Fatalf("unexpected episode id %q", id)
		}
		return EpisodeInfo{
			Title: "Episode One",
			EpisodeMetadata: EpisodeMetadata{
				AudioLocale:   "ja-JP",
				EpisodeNumber: 1,
				SeasonNumber:  1,
				SeriesTitle:   "Test Series",
				Versions: []*DubVersion{
					{AudioLocale: "ja-JP", GUID: "GWDU82Z05"},
					{AudioLocale: "en-US", GUID: "G9DUE0X0N"},
				},
			},
		}
	}

	result, err := Inspect(InspectRequest{
		URL:       "https://www.crunchyroll.com/watch/GWDU82Z05/episode-one",
		ETPRTFile: writeInspectETPRT(t),
	}, DefaultRuntimeConfig())
	if err != nil {
		t.Fatal(err)
	}
	if result.ContentType != "watch" || result.ContentID != "GWDU82Z05" {
		t.Fatalf("content identity: %+v", result)
	}
	if result.DefaultEpisodeID != "GWDU82Z05" {
		t.Fatalf("default episode: %q", result.DefaultEpisodeID)
	}
	if result.OriginalAudio != "ja-JP" {
		t.Fatalf("original audio: %q", result.OriginalAudio)
	}
	if len(result.Episodes) != 1 || result.Episodes[0].Title != "Episode One" {
		t.Fatalf("episodes: %+v", result.Episodes)
	}
	if len(result.AudioLocales) != 2 {
		t.Fatalf("audio locales: %v", result.AudioLocales)
	}
	if len(result.VideoQualities) != 0 || len(result.SubtitleLocales) != 0 {
		t.Fatalf("probe fields should be empty without ProbePlayback: %+v", result)
	}
}

func TestInspectSeriesFillsSeasonOneEpisodes(t *testing.T) {
	originalRefresh := inspectRefreshAccessToken
	originalSeasons := inspectGetSeasons
	originalEpisodes := inspectGetSeasonEpisodes
	t.Cleanup(func() {
		inspectRefreshAccessToken = originalRefresh
		inspectGetSeasons = originalSeasons
		inspectGetSeasonEpisodes = originalEpisodes
	})

	inspectRefreshAccessToken = func() error { return nil }
	inspectGetSeasons = func(contentId, audioLocale, subLocale string) []Season {
		if contentId != "GJ0H7Q5ZJ" {
			t.Fatalf("unexpected series id %q", contentId)
		}
		return []Season{
			{ID: "season-2", SeasonNumber: 2},
			{ID: "season-1", SeasonNumber: 1},
		}
	}
	inspectGetSeasonEpisodes = func(seasonId, audioLocale, subLocale string) []SeasonEpisode {
		if seasonId != "season-1" {
			t.Fatalf("expected season 1 episodes first, got %q", seasonId)
		}
		return []SeasonEpisode{
			{
				ID:            "E001",
				SeasonNumber:  1,
				EpisodeNumber: 1,
				Title:         "Begin",
				SeriesTitle:   "Series",
				AudioLocale:   "ja-JP",
				Versions: []*DubVersion{
					{AudioLocale: "ja-JP", GUID: "E001"},
					{AudioLocale: "en-US", GUID: "E001EN"},
				},
			},
			{
				ID:            "E002",
				SeasonNumber:  1,
				EpisodeNumber: 2,
				Title:         "Next",
				SeriesTitle:   "Series",
				AudioLocale:   "ja-JP",
			},
		}
	}

	result, err := Inspect(InspectRequest{
		URL:       "https://www.crunchyroll.com/series/GJ0H7Q5ZJ/series",
		ETPRTFile: writeInspectETPRT(t),
	}, DefaultRuntimeConfig())
	if err != nil {
		t.Fatal(err)
	}
	if result.ContentType != "series" || result.ContentID != "GJ0H7Q5ZJ" {
		t.Fatalf("content identity: %+v", result)
	}
	if len(result.Seasons) != 2 {
		t.Fatalf("seasons: %+v", result.Seasons)
	}
	if result.DefaultEpisodeID != "E001" {
		t.Fatalf("default episode want S1E1 id, got %q", result.DefaultEpisodeID)
	}
	if len(result.Episodes) != 2 {
		t.Fatalf("episodes: %+v", result.Episodes)
	}
	if result.OriginalAudio != "ja-JP" {
		t.Fatalf("original audio: %q", result.OriginalAudio)
	}
	if len(result.AudioLocales) < 2 {
		t.Fatalf("audio locales should include dubs: %v", result.AudioLocales)
	}
}

func TestInspectProbePlaybackListsQualitiesAndReleasesStream(t *testing.T) {
	originalRefresh := inspectRefreshAccessToken
	originalInfo := inspectGetEpisodeInfo
	originalOpen := openDownloadPlayback
	originalClose := closeDownloadPlayback
	originalParse := parseDownloadManifest
	t.Cleanup(func() {
		inspectRefreshAccessToken = originalRefresh
		inspectGetEpisodeInfo = originalInfo
		openDownloadPlayback = originalOpen
		closeDownloadPlayback = originalClose
		parseDownloadManifest = originalParse
	})

	inspectRefreshAccessToken = func() error { return nil }
	inspectGetEpisodeInfo = func(string) EpisodeInfo {
		return EpisodeInfo{
			Title: "Probed",
			EpisodeMetadata: EpisodeMetadata{
				AudioLocale:   "ja-JP",
				EpisodeNumber: 3,
				SeasonNumber:  1,
				SeriesTitle:   "Probe Series",
				Versions:      []*DubVersion{{AudioLocale: "ja-JP", GUID: "PROBE01"}},
			},
		}
	}

	var releasedID, releasedToken string
	openDownloadPlayback = func(id string) Episode {
		if id != "PROBE01" {
			t.Fatalf("unexpected playback id %q", id)
		}
		return Episode{
			ManifestURL: "https://cdn.example/manifest.mpd",
			Token:       "stream-token",
			Subtitles: map[string]*Subtitle{
				"en-US": {Language: "en-US", URL: "https://cdn.example/en.ass"},
				"es-419": {Language: "es-419", URL: "https://cdn.example/es.ass"},
			},
			Captions: map[string]*Subtitle{
				"en-US": {Language: "en-US", URL: "https://cdn.example/en.vtt"},
			},
		}
	}
	closeDownloadPlayback = func(id, streamToken string) bool {
		releasedID, releasedToken = id, streamToken
		return true
	}

	h720, h1080 := uint64(720), uint64(1080)
	id720, id1080 := "video/avc1/720p", "video/avc1/1080p"
	id192 := "audio/ja-JP/192k"
	bw192 := uint64(192000)
	parseDownloadManifest = func(url string) *mpd.MPD {
		if url != "https://cdn.example/manifest.mpd" {
			t.Fatalf("unexpected manifest url %q", url)
		}
		return &mpd.MPD{
			Period: []*mpd.Period{{
				AdaptationSets: []*mpd.AdaptationSet{
					{
						Representations: []mpd.Representation{
							{Height: &h720, ID: &id720, BaseURL: []*mpd.BaseURL{{Value: "https://cdn.example/720"}}},
							{Height: &h1080, ID: &id1080, BaseURL: []*mpd.BaseURL{{Value: "https://cdn.example/1080"}}},
						},
					},
					{
						Representations: []mpd.Representation{
							{ID: &id192, Bandwidth: &bw192, BaseURL: []*mpd.BaseURL{{Value: "https://cdn.example/192"}}},
						},
					},
				},
			}},
		}
	}

	result, err := Inspect(InspectRequest{
		URL:            "https://www.crunchyroll.com/watch/PROBE01/probed",
		ETPRTFile:      writeInspectETPRT(t),
		ProbePlayback:  true,
		ProbeContentID: "PROBE01",
	}, DefaultRuntimeConfig())
	if err != nil {
		t.Fatal(err)
	}
	if releasedID != "PROBE01" || releasedToken != "stream-token" {
		t.Fatalf("stream not released: id=%q token=%q", releasedID, releasedToken)
	}
	if len(result.VideoQualities) < 2 || result.VideoQualities[0] != "1080p" {
		t.Fatalf("video qualities: %v", result.VideoQualities)
	}
	if len(result.AudioQualities) != 1 || result.AudioQualities[0] != "192k" {
		t.Fatalf("audio qualities: %v", result.AudioQualities)
	}
	if len(result.SubtitleLocales) != 2 {
		t.Fatalf("subtitle locales: %v", result.SubtitleLocales)
	}
	if len(result.CaptionLocales) != 1 || result.CaptionLocales[0] != "en-US" {
		t.Fatalf("caption locales: %v", result.CaptionLocales)
	}
}
