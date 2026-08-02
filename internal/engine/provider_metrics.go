package engine

import (
	"net/http"
	"strings"
	"sync/atomic"
)

type providerCallMetrics struct {
	Authentication int `json:"authentication_calls"`
	Catalog        int `json:"catalog_calls"`
	PlaybackOpen   int `json:"playback_open_calls"`
	SubtitleFetch  int `json:"subtitle_fetch_calls"`
	StreamRelease  int `json:"stream_release_calls"`
}

func (m providerCallMetrics) total() int {
	return m.Authentication + m.Catalog + m.PlaybackOpen + m.SubtitleFetch + m.StreamRelease
}

var providerCalls struct {
	authentication atomic.Int64
	catalog        atomic.Int64
	playbackOpen   atomic.Int64
	subtitleFetch  atomic.Int64
	streamRelease  atomic.Int64
}

func resetProviderCallMetrics() {
	providerCalls.authentication.Store(0)
	providerCalls.catalog.Store(0)
	providerCalls.playbackOpen.Store(0)
	providerCalls.subtitleFetch.Store(0)
	providerCalls.streamRelease.Store(0)
}

func recordProviderCall(req *http.Request) {
	path := req.URL.Path
	switch {
	case strings.Contains(path, "/auth/v1/token"):
		providerCalls.authentication.Add(1)
	case strings.Contains(path, "/content/v2/"):
		providerCalls.catalog.Add(1)
	case strings.Contains(path, "/playback/v3/") && strings.HasSuffix(path, "/play"):
		providerCalls.playbackOpen.Add(1)
	case req.Method == http.MethodDelete && strings.Contains(path, "/playback/v1/token/"):
		providerCalls.streamRelease.Add(1)
	default:
		providerCalls.subtitleFetch.Add(1)
	}
}

func providerCallMetricsSnapshot() providerCallMetrics {
	return providerCallMetrics{
		Authentication: int(providerCalls.authentication.Load()),
		Catalog:        int(providerCalls.catalog.Load()),
		PlaybackOpen:   int(providerCalls.playbackOpen.Load()),
		SubtitleFetch:  int(providerCalls.subtitleFetch.Load()),
		StreamRelease:  int(providerCalls.streamRelease.Load()),
	}
}
