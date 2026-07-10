package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the JSON config file at ./config.json (project root).
// Pointer fields enable explicit-only overrides: nil = absent from file.
type Config struct {
	AudioLang      *string `json:"audio_lang,omitempty"`
	SubsLang       *string `json:"subs_lang,omitempty"`
	VideoQuality   *string `json:"video_quality,omitempty"`
	AudioQuality   *string `json:"audio_quality,omitempty"`
	Workers        *int    `json:"workers,omitempty"`
	OutputDir      *string `json:"output_dir,omitempty"`
	EtpRt          *string `json:"etp_rt,omitempty"`
	WidevineDevice *string `json:"widevine_device,omitempty"`
}

// ConfigDir returns the current working directory as the config directory.
// The config file lives at ./config.json alongside the project root.
func ConfigDir() (string, error) {
	return os.Getwd()
}

// ConfigPath returns the full path to the config file.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config file at path and returns a Config.
// If the file does not exist, it returns an empty Config (all nil fields)
// with no error so the caller can proceed with defaults.
// If the file contains invalid JSON, it returns a partial Config (whatever
// was decoded) with a wrapped error so the caller can warn and continue.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return cfg, fmt.Errorf("invalid config JSON: %w", err)
	}
	return cfg, nil
}

// WriteSkeleton creates a minimal config file at path with default values.
// It creates the parent directory if it does not exist.
func WriteSkeleton(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	skeleton := map[string]interface{}{
		"audio_lang":    "ja-JP",
		"subs_lang":     "en-US",
		"video_quality": "1080p",
		"audio_quality": "192k",
		"workers":       10,
	}

	data, err := json.MarshalIndent(skeleton, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding skeleton config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Merge applies overlay's non-nil fields on top of base.
// Returns a new Config without mutating either input.
// This implements explicit-only override: only fields the user
// explicitly set in the overlay will override the base values.
func Merge(base, overlay *Config) *Config {
	result := &Config{}
	if base != nil {
		*result = *base // shallow copy
	}

	if overlay == nil {
		return result
	}

	if overlay.AudioLang != nil {
		result.AudioLang = overlay.AudioLang
	}
	if overlay.SubsLang != nil {
		result.SubsLang = overlay.SubsLang
	}
	if overlay.VideoQuality != nil {
		result.VideoQuality = overlay.VideoQuality
	}
	if overlay.AudioQuality != nil {
		result.AudioQuality = overlay.AudioQuality
	}
	if overlay.Workers != nil {
		result.Workers = overlay.Workers
	}
	if overlay.OutputDir != nil {
		result.OutputDir = overlay.OutputDir
	}
	if overlay.EtpRt != nil {
		result.EtpRt = overlay.EtpRt
	}
	if overlay.WidevineDevice != nil {
		result.WidevineDevice = overlay.WidevineDevice
	}

	return result
}
