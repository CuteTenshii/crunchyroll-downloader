package drm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"
	"github.com/unki2aut/go-mpd"

	"crunchyroll-downloader/internal/api"
)

const (
	widevineDevicePathEnv     = "WIDEVINE_DEVICE_PATH"
	widevineClientIDPathEnv   = "WIDEVINE_CLIENT_ID_PATH"
	widevinePrivateKeyPathEnv = "WIDEVINE_PRIVATE_KEY_PATH"
)

var (
	widevineDeviceOnce   sync.Once
	widevineDevice       *widevine.Device
	widevineDeviceErr    error
	widevineDeviceLoader = loadWidevineDevice
	widevineDevicePath   string // set by SetWidevinePath before first GetWidevineDevice call
)

// SetWidevinePath sets the explicit device path from CLI/config resolution.
// Must be called before any call to GetWidevineDevice.
func SetWidevinePath(path string) {
	widevineDevicePath = path
}

// DevicePathFormat indicates the type of Widevine device path provided.
type DevicePathFormat int

const (
	FormatUnknown DevicePathFormat = iota
	FormatWVD
	FormatRawDir
)

// DetectDevicePath examines a path and determines its format.
func DetectDevicePath(path string) (DevicePathFormat, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FormatUnknown, fmt.Errorf("accessing Widevine device path: %w", err)
	}

	if info.IsDir() {
		_, errCID := os.Stat(filepath.Join(path, "client_id.bin"))
		_, errPk := os.Stat(filepath.Join(path, "private_key.pem"))
		if errCID == nil && errPk == nil {
			return FormatRawDir, nil
		}
		return FormatUnknown, fmt.Errorf("directory does not contain client_id.bin and private_key.pem")
	}

	if strings.HasSuffix(strings.ToLower(path), ".wvd") {
		return FormatWVD, nil
	}

	return FormatUnknown, fmt.Errorf("unrecognized file format (expected .wvd file or directory with client_id.bin + private_key.pem)")
}

type widevineDeviceConfig struct {
	wvdPath        string
	clientIDPath   string
	privateKeyPath string
}

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
	widevineDeviceOnce.Do(func() {
		widevineDevice, widevineDeviceErr = widevineDeviceLoader()
	})

	return widevineDevice, widevineDeviceErr
}

func loadWidevineDevice() (*widevine.Device, error) {
	config, err := discoverWidevineDeviceConfig()
	if err != nil {
		return nil, err
	}

	if config.wvdPath != "" {
		wvd, err := os.Open(config.wvdPath)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", widevineDevicePathEnv, err)
		}
		defer wvd.Close()

		return widevine.NewDevice(widevine.FromWVD(io.NopCloser(wvd)))
	}

	clientID, err := os.ReadFile(config.clientIDPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", widevineClientIDPathEnv, err)
	}

	privateKey, err := os.ReadFile(config.privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", widevinePrivateKeyPathEnv, err)
	}

	return widevine.NewDevice(widevine.FromRaw(clientID, privateKey))
}

func discoverWidevineDeviceConfig() (widevineDeviceConfig, error) {
	// Priority: explicit path (from SetWidevinePath) > env vars (legacy names) > error
	if widevineDevicePath != "" {
		fmt, err := DetectDevicePath(widevineDevicePath)
		if err != nil {
			return widevineDeviceConfig{}, err
		}
		switch fmt {
		case FormatWVD:
			return widevineDeviceConfig{wvdPath: widevineDevicePath}, nil
		case FormatRawDir:
			return widevineDeviceConfig{
				clientIDPath:   filepath.Join(widevineDevicePath, "client_id.bin"),
				privateKeyPath: filepath.Join(widevineDevicePath, "private_key.pem"),
			}, nil
		}
	}

	// Fallback: legacy env var names (keep per D-15)
	if v, ok := os.LookupEnv(widevineDevicePathEnv); ok && v != "" {
		return widevineDeviceConfig{wvdPath: v}, nil
	}
	if cid, ok := os.LookupEnv(widevineClientIDPathEnv); ok && cid != "" {
		if pk, ok := os.LookupEnv(widevinePrivateKeyPathEnv); ok && pk != "" {
			return widevineDeviceConfig{clientIDPath: cid, privateKeyPath: pk}, nil
		}
		return widevineDeviceConfig{}, fmt.Errorf("incomplete Widevine device configuration: set %s and %s together, or set %s to a .wvd file",
			widevineClientIDPathEnv,
			widevinePrivateKeyPathEnv,
			widevineDevicePathEnv,
		)
	}

	return widevineDeviceConfig{}, missingWidevineDeviceError()
}

func missingWidevineDeviceError() error {
	return errors.New("no Widevine device configured: pass --widevine-device flag with a .wvd file or directory containing client_id.bin + private_key.pem, or set WIDEVINE_DEVICE_PATH / WIDEVINE_CLIENT_ID_PATH + WIDEVINE_PRIVATE_KEY_PATH environment variables")
}

func GetLicense(ctx context.Context, client *api.Client, psshData, contentId, videoToken string) ([]*widevine.Key, error) {
	device, err := GetWidevineDevice()
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, missingWidevineDeviceError()
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
