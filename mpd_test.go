package main

import (
	"testing"

	"github.com/unki2aut/go-mpd"
)

func TestGetBaseURLFallsBackToHighestVideoRepresentation(t *testing.T) {
	representation := func(id, url string, height uint64) mpd.Representation {
		return mpd.Representation{
			ID:      &id,
			Height:  &height,
			BaseURL: []*mpd.BaseURL{{Value: url}},
		}
	}
	set := &mpd.AdaptationSet{Representations: []mpd.Representation{
		representation("video/avc1/480p", "https://example.test/480", 480),
		representation("video/avc1/720p", "https://example.test/720", 720),
	}}

	baseURL, representationID := getBaseUrl(set, true, "1080p")
	if baseURL == nil || *baseURL != "https://example.test/720" {
		t.Fatalf("expected highest available video URL, got %v", baseURL)
	}
	if representationID == nil || *representationID != "video/avc1/720p" {
		t.Fatalf("expected highest available representation, got %v", representationID)
	}
}

func TestGetBaseURLPrefersExactVideoHeight(t *testing.T) {
	representation := func(id, url string, height uint64) mpd.Representation {
		return mpd.Representation{
			ID:      &id,
			Height:  &height,
			BaseURL: []*mpd.BaseURL{{Value: url}},
		}
	}
	set := &mpd.AdaptationSet{Representations: []mpd.Representation{
		representation("video/avc1/480p", "https://example.test/480", 480),
		representation("video/avc1/720p", "https://example.test/720", 720),
		representation("video/avc1/1080p", "https://example.test/1080", 1080),
	}}

	baseURL, representationID := getBaseUrl(set, true, "720p")
	if baseURL == nil || *baseURL != "https://example.test/720" {
		t.Fatalf("expected exact video URL, got %v", baseURL)
	}
	if representationID == nil || *representationID != "video/avc1/720p" {
		t.Fatalf("expected exact representation, got %v", representationID)
	}
}
