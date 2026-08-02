package engine

import "testing"

func TestPickImageURLPrefersUnderMaxWidth(t *testing.T) {
	images := []CRImage{
		{Width: 120, Source: "https://cdn.example/120.jpg"},
		{Width: 320, Source: "https://cdn.example/320.jpg"},
		{Width: 640, Source: "https://cdn.example/640.jpg"},
	}
	got := pickImageURL(images, 320)
	if got != "https://cdn.example/320.jpg" {
		t.Fatalf("got %q", got)
	}
}

func TestPickImageURLFallsBackToSmallestWhenAllLarger(t *testing.T) {
	images := []CRImage{
		{Width: 800, Source: "https://cdn.example/800.jpg"},
		{Width: 1200, Source: "https://cdn.example/1200.jpg"},
	}
	got := pickImageURL(images, 320)
	if got != "https://cdn.example/800.jpg" {
		t.Fatalf("got %q", got)
	}
}

func TestThumbnailFromImages(t *testing.T) {
	imgs := CRImages{
		Thumbnail: []CRImage{{Width: 320, Source: "https://cdn.example/thumb.jpg"}},
		PosterTall: []CRImage{{Width: 480, Source: "https://cdn.example/tall.jpg"}},
	}
	if got := thumbnailFromImages(imgs); got != "https://cdn.example/thumb.jpg" {
		t.Fatalf("got %q", got)
	}
}
