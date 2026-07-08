package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	token      string
	etpRt      string
	deviceID   string
	baseURL    string
	authURL    string
	licenseURL string
	Debug      bool
}

var userAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0"

const (
	defaultBaseURL    = "https://www.crunchyroll.com"
	defaultAuthURL    = defaultBaseURL + "/auth/v1/token"
	defaultLicenseURL = defaultBaseURL + "/license/v1/license/widevine"
)

func New(etpRt string) (*Client, error) {
	return NewWithContext(context.Background(), etpRt)
}

func NewWithContext(ctx context.Context, etpRt string) (*Client, error) {
	if etpRt == "" {
		return nil, fmt.Errorf("etp_rt cookie is required")
	}

	c := &Client{
		httpClient: configuredHTTPClient(),
		etpRt:      etpRt,
		baseURL:    defaultBaseURL,
		authURL:    defaultAuthURL,
		licenseURL: defaultLicenseURL,
	}

	token, err := c.fetchAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching access token: %w", err)
	}
	c.token = token

	return c, nil
}

func configuredHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   20,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: 60 * time.Second,
	}
}

func (c *Client) url(path string) string {
	return c.baseURL + path
}

func (c *Client) newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
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
		resp.Body.Close()
		fmt.Println("Access token expired. Refetching one...")
		token, err := c.fetchAccessToken(req.Context())
		if err != nil {
			return nil, fmt.Errorf("refreshing token: %w", err)
		}
		c.token = token

		retryReq := req.Clone(req.Context())
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("resetting request body for retry: %w", err)
			}
			retryReq.Body = body
			retryReq.GetBody = req.GetBody
			retryReq.ContentLength = req.ContentLength
		} else if req.Body != nil {
			return nil, fmt.Errorf("cannot retry request with non-rewindable body")
		}
		retryReq.Header.Set("Authorization", "Bearer "+c.token)

		resp, err = c.httpClient.Do(retryReq)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			return nil, fmt.Errorf("unauthorized after token refresh retry")
		}
		return resp, nil
	}
	return resp, nil
}
