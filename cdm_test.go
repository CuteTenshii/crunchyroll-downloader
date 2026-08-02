package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/iyear/gowidevine"
)

func TestOpenPrivateRegularFileRejectsBroadModeAndSymlink(t *testing.T) {
	dir := t.TempDir()
	credential := filepath.Join(dir, "device.wvd")
	if err := os.WriteFile(credential, []byte("fixture-not-a-real-device"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := openPrivateRegularFile(credential)
	if err != nil {
		t.Fatalf("mode-0600 regular file rejected: %v", err)
	}
	file.Close()
	if runtime.GOOS != "windows" {
		// Windows does not honor Unix permission bits; skip the 0644 rejection.
		if err := os.Chmod(credential, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := openPrivateRegularFile(credential); err == nil {
			t.Fatal("mode-0644 credential was accepted")
		}
	}
	link := filepath.Join(dir, "device-link.wvd")
	if err := os.Symlink(credential, link); err != nil {
		t.Fatal(err)
	}
	if _, err := openPrivateRegularFile(link); err == nil {
		t.Fatal("symlink credential was accepted")
	}
}

func TestGetWidevineDeviceRequiresPairedRawPaths(t *testing.T) {
	t.Setenv("CRUNCHYROLL_WIDEVINE_DEVICE_FILE", "")
	t.Setenv("CRUNCHYROLL_WIDEVINE_CLIENT_ID_FILE", filepath.Join(t.TempDir(), "client-id"))
	t.Setenv("CRUNCHYROLL_WIDEVINE_PRIVATE_KEY_FILE", "")
	if _, err := getWidevineDevice(); err == nil {
		t.Fatal("unpaired raw credential path was accepted")
	}
}

// TestLoadWVD verifies that the .wvd file in the repo folder parses correctly
// through the gowidevine library — that a CDM can be constructed and that the
// embedded RSA private key and DRM certificate are valid. This does NOT contact
// any license server; it only validates the file is structurally and
// cryptographically loadable.
func TestLoadWVD(t *testing.T) {
	if os.Getenv("CRUNCHYROLL_WIDEVINE_DEVICE_FILE") == "" {
		t.Skip("CRUNCHYROLL_WIDEVINE_DEVICE_FILE is not configured")
	}

	device, err := getWidevineDevice()
	if err != nil {
		t.Fatalf("getWidevineDevice returned an error: %v", err)
	}
	if device == nil {
		t.Fatal("getWidevineDevice returned nil — no .wvd or client_id/private_key found")
	}

	// Construct a CDM from the device. This exercises the key parsing path.
	cdm := widevine.NewCDM(device)
	if cdm == nil {
		t.Fatal("widevine.NewCDM returned nil")
	}

	// Validate the embedded credentials parsed correctly.
	clientID := device.ClientID()
	if clientID == nil {
		t.Fatal("ClientID() returned nil — client identification blob did not parse")
	}

	cert := device.DrmCertificate()
	if cert == nil {
		t.Fatal("DrmCertificate() returned nil — DRM certificate did not parse")
	}

	privKey := device.PrivateKey()
	if privKey == nil {
		t.Fatal("PrivateKey() returned nil — RSA private key did not parse")
	}

	// A valid RSA private key must have a non-nil public key and positive N.
	if privKey.PublicKey.N == nil || privKey.PublicKey.N.Sign() <= 0 {
		t.Fatal("parsed RSA private key has invalid modulus")
	}

	fmt.Println("✅ WVD file loaded successfully — CDM constructed.")
	fmt.Printf("   DRM cert system ID: %d\n", cert.SystemId)
	fmt.Printf("   RSA key bits:       %d\n", privKey.PublicKey.N.BitLen())
}
