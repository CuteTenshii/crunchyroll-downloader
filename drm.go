package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"
	"github.com/unki2aut/go-mpd"
)

var keys []*widevine.Key

// getPssh finds the PSSH in the MPD manifest
func getPssh(mpd *mpd.MPD) *string {
	set := mpd.Period[0].AdaptationSets[0]
	if set == nil {
		return nil
	}

	for _, contentProtection := range set.ContentProtections {
		if contentProtection.CencPSSH != nil {
			return contentProtection.CencPSSH
		}
	}

	return nil
}

type CrunchyrollWidevineLicenseResponse struct {
	License string `json:"license"`
}

func sendChallenge(contentId, videoToken string, challenge []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, "https://www.crunchyroll.com/license/v1/license/widevine", io.NopCloser(bytes.NewReader(challenge)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Cr-Content-Id", contentId)
	req.Header.Set("X-Cr-Video-Token", videoToken)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse JSON response
	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result CrunchyrollWidevineLicenseResponse
	if err = json.Unmarshal(res, &result); err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(result.License)
	if err != nil {
		return nil, err
	}

	return decoded, nil
}

func getWidevineDevice() (*widevine.Device, error) {
	var clientID []byte
	var privateKey []byte
	files, _ := os.ReadDir(".")
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".wvd") {
			wvd, err := os.Open(file.Name())
			if err != nil {
				return nil, err
			}

			return widevine.NewDevice(widevine.FromWVD(io.NopCloser(wvd)))
		} else if file.Name() == "client_id.bin" {
			f, err := os.Open("client_id.bin")
			if err != nil {
				return nil, err
			}
			defer f.Close()

			clientID, err = io.ReadAll(f)
		} else if file.Name() == "private_key.pem" {
			f, err := os.Open("private_key.pem")
			if err != nil {
				return nil, err
			}
			defer f.Close()

			privateKey, err = io.ReadAll(f)
			break
		}
	}

	if len(clientID) > 0 && len(privateKey) > 0 {
		return widevine.NewDevice(widevine.FromRaw(clientID, privateKey))
	}

	return nil, nil
}

func getLicense(psshData, contentId, videoToken string) error {
	device, err := getWidevineDevice()
	if device == nil {
		return errors.New("no widevine device provided. You either need:\n- a \".wvd\" file,\n- or \"client_id.bin\" and \"private_key.pem\" files.\nI'm not sharing links for obvious reasons, but search \"ready to use cdms\" on Google :)\n")
	} else if err != nil {
		return err
	}
	cdm := widevine.NewCDM(device)
	decodedPssh, err := base64.StdEncoding.DecodeString(psshData)
	if err != nil {
		return err
	}
	pssh, err := widevine.NewPSSH(decodedPssh)
	if err != nil {
		return err
	}

	challenge, parseLicense, err := cdm.GetLicenseChallenge(pssh, widevinepb.LicenseType_AUTOMATIC, false)
	if err != nil {
		return err
	}
	resp, err := sendChallenge(contentId, videoToken, challenge)
	if err != nil {
		return err
	}
	keys, err = parseLicense(resp)
	if err != nil {
		return err
	}

	return nil
}

// decryptMP4 decrypts a fragmented MP4 read from media, choosing the content
// key whose ID matches the track's KID. gowidevine's DecryptMP4Auto instead
// uses the first content key in the license, which decrypts to garbage when the
// license carries a separate key per track.
//
// initData is the initialization segment on its own. The KID lives there, so
// the key can be selected without reading the media.
func decryptMP4(initData []byte, media io.Reader, licenseKeys []*widevine.Key, output io.Writer) error {
	init, err := mp4.DecodeFile(bytes.NewReader(initData))
	if err != nil {
		return fmt.Errorf("decode MP4 init segment: %w", err)
	}
	if init.Init == nil {
		return errors.New("MP4 has no initialization segment")
	}

	decryptInfo, err := mp4.DecryptInit(init.Init)
	if err != nil {
		return fmt.Errorf("read MP4 encryption info: %w", err)
	}

	var unmatched []string
	for _, track := range decryptInfo.TrackInfos {
		if track.Sinf == nil || track.Sinf.Schi == nil || track.Sinf.Schi.Tenc == nil {
			continue
		}
		kid := track.Sinf.Schi.Tenc.DefaultKID
		// Match this track's KID with a content key returned by the license.
		for _, key := range licenseKeys {
			if key.Type == widevinepb.License_KeyContainer_CONTENT && bytes.Equal(key.ID, kid) {
				return widevine.DecryptMP4(media, key.Key, output)
			}
		}
		// Keep looking: a later track may still match a key in the license.
		unmatched = append(unmatched, fmt.Sprintf("%x", kid))
	}

	if len(unmatched) > 0 {
		return fmt.Errorf("no license key found for MP4 KID(s) %s", strings.Join(unmatched, ", "))
	}
	return errors.New("MP4 has no encrypted tracks")
}
