package main

import (
	"net/http"
	"testing"
)

func TestProviderCallMetricsClassifyEveryProviderBoundary(t *testing.T) {
	resetProviderCallMetrics()
	requests := []struct {
		method string
		url    string
	}{
		{http.MethodPost, "https://www.crunchyroll.com/auth/v1/token"},
		{http.MethodGet, "https://www.crunchyroll.com/content/v2/cms/series/id/seasons"},
		{http.MethodGet, "https://www.crunchyroll.com/playback/v3/episode/web/firefox/play"},
		{http.MethodGet, "https://static.crunchyroll.com/subtitles/track.ass?signature=secret"},
		{http.MethodDelete, "https://www.crunchyroll.com/playback/v1/token/episode/secret"},
	}
	for _, item := range requests {
		req, err := http.NewRequest(item.method, item.url, nil)
		if err != nil {
			t.Fatal(err)
		}
		recordProviderCall(req)
	}
	metrics := providerCallMetricsSnapshot()
	if metrics.Authentication != 1 || metrics.Catalog != 1 || metrics.PlaybackOpen != 1 || metrics.SubtitleFetch != 1 || metrics.StreamRelease != 1 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
	if metrics.total() != 5 {
		t.Fatalf("provider total = %d, want 5", metrics.total())
	}
}
