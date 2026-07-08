package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (c *Client) SendChallenge(contentId, videoToken string, challenge []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost,
		"https://www.crunchyroll.com/license/v1/license/widevine",
		io.NopCloser(bytes.NewReader(challenge)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Cr-Content-Id", contentId)
	req.Header.Set("X-Cr-Video-Token", videoToken)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result CrunchyrollWidevineLicenseResponse
	if err := json.Unmarshal(res, &result); err != nil {
		return nil, fmt.Errorf("parsing license response: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(result.License)
	if err != nil {
		return nil, fmt.Errorf("decoding license: %w", err)
	}

	return decoded, nil
}
