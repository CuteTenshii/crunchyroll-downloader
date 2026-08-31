package main

import "testing"

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty falls back", "", "Unknown"},
		{"plain title untouched", "Frieren Beyond Journey's End", "Frieren Beyond Journey_s End"},
		{"path separators", "a/b\\c", "a_b_c"},
		{"windows reserved chars", `a:b*c?d"e<f>g|h`, "a_b_c_d_e_f_g_h"},
		{"typographic quotes", "“Hello” ‘s ’t", "_Hello_ ‘s _t"},
		{"backtick", "a`b", "a_b"},
		{"runs collapse to one", "a///b", "a_b"},
		{"pre-existing runs collapse", "a___b", "a_b"},
		{"trailing dots trimmed", "Episode 1...", "Episode 1"},
		{"trailing spaces trimmed", "Episode 1   ", "Episode 1"},
		{"trailing mix trimmed", "Episode 1. . ", "Episode 1"},
		{"leading space kept", " Episode", " Episode"},
		{"non-latin preserved", "進撃の巨人", "進撃の巨人"},
		{"all illegal collapses", "///", "_"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeFilename(tc.in); got != tc.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
