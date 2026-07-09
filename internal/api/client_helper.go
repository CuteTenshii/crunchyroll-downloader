package api

import "net/http"

// NewTestClient creates a minimal *Client for testing without real HTTP calls.
// The httpClient is used directly (no auth flow). If the RoundTripper matches
// requests to a local test server the caller controls, tests can verify
// integration behavior without real credentials.
//
// NOTE: This is exported to enable downstream test packages to construct
// a Client without going through the full New-flow (which requires a real
// etp_rt cookie and makes a real HTTP call for the access token).
func NewTestClient(httpClient *http.Client, baseURL, token string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		token:      token,
	}
}
