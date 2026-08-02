package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreferencesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")
	p := Preferences{
		CookieFile:    `C:\secrets\etp_rt.txt`,
		OutputDir:     `./Downloads`,
		VideoQuality:  "max",
		AudioQuality:  "max",
		AudioLangs:    []string{"ja-JP"},
		SubtitleLangs: []string{},
		Mode:          "normal",
	}
	if err := SavePreferences(path, p); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.CookieFile != p.CookieFile || got.VideoQuality != "max" {
		t.Fatalf("%#v", got)
	}
	if got.AudioQuality != "max" || got.Mode != "normal" {
		t.Fatalf("%#v", got)
	}
	if len(got.AudioLangs) != 1 || got.AudioLangs[0] != "ja-JP" {
		t.Fatalf("audio langs: %#v", got.AudioLangs)
	}
}

func TestLoadPreferencesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	got, err := LoadPreferences(path)
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if got.URL != "" || got.CookieFile != "" || got.Mode != "" {
		t.Fatalf("want zero prefs, got %#v", got)
	}
}

func TestSavePreferencesCreatesParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "preferences.json")
	p := Preferences{URL: "https://www.crunchyroll.com/series/EXAMPLE", Mode: "advanced"}
	if err := SavePreferences(path, p); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != p.URL || got.Mode != "advanced" {
		t.Fatalf("%#v", got)
	}
}

func TestDefaultPreferencesPath(t *testing.T) {
	path, err := DefaultPreferencesPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.ToSlash(path), "crunchyroll-downloader/preferences.json") &&
		!strings.Contains(path, filepath.Join("crunchyroll-downloader", "preferences.json")) {
		t.Fatalf("unexpected path: %s", path)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("want absolute path, got %s", path)
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cfg, "crunchyroll-downloader", "preferences.json")
	if path != want {
		t.Fatalf("got %q want %q", path, want)
	}
}
