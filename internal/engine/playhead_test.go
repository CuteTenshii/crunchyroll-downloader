package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPlayheadPOSTBody(t *testing.T) {
	var got struct {
		ContentID string  `json:"content_id"`
		Playhead  float64 `json:"playhead"`
	}
	var method, path, locale, audio, auth, contentType, userAgent string
	var decodeErr error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		locale = r.URL.Query().Get("locale")
		audio = r.URL.Query().Get("preferred_audio_language")
		auth = r.Header.Get("Authorization")
		contentType = r.Header.Get("Content-Type")
		userAgent = r.Header.Get("User-Agent")
		decodeErr = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	oldToken := token
	token = "test-token"
	defer func() { token = oldToken }()
	if err := PostPlayheadWithBase(srv.URL, "acct", "GWep", 372.5, "pt-BR", "ja-JP"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost {
		t.Fatalf("method %s", method)
	}
	if path != "/content/v2/acct/playheads" {
		t.Fatalf("path %s", path)
	}
	if locale != "pt-BR" {
		t.Fatalf("locale %q", locale)
	}
	if audio != "ja-JP" {
		t.Fatalf("preferred_audio_language %q", audio)
	}
	if auth != "Bearer test-token" {
		t.Fatalf("Authorization %q", auth)
	}
	if contentType != "application/json" {
		t.Fatalf("Content-Type %q", contentType)
	}
	wantUA := "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0"
	if userAgent != wantUA {
		t.Fatalf("User-Agent %q", userAgent)
	}
	if decodeErr != nil {
		t.Fatal(decodeErr)
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

func TestPlayheadPOSTHTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	err := PostPlayheadWithBase(srv.URL, "acct", "GWep", 1, "pt-BR", "ja-JP")
	if err == nil || !strings.Contains(err.Error(), "playhead POST HTTP 500") {
		t.Fatalf("err=%v", err)
	}
}

func TestPlayheadRequiresAccountAndContentID(t *testing.T) {
	const want = "playhead requires account and content id"
	cases := []struct {
		name      string
		accountID string
		contentID string
	}{
		{"empty account", "", "GWep"},
		{"empty content", "acct", ""},
		{"whitespace account", "  ", "id"},
	}
	for _, tc := range cases {
		err := PostPlayheadWithBase("http://127.0.0.1:1", tc.accountID, tc.contentID, 1, "pt-BR", "ja-JP")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: err=%v", tc.name, err)
		}
	}
}

func TestPlayheadPOSTNegativeClampedToZero(t *testing.T) {
	var got struct {
		Playhead float64 `json:"playhead"`
	}
	var decodeErr error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decodeErr = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if err := PostPlayheadWithBase(srv.URL, "acct", "GWep", -12, "pt-BR", "ja-JP"); err != nil {
		t.Fatal(err)
	}
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if got.Playhead != 0 {
		t.Fatalf("playhead %v, want 0", got.Playhead)
	}
}

func TestPlayheadDebounceSameSecond(t *testing.T) {
	d := NewPlayheadDebouncer(time.Second)
	if !d.ShouldPost("id", 10) {
		t.Fatal("first should post")
	}
	d.MarkPosted("id", 10)
	if d.ShouldPost("id", 10) {
		t.Fatal("duplicate within window")
	}
}

func TestPlayheadDebouncePeekDoesNotMark(t *testing.T) {
	d := NewPlayheadDebouncer(time.Second)
	if !d.ShouldPost("id", 10) {
		t.Fatal("first should post")
	}
	if !d.ShouldPost("id", 10) {
		t.Fatal("unmarked peek must still post")
	}
	d.MarkPosted("id", 10)
	if d.ShouldPost("id", 10) {
		t.Fatal("after MarkPosted same second must skip")
	}
}

func TestPlayheadDebounceDifferentSecond(t *testing.T) {
	d := NewPlayheadDebouncer(time.Second)
	if !d.ShouldPost("id", 10) {
		t.Fatal("first should post")
	}
	d.MarkPosted("id", 10)
	if !d.ShouldPost("id", 11) {
		t.Fatal("different second should post")
	}
}

func TestPlayheadDebounceAfterWindow(t *testing.T) {
	now := time.Unix(1_000, 0)
	d := NewPlayheadDebouncer(time.Second)
	d.now = func() time.Time { return now }
	if !d.ShouldPost("id", 10) {
		t.Fatal("first should post")
	}
	d.MarkPosted("id", 10)
	now = now.Add(time.Second)
	if !d.ShouldPost("id", 10) {
		t.Fatal("after window expires should post again")
	}
}

func TestPlayheadDebounceDifferentContentID(t *testing.T) {
	d := NewPlayheadDebouncer(time.Second)
	if !d.ShouldPost("id", 10) {
		t.Fatal("first should post")
	}
	d.MarkPosted("id", 10)
	if !d.ShouldPost("other", 10) {
		t.Fatal("different content id should post")
	}
}
