package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSeasonsUsesClientRequestPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/content/v2/cms/series/G123456789/seasons" {
			t.Fatalf("path = %q, want seasons path", r.URL.Path)
		}
		if got := r.URL.Query().Get("preferred_audio_language"); got != "ja-JP" {
			t.Fatalf("preferred_audio_language = %q, want ja-JP", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer initial-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"season-1","season_number":1}]}`)
	}))
	defer server.Close()

	seasons, err := newTestClient(server.URL).GetSeasons(context.Background(), "G123456789", "ja-JP", "en-US")
	if err != nil {
		t.Fatalf("GetSeasons() error = %v", err)
	}
	if len(seasons) != 1 || seasons[0].ID != "season-1" {
		t.Fatalf("seasons = %#v, want season-1", seasons)
	}
}
