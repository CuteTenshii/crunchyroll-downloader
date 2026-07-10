package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDotenvEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := parseDotenv(path); err != nil {
		t.Fatalf("parseDotenv(empty) error = %v", err)
	}
}

func TestParseDotenvCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# this is a comment\n\n# another comment\n  \n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := parseDotenv(path); err != nil {
		t.Fatalf("parseDotenv(comments) error = %v", err)
	}
}

func TestParseDotenvKeyValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "MY_KEY=my_value\nANOTHER_KEY=another_value\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	origMyKey, hadMyKey := os.LookupEnv("MY_KEY")
	origAnotherKey, hadAnotherKey := os.LookupEnv("ANOTHER_KEY")
	t.Cleanup(func() {
		if hadMyKey {
			os.Setenv("MY_KEY", origMyKey)
		} else {
			os.Unsetenv("MY_KEY")
		}
		if hadAnotherKey {
			os.Setenv("ANOTHER_KEY", origAnotherKey)
		} else {
			os.Unsetenv("ANOTHER_KEY")
		}
	})
	os.Unsetenv("MY_KEY")
	os.Unsetenv("ANOTHER_KEY")

	if err := parseDotenv(path); err != nil {
		t.Fatalf("parseDotenv() error = %v", err)
	}
	if got := os.Getenv("MY_KEY"); got != "my_value" {
		t.Fatalf("MY_KEY = %q, want my_value", got)
	}
	if got := os.Getenv("ANOTHER_KEY"); got != "another_value" {
		t.Fatalf("ANOTHER_KEY = %q, want another_value", got)
	}
}

func TestParseDotenvQuotedValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "DOUBLE=\"quoted value\"\nSINGLE='single quoted'\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	origDouble, hadDouble := os.LookupEnv("DOUBLE")
	origSingle, hadSingle := os.LookupEnv("SINGLE")
	t.Cleanup(func() {
		if hadDouble {
			os.Setenv("DOUBLE", origDouble)
		} else {
			os.Unsetenv("DOUBLE")
		}
		if hadSingle {
			os.Setenv("SINGLE", origSingle)
		} else {
			os.Unsetenv("SINGLE")
		}
	})
	os.Unsetenv("DOUBLE")
	os.Unsetenv("SINGLE")

	if err := parseDotenv(path); err != nil {
		t.Fatalf("parseDotenv(quoted) error = %v", err)
	}
	if got := os.Getenv("DOUBLE"); got != "quoted value" {
		t.Fatalf("DOUBLE = %q, want 'quoted value'", got)
	}
	if got := os.Getenv("SINGLE"); got != "single quoted" {
		t.Fatalf("SINGLE = %q, want 'single quoted'", got)
	}
}

func TestParseDotenvNoEquals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "KEY_WITHOUT_EQUALS\nHAS_EQUALS=value\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	origHas, hadHas := os.LookupEnv("HAS_EQUALS")
	t.Cleanup(func() {
		if hadHas {
			os.Setenv("HAS_EQUALS", origHas)
		} else {
			os.Unsetenv("HAS_EQUALS")
		}
	})
	os.Unsetenv("HAS_EQUALS")

	if err := parseDotenv(path); err != nil {
		t.Fatalf("parseDotenv(noequals) error = %v", err)
	}
	if _, ok := os.LookupEnv("KEY_WITHOUT_EQUALS"); ok {
		t.Fatal("KEY_WITHOUT_EQUALS should not be set")
	}
	if got := os.Getenv("HAS_EQUALS"); got != "value" {
		t.Fatalf("HAS_EQUALS = %q, want 'value'", got)
	}
}

func TestLoadDotenvFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("FOUND_KEY=found_value"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Chdir(dir)

	foundPath, err := LoadDotenv()
	if err != nil {
		t.Fatalf("LoadDotenv() error = %v", err)
	}
	if foundPath != path {
		t.Fatalf("LoadDotenv() path = %q, want %q", foundPath, path)
	}
}

func TestLoadDotenvNotFound(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	foundPath, err := LoadDotenv()
	if err != nil {
		t.Fatalf("LoadDotenv() error = %v", err)
	}
	if foundPath != "" {
		t.Fatalf("LoadDotenv() path = %q, want empty", foundPath)
	}
}

func TestConfigDirReturnsCwd(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() error = %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	if dir != cwd {
		t.Fatalf("ConfigDir() = %q, want cwd %q", dir, cwd)
	}
}

func TestConfigPath(t *testing.T) {
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	want := filepath.Join(cwd, "config.json")
	if path != want {
		t.Fatalf("ConfigPath() = %q, want %q", path, want)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for missing file", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil, want non-nil Config")
	}
	// All fields should be nil
	if cfg.AudioLang != nil {
		t.Fatal("AudioLang should be nil for missing file")
	}
	if cfg.SubsLang != nil {
		t.Fatal("SubsLang should be nil for missing file")
	}
	if cfg.VideoQuality != nil {
		t.Fatal("VideoQuality should be nil for missing file")
	}
	if cfg.AudioQuality != nil {
		t.Fatal("AudioQuality should be nil for missing file")
	}
	if cfg.Workers != nil {
		t.Fatal("Workers should be nil for missing file")
	}
	if cfg.OutputDir != nil {
		t.Fatal("OutputDir should be nil for missing file")
	}
	if cfg.EtpRt != nil {
		t.Fatal("EtpRt should be nil for missing file")
	}
	if cfg.WidevineDevice != nil {
		t.Fatal("WidevineDevice should be nil for missing file")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid config JSON") {
		t.Fatalf("Load() error = %q, want 'invalid config JSON'", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil Config on invalid JSON, want partial Config")
	}
}

func TestLoadValidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "valid.json")
	jsonContent := `{
		"audio_lang": "en-US",
		"subs_lang": "pt-BR",
		"video_quality": "720p",
		"audio_quality": "128k",
		"workers": 5,
		"output_dir": "/tmp/output",
		"etp_rt": "my-cookie",
		"widevine_device": "/tmp/device.wvd"
	}`
	if err := os.WriteFile(path, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil Config")
	}

	if cfg.AudioLang == nil || *cfg.AudioLang != "en-US" {
		t.Fatalf("AudioLang = %v, want 'en-US'", cfg.AudioLang)
	}
	if cfg.SubsLang == nil || *cfg.SubsLang != "pt-BR" {
		t.Fatalf("SubsLang = %v, want 'pt-BR'", cfg.SubsLang)
	}
	if cfg.VideoQuality == nil || *cfg.VideoQuality != "720p" {
		t.Fatalf("VideoQuality = %v, want '720p'", cfg.VideoQuality)
	}
	if cfg.AudioQuality == nil || *cfg.AudioQuality != "128k" {
		t.Fatalf("AudioQuality = %v, want '128k'", cfg.AudioQuality)
	}
	if cfg.Workers == nil || *cfg.Workers != 5 {
		t.Fatalf("Workers = %v, want 5", cfg.Workers)
	}
	if cfg.OutputDir == nil || *cfg.OutputDir != "/tmp/output" {
		t.Fatalf("OutputDir = %v, want '/tmp/output'", cfg.OutputDir)
	}
	if cfg.EtpRt == nil || *cfg.EtpRt != "my-cookie" {
		t.Fatalf("EtpRt = %v, want 'my-cookie'", cfg.EtpRt)
	}
	if cfg.WidevineDevice == nil || *cfg.WidevineDevice != "/tmp/device.wvd" {
		t.Fatalf("WidevineDevice = %v, want '/tmp/device.wvd'", cfg.WidevineDevice)
	}
}

func TestWriteSkeleton(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := WriteSkeleton(path); err != nil {
		t.Fatalf("WriteSkeleton() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("WriteSkeleton wrote empty file")
	}

	// Verify it's valid JSON
	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		t.Fatalf("skeleton JSON is invalid: %v", err)
	}

	// Check defaults
	if cfg.AudioLang == nil || *cfg.AudioLang != "ja-JP" {
		t.Fatalf("AudioLang = %v, want 'ja-JP'", cfg.AudioLang)
	}
	if cfg.SubsLang == nil || *cfg.SubsLang != "en-US" {
		t.Fatalf("SubsLang = %v, want 'en-US'", cfg.SubsLang)
	}
	if cfg.VideoQuality == nil || *cfg.VideoQuality != "1080p" {
		t.Fatalf("VideoQuality = %v, want '1080p'", cfg.VideoQuality)
	}
	if cfg.AudioQuality == nil || *cfg.AudioQuality != "192k" {
		t.Fatalf("AudioQuality = %v, want '192k'", cfg.AudioQuality)
	}
	if cfg.Workers == nil || *cfg.Workers != 10 {
		t.Fatalf("Workers = %v, want 10", cfg.Workers)
	}
	// OutputDir, EtpRt, WidevineDevice should not be in the skeleton
	if cfg.OutputDir != nil {
		t.Fatal("OutputDir should be nil in skeleton")
	}
	if cfg.EtpRt != nil {
		t.Fatal("EtpRt should be nil in skeleton")
	}
	if cfg.WidevineDevice != nil {
		t.Fatal("WidevineDevice should be nil in skeleton")
	}
}

func TestWriteSkeletonCreatesDirectory(t *testing.T) {
	base := t.TempDir()
	// Use a nested directory that doesn't exist yet
	path := filepath.Join(base, "nested", "subdir", "config.json")

	if err := WriteSkeleton(path); err != nil {
		t.Fatalf("WriteSkeleton() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("WriteSkeleton did not create the config file in nested directory")
	}
}

func TestMergeNilOverlay(t *testing.T) {
	base := &Config{}
	en := "en-US"
	base.AudioLang = &en

	result := Merge(base, nil)
	if result == nil {
		t.Fatal("Merge(base, nil) returned nil")
	}
	if result.AudioLang == nil || *result.AudioLang != "en-US" {
		t.Fatalf("AudioLang = %v, want 'en-US'", result.AudioLang)
	}
}

func TestMergeOverlayFields(t *testing.T) {
	base := &Config{}
	ja := "ja-JP"
	en := "en-US"
	base.AudioLang = &ja
	base.SubsLang = &en

	overlay := &Config{}
	videoQ := "720p"
	overlay.VideoQuality = &videoQ
	wk := 5
	overlay.Workers = &wk

	result := Merge(base, overlay)
	if result == nil {
		t.Fatal("Merge() returned nil")
	}

	// Base fields not in overlay should be preserved
	if result.AudioLang == nil || *result.AudioLang != "ja-JP" {
		t.Fatalf("AudioLang = %v, want 'ja-JP'", result.AudioLang)
	}
	if result.SubsLang == nil || *result.SubsLang != "en-US" {
		t.Fatalf("SubsLang = %v, want 'en-US'", result.SubsLang)
	}

	// Overlay fields should override
	if result.VideoQuality == nil || *result.VideoQuality != "720p" {
		t.Fatalf("VideoQuality = %v, want '720p'", result.VideoQuality)
	}
	if result.Workers == nil || *result.Workers != 5 {
		t.Fatalf("Workers = %v, want 5", result.Workers)
	}

	// Fields set in neither should be nil
	if result.AudioQuality != nil {
		t.Fatal("AudioQuality should be nil")
	}
	if result.OutputDir != nil {
		t.Fatal("OutputDir should be nil")
	}
	if result.EtpRt != nil {
		t.Fatal("EtpRt should be nil")
	}
	if result.WidevineDevice != nil {
		t.Fatal("WidevineDevice should be nil")
	}
}

func TestMergeNilFieldsFallThrough(t *testing.T) {
	base := &Config{}
	ja := "ja-JP"
	base.AudioLang = &ja

	overlay := &Config{}
	// overlay.AudioLang is nil — should not override
	en := "en-US"
	overlay.SubsLang = &en

	result := Merge(base, overlay)
	if result == nil {
		t.Fatal("Merge() returned nil")
	}

	// Nil overlay fields should not override base
	if result.AudioLang == nil || *result.AudioLang != "ja-JP" {
		t.Fatalf("AudioLang = %v, want 'ja-JP' (should not be overridden)", result.AudioLang)
	}

	// Non-nil overlay fields should be set
	if result.SubsLang == nil || *result.SubsLang != "en-US" {
		t.Fatalf("SubsLang = %v, want 'en-US'", result.SubsLang)
	}
}

func TestMergeBothNil(t *testing.T) {
	result := Merge(nil, nil)
	if result == nil {
		t.Fatal("Merge(nil, nil) returned nil, want empty Config")
	}

	// All fields should be nil
	if result.AudioLang != nil {
		t.Fatal("AudioLang should be nil")
	}
	if result.SubsLang != nil {
		t.Fatal("SubsLang should be nil")
	}
	if result.VideoQuality != nil {
		t.Fatal("VideoQuality should be nil")
	}
	if result.AudioQuality != nil {
		t.Fatal("AudioQuality should be nil")
	}
	if result.Workers != nil {
		t.Fatal("Workers should be nil")
	}
	if result.OutputDir != nil {
		t.Fatal("OutputDir should be nil")
	}
	if result.EtpRt != nil {
		t.Fatal("EtpRt should be nil")
	}
	if result.WidevineDevice != nil {
		t.Fatal("WidevineDevice should be nil")
	}
}


