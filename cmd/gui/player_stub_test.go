//go:build !windows

package main

import "testing"

func TestStubNewMpvHost(t *testing.T) {
	h, err := newMpvHost()
	if err != nil {
		t.Fatalf("newMpvHost: %v", err)
	}
	if err := h.Attach(0); !libmpvError(err) {
		t.Fatalf("Attach: %v", err)
	}
	if err := h.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	a := NewApp()
	if err := a.StartPlay(PlayRequest{}); !libmpvError(err) {
		t.Fatalf("StartPlay: %v", err)
	}
}
