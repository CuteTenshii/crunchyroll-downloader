package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (c *Client) GetSeasonEpisodes(ctx context.Context, contentId, audioLocale, subLocale string) ([]SeasonEpisode, error) {
	if audioLocale == "" {
		audioLocale = "ja-JP"
	}
	if subLocale == "" {
		subLocale = "en-US"
	}

	req, err := c.newRequest(ctx, http.MethodGet,
		c.url(fmt.Sprintf("/content/v2/cms/seasons/%s/episodes?preferred_audio_language=%s&locale=%s",
			contentId, audioLocale, subLocale)), nil)
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

	var episodes SeasonEpisodes
	if err := json.Unmarshal(body, &episodes); err != nil {
		return nil, err
	}

	return episodes.Data, nil
}

func (c *Client) GetSeasons(ctx context.Context, contentId, audioLocale, subLocale string) ([]Season, error) {
	if audioLocale == "" {
		audioLocale = "ja-JP"
	}
	if subLocale == "" {
		subLocale = "en-US"
	}

	req, err := c.newRequest(ctx, http.MethodGet,
		c.url(fmt.Sprintf("/content/v2/cms/series/%s/seasons?force_locale=&preferred_audio_language=%s&locale=%s",
			contentId, audioLocale, subLocale)), nil)
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

	var seasons Seasons
	if err := json.Unmarshal(body, &seasons); err != nil {
		return nil, err
	}

	return seasons.Data, nil
}
