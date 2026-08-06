package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/google/uuid"
)

var deviceID = uuid.NewString()

// token is the current bearer access token for provider API calls.
var token = ""

var tokenEndpoint = "https://www.crunchyroll.com/auth/v1/token"
var tokenHTTPClient = &http.Client{Timeout: providerHTTPTimeout}

var authState struct {
	sync.Mutex
	etpRT     string
	accountID string
}

type CrunchyrollTokenResponse struct {
	AccessToken string `json:"access_token"`
	AccountID   string `json:"account_id"`
}

// TokenError deliberately carries status and a bounded server diagnostic, not
// the request cookie or response body.
type TokenError struct {
	StatusCode int
	Problem    string
}

func (e *TokenError) Error() string {
	if e.StatusCode == 0 {
		return "token exchange failed: " + e.Problem
	}
	return fmt.Sprintf("token exchange failed with HTTP %d: %s", e.StatusCode, e.Problem)
}

// AccountIdentityMismatchError prevents a refreshed token from quietly moving
// an in-progress serial run to a different account when the token endpoint
// provides account_id.
type AccountIdentityMismatchError struct {
	Expected string
	Actual   string
}

func (e *AccountIdentityMismatchError) Error() string {
	return "token refresh returned a different account identity"
}

func setETPRT(secret string) {
	authState.Lock()
	defer authState.Unlock()
	authState.etpRT = secret
}

func getETPRT() string {
	authState.Lock()
	defer authState.Unlock()
	return authState.etpRT
}

// GetAccountID returns the account id pinned from the last successful token exchange.
func GetAccountID() string {
	authState.Lock()
	defer authState.Unlock()
	return authState.accountID
}

// GetAccessToken fetches an access token with a supplied cookie. Callers that
// own a run should use refreshAccessToken so account identity pinning applies.
func GetAccessToken(etpRT string) (CrunchyrollTokenResponse, error) {
	body := url.Values{}
	body.Set("device_id", deviceID)
	body.Set("device_type", "Firefox on Linux")
	body.Set("grant_type", "etp_rt_cookie")

	req, err := http.NewRequest(http.MethodPost, tokenEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return CrunchyrollTokenResponse{}, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Authorization", "Basic bm9haWhkZXZtXzZpeWcwYThsMHE6")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	req.AddCookie(&http.Cookie{Name: "device_id", Value: deviceID})
	req.AddCookie(&http.Cookie{Name: "etp_rt", Value: etpRT})

	recordProviderCall(req)
	resp, err := tokenHTTPClient.Do(req)
	if err != nil {
		return CrunchyrollTokenResponse{}, fmt.Errorf("send token request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return CrunchyrollTokenResponse{}, fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return CrunchyrollTokenResponse{}, &TokenError{StatusCode: resp.StatusCode, Problem: "token endpoint rejected the credential"}
	}
	var result CrunchyrollTokenResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return CrunchyrollTokenResponse{}, &TokenError{StatusCode: resp.StatusCode, Problem: "invalid JSON response"}
	}
	if strings.TrimSpace(result.AccessToken) == "" {
		return CrunchyrollTokenResponse{}, &TokenError{StatusCode: resp.StatusCode, Problem: "response has no access token"}
	}
	if strings.TrimSpace(result.AccountID) == "" {
		return CrunchyrollTokenResponse{}, &TokenError{StatusCode: resp.StatusCode, Problem: "response has no account identity"}
	}
	return result, nil
}

func refreshAccessToken() error {
	authState.Lock()
	defer authState.Unlock()
	if authState.etpRT == "" {
		return &TokenError{Problem: "credential is not configured"}
	}
	result, err := GetAccessToken(authState.etpRT)
	if err != nil {
		return err
	}
	if authState.accountID != "" && authState.accountID != result.AccountID {
		return &AccountIdentityMismatchError{Expected: authState.accountID, Actual: result.AccountID}
	}
	if authState.accountID == "" {
		authState.accountID = result.AccountID
	}
	token = result.AccessToken
	return nil
}
