//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestOpenLibmpvMissingFile(t *testing.T) {
	dll, err := openLibmpv("definitely-not-a-dll")
	if dll != nil {
		t.Fatal("expected nil DLL")
	}
	if err == nil || !strings.Contains(err.Error(), "player library missing") {
		t.Fatalf("openLibmpv: %v", err)
	}
	if !libmpvError(err) {
		t.Fatalf("libmpvError(%v) = false", err)
	}
}
