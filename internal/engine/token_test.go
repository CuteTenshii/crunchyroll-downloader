package engine

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAccessTokenRejectsMissingAccountIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"token"}`))
	}))
	defer server.Close()
	oldEndpoint := tokenEndpoint
	tokenEndpoint = server.URL
	defer func() { tokenEndpoint = oldEndpoint }()
	_, err := GetAccessToken("cookie")
	var tokenErr *TokenError
	if !errors.As(err, &tokenErr) || tokenErr.Problem != "response has no account identity" {
		t.Fatalf("expected missing-identity TokenError, got %v", err)
	}
}

func TestRefreshAccessTokenRejectsDifferentAccountBeforeAdoptingToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"new-token","account_id":"other-account"}`))
	}))
	defer server.Close()
	oldEndpoint, oldToken := tokenEndpoint, token
	tokenEndpoint, token = server.URL, "old-token"
	authState.Lock()
	oldSecret, oldAccountID := authState.etpRT, authState.accountID
	authState.etpRT, authState.accountID = "cookie", "expected-account"
	authState.Unlock()
	defer func() {
		tokenEndpoint, token = oldEndpoint, oldToken
		authState.Lock()
		authState.etpRT, authState.accountID = oldSecret, oldAccountID
		authState.Unlock()
	}()
	err := refreshAccessToken()
	var mismatch *AccountIdentityMismatchError
	if !errors.As(err, &mismatch) || token != "old-token" {
		t.Fatalf("expected identity mismatch without token adoption, got err=%v token=%q", err, token)
	}
}
