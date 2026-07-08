package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (c *Client) SendChallenge(ctx context.Context, contentId, videoToken string, challenge []byte) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodPost,
		c.licenseURL,
		bytes.NewReader(challenge))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Cr-Content-Id", contentId)
	req.Header.Set("X-Cr-Video-Token", videoToken)
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")

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
