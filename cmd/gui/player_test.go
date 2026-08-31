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
	if a.playGen != 1 {
		t.Fatalf("playGen=%d want 1", a.playGen)
	}
	if err := a.StopPlay(); err != nil {
		t.Fatalf("StopPlay: %v", err)
	}
}

type countingHost struct {
	destroys int
}

func (c *countingHost) Attach(uintptr) error       { return nil }
func (c *countingHost) LoadFile(string) error      { return nil }
func (c *countingHost) Pause(bool) error           { return nil }
func (c *countingHost) Seek(float64) error         { return nil }
func (c *countingHost) Position() (float64, error) { return 0, nil }
func (c *countingHost) Duration() (float64, error) { return 0, nil }
func (c *countingHost) SetVolume(int) error        { return nil }
func (c *countingHost) Destroy() error {
	c.destroys++
	return nil
}

func TestStopPlayMatchingGenDestroysHost(t *testing.T) {
	h := &countingHost{}
	a := NewApp()
	a.playHost = h
	a.playGen = 2
	if err := a.StopPlay(); err != nil {
		t.Fatalf("StopPlay: %v", err)
	}
	if h.destroys != 1 {
		t.Fatalf("destroys=%d want 1", h.destroys)
	}
	if a.playHost != nil {
		t.Fatal("host still set")
	}
}

func TestStopPlayStaleGenIsNoop(t *testing.T) {
	old := &countingHost{}
	a := NewApp()
	a.playHost = old
	a.playGen = 4

	a.playMu.Lock()
	gen := a.playGen
	a.playMu.Unlock()

	newer := &countingHost{}
	a.playMu.Lock()
	a.playGen++
	a.playHost = newer
	a.playMu.Unlock()

	if err := a.clearPlayIfGen(gen); err != nil {
		t.Fatalf("clearPlayIfGen: %v", err)
	}
	if old.destroys != 0 || newer.destroys != 0 {
		t.Fatalf("stale stop destroyed hosts old=%d new=%d", old.destroys, newer.destroys)
	}
	if a.playHost != newer {
		t.Fatal("new session host was cleared")
	}
}

func TestStartPlayBumpsGenBeforeFactoryError(t *testing.T) {
	orig := mpvHostFactory
	mpvHostFactory = func() (MpvHost, error) { return nil, missingPlayerErr() }
	defer func() { mpvHostFactory = orig }()

	a := NewApp()
	a.playGen = 7
	stale := a.playGen
	if err := a.StartPlay(""); !libmpvError(err) {
		t.Fatalf("StartPlay: %v", err)
	}
	if a.playGen <= stale {
		t.Fatalf("playGen=%d not greater than stale %d", a.playGen, stale)
	}
	if err := a.clearPlayIfGen(stale); err != nil {
		t.Fatalf("stale clear: %v", err)
	}
	if a.playGen != stale+1 {
		t.Fatalf("playGen=%d want %d", a.playGen, stale+1)
	}
}
