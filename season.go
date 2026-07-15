package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type SeasonEpisodes struct {
	Data []SeasonEpisode `json:"data"`
}

type SeasonEpisode struct {
	ID                 string        `json:"id"`
	Versions           []*DubVersion `json:"versions"`
	SeasonNumber       int           `json:"season_number"`
	EpisodeNumber      int           `json:"episode_number"`
	SeriesTitle        string        `json:"series_title"`
	AudioLocale        string        `json:"audio_locale"`
	Title              string        `json:"title"`
	Description        string        `json:"description"`
	AvailabilityStarts string        `json:"availability_starts"`
}

func getSeasonEpisodes(contentId string, audio_locale string, sub_locale string) []SeasonEpisode {
	if audio_locale == "" {
		audio_locale = "ja-JP"
	}

	if sub_locale == "" {
		sub_locale = "en-US"
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://www.crunchyroll.com/content/v2/cms/seasons/%s/episodes?preferred_audio_language=%s&locale=%s", contentId, audio_locale, sub_locale), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := DoRequest(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var episodes SeasonEpisodes
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(body, &episodes); err != nil {
		panic(err)
	}

	return episodes.Data
}

type Seasons struct {
	Data []Season `json:"data"`
}

type Season struct {
	ID           string `json:"id"`
	SeasonNumber int    `json:"season_number"`
}

func getSeasons(contentId string, audioLocale string, subLocale string) []Season {
	if audioLocale == "" {
		audioLocale = "ja-JP"
	}

	if subLocale == "" {
		subLocale = "en-US"
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://www.crunchyroll.com/content/v2/cms/series/%s/seasons?force_locale=&preferred_audio_language=%s&locale=%s", contentId, audioLocale, subLocale), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := DoRequest(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var seasons Seasons
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(body, &seasons); err != nil {
		panic(err)
	}

	return seasons.Data
}
