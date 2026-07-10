package drm

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/iyear/gowidevine"
	"github.com/unki2aut/go-mpd"
)

func TestGetPsshWithProtection(t *testing.T) {
	data, err := os.ReadFile("../media/testdata/mpd/with-content-protection.mpd")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	m := new(mpd.MPD)
	if err := m.Decode(data); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	pssh := GetPssh(m)
	if pssh == nil {
		t.Fatal("GetPssh(with-content-protection) = nil, want non-nil PSSH")
	}
	if *pssh == "" {
		t.Fatal("GetPssh() returned empty PSSH string")
	}
}

func TestGetPsshWithoutProtection(t *testing.T) {
	data, err := os.ReadFile("../media/testdata/mpd/no-content-protection.mpd")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	m := new(mpd.MPD)
	if err := m.Decode(data); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	pssh := GetPssh(m)
	if pssh != nil {
		t.Fatal("GetPssh(no-content-protection) = non-nil, want nil")
	}
}

func TestGetWidevineDeviceCachesLoader(t *testing.T) {
	resetWidevineDeviceCache(t)

	device := &widevine.Device{}
	loads := 0
	widevineDeviceLoader = func() (*widevine.Device, error) {
		loads++
		if loads > 1 {
			return nil, errors.New("loader called more than once")
		}

		return device, nil
	}

	first, err := GetWidevineDevice()
	if err != nil {
		t.Fatalf("first GetWidevineDevice() error = %v", err)
	}
	second, err := GetWidevineDevice()
	if err != nil {
		t.Fatalf("second GetWidevineDevice() error = %v", err)
	}

	if first != device || second != device {
		t.Fatalf("GetWidevineDevice() did not return cached device")
	}
	if loads != 1 {
		t.Fatalf("loader calls = %d, want 1", loads)
	}
}

func TestDiscoverWidevineDeviceConfigPrefersWVD(t *testing.T) {
	resetWidevineDeviceCache(t)
	clearWidevineEnv(t)
	chdirTemp(t)

	wvdPath := filepath.Join(t.TempDir(), "device.wvd")
	clientIDPath := filepath.Join(t.TempDir(), "client_id.bin")
	privateKeyPath := filepath.Join(t.TempDir(), "private_key.pem")
	writeFile(t, wvdPath, "wvd")
	writeFile(t, clientIDPath, "client")
	writeFile(t, privateKeyPath, "key")
	t.Setenv(widevineDevicePathEnv, wvdPath)

	config, err := discoverWidevineDeviceConfig()
	if err != nil {
		t.Fatalf("discoverWidevineDeviceConfig() error = %v", err)
	}

	if config.wvdPath != wvdPath {
		t.Fatalf("wvdPath = %q, want %q", config.wvdPath, wvdPath)
	}
}

func TestDiscoverWidevineDeviceConfigEnvOverrideWithSetWidevinePath(t *testing.T) {
	resetWidevineDeviceCache(t)
	clearWidevineEnv(t)
	chdirTemp(t)

	rawDir := t.TempDir()
	writeFile(t, filepath.Join(rawDir, "client_id.bin"), "client")
	writeFile(t, filepath.Join(rawDir, "private_key.pem"), "key")
	wvdPath := filepath.Join(t.TempDir(), "fallback.wvd")
	writeFile(t, wvdPath, "fallback")

	// Set env var to a valid WVD path, but set explicit path via SetWidevinePath
	t.Setenv(widevineDevicePathEnv, wvdPath)
	SetWidevinePath(rawDir)

	config, err := discoverWidevineDeviceConfig()
	if err != nil {
		t.Fatalf("discoverWidevineDeviceConfig() error = %v", err)
	}
	if config.wvdPath != "" {
		t.Fatalf("wvdPath = %q, want empty (should use raw dir)", config.wvdPath)
	}
	if config.clientIDPath != filepath.Join(rawDir, "client_id.bin") {
		t.Fatalf("clientIDPath = %q, want %q", config.clientIDPath, filepath.Join(rawDir, "client_id.bin"))
	}
	if config.privateKeyPath != filepath.Join(rawDir, "private_key.pem") {
		t.Fatalf("privateKeyPath = %q, want %q", config.privateKeyPath, filepath.Join(rawDir, "private_key.pem"))
	}
}

func TestLoadWidevineDeviceDoesNotFallbackWhenWVDConfigured(t *testing.T) {
	resetWidevineDeviceCache(t)
	clearWidevineEnv(t)
	chdirTemp(t)

	clientIDPath := filepath.Join(t.TempDir(), "client_id.bin")
	privateKeyPath := filepath.Join(t.TempDir(), "private_key.pem")
	writeFile(t, clientIDPath, "client")
	writeFile(t, privateKeyPath, "key")
	t.Setenv(widevineDevicePathEnv, filepath.Join(t.TempDir(), "missing.wvd"))
	t.Setenv(widevineClientIDPathEnv, clientIDPath)
	t.Setenv(widevinePrivateKeyPathEnv, privateKeyPath)

	_, err := loadWidevineDevice()
	if err == nil {
		t.Fatal("loadWidevineDevice() error = nil, want WVD open error")
	}
	if !strings.Contains(err.Error(), widevineDevicePathEnv) {
		t.Fatalf("loadWidevineDevice() error = %q, want %s", err, widevineDevicePathEnv)
	}
}

func TestDiscoverWidevineDeviceConfigRequiresRawPairTogether(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "client id without private key",
			env: map[string]string{
				widevineClientIDPathEnv: "/tmp/client_id.bin",
			},
		},
		{
			name: "private key without client id",
			env: map[string]string{
				widevinePrivateKeyPathEnv: "/tmp/private_key.pem",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetWidevineDeviceCache(t)
			clearWidevineEnv(t)
			chdirTemp(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			_, err := discoverWidevineDeviceConfig()
			if err == nil {
				t.Fatal("discoverWidevineDeviceConfig() error = nil, want incomplete raw pair error")
			}
			if !strings.Contains(err.Error(), widevineClientIDPathEnv) || !strings.Contains(err.Error(), widevinePrivateKeyPathEnv) {
				t.Fatalf("discoverWidevineDeviceConfig() error = %q, want both raw pair env names", err)
			}
		})
	}
}

func TestDiscoverWidevineDeviceConfigAcceptsRawPair(t *testing.T) {
	resetWidevineDeviceCache(t)
	clearWidevineEnv(t)
	chdirTemp(t)

	clientIDPath := filepath.Join(t.TempDir(), "client_id.bin")
	privateKeyPath := filepath.Join(t.TempDir(), "private_key.pem")
	t.Setenv(widevineClientIDPathEnv, clientIDPath)
	t.Setenv(widevinePrivateKeyPathEnv, privateKeyPath)

	config, err := discoverWidevineDeviceConfig()
	if err != nil {
		t.Fatalf("discoverWidevineDeviceConfig() error = %v", err)
	}
	if config.clientIDPath != clientIDPath || config.privateKeyPath != privateKeyPath {
		t.Fatalf("raw pair = (%q, %q), want (%q, %q)", config.clientIDPath, config.privateKeyPath, clientIDPath, privateKeyPath)
	}
}

func TestDiscoverWidevineDeviceConfigMissingDeviceError(t *testing.T) {
	resetWidevineDeviceCache(t)
	clearWidevineEnv(t)
	chdirTemp(t)

	_, err := discoverWidevineDeviceConfig()
	if err == nil {
		t.Fatal("discoverWidevineDeviceConfig() error = nil, want missing config error")
	}
	for _, want := range []string{"--widevine-device", "WIDEVINE_DEVICE_PATH", "WIDEVINE_CLIENT_ID_PATH", "WIDEVINE_PRIVATE_KEY_PATH"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("discoverWidevineDeviceConfig() error = %q, want %q", err, want)
		}
	}
}

func TestDetectDevicePathAcceptsWVD(t *testing.T) {
	wvdPath := filepath.Join(t.TempDir(), "device.wvd")
	writeFile(t, wvdPath, "some-wvd-content")

	format, err := DetectDevicePath(wvdPath)
	if err != nil {
		t.Fatalf("DetectDevicePath() error = %v", err)
	}
	if format != FormatWVD {
		t.Fatalf("format = %d, want FormatWVD (%d)", format, FormatWVD)
	}
}

func TestDetectDevicePathAcceptsRawDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "client_id.bin"), "client")
	writeFile(t, filepath.Join(dir, "private_key.pem"), "key")

	format, err := DetectDevicePath(dir)
	if err != nil {
		t.Fatalf("DetectDevicePath() error = %v", err)
	}
	if format != FormatRawDir {
		t.Fatalf("format = %d, want FormatRawDir (%d)", format, FormatRawDir)
	}
}

func TestDetectDevicePathRejectsMissingPath(t *testing.T) {
	_, err := DetectDevicePath("/nonexistent/path/device.wvd")
	if err == nil {
		t.Fatal("DetectDevicePath() error = nil, want error for missing path")
	}
	if !strings.Contains(err.Error(), "accessing Widevine device path") {
		t.Fatalf("DetectDevicePath() error = %q, want 'accessing Widevine device path'", err)
	}
}

func TestDetectDevicePathRejectsPlainFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "notes.txt"), "hello")

	_, err := DetectDevicePath(filepath.Join(dir, "notes.txt"))
	if err == nil {
		t.Fatal("DetectDevicePath() error = nil, want error for unrecognized file")
	}
	if !strings.Contains(err.Error(), "unrecognized file format") {
		t.Fatalf("DetectDevicePath() error = %q, want 'unrecognized file format'", err)
	}
}

func TestDetectDevicePathRejectsEmptyDir(t *testing.T) {
	dir := t.TempDir()

	_, err := DetectDevicePath(dir)
	if err == nil {
		t.Fatal("DetectDevicePath() error = nil, want error for empty directory")
	}
	if !strings.Contains(err.Error(), "does not contain client_id.bin") {
		t.Fatalf("DetectDevicePath() error = %q, want 'does not contain client_id.bin'", err)
	}
}

func TestSetWidevinePathOverridesEnvVars(t *testing.T) {
	resetWidevineDeviceCache(t)
	clearWidevineEnv(t)
	chdirTemp(t)

	// Create a raw dir to use via SetWidevinePath
	rawDir := t.TempDir()
	writeFile(t, filepath.Join(rawDir, "client_id.bin"), "client")
	writeFile(t, filepath.Join(rawDir, "private_key.pem"), "key")

	// Create a WVD file to use as env var (should be ignored)
	wvdPath := filepath.Join(t.TempDir(), "ignored.wvd")
	writeFile(t, wvdPath, "ignored")
	t.Setenv(widevineDevicePathEnv, wvdPath)

	// Set explicit path — should take priority
	SetWidevinePath(rawDir)

	config, err := discoverWidevineDeviceConfig()
	if err != nil {
		t.Fatalf("discoverWidevineDeviceConfig() error = %v", err)
	}
	if config.clientIDPath != filepath.Join(rawDir, "client_id.bin") {
		t.Fatalf("clientIDPath = %q, want %q", config.clientIDPath, filepath.Join(rawDir, "client_id.bin"))
	}
	if config.privateKeyPath != filepath.Join(rawDir, "private_key.pem") {
		t.Fatalf("privateKeyPath = %q, want %q", config.privateKeyPath, filepath.Join(rawDir, "private_key.pem"))
	}
}

func TestSetWidevinePathWVDOverridesEnvVars(t *testing.T) {
	resetWidevineDeviceCache(t)
	clearWidevineEnv(t)
	chdirTemp(t)

	wvdPath := filepath.Join(t.TempDir(), "device.wvd")
	writeFile(t, wvdPath, "wvd-content")
	envPath := filepath.Join(t.TempDir(), "env.wvd")
	writeFile(t, envPath, "env-content")
	t.Setenv(widevineDevicePathEnv, envPath)

	SetWidevinePath(wvdPath)

	config, err := discoverWidevineDeviceConfig()
	if err != nil {
		t.Fatalf("discoverWidevineDeviceConfig() error = %v", err)
	}
	if config.wvdPath != wvdPath {
		t.Fatalf("wvdPath = %q, want %q", config.wvdPath, wvdPath)
	}
}

func resetWidevineDeviceCache(t *testing.T) {
	t.Helper()

	originalLoader := widevineDeviceLoader
	originalPath := widevineDevicePath
	widevineDeviceOnce = sync.Once{}
	widevineDevice = nil
	widevineDeviceErr = nil
	widevineDevicePath = ""

	t.Cleanup(func() {
		widevineDeviceOnce = sync.Once{}
		widevineDevice = nil
		widevineDeviceErr = nil
		widevineDeviceLoader = originalLoader
		widevineDevicePath = originalPath
	})
}

func clearWidevineEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{widevineDevicePathEnv, widevineClientIDPathEnv, widevinePrivateKeyPathEnv} {
		oldValue, hadValue := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("Unsetenv(%s): %v", key, err)
		}
		t.Cleanup(func() {
			if hadValue {
				_ = os.Setenv(key, oldValue)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func chdirTemp(t *testing.T) {
	t.Helper()

	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(%s): %v", tempDir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalCWD)
	})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}
