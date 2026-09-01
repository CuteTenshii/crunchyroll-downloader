package main

import "testing"

func TestParseUrl(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		wantContentType string
		wantContentId   string
	}{
		{
			name:            "series without locale",
			url:             "https://www.crunchyroll.com/series/G0XHWM0D3/some-slug",
			wantContentType: "series",
			wantContentId:   "G0XHWM0D3",
		},
		{
			name:            "series with locale prefix",
			url:             "https://www.crunchyroll.com/fr/series/G0XHWM0D3/trapped-in-a-dating-sim-the-world-of-otome-games-is-tough-for-mobs",
			wantContentType: "series",
			wantContentId:   "G0XHWM0D3",
		},
		{
			name:            "watch with locale prefix",
			url:             "https://www.crunchyroll.com/fr/watch/G0XHWM0D3/episode-slug",
			wantContentType: "watch",
			wantContentId:   "G0XHWM0D3",
		},
		{
			name:            "trailing slash",
			url:             "https://www.crunchyroll.com/series/G0XHWM0D3/",
			wantContentType: "series",
			wantContentId:   "G0XHWM0D3",
		},
		{
			name:            "no content type",
			url:             "https://www.crunchyroll.com/",
			wantContentType: "",
			wantContentId:   "",
		},
		{
			name:            "content type at end without id",
			url:             "https://www.crunchyroll.com/series",
			wantContentType: "",
			wantContentId:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			contentType, contentId := parseUrl(tc.url)
			if contentType != tc.wantContentType || contentId != tc.wantContentId {
				t.Errorf("parseUrl(%q) = (%q, %q), want (%q, %q)",
					tc.url, contentType, contentId, tc.wantContentType, tc.wantContentId)
			}
		})
	}
}
