package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

var client = &http.Client{}
var deviceId = uuid.NewString()

type CrunchyrollTokenResponse struct {
	AccessToken string `json:"access_token"`
}

// getAccessToken fetches an access token from Crunchyroll
func getAccessToken(etpRt string) string {
	body := url.Values{}
	body.Set("device_id", deviceId)
	body.Set("device_type", "Firefox on Linux")
	body.Set("grant_type", "etp_rt_cookie")

	req, err := http.NewRequest(http.MethodPost, "https://www.crunchyroll.com/auth/v1/token", strings.NewReader(body.Encode()))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Basic bm9haWhkZXZtXzZpeWcwYThsMHE6")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	req.AddCookie(&http.Cookie{Name: "device_id", Value: deviceId})
	req.AddCookie(&http.Cookie{Name: "etp_rt", Value: etpRt})

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Parse JSON response
	res, err := io.ReadAll(resp.Body)
	var result CrunchyrollTokenResponse
	if err := json.Unmarshal(res, &result); err != nil {
		panic(fmt.Errorf("failed to get access token: %w", err))
	}

	return result.AccessToken
}
