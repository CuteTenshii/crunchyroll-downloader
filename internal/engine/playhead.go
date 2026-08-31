package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const playFinishRatio = 0.95

func IsPlayFinished(positionSec, durationSec float64) bool {
	if durationSec <= 0 {
		return false
	}
	return positionSec >= durationSec*playFinishRatio
}

func FinishPlayheadSeconds(durationSec float64) float64 {
	if durationSec < 0 {
		return 0
	}
	return durationSec
}

type playheadPOSTBody struct {
	ContentID string  `json:"content_id"`
	Playhead  float64 `json:"playhead"`
}

func PostPlayhead(accountID, contentID string, playheadSec float64, locale, audioLang string) error {
	return PostPlayheadWithBase("https://www.crunchyroll.com", accountID, contentID, playheadSec, locale, audioLang)
}

func PostPlayheadWithBase(base, accountID, contentID string, playheadSec float64, locale, audioLang string) error {
	accountID = strings.TrimSpace(accountID)
	contentID = strings.TrimSpace(contentID)
	if accountID == "" || contentID == "" {
		return fmt.Errorf("playhead requires account and content id")
	}
	if playheadSec < 0 {
		playheadSec = 0
	}
	locale = normalizeDiscoverLocale(locale)
	q := url.Values{}
	if locale != "" {
		q.Set("locale", locale)
	}
	if strings.TrimSpace(audioLang) != "" {
		q.Set("preferred_audio_language", audioLang)
	}
	endpoint := strings.TrimRight(base, "/") + "/content/v2/" + url.PathEscape(accountID) + "/playheads"
	if enc := q.Encode(); enc != "" {
		endpoint += "?" + enc
	}
	payload, err := json.Marshal(playheadPOSTBody{ContentID: contentID, Playhead: playheadSec})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := DoRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("playhead POST HTTP %d", resp.StatusCode)
	}
	return nil
}

func GetPlayheads(accountID string, contentIDs []string) (map[string]PlayheadInfo, error) {
	return fetchPlayheads(accountID, contentIDs)
}
