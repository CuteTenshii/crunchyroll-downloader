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
		Locale:        "pt-BR",
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
	if got.Locale != "pt-BR" {
		t.Fatalf("locale: got %q", got.Locale)
	}
	// Legacy CookieFile should migrate into a default cookie profile on save/load.
	if len(got.CookieProfiles) != 1 {
		t.Fatalf("want 1 migrated profile, got %#v", got.CookieProfiles)
	}
	if got.ActiveProfileID == "" || got.CookieProfiles[0].CookieFile != p.CookieFile {
		t.Fatalf("migrated profile: %#v active=%q", got.CookieProfiles, got.ActiveProfileID)
	}
}

func TestCookieProfileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")

	p := Preferences{Locale: "en-US", Mode: "normal"}
	created, err := p.UpsertCookieProfile(CookieProfile{
		Name:       "Primary",
		CookieFile: `/secrets/etp_a.txt`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("expected generated id")
	}
	if p.ActiveProfileID != created.ID || p.CookieFile != `/secrets/etp_a.txt` {
		t.Fatalf("active not set: %#v", p)
	}

	second, err := p.UpsertCookieProfile(CookieProfile{
		Name:       "Secondary",
		CookieFile: `/secrets/etp_b.txt`,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Second insert should not steal active unless explicitly switched.
	if p.ActiveProfileID != created.ID {
		t.Fatalf("active changed unexpectedly: %q", p.ActiveProfileID)
	}

	if err := p.SwitchCookieProfile(second.ID); err != nil {
		t.Fatal(err)
	}
	if p.ActiveProfileID != second.ID || p.CookieFile != `/secrets/etp_b.txt` {
		t.Fatalf("switch failed: %#v", p)
	}

	// Update by id.
	updated, err := p.UpsertCookieProfile(CookieProfile{
		ID:         second.ID,
		Name:       "Secondary Renamed",
		CookieFile: `/secrets/etp_b2.txt`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Secondary Renamed" || p.CookieFile != `/secrets/etp_b2.txt` {
		t.Fatalf("update: %#v cookie=%q", updated, p.CookieFile)
	}

	if err := SavePreferences(path, p); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.CookieProfiles) != 2 {
		t.Fatalf("want 2 profiles, got %#v", got.CookieProfiles)
	}
	if got.ActiveProfileID != second.ID || got.CookieFile != `/secrets/etp_b2.txt` {
		t.Fatalf("reload active: id=%q file=%q", got.ActiveProfileID, got.CookieFile)
	}
	if got.Locale != "en-US" {
		t.Fatalf("locale lost: %q", got.Locale)
	}

	if err := got.DeleteCookieProfile(second.ID); err != nil {
		t.Fatal(err)
	}
	if got.ActiveProfileID != created.ID || got.CookieFile != `/secrets/etp_a.txt` {
		t.Fatalf("delete active should fall back: %#v", got)
	}
	if len(got.ListCookieProfiles()) != 1 {
		t.Fatalf("list after delete: %#v", got.ListCookieProfiles())
	}
}

func TestSwitchCookieProfileMissing(t *testing.T) {
	p := Preferences{}
	if err := p.SwitchCookieProfile("nope"); err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestEnsureCookieProfilesSyncsCookieFile(t *testing.T) {
	p := Preferences{
		CookieProfiles: []CookieProfile{
			{ID: "a", Name: "A", CookieFile: "/a.txt"},
			{ID: "b", Name: "B", CookieFile: "/b.txt"},
		},
		ActiveProfileID: "b",
		CookieFile:      "/stale.txt",
	}
	p.EnsureCookieProfiles()
	if p.CookieFile != "/b.txt" {
		t.Fatalf("want CookieFile from active profile, got %q", p.CookieFile)
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
