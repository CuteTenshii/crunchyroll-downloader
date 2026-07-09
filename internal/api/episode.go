package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (c *Client) GetEpisode(ctx context.Context, id string) (*Episode, error) {
	req, err := c.newRequest(ctx, http.MethodGet,
		c.url(fmt.Sprintf("/playback/v3/%s/web/firefox/play", id)), nil)
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

	var episode Episode
	if err := json.Unmarshal(body, &episode); err != nil {
		return nil, err
	}

	if episode.Error != nil {
		return nil, fmt.Errorf("API error: %v", episode.Error)
	}

	if c.Debug {
		fmt.Printf("\n%s\n", string(body))
	}

	return &episode, nil
}

func (c *Client) GetEpisodeInfo(ctx context.Context, id string) (*EpisodeInfo, error) {
	req, err := c.newRequest(ctx, http.MethodGet,
		c.url(fmt.Sprintf("/content/v2/cms/objects/%s?ratings=true&preferred_audio_language=ja-JP&locale=en-US", id)), nil)
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

	var info EpisodeMetadataResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}

	if len(info.Data) == 0 {
		return nil, fmt.Errorf("no episode info found for id: %s", id)
	}

	return &info.Data[0], nil
}

func (c *Client) DeleteStream(ctx context.Context, contentId, sToken string) (bool, error) {
	req, err := c.newRequest(ctx, http.MethodDelete,
		c.url(fmt.Sprintf("/playback/v1/token/%s/%s", contentId, sToken)), nil)
	if err != nil {
		return false, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return false, err
	}
	resp.Body.Close()

	return resp.StatusCode == http.StatusNoContent, nil
}
