package engine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Preferences holds GUI/CLI user settings persisted as JSON under the
// user config directory. Secrets are stored only as filesystem paths —
// never raw cookie or private-key material.
type Preferences struct {
	URL            string   `json:"url"`
	CookieFile     string   `json:"cookieFile"`
	OutputDir      string   `json:"outputDir"`
	Mode           string   `json:"mode"` // normal | advanced
	AudioLangs     []string `json:"audioLangs"`
	SubtitleLangs  []string `json:"subtitleLangs"`
	CaptionLangs   []string `json:"captionLangs"`
	VideoQuality   string   `json:"videoQuality"` // "max" or "1080p"
	AudioQuality   string   `json:"audioQuality"` // "max" or "192k"
	LastSeason     int      `json:"lastSeason"`
	WVDPath        string   `json:"wvdPath"`
	ClientIDPath   string   `json:"clientIdPath"`
	PrivateKeyPath string   `json:"privateKeyPath"`
	StrictLanguages bool    `json:"strictLanguages"`

	// Optional advanced numerics/toggles.
	Playback4294Retries    int  `json:"playback4294Retries,omitempty"`
	Playback4294BackoffSec int  `json:"playback4294BackoffSec,omitempty"`
	IndexWindow            int  `json:"indexWindow,omitempty"`
	IndexCircuitLimit      int  `json:"indexCircuitLimit,omitempty"`
	DebugManifest          bool `json:"debugManifest,omitempty"`
	ProbeEveryEpisode      bool `json:"probeEveryEpisode,omitempty"`
}

// DefaultPreferencesPath returns
// os.UserConfigDir()/crunchyroll-downloader/preferences.json.
func DefaultPreferencesPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "crunchyroll-downloader", "preferences.json"), nil
}

// LoadPreferences reads preferences from path. A missing file returns
// zero Preferences and a nil error so first-run has empty defaults.
func LoadPreferences(path string) (Preferences, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Preferences{}, nil
		}
		return Preferences{}, err
	}
	var p Preferences
	if err := json.Unmarshal(data, &p); err != nil {
		return Preferences{}, err
	}
	return p, nil
}

// SavePreferences writes p to path as indented JSON. The parent directory
// is created if needed. Write is atomic where the OS allows: data is
// written to a sibling temp file then renamed into place. File mode is
// 0600 when the platform supports permission bits.
func SavePreferences(path string, p Preferences) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, "preferences-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if anything fails before rename.
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		// Windows may not support Unix permission bits; ignore.
		_ = err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// On Windows, Rename fails if the destination already exists.
	if err := os.Rename(tmpName, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		if err2 := os.Rename(tmpName, path); err2 != nil {
			return err2
		}
	}
	success = true
	return nil
}
