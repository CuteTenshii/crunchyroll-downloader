package main

import (
	"reflect"
	"slices"
	"testing"
)

func TestParseEpisodeSelection(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    []episodeRange
		wantErr bool
	}{
		{"empty means no filter", "", nil, false},
		{"single episode", "5", []episodeRange{{5, 5}}, false},
		{"range", "1-3", []episodeRange{{1, 3}}, false},
		{"mixed list", "1-3,5,8-10", []episodeRange{{1, 3}, {5, 5}, {8, 10}}, false},
		{"spaces around values", " 1 - 3 , 5 ", []episodeRange{{1, 3}, {5, 5}}, false},
		{"single-number range", "4-4", []episodeRange{{4, 4}}, false},
		{"special episode zero", "0", []episodeRange{{0, 0}}, false},
		{"duplicate ranges kept", "1-2,1-2", []episodeRange{{1, 2}, {1, 2}}, false},
		{"whitespace only", "   ", nil, true},
		{"empty list item", "1,,2", nil, true},
		{"empty range start", "-3", nil, true},
		{"empty range end", "1-", nil, true},
		{"too many hyphens", "1-2-3", nil, true},
		{"reversed range", "5-3", nil, true},
		{"negative number", "-1", nil, true},
		{"not a number", "one", nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEpisodeSelection(tc.spec)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseEpisodeSelection(%q) error = %v, wantErr %v", tc.spec, err, tc.wantErr)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseEpisodeSelection(%q) = %v, want %v", tc.spec, got, tc.want)
			}
		})
	}
}

func TestFormatEpisodeNumbers(t *testing.T) {
	tests := []struct {
		name    string
		numbers []int
		want    string
	}{
		{name: "empty", numbers: nil, want: ""},
		{name: "single", numbers: []int{5}, want: "5"},
		{name: "consecutive", numbers: []int{1, 2, 3}, want: "1-3"},
		{name: "specials", numbers: []int{0, 1, 2}, want: "0-2"},
		{name: "gaps", numbers: []int{1, 2, 3, 5, 7, 8}, want: "1-3,5,7-8"},
		{name: "unordered", numbers: []int{8, 1, 3, 2}, want: "1-3,8"},
		{name: "duplicates", numbers: []int{2, 1, 2, 3, 3}, want: "1-3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			episodes := make([]SeasonEpisode, len(tc.numbers))
			for i, number := range tc.numbers {
				episodes[i] = SeasonEpisode{EpisodeNumber: number}
			}
			if got := formatEpisodeNumbers(episodes); got != tc.want {
				t.Fatalf("formatEpisodeNumbers(%v) = %q, want %q", tc.numbers, got, tc.want)
			}
		})
	}
}

func TestFilterEpisodes(t *testing.T) {
	episodes := []SeasonEpisode{
		{EpisodeNumber: 0, Title: "special"},
		{EpisodeNumber: 1, Title: "one"},
		{EpisodeNumber: 2, Title: "two"},
		{EpisodeNumber: 3, Title: "three"},
		{EpisodeNumber: 4, Title: "four"},
		{EpisodeNumber: 5, Title: "five"},
	}

	tests := []struct {
		name string
		spec string
		want []int
		all  bool
	}{
		{name: "no filter keeps everything", spec: "", all: true},
		{name: "range preserves API order", spec: "2-4", want: []int{2, 3, 4}},
		{name: "mixed list", spec: "5,0,2-3", want: []int{0, 2, 3, 5}},
		{name: "overlapping ranges do not duplicate", spec: "1-3,2-4", want: []int{1, 2, 3, 4}},
		{name: "no matches", spec: "8-10", want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ranges, err := parseEpisodeSelection(tc.spec)
			if err != nil {
				t.Fatalf("parseEpisodeSelection(%q) returned error: %v", tc.spec, err)
			}

			got := filterEpisodes(episodes, ranges)
			if tc.all {
				if !reflect.DeepEqual(got, episodes) {
					t.Fatalf("filterEpisodes with no filter = %v, want the original list", got)
				}
				return
			}

			gotNumbers := make([]int, 0, len(got))
			for _, episode := range got {
				gotNumbers = append(gotNumbers, episode.EpisodeNumber)
			}
			if !slices.Equal(gotNumbers, tc.want) {
				t.Fatalf("filterEpisodes(%q) numbers = %v, want %v", tc.spec, gotNumbers, tc.want)
			}
		})
	}
}
