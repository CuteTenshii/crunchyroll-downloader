package main

import (
	"net/http"
)

func DoRequest(req *http.Request) (*http.Response, error) {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		print("Access token expired. Refetching one...\n")
		resp.Body.Close()
		// Refetch an access token
		token = GetAccessToken(*etpRt)
		req.Header.Set("Authorization", "Bearer "+token)
		// The first attempt already consumed req.Body, so reset it from
		// GetBody before retrying or the retry sends an empty/truncated
		// payload (which broke license requests as "invalid license type").
		if req.GetBody != nil {
			if body, err := req.GetBody(); err == nil {
				req.Body = body
			}
		}
		// and retry the request
		return DoRequest(req)
	}

	return resp, err
}
