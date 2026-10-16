package main

import "testing"

func TestSubsOutputFile(t *testing.T) {
	tests := []struct {
		name       string
		outputFile string
		locale     string
		format     string
		isCC       bool
		want       string
	}{
		{
			name:       "subtitle shares the video's exact base name",
			outputFile: "Series/Series S01E02 - Title [1080p].mkv",
			locale:     "en-US",
			format:     "ass",
			want:       "Series/Series S01E02 - Title [1080p] [en-US].ass",
		},
		{
			name:       "defaults to ass when format is empty",
			outputFile: "Series/Series S01E02 - Title [1080p].mkv",
			locale:     "fr-FR",
			want:       "Series/Series S01E02 - Title [1080p] [fr-FR].ass",
		},
		{
			name:       "caption tagged with locale and [CC]",
			outputFile: "Series/Series S01E02 - Title [1080p].mkv",
			locale:     "en-US",
			format:     "vtt",
			isCC:       true,
			want:       "Series/Series S01E02 - Title [1080p] [en-US] [CC].vtt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := subsOutputFile(tc.outputFile, tc.locale, tc.format, tc.isCC)
			if got != tc.want {
				t.Errorf("subsOutputFile() = %q, want %q", got, tc.want)
			}
		})
	}
}
