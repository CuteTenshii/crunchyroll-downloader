package main

import "testing"

func TestAudioOutputFile(t *testing.T) {
	info := EpisodeInfo{
		EpisodeMetadata: EpisodeMetadata{SeasonNumber: 1, EpisodeNumber: 2},
		Title:           "Dawn and Confusion",
	}

	tests := []struct {
		name     string
		audioMux bool
		locale   string
		want     string
	}{
		{
			name:   "per-dub m4a tagged with locale",
			locale: "ja-JP",
			want:   "Series/Series S01E02 - Title [ja-JP].m4a",
		},
		{
			name:   "per-dub m4a for second dub",
			locale: "en-US",
			want:   "Series/Series S01E02 - Title [en-US].m4a",
		},
		{
			name:     "muxed mka tagged with audio quality",
			audioMux: true,
			locale:   "ja-JP",
			want:     "Series/Series S01E02 - Title [192k].mka",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldMux, oldQuality := *audioMux, *audioQuality
			*audioMux, *audioQuality = tc.audioMux, "192k"
			defer func() { *audioMux, *audioQuality = oldMux, oldQuality }()

			got := audioOutputFile("Series", "Title", info, tc.locale)
			if got != tc.want {
				t.Errorf("audioOutputFile() = %q, want %q", got, tc.want)
			}
		})
	}
}
