package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlayheadPOSTBody(t *testing.T) {
	var got struct {
		ContentID string  `json:"content_id"`
		Playhead  float64 `json:"playhead"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/playheads") {
			t.Fatalf("path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if err := PostPlayheadWithBase(srv.URL, "acct", "GWep", 372.5, "pt-BR", "ja-JP"); err != nil {
		t.Fatal(err)
	}
	if got.ContentID != "GWep" || got.Playhead != 372.5 {
		t.Fatalf("%+v", got)
	}
}

func TestFinishPlayheadSeconds(t *testing.T) {
	if !IsPlayFinished(23.0, 24.0) { // >= 95%
		t.Fatal("expected finish")
	}
	if IsPlayFinished(10.0, 24.0) {
		t.Fatal("mid episode is not finish")
	}
	if s := FinishPlayheadSeconds(24.1); s != 24.1 {
		t.Fatalf("%v", s)
	}
}

func TestGetPlayheadsEmptyIDs(t *testing.T) {
	got, err := GetPlayheads("acct", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}
