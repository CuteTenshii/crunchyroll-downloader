package main

import (
	"strings"
	"testing"
)

func TestMissingPlayerErr(t *testing.T) {
	err := missingPlayerErr()
	if err == nil {
		t.Fatal("expected error")
	}
	if !libmpvError(err) {
		t.Fatalf("libmpvError(%v) = false", err)
	}
	if !strings.Contains(err.Error(), "player library missing") {
		t.Fatalf("error %q missing required string", err)
	}
	if libmpvError(nil) {
		t.Fatal("libmpvError(nil) = true")
	}
}

func TestMissingMpvHostMethods(t *testing.T) {
	h := newMissingMpvHost()
	if err := h.Attach(0); !libmpvError(err) {
		t.Fatalf("Attach: %v", err)
	}
	if err := h.LoadFile("x"); !libmpvError(err) {
		t.Fatalf("LoadFile: %v", err)
	}
	if err := h.Pause(true); !libmpvError(err) {
		t.Fatalf("Pause: %v", err)
	}
	if err := h.Seek(1); !libmpvError(err) {
		t.Fatalf("Seek: %v", err)
	}
	if _, err := h.Position(); !libmpvError(err) {
		t.Fatalf("Position: %v", err)
	}
	if _, err := h.Duration(); !libmpvError(err) {
		t.Fatalf("Duration: %v", err)
	}
	if err := h.SetVolume(50); !libmpvError(err) {
		t.Fatalf("SetVolume: %v", err)
	}
	if err := h.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}

func TestStartPlayFactoryError(t *testing.T) {
	orig := mpvHostFactory
	mpvHostFactory = func() (MpvHost, error) { return nil, missingPlayerErr() }
	defer func() { mpvHostFactory = orig }()

	a := NewApp()
	err := a.StartPlay("")
	if !libmpvError(err) {
		t.Fatalf("StartPlay: %v", err)
	}
	if err := a.StopPlay(); err != nil {
		t.Fatalf("StopPlay: %v", err)
	}
}
