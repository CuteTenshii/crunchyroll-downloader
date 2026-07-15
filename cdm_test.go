package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/iyear/gowidevine"
)

// TestLoadWVD verifies that the .wvd file in the repo folder parses correctly
// through the gowidevine library — that a CDM can be constructed and that the
// embedded RSA private key and DRM certificate are valid. This does NOT contact
// any license server; it only validates the file is structurally and
// cryptographically loadable.
func TestLoadWVD(t *testing.T) {
	// getWidevineDevice reads from the current working directory, so make sure
	// we're running from the repo root where the .wvd lives.
	if _, err := os.Stat("1668035862.wvd"); err != nil {
		t.Skipf("no .wvd file in working directory: %v", err)
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
