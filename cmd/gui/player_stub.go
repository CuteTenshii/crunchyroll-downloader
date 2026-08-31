//go:build !windows

package main

func newMpvHost() (MpvHost, error) {
	return newMissingMpvHost(), nil
}

func (a *App) ensurePlaySurfaceLocked() (uintptr, error) {
	return 0, nil
}

func (a *App) movePlaySurfaceLocked() error {
	return nil
}

func (a *App) destroyPlaySurfaceLocked() error {
	a.playChild = 0
	a.playParent = 0
	return nil
}
