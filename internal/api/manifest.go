package api

import (
	"fmt"
	"io"
	"net/http"
)

func (c *Client) FetchManifest(url string) ([]byte, error) {
	req, err := c.newRequest(http.MethodGet, url)
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
		fmt.Printf("\n%s\n", string(body))
	}

	return body, nil
}
