package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/google/uuid"
)

// defaultClientAuth is the public Crunchyroll client identifier used for Basic Auth.
// It is documented here so users know the default value. If Crunchyroll rotates
// this credential, set the CRUNCHYROLL_CLIENT_AUTH environment variable to the new value.
const defaultClientAuth = "Basic bm9haWhkZXZtXzZpeWcwYThsMHE6"

// getClientAuth returns the Basic Auth credential to use for token requests.
// If the CRUNCHYROLL_CLIENT_AUTH environment variable is set and non-empty, it
// is returned. Otherwise, the compiled-in defaultClientAuth is used.
// The environment variable is checked on every call, enabling credential rotation
// without restarting the process.
func getClientAuth() string {
	if auth, ok := os.LookupEnv("CRUNCHYROLL_CLIENT_AUTH"); ok && auth != "" {
		return auth
	}
	return defaultClientAuth
}

func (c *Client) fetchAccessToken(ctx context.Context) (string, error) {
	c.deviceID = uuid.NewString()

	body := url.Values{}
	body.Set("device_id", c.deviceID)
	body.Set("device_type", "Firefox on Linux")
	body.Set("grant_type", "etp_rt_cookie")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.authURL,
		strings.NewReader(body.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", getClientAuth())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.AddCookie(&http.Cookie{Name: "device_id", Value: c.deviceID})
	req.AddCookie(&http.Cookie{Name: "etp_rt", Value: c.etpRt})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result CrunchyrollTokenResponse
	if err := json.Unmarshal(res, &result); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}

	return result.AccessToken, nil
}
