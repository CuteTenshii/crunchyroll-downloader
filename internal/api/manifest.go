package api

import (
	"context"
	"io"
	"net/http"

	"crunchyroll-downloader/internal/output"
)

func (c *Client) FetchManifest(ctx context.Context, url string) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		output.Global.Debug("\n%s", string(body))
	}

	return body, nil
}
