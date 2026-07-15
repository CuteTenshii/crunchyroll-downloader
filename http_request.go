package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

var authenticatedHTTPClient = &http.Client{Timeout: providerHTTPTimeout}

const maxRetryRequestBodyBytes int64 = 8 << 20

type RequestRetryBodyError struct{ Problem string }

func (e *RequestRetryBodyError) Error() string {
	return "cannot safely replay authenticated request body: " + e.Problem
}

// ensureReplayableBody captures only requests whose constructors did not set
// GetBody (for example io.NopCloser(bytes.NewReader(...))). The buffered bytes
// stay in memory, are size-bounded, and are never logged.
func ensureReplayableBody(req *http.Request) error {
	if req.Body == nil || req.GetBody != nil {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxRetryRequestBodyBytes+1))
	if closeErr := req.Body.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return &RequestRetryBodyError{Problem: "read body"}
	}
	if int64(len(body)) > maxRetryRequestBodyBytes {
		return &RequestRetryBodyError{Problem: "body exceeds replay limit"}
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return nil
}

// DoRequest makes one request and permits at most one credential refresh. The
// initial 401 body is closed before retrying, avoiding leaked connections and
// unbounded recursive retries when credentials are no longer accepted.
func DoRequest(req *http.Request) (*http.Response, error) {
	if err := ensureReplayableBody(req); err != nil {
		return nil, err
	}
	resp, err := authenticatedHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	_ = resp.Body.Close()

	if err := refreshAccessToken(); err != nil {
		return nil, fmt.Errorf("refresh access token after 401: %w", err)
	}
	retry := req.Clone(req.Context())
	retry.Header = req.Header.Clone()
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, &RequestRetryBodyError{Problem: "reconstruct body"}
		}
		retry.Body = body
	}
	retry.Header.Set("Authorization", "Bearer "+token)
	resp, err = authenticatedHTTPClient.Do(retry)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		return nil, &HTTPStatusError{URL: redactURL(retry.URL.String()), StatusCode: http.StatusUnauthorized}
	}
	return resp, nil
}
