package engine

// ProgressPhase names a stage of Inspect or download work for GUI/CLI progress.
type ProgressPhase string

const (
	PhaseIdle     ProgressPhase = "idle"
	PhaseInspect  ProgressPhase = "inspect"
	PhaseDownload ProgressPhase = "download"
	PhaseDecrypt  ProgressPhase = "decrypt"
	PhaseMux      ProgressPhase = "mux"
	PhaseBackoff  ProgressPhase = "backoff"
	PhaseDone     ProgressPhase = "done"
)

// ProgressEvent is a single progress update pushed via callback (and later Wails events).
// Fraction is 0..1 when known, or -1 for indeterminate. Segment fields are optional.
type ProgressEvent struct {
	Phase        ProgressPhase `json:"phase"`
	Message      string        `json:"message"`
	Level        string        `json:"level"` // info|ok|warn|error
	EpisodeID    string        `json:"episodeId,omitempty"`
	EpisodeLabel string        `json:"episodeLabel,omitempty"`
	QueueIndex   int           `json:"queueIndex"`
	QueueTotal   int           `json:"queueTotal"`
	SegmentDone  int           `json:"segmentDone,omitempty"`
	SegmentTotal int           `json:"segmentTotal,omitempty"`
	Fraction     float64       `json:"fraction,omitempty"` // 0..1 or -1 indeterminate
}
