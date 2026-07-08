package api

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendChallengeRetriesRewindableBodyAfterRefresh(t *testing.T) {
	var authCalls int
	var licenseCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/v1/token":
			authCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"refreshed-token"}`)
		case "/license/v1/license/widevine":
			licenseCalls++
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("reading challenge body: %v", err)
			}
			if string(body) != "challenge" {
				t.Fatalf("challenge body = %q, want challenge", body)
			}
			if licenseCalls == 1 {
				http.Error(w, "expired", http.StatusUnauthorized)
				return
			}
			if got := r.Header.Get("Authorization"); got != "Bearer refreshed-token" {
				t.Fatalf("Authorization = %q, want refreshed token", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"license":"`+base64.StdEncoding.EncodeToString([]byte("license"))+`"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	license, err := newTestClient(server.URL).SendChallenge(context.Background(), "content", "video-token", []byte("challenge"))
	if err != nil {
		t.Fatalf("SendChallenge() error = %v", err)
	}
	if string(license) != "license" {
		t.Fatalf("license = %q, want license", license)
	}
	if authCalls != 1 {
		t.Fatalf("auth calls = %d, want 1", authCalls)
	}
	if licenseCalls != 2 {
		t.Fatalf("license calls = %d, want 2", licenseCalls)
	}
}
