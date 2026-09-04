package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// EpisodeError is the playback endpoint's polymorphic "error" field. It is a
// string message on failure, but Crunchyroll also returns false, null or a bare
// number when playback is fine, which a plain *string cannot unmarshal.
type EpisodeError string

// UnmarshalJSON accepts any shape Crunchyroll returns for "error", storing the
// message for real errors and the empty string for false/null/0.
func (e *EpisodeError) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	switch s {
	case "", "null", "false", "0":
		*e = ""
		return nil
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var msg string
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		*e = EpisodeError(msg)
		return nil
	}
	*e = EpisodeError(s)
	return nil
}

type Episode struct {
	// Dash manifest file URL
	ManifestURL string `json:"url"`
	// List of .ass files (translation-style subtitles)
	Subtitles map[string]*Subtitle `json:"subtitles"`
	// List of .vtt files (closed captions, transcribing the dub audio)
	Captions map[string]*Subtitle `json:"captions"`
	// Token to give to the Widevine CDM challenge
	Token string `json:"token"`
	// Error, empty when there's no error
	Error EpisodeError `json:"error"`
	// Reason explains the error (e.g. "Too many requests") when present
	Reason string `json:"reason"`
}

type Subtitle struct {
	// Language represents a subtitle language in the "en-US" format
	Language string `json:"language"`
	// Format of the file, e.g. "ass" or "vtt"
	Format string `json:"format"`
	// Direct URL to the subtitle/caption file
	URL string `json:"url"`
}

func getEpisode(id string) (Episode, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://www.crunchyroll.com/playback/v3/%s/web/firefox/play", id), nil)
	if err != nil {
		return Episode{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := DoRequest(req)
	if err != nil {
		return Episode{}, err
	}
	defer resp.Body.Close()

	var episode Episode
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Episode{}, err
	}
	if err = json.Unmarshal(body, &episode); err != nil {
		return Episode{}, err
	}
	if episode.Error != "" {
		fmt.Printf("Error: %s", episode.Error)
		if episode.Reason != "" {
			fmt.Printf(" (%s)", episode.Reason)
		}
		fmt.Println()
		if strings.HasPrefix(string(episode.Error), "429") {
			fmt.Println("Crunchyroll is rate-limiting this account. Wait a while before retrying, or use a different account.")
		}
		return Episode{}, fmt.Errorf("playback error: %s", episode.Error)
	}

	if *debug {
		fmt.Printf("\n%s\n", string(body))
	}

	return episode, nil
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
	resp, err := DoRequest(req)
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

// deleteStream removes the stream to make Crunchyroll think we "left" the playback.
// It returns an error rather than panicking so a transient network failure during
// cleanup can't crash the whole process or strand other streams mid-teardown.
func deleteStream(contentId, sToken string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("https://www.crunchyroll.com/playback/v1/token/%s/%s", contentId, sToken), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := DoRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}
