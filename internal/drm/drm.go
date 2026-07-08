package drm

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
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
	widevineEnvFile           = ".env"
)

var (
	widevineDeviceOnce   sync.Once
	widevineDevice       *widevine.Device
	widevineDeviceErr    error
	widevineDeviceLoader = loadWidevineDevice
)

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
	envFileValues, err := readDotEnv(widevineEnvFile)
	if err != nil {
		return widevineDeviceConfig{}, err
	}

	config := widevineDeviceConfig{
		wvdPath:        envValue(widevineDevicePathEnv, envFileValues),
		clientIDPath:   envValue(widevineClientIDPathEnv, envFileValues),
		privateKeyPath: envValue(widevinePrivateKeyPathEnv, envFileValues),
	}

	if config.wvdPath != "" {
		return config, nil
	}

	hasClientID := config.clientIDPath != ""
	hasPrivateKey := config.privateKeyPath != ""
	if hasClientID != hasPrivateKey {
		return widevineDeviceConfig{}, fmt.Errorf("incomplete Widevine device configuration: set %s and %s together, or set %s to a .wvd file",
			widevineClientIDPathEnv,
			widevinePrivateKeyPathEnv,
			widevineDevicePathEnv,
		)
	}

	if hasClientID && hasPrivateKey {
		return config, nil
	}

	return widevineDeviceConfig{}, missingWidevineDeviceError()
}

func envValue(key string, envFileValues map[string]string) string {
	if value, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(value)
	}

	return strings.TrimSpace(envFileValues[key])
}

func readDotEnv(path string) (map[string]string, error) {
	values := map[string]string{}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key != "" {
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	return values, nil
}

func missingWidevineDeviceError() error {
	return errors.New("no Widevine device configured: set WIDEVINE_DEVICE_PATH to a .wvd file, or set WIDEVINE_CLIENT_ID_PATH and WIDEVINE_PRIVATE_KEY_PATH together")
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
