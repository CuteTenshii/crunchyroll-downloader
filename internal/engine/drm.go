package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// widevineSearchDirs returns directories that may contain operator-provided CDM files.
// Matches pre-PR#25 "files next to the binary / in the download folder" workflows.
func widevineSearchDirs() []string {
	seen := map[string]struct{}{}
	var dirs []string
	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		abs, err := filepath.Abs(dir)
		if err == nil {
			dir = abs
		}
		if _, ok := seen[dir]; ok {
			return
		}
		seen[dir] = struct{}{}
		dirs = append(dirs, dir)
	}

	if cwd, err := os.Getwd(); err == nil {
		add(cwd)
	}
	if exe, err := os.Executable(); err == nil {
		add(filepath.Dir(exe))
	}
	if home, err := os.UserHomeDir(); err == nil {
		// User's existing project that already holds client_id.bin / private_key.pem
		add(filepath.Join(home, "Documents", "Projects", "Crunchyroll-Downloader"))
		add(filepath.Join(home, "Documents", "Github", "crunchyroll-downloader"))
	}
	return dirs
}

// resolveLocalWidevinePaths finds a .wvd or client_id+private_key pair on disk
// when environment variables are not set (old crdl-windows.exe behavior).
func resolveLocalWidevinePaths() (wvdPath, clientIDPath, privateKeyPath string) {
	for _, dir := range widevineSearchDirs() {
		for _, name := range []string{"device.wvd", "device_client.wvd"} {
			p := filepath.Join(dir, name)
			if regularFileExists(p) {
				return p, "", ""
			}
		}
		clientID := filepath.Join(dir, "client_id.bin")
		privateKey := filepath.Join(dir, "private_key.pem")
		if regularFileExists(clientID) && regularFileExists(privateKey) {
			return "", clientID, privateKey
		}
	}
	return "", "", ""
}

func loadWidevineFromWVD(path string) (*widevine.Device, error) {
	wvd, err := openPrivateRegularFile(path)
	if err != nil {
		// Local operator files on Windows often aren't mode 0600; fall back to Open.
		f, openErr := os.Open(path)
		if openErr != nil {
			return nil, fmt.Errorf("open Widevine device file: %w", err)
		}
		defer f.Close()
		return widevine.NewDevice(widevine.FromWVD(io.NopCloser(f)))
	}
	defer wvd.Close()
	return widevine.NewDevice(widevine.FromWVD(io.NopCloser(wvd)))
}

func loadWidevineFromRaw(clientIDPath, privateKeyPath string) (*widevine.Device, error) {
	readFile := func(path string) ([]byte, error) {
		f, err := openPrivateRegularFile(path)
		if err != nil {
			return os.ReadFile(path)
		}
		defer f.Close()
		return io.ReadAll(f)
	}
	clientID, err := readFile(clientIDPath)
	if err != nil {
		return nil, fmt.Errorf("read Widevine client id file: %w", err)
	}
	privateKey, err := readFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read Widevine private key file: %w", err)
	}
	if len(clientID) == 0 || len(privateKey) == 0 {
		return nil, nil
	}
	return widevine.NewDevice(widevine.FromRaw(clientID, privateKey))
}

// ErrNoAuthorizedWidevine is returned when full-media work has no explicit
// operator-owned device configuration. Play uses this without scanning cwd.
var ErrNoAuthorizedWidevine = errors.New("no authorized Widevine device configured; set CRUNCHYROLL_WIDEVINE_DEVICE_FILE or both private raw-device file variables for full-media downloads (subtitle indexing requires none)")

// explicitWidevineEnvSet reports whether the operator configured a device via
// environment variables. It does not search the working tree or well-known folders.
func explicitWidevineEnvSet() bool {
	wvd := strings.TrimSpace(os.Getenv("CRUNCHYROLL_WIDEVINE_DEVICE_FILE"))
	clientID := strings.TrimSpace(os.Getenv("CRUNCHYROLL_WIDEVINE_CLIENT_ID_FILE"))
	privateKey := strings.TrimSpace(os.Getenv("CRUNCHYROLL_WIDEVINE_PRIVATE_KEY_FILE"))
	return wvd != "" || clientID != "" || privateKey != ""
}

func getWidevineDevice() (*widevine.Device, error) {
	wvdPath := strings.TrimSpace(os.Getenv("CRUNCHYROLL_WIDEVINE_DEVICE_FILE"))
	clientIDPath := strings.TrimSpace(os.Getenv("CRUNCHYROLL_WIDEVINE_CLIENT_ID_FILE"))
	privateKeyPath := strings.TrimSpace(os.Getenv("CRUNCHYROLL_WIDEVINE_PRIVATE_KEY_FILE"))

	// Fall back to local files (cwd / next-to-exe / known project folders)
	// so full downloads work like the old crdl-windows.exe without env setup.
	if wvdPath == "" && clientIDPath == "" && privateKeyPath == "" {
		wvdPath, clientIDPath, privateKeyPath = resolveLocalWidevinePaths()
	}

	if wvdPath != "" {
		return loadWidevineFromWVD(wvdPath)
	}

	if clientIDPath == "" && privateKeyPath == "" {
		return nil, nil
	}
	if clientIDPath == "" || privateKeyPath == "" {
		return nil, fmt.Errorf("both CRUNCHYROLL_WIDEVINE_CLIENT_ID_FILE and CRUNCHYROLL_WIDEVINE_PRIVATE_KEY_FILE are required")
	}
	return loadWidevineFromRaw(clientIDPath, privateKeyPath)
}

func openPrivateRegularFile(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("credential path must be a regular file")
	}
	if !isPrivateCredentialMode(info.Mode()) {
		return nil, fmt.Errorf("credential file mode must be 0600")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !os.SameFile(info, opened) {
		file.Close()
		return nil, fmt.Errorf("credential file changed while opening")
	}
	return file, nil
}

func getLicense(psshData, contentId, videoToken string) error {
	device, err := getWidevineDevice()
	if err != nil {
		return err
	}
	if device == nil {
		return ErrNoAuthorizedWidevine
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
