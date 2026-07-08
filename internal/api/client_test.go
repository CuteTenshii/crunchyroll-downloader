package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(serverURL string) *Client {
	return &Client{
		httpClient: configuredHTTPClient(),
		token:      "initial-token",
		etpRt:      "cookie",
		baseURL:    serverURL,
		authURL:    serverURL + "/auth/v1/token",
		licenseURL: serverURL + "/license/v1/license/widevine",
	}
}

func TestConfiguredHTTPClientUsesTimeoutsAndKeepAlive(t *testing.T) {
	client := configuredHTTPClient()
	if client.Timeout != 60*time.Second {
		t.Fatalf("configuredHTTPClient().Timeout = %v, want 60s", client.Timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("configuredHTTPClient().Transport = %T, want *http.Transport", client.Transport)
	}
	if transport.MaxIdleConnsPerHost < 10 {
		t.Fatalf("MaxIdleConnsPerHost = %d, want >= 10", transport.MaxIdleConnsPerHost)
	}
	if transport.MaxIdleConns == 0 {
		t.Fatal("MaxIdleConns = 0, want idle connection pool configured")
	}
	if transport.IdleConnTimeout == 0 {
		t.Fatal("IdleConnTimeout = 0, want idle connection reuse timeout configured")
	}
	if transport.DialContext == nil {
		t.Fatal("DialContext = nil, want configured dialer timeout")
	}
}

func TestDoRefreshesTokenOnceAndRetries(t *testing.T) {
	var authCalls int
	var resourceCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/v1/token":
			authCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"refreshed-token"}`)
		case "/resource":
			resourceCalls++
			if r.Header.Get("Authorization") == "Bearer refreshed-token" {
				_, _ = io.WriteString(w, `ok`)
				return
			}
			http.Error(w, "expired", http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	req, err := client.newRequest(context.Background(), http.MethodGet, server.URL+"/resource", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Do() status = %d, want 200", resp.StatusCode)
	}
	if authCalls != 1 {
		t.Fatalf("auth calls = %d, want 1", authCalls)
	}
	if resourceCalls != 2 {
		t.Fatalf("resource calls = %d, want 2", resourceCalls)
	}
}

func TestDoReturnsErrorAfterOneFailedRefreshRetry(t *testing.T) {
	var authCalls int
	var resourceCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/v1/token":
			authCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"still-rejected"}`)
		case "/resource":
			resourceCalls++
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	req, err := client.newRequest(context.Background(), http.MethodGet, server.URL+"/resource", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err == nil {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatal("Do() error = nil, want unauthorized retry error")
	}
	if authCalls != 1 {
		t.Fatalf("auth calls = %d, want 1", authCalls)
	}
	if resourceCalls != 2 {
		t.Fatalf("resource calls = %d, want 2", resourceCalls)
	}
}

func TestDoReturnsErrorForNonRewindableUnauthorizedRequest(t *testing.T) {
	var authCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/v1/token":
			authCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"refreshed-token"}`)
		case "/resource":
			http.Error(w, "expired", http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	req, err := client.newRequest(context.Background(), http.MethodPost, server.URL+"/resource", io.NopCloser(strings.NewReader("body")))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("Do() error = nil, want non-rewindable retry error")
	}
	if !strings.Contains(err.Error(), "cannot retry request with non-rewindable body") {
		t.Fatalf("Do() error = %q, want non-rewindable body message", err)
	}
	if authCalls != 1 {
		t.Fatalf("auth calls = %d, want 1", authCalls)
	}
}

func TestNewRequestPropagatesCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req, err := client.newRequest(ctx, http.MethodGet, server.URL+"/resource", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do() error = %v, want context.Canceled", err)
	}
}
