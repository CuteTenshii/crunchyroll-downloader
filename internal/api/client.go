package api

import (
	"fmt"
	"net/http"
)

type Client struct {
	httpClient *http.Client
	token      string
	etpRt      string
	deviceID   string
	Debug      bool
}

var userAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0"

func New(etpRt string) (*Client, error) {
	if etpRt == "" {
		return nil, fmt.Errorf("etp_rt cookie is required")
	}

	c := &Client{
		httpClient: &http.Client{},
		etpRt:      etpRt,
	}

	token, err := c.fetchAccessToken()
	if err != nil {
		return nil, fmt.Errorf("fetching access token: %w", err)
	}
	c.token = token

	return c, nil
}

func (c *Client) newRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		fmt.Println("Access token expired. Refetching one...")
		token, err := c.fetchAccessToken()
		if err != nil {
			return nil, fmt.Errorf("refreshing token: %w", err)
		}
		c.token = token
		req.Header.Set("Authorization", "Bearer "+c.token)
		return c.httpClient.Do(req)
	}
	return resp, nil
}
