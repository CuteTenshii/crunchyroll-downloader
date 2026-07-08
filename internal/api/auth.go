package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

func (c *Client) fetchAccessToken() (string, error) {
	c.deviceID = uuid.NewString()

	body := url.Values{}
	body.Set("device_id", c.deviceID)
	body.Set("device_type", "Firefox on Linux")
	body.Set("grant_type", "etp_rt_cookie")

	req, err := http.NewRequest(http.MethodPost, "https://www.crunchyroll.com/auth/v1/token",
		strings.NewReader(body.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Basic bm9haWhkZXZtXzZpeWcwYThsMHE6")
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
