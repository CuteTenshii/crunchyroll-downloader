package engine

import (
	"encoding/json"
	"testing"
)

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
		Thumbnail: [][]CRImage{
			{
				{Width: 120, Source: "https://cdn.example/120.jpg"},
				{Width: 320, Source: "https://cdn.example/thumb.jpg"},
			},
		},
		PosterTall: [][]CRImage{
			{{Width: 480, Source: "https://cdn.example/tall.jpg"}},
		},
	}
	if got := thumbnailFromImages(imgs); got != "https://cdn.example/thumb.jpg" {
		t.Fatalf("got %q", got)
	}
}

// Crunchyroll CMS returns nested arrays: images.thumbnail = [[{...}, {...}]].
func TestCRImagesUnmarshalNestedCMSShape(t *testing.T) {
	raw := []byte(`{
		"thumbnail": [[
			{"width": 120, "height": 68, "source": "https://cdn.example/120.jpg", "type": "thumbnail"},
			{"width": 320, "height": 180, "source": "https://cdn.example/320.jpg", "type": "thumbnail"}
		]],
		"poster_tall": [[
			{"width": 240, "height": 360, "source": "https://cdn.example/tall.jpg", "type": "poster_tall"}
		]],
		"poster_wide": []
	}`)
	var imgs CRImages
	if err := json.Unmarshal(raw, &imgs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := thumbnailFromImages(imgs); got != "https://cdn.example/320.jpg" {
		t.Fatalf("thumbnail = %q", got)
	}
	if got := posterFromImages(imgs); got != "https://cdn.example/tall.jpg" {
		t.Fatalf("poster = %q", got)
	}
}

// Season episode payload shape: data[].images nested groups (regression for Inspect panic).
func TestSeasonEpisodeImagesUnmarshal(t *testing.T) {
	raw := []byte(`{
		"data": [{
			"id": "GE123",
			"title": "Episode 1",
			"season_number": 1,
			"episode_number": 1,
			"series_title": "Slime",
			"audio_locale": "ja-JP",
			"images": {
				"thumbnail": [[
					{"width": 320, "height": 180, "source": "https://cdn.example/ep.jpg", "type": "thumbnail"}
				]]
			}
		}]
	}`)
	var payload SeasonEpisodes
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal season episodes: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("len=%d", len(payload.Data))
	}
	if got := thumbnailFromImages(payload.Data[0].Images); got != "https://cdn.example/ep.jpg" {
		t.Fatalf("thumb=%q", got)
	}
}
