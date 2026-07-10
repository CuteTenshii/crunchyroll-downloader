package output

import (
	"encoding/json"
	"os"
	"time"
)

// event is the base NDJSON event type.
type event struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`

	// Episode fields
	EpisodeNumber int `json:"episode_number,omitempty"`
	SeasonNumber  int `json:"season_number,omitempty"`
	TotalEpisodes int `json:"total_episodes,omitempty"`

	Title       string `json:"title,omitempty"`
	SeriesTitle string `json:"series_title,omitempty"`

	// Progress fields
	Downloaded  int     `json:"downloaded,omitempty"`
	Total       int     `json:"total,omitempty"`
	Percent     float64 `json:"percent,omitempty"`
	BytesPerSec int64   `json:"bytes_per_sec,omitempty"`
	ETASecs     int     `json:"eta_secs,omitempty"`
	Stream      string  `json:"stream,omitempty"`
	Locale      string  `json:"locale,omitempty"`

	// Message/Error fields
	Message string `json:"message,omitempty"`
	Success bool   `json:"success,omitempty"`
	Fatal   bool   `json:"fatal,omitempty"`

	// Completion fields
	DurationSecs float64 `json:"duration_secs,omitempty"`
	SizeBytes    int64   `json:"size_bytes,omitempty"`

	// Summary fields
	Successful int          `json:"successful,omitempty"`
	Failed     int          `json:"failed,omitempty"`
	Errors     []eventError `json:"errors,omitempty"`
}

// eventError holds per-episode error details for season_summary events.
type eventError struct {
	EpisodeNumber int    `json:"episode_number"`
	Title         string `json:"title"`
	Message       string `json:"message"`
}

// emitEvent writes a single NDJSON event to stdout.
func emitEvent(evt event) {
	evt.Timestamp = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(evt)
	if err != nil {
		return // silently skip un-marshalable events
	}
	os.Stdout.Write(data)
	os.Stdout.Write([]byte{'\n'})
}
