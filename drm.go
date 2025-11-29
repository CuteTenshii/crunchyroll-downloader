package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"
	"github.com/unki2aut/go-mpd"
)

var keys []*widevine.Key

func findSet(sets []*mpd.AdaptationSet, mimeType string) *mpd.AdaptationSet {
	for _, set := range sets {
		if set.MimeType == mimeType {
			return set
		}
	}
	return nil
}

// getPssh finds the PSSH in the MPD manifest
func getPssh(mpd *mpd.MPD) *string {
	period := mpd.Period[0]
	videoSet := findSet(period.AdaptationSets, "video/mp4")
	audioSet := findSet(period.AdaptationSets, "audio/mp4")
	if videoSet == nil || audioSet == nil {
		return nil
	}
	contentProtections := videoSet.ContentProtections
	for _, contentProtection := range contentProtections {
		if contentProtection.CencPSSH != nil {
			return contentProtection.CencPSSH
		}
	}
	return nil
}

type CrunchyrollWidevineLicenseResponse struct {
	License string `json:"license"`
}

func sendChallenge(contentId, videoToken string, challenge []byte) []byte {
	req, err := http.NewRequest(http.MethodPost, "https://www.crunchyroll.com/license/v1/license/widevine", io.NopCloser(bytes.NewReader(challenge)))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("x-cr-content-id", contentId)
	req.Header.Set("x-cr-video-token", videoToken)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	// Parse JSON response
	res, err := io.ReadAll(resp.Body)
	var result CrunchyrollWidevineLicenseResponse
	if err := json.Unmarshal(res, &result); err != nil {
		panic(fmt.Errorf("failed to get access token: %w", err))
	}
	decoded, err := base64.StdEncoding.DecodeString(result.License)
	if err != nil {
		panic(err)
	}
	return decoded
}

func getLicense(psshData, contentId, videoToken string) []*widevine.Key {
	wvd, err := os.Open("./lenovo_lenovo_tb-j616f_16.1.0_d57ca855_22594_l1.wvd")
	if err != nil {
		panic(err)
	}
	var content []byte
	wvd.Read(content)
	device, err := widevine.NewDevice(widevine.FromWVD(io.NopCloser(wvd)))
	if err != nil {
		panic(err)
	}
	cdm := widevine.NewCDM(device)
	decodedPssh, err := base64.StdEncoding.DecodeString(psshData)
	if err != nil {
		panic(err)
	}
	pssh, err := widevine.NewPSSH(decodedPssh)
	if err != nil {
		panic(err)
	}

	challenge, parseLicense, err := cdm.GetLicenseChallenge(pssh, widevinepb.LicenseType_AUTOMATIC, false)
	if err != nil {
		panic(err)
	}
	resp := sendChallenge(contentId, videoToken, challenge)
	keys, err = parseLicense(resp)
	if err != nil {
		panic(err)
	}

	return keys
}

func decryptPart(data []byte) ([]byte, error) {
	var res bytes.Buffer
	err := widevine.DecryptMP4Auto(io.NopCloser(bytes.NewReader(data)), keys, &res)

	return res.Bytes(), err
}
