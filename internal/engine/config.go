package engine

import "time"

const (
	defaultPlayback4294Retries        = 2
	defaultPlayback4294Backoff        = 8 * time.Second
	maxPlayback4294Retries            = 5
	maxPlayback4294Backoff            = time.Minute
	defaultIndexWindow                = 25
	defaultIndexTerminalRecheckWindow = 3
	defaultIndexCircuitLimit          = 3
	defaultIndexCooldown              = 30 * time.Minute
)

// RuntimeConfig holds download/index engine settings that were previously
// package-level flag pointers. CLI and GUI set this before calling Run.
type RuntimeConfig struct {
	VideoQuality               string
	AudioQuality               string
	DebugManifest              bool
	Playback4294Retries        int
	Playback4294Backoff        time.Duration
	IndexDelaySeconds          int
	IndexWindow                int
	IndexTerminalRecheckWindow int
	IndexCircuitLimit          int
	IndexCircuitCooldown       time.Duration
	IndexPriorityIDs           string
	IndexSummaryPath           string
}

// DefaultRuntimeConfig returns the same defaults the CLI flags used.
func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		VideoQuality:               "1080p",
		AudioQuality:               "192k",
		Playback4294Retries:        defaultPlayback4294Retries,
		Playback4294Backoff:        defaultPlayback4294Backoff,
		IndexDelaySeconds:          3,
		IndexWindow:                defaultIndexWindow,
		IndexTerminalRecheckWindow: defaultIndexTerminalRecheckWindow,
		IndexCircuitLimit:          defaultIndexCircuitLimit,
		IndexCircuitCooldown:       defaultIndexCooldown,
	}
}

// activeConfig is the process-wide engine config. Tests may mutate fields;
// production callers should use SetRuntimeConfig / Configure before Run.
var activeConfig = DefaultRuntimeConfig()

// SetRuntimeConfig replaces the active engine configuration.
func SetRuntimeConfig(cfg RuntimeConfig) {
	activeConfig = cfg
}

// CurrentRuntimeConfig returns a copy of the active engine configuration.
func CurrentRuntimeConfig() RuntimeConfig {
	return activeConfig
}
