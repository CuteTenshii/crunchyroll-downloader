package engine

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoRequestRetries401OnlyOnce(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"fresh","account_id":"same"}`))
		default:
			requests++
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	oldEndpoint, oldToken := tokenEndpoint, token
	tokenEndpoint = server.URL + "/token"
	token = "expired"
	authState.Lock()
	oldSecret, oldAccountID := authState.etpRT, authState.accountID
	authState.etpRT, authState.accountID = "cookie", "same"
	authState.Unlock()
	defer func() {
		tokenEndpoint, token = oldEndpoint, oldToken
		authState.Lock()
		authState.etpRT, authState.accountID = oldSecret, oldAccountID
		authState.Unlock()
	}()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/resource", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := DoRequest(req)
	if resp != nil {
		t.Fatal("terminal 401 response must be closed and discarded")
	}
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected terminal HTTPStatusError, got %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want exactly 2", requests)
	}
}

func TestProviderHTTPClientsHaveBoundedTimeouts(t *testing.T) {
	for name, client := range map[string]*http.Client{
		"subtitle":      subtitleHTTPClient,
		"segment":       segmentHTTPClient,
		"manifest":      manifestHTTPClient,
		"token":         tokenHTTPClient,
		"authenticated": authenticatedHTTPClient,
	} {
		if client == nil || client.Timeout <= 0 {
			t.Fatalf("%s client does not have a bounded timeout", name)
		}
	}
}

func TestDoRequestReplaysPOSTBodyAfter401(t *testing.T) {
	want := []byte("widevine-challenge-bytes")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"access_token":"fresh","account_id":"same"}`))
		case "/license":
			requests++
			got, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read request body: %v", err)
				return
			}
			if !bytes.Equal(got, want) {
				t.Errorf("request %d body = %q, want %q", requests, got, want)
			}
			if requests == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	oldEndpoint, oldToken := tokenEndpoint, token
	tokenEndpoint, token = server.URL+"/token", "expired"
	authState.Lock()
	oldSecret, oldAccountID := authState.etpRT, authState.accountID
	authState.etpRT, authState.accountID = "cookie", "same"
	authState.Unlock()
	defer func() {
		tokenEndpoint, token = oldEndpoint, oldToken
		authState.Lock()
		authState.etpRT, authState.accountID = oldSecret, oldAccountID
		authState.Unlock()
	}()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/license", io.NopCloser(bytes.NewReader(want)))
	if err != nil {
		t.Fatal(err)
	}
	if req.GetBody != nil {
		t.Fatal("test requires a request constructor without GetBody")
	}
	resp, err := DoRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || requests != 2 {
		t.Fatalf("status=%d requests=%d, want 204 and 2", resp.StatusCode, requests)
	}
}
