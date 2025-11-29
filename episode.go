package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Episode struct {
	// Dash manifest file URL
	ManifestUrl string `json:"url"`
	// List of .ass files
	Subtitles map[string]*Subtitle `json:"subtitles"`
	// Token to give to Widevine CDM challenge
	Token string  `json:"token"`
	Error *string `json:"error"`
}

type Subtitle struct {
	// example: en-US
	Language string `json:"language"`
	// Direct URL to the .ass file
	Url string `json:"url"`
}

func getEpisode(id string) Episode {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://www.crunchyroll.com/playback/v3/%s/web/firefox/play", id), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var episode Episode
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(body, &episode); err != nil {
		panic(err)
	}
	if episode.Error != nil {
		panic(*episode.Error)
	}

	return episode
}

type EpisodeMetadataResponse struct {
	Data []EpisodeInfo `json:"data"`
}

type EpisodeInfo struct {
	EpisodeMetadata EpisodeMetadata `json:"episode_metadata"`
	// Episode title
	Title string `json:"title"`
}

type EpisodeMetadata struct {
	AudioLocale   string `json:"audio_locale"`
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	SeriesTitle   string `json:"series_title"`
	// AvailabilityStarts represents the date when the episode was released on Crunchyroll
	AvailabilityStarts string        `json:"availability_starts"`
	Versions           []*DubVersion `json:"versions"`
}

type DubVersion struct {
	AudioLocale string `json:"audio_locale"`
	GUID        string `json:"guid"`
}

func getEpisodeInfo(id string) EpisodeInfo {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://www.crunchyroll.com/content/v2/cms/objects/%s?ratings=true&preferred_audio_language=ja-JP&locale=en-US", id), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var info EpisodeMetadataResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(body, &info); err != nil {
		panic(err)
	}

	return info.Data[0]
}
