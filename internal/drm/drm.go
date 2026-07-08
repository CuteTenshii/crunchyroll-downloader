package drm

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"
	"github.com/unki2aut/go-mpd"

	"crunchyroll-downloader/internal/api"
)

func GetPssh(mpd *mpd.MPD) *string {
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

func GetWidevineDevice() (*widevine.Device, error) {
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
			clientID, err = io.ReadAll(f)
			f.Close()
			if err != nil {
				return nil, err
			}
		} else if file.Name() == "private_key.pem" {
			f, err := os.Open("private_key.pem")
			if err != nil {
				return nil, err
			}
			privateKey, err = io.ReadAll(f)
			f.Close()
			if err != nil {
				return nil, err
			}
		}
	}

	if len(clientID) > 0 && len(privateKey) > 0 {
		return widevine.NewDevice(widevine.FromRaw(clientID, privateKey))
	}

	return nil, nil
}

func GetLicense(ctx context.Context, client *api.Client, psshData, contentId, videoToken string) ([]*widevine.Key, error) {
	device, err := GetWidevineDevice()
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, errors.New("no widevine device provided. You either need:\n- a \".wvd\" file,\n- or \"client_id.bin\" and \"private_key.pem\" files.\n")
	}

	cdm := widevine.NewCDM(device)

	decodedPssh, err := base64.StdEncoding.DecodeString(psshData)
	if err != nil {
		return nil, err
	}

	pssh, err := widevine.NewPSSH(decodedPssh)
	if err != nil {
		return nil, err
	}

	challenge, parseLicense, err := cdm.GetLicenseChallenge(pssh, widevinepb.LicenseType_AUTOMATIC, false)
	if err != nil {
		return nil, err
	}

	resp, err := client.SendChallenge(ctx, contentId, videoToken, challenge)
	if err != nil {
		return nil, err
	}

	keys, err := parseLicense(resp)
	if err != nil {
		return nil, err
	}

	return keys, nil
}
