package main

import (
	"encoding/json"
	"testing"
)

func TestEpisodeUnmarshalErrorField(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr string
	}{
		{"string error", `{"error":"region locked"}`, "region locked"},
		{"false", `{"error":false}`, ""},
		{"null", `{"error":null}`, ""},
		{"zero number", `{"error":0}`, ""},
		{"nonzero number", `{"error":403}`, "403"},
		{"true", `{"error":true}`, "true"},
		{"missing", `{}`, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ep Episode
			if err := json.Unmarshal([]byte(tc.json), &ep); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if string(ep.Error) != tc.wantErr {
				t.Fatalf("Error = %q, want %q", ep.Error, tc.wantErr)
			}
		})
	}
}
