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

func TestBatchIndexCatalogMetricsAreIsolatedPerSummary(t *testing.T) {
	resetProviderCallMetrics()
	t.Cleanup(resetProviderCallMetrics)
	providerCalls.authentication.Add(1) // Shared batch authentication is outside each catalog scope.

	beginBatchIndexCatalogMetrics(true)
	providerCalls.catalog.Add(3)
	providerCalls.playbackOpen.Add(2)
	providerCalls.subtitleFetch.Add(1)
	first := providerCallMetricsSnapshot()
	if first.Authentication != 0 || first.Catalog != 3 || first.PlaybackOpen != 2 || first.SubtitleFetch != 1 || first.total() != 6 {
		t.Fatalf("first catalog metrics=%#v", first)
	}

	beginBatchIndexCatalogMetrics(true)
	providerCalls.catalog.Add(2)
	providerCalls.playbackOpen.Add(1)
	second := providerCallMetricsSnapshot()
	if second.Authentication != 0 || second.Catalog != 2 || second.PlaybackOpen != 1 || second.SubtitleFetch != 0 || second.total() != 3 {
		t.Fatalf("second catalog inherited cumulative metrics: %#v", second)
	}
}
