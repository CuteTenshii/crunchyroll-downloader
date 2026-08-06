package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// CookieProfile is a named path to an etp_rt cookie file. Only filesystem
// paths are stored — never raw cookie values.
type CookieProfile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CookieFile string `json:"cookieFile"`
}

// CRProfile is a Crunchyroll multiprofile entry when the accounts API exposes
// them. Best-effort only; missing APIs yield an empty list.
type CRProfile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsSelected bool   `json:"isSelected,omitempty"`
}

// Preferences holds GUI/CLI user settings persisted as JSON under the
// user config directory. Secrets are stored only as filesystem paths —
// never raw cookie or private-key material.
type Preferences struct {
	URL        string `json:"url"`
	CookieFile string `json:"cookieFile"`
	OutputDir  string `json:"outputDir"`
	Mode       string `json:"mode"` // normal | advanced
	// Locale is the Discover/CMS locale (e.g. pt-BR). Empty means engine default pt-BR.
	Locale         string   `json:"locale,omitempty"`
	AudioLangs     []string `json:"audioLangs"`
	SubtitleLangs  []string `json:"subtitleLangs"`
	CaptionLangs   []string `json:"captionLangs"`
	VideoQuality   string   `json:"videoQuality"` // "max" or "1080p"
	AudioQuality   string   `json:"audioQuality"` // "max" or "192k"
	LastSeason     int      `json:"lastSeason"`
	WVDPath        string   `json:"wvdPath"`
	ClientIDPath   string   `json:"clientIdPath"`
	PrivateKeyPath  string   `json:"privateKeyPath"`
	StrictLanguages bool     `json:"strictLanguages"`

	// CookieProfiles lists named cookie-file accounts for the GUI.
	// CookieFile remains the active path used by download/inspect.
	CookieProfiles    []CookieProfile `json:"cookieProfiles,omitempty"`
	ActiveProfileID   string          `json:"activeProfileId,omitempty"`
	ActiveCRProfileID string          `json:"activeCrProfileId,omitempty"`

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
// Loaded prefs are migrated so a legacy CookieFile becomes a default
// cookie profile when needed.
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
	p.EnsureCookieProfiles()
	return p, nil
}

// SavePreferences writes p to path as indented JSON. The parent directory
// is created if needed. Write is atomic where the OS allows: data is
// written to a sibling temp file then renamed into place. File mode is
// 0600 when the platform supports permission bits.
func SavePreferences(path string, p Preferences) error {
	p.EnsureCookieProfiles()
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

// EnsureCookieProfiles migrates legacy CookieFile-only prefs into a profile
// list and keeps CookieFile aligned with the active profile.
func (p *Preferences) EnsureCookieProfiles() {
	if p == nil {
		return
	}

	// Drop empty-id junk entries.
	cleaned := p.CookieProfiles[:0]
	for _, prof := range p.CookieProfiles {
		if strings.TrimSpace(prof.ID) == "" {
			continue
		}
		prof.ID = strings.TrimSpace(prof.ID)
		prof.Name = strings.TrimSpace(prof.Name)
		prof.CookieFile = strings.TrimSpace(prof.CookieFile)
		cleaned = append(cleaned, prof)
	}
	p.CookieProfiles = cleaned
	p.ActiveProfileID = strings.TrimSpace(p.ActiveProfileID)
	p.ActiveCRProfileID = strings.TrimSpace(p.ActiveCRProfileID)
	p.CookieFile = strings.TrimSpace(p.CookieFile)

	// Legacy: CookieFile set but no profiles → synthesize a default profile.
	if len(p.CookieProfiles) == 0 && p.CookieFile != "" {
		id := uuid.NewString()
		p.CookieProfiles = []CookieProfile{{
			ID:         id,
			Name:       "Default",
			CookieFile: p.CookieFile,
		}}
		p.ActiveProfileID = id
		return
	}

	if len(p.CookieProfiles) == 0 {
		p.ActiveProfileID = ""
		return
	}

	// Resolve active profile.
	if idx := p.indexOfProfile(p.ActiveProfileID); idx >= 0 {
		p.CookieFile = p.CookieProfiles[idx].CookieFile
		return
	}

	// Active id missing: prefer a profile whose cookie path matches CookieFile.
	if p.CookieFile != "" {
		for i, prof := range p.CookieProfiles {
			if prof.CookieFile == p.CookieFile {
				p.ActiveProfileID = p.CookieProfiles[i].ID
				return
			}
		}
	}

	// Fall back to the first profile.
	p.ActiveProfileID = p.CookieProfiles[0].ID
	p.CookieFile = p.CookieProfiles[0].CookieFile
}

func (p *Preferences) indexOfProfile(id string) int {
	id = strings.TrimSpace(id)
	if id == "" {
		return -1
	}
	for i, prof := range p.CookieProfiles {
		if prof.ID == id {
			return i
		}
	}
	return -1
}

// ListCookieProfiles returns a copy of configured cookie profiles after migration.
func (p Preferences) ListCookieProfiles() []CookieProfile {
	p.EnsureCookieProfiles()
	if len(p.CookieProfiles) == 0 {
		return []CookieProfile{}
	}
	out := make([]CookieProfile, len(p.CookieProfiles))
	copy(out, p.CookieProfiles)
	return out
}

// UpsertCookieProfile creates or updates a cookie profile by id. An empty id
// generates a new one. CookieFile on Preferences is updated when the upserted
// profile is (or becomes) active.
func (p *Preferences) UpsertCookieProfile(in CookieProfile) (CookieProfile, error) {
	if p == nil {
		return CookieProfile{}, fmt.Errorf("preferences is nil")
	}
	p.EnsureCookieProfiles()

	in.ID = strings.TrimSpace(in.ID)
	in.Name = strings.TrimSpace(in.Name)
	in.CookieFile = strings.TrimSpace(in.CookieFile)
	if in.CookieFile == "" {
		return CookieProfile{}, fmt.Errorf("cookie file path is required")
	}
	if in.Name == "" {
		in.Name = "Profile"
	}
	if in.ID == "" {
		in.ID = uuid.NewString()
	}

	if idx := p.indexOfProfile(in.ID); idx >= 0 {
		p.CookieProfiles[idx] = in
	} else {
		p.CookieProfiles = append(p.CookieProfiles, in)
	}

	// First profile or active match → keep CookieFile in sync.
	if p.ActiveProfileID == "" || p.ActiveProfileID == in.ID {
		p.ActiveProfileID = in.ID
		p.CookieFile = in.CookieFile
	}
	return in, nil
}

// DeleteCookieProfile removes a profile by id. If the active profile is
// removed, the first remaining profile becomes active (or CookieFile clears).
func (p *Preferences) DeleteCookieProfile(id string) error {
	if p == nil {
		return fmt.Errorf("preferences is nil")
	}
	p.EnsureCookieProfiles()
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("profile id is required")
	}
	idx := p.indexOfProfile(id)
	if idx < 0 {
		return fmt.Errorf("cookie profile %q not found", id)
	}
	wasActive := p.ActiveProfileID == id
	p.CookieProfiles = append(p.CookieProfiles[:idx], p.CookieProfiles[idx+1:]...)
	if wasActive {
		p.ActiveCRProfileID = ""
		if len(p.CookieProfiles) == 0 {
			p.ActiveProfileID = ""
			p.CookieFile = ""
		} else {
			p.ActiveProfileID = p.CookieProfiles[0].ID
			p.CookieFile = p.CookieProfiles[0].CookieFile
		}
	}
	return nil
}

// SwitchCookieProfile sets the active cookie profile and aligns CookieFile.
// Callers should re-authenticate after a successful switch. ActiveCRProfileID
// is cleared because multiprofile is account-scoped.
func (p *Preferences) SwitchCookieProfile(id string) error {
	if p == nil {
		return fmt.Errorf("preferences is nil")
	}
	p.EnsureCookieProfiles()
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("profile id is required")
	}
	idx := p.indexOfProfile(id)
	if idx < 0 {
		return fmt.Errorf("cookie profile %q not found", id)
	}
	prof := p.CookieProfiles[idx]
	if strings.TrimSpace(prof.CookieFile) == "" {
		return fmt.Errorf("cookie profile %q has no cookie file path", id)
	}
	p.ActiveProfileID = prof.ID
	p.CookieFile = prof.CookieFile
	p.ActiveCRProfileID = ""
	return nil
}
