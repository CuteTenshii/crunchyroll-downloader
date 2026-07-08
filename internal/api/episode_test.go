package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetEpisodeInfoUsesClientRequestPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/content/v2/cms/objects/G123456789" {
			t.Fatalf("path = %q, want episode info path", r.URL.Path)
		}
		if r.Context().Err() != nil {
			t.Fatalf("request context error = %v", r.Context().Err())
		}
		if got := r.Header.Get("Authorization"); got != "Bearer initial-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"title":"Pilot","episode_metadata":{"series_title":"Series","season_number":1,"episode_number":1,"audio_locale":"ja-JP"}}]}`)
	}))
	defer server.Close()

	info, err := newTestClient(server.URL).GetEpisodeInfo(context.Background(), "G123456789")
	if err != nil {
		t.Fatalf("GetEpisodeInfo() error = %v", err)
	}
	if info.Title != "Pilot" {
		t.Fatalf("Title = %q, want Pilot", info.Title)
	}
}
