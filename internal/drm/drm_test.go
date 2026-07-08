package drm

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/iyear/gowidevine"
)

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

func TestDiscoverWidevineDeviceConfigPrefersWVDFromDotEnv(t *testing.T) {
	clearWidevineEnv(t)
	chdirTemp(t)

	wvdPath := filepath.Join(t.TempDir(), "device.wvd")
	clientIDPath := filepath.Join(t.TempDir(), "client_id.bin")
	privateKeyPath := filepath.Join(t.TempDir(), "private_key.pem")
	writeFile(t, wvdPath, "wvd")
	writeFile(t, clientIDPath, "client")
	writeFile(t, privateKeyPath, "key")
	writeFile(t, widevineEnvFile, strings.Join([]string{
		widevineDevicePathEnv + "=" + wvdPath,
		widevineClientIDPathEnv + "=" + clientIDPath,
		widevinePrivateKeyPathEnv + "=" + privateKeyPath,
	}, "\n"))

	config, err := discoverWidevineDeviceConfig()
	if err != nil {
		t.Fatalf("discoverWidevineDeviceConfig() error = %v", err)
	}

	if config.wvdPath != wvdPath {
		t.Fatalf("wvdPath = %q, want %q", config.wvdPath, wvdPath)
	}
}

func TestDiscoverWidevineDeviceConfigEnvOverridesDotEnv(t *testing.T) {
	clearWidevineEnv(t)
	chdirTemp(t)

	dotEnvPath := filepath.Join(t.TempDir(), "dotenv.wvd")
	envPath := filepath.Join(t.TempDir(), "env.wvd")
	writeFile(t, dotEnvPath, "dotenv")
	writeFile(t, envPath, "env")
	writeFile(t, widevineEnvFile, widevineDevicePathEnv+"="+dotEnvPath)
	t.Setenv(widevineDevicePathEnv, envPath)

	config, err := discoverWidevineDeviceConfig()
	if err != nil {
		t.Fatalf("discoverWidevineDeviceConfig() error = %v", err)
	}
	if config.wvdPath != envPath {
		t.Fatalf("wvdPath = %q, want env override %q", config.wvdPath, envPath)
	}
}

func TestLoadWidevineDeviceDoesNotFallbackWhenWVDConfigured(t *testing.T) {
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
	clearWidevineEnv(t)
	chdirTemp(t)

	_, err := discoverWidevineDeviceConfig()
	if err == nil {
		t.Fatal("discoverWidevineDeviceConfig() error = nil, want missing config error")
	}
	for _, want := range []string{widevineDevicePathEnv, widevineClientIDPathEnv, widevinePrivateKeyPathEnv} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("discoverWidevineDeviceConfig() error = %q, want %s", err, want)
		}
	}
}

func resetWidevineDeviceCache(t *testing.T) {
	t.Helper()

	originalLoader := widevineDeviceLoader
	widevineDeviceOnce = sync.Once{}
	widevineDevice = nil
	widevineDeviceErr = nil

	t.Cleanup(func() {
		widevineDeviceOnce = sync.Once{}
		widevineDevice = nil
		widevineDeviceErr = nil
		widevineDeviceLoader = originalLoader
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
