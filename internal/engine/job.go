package engine

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

// DownloadJob describes a multi-episode download queue for the GUI/CLI job runner.
// Authentication (etp_rt / access token) must already be loaded before RunDownloadJob.
// JSON tags are required so Wails includes fields when binding from the frontend.
type DownloadJob struct {
	EpisodeIDs    []string `json:"episodeIds"`
	AudioLangs    []string `json:"audioLangs"`    // empty = original locale from episode metadata
	SubtitleLangs []string `json:"subtitleLangs"` // empty = none
	CaptionLangs  []string `json:"captionLangs"`  // empty = none
	VideoQuality  string   `json:"videoQuality"`  // "max" or concrete e.g. 1080p
	AudioQuality  string   `json:"audioQuality"`  // "max" or concrete e.g. 192k
	OutputDir     string   `json:"outputDir"`
	StrictLangs   bool     `json:"strictLangs"`
}

// Test seams so job tests do not hit live Crunchyroll or the full download path.
var (
	jobGetEpisodeInfo  = getEpisodeInfo
	jobDownloadEpisode = downloadEpisode
)

// activeJobProgress holds per-run emit/ctx state for optional segment progress and
// best-effort mid-episode cancel from downloadParts.
type activeJobProgress struct {
	ctx          context.Context
	emit         func(ProgressEvent)
	queueIndex   int
	queueTotal   int
	episodeID    string
	episodeLabel string
}

var (
	jobProgressMu sync.RWMutex
	jobProgress   *activeJobProgress
)

func setJobProgress(p *activeJobProgress) {
	jobProgressMu.Lock()
	jobProgress = p
	jobProgressMu.Unlock()
}

func clearJobProgress() {
	jobProgressMu.Lock()
	jobProgress = nil
	jobProgressMu.Unlock()
}

func currentJobProgress() *activeJobProgress {
	jobProgressMu.RLock()
	defer jobProgressMu.RUnlock()
	return jobProgress
}

// RunDownloadJob downloads each episode in order, emitting ProgressEvent updates.
//
// OutputDir: when non-empty, the process working directory is changed to OutputDir
// for the duration of the job (downloadEpisode creates series folders under CWD)
// and restored afterward. Prefer an absolute OutputDir.
//
// VideoQuality/AudioQuality of "max" (case-insensitive) are passed through to
// getBaseUrl, which selects the highest available representation from the MPD.
//
// Missing audio locales are filtered before download when StrictLangs is false
// (warn + continue with remaining). Subtitle/CC availability is enforced inside
// downloadEpisode; missing-locale errors are warned and skipped when !StrictLangs.
func RunDownloadJob(ctx context.Context, job DownloadJob, cfg RuntimeConfig, emit func(ProgressEvent)) (err error) {
	if emit == nil {
		emit = func(ProgressEvent) {}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	SetRuntimeConfig(cfg)

	if dir := strings.TrimSpace(job.OutputDir); dir != "" {
		origWD, wdErr := os.Getwd()
		if wdErr != nil {
			return fmt.Errorf("get working directory: %w", wdErr)
		}
		if err := os.MkdirAll(dir, 0o777); err != nil {
			return fmt.Errorf("create output directory %q: %w", dir, err)
		}
		if err := os.Chdir(dir); err != nil {
			return fmt.Errorf("chdir to output directory %q: %w", dir, err)
		}
		defer func() {
			if restoreErr := os.Chdir(origWD); restoreErr != nil && err == nil {
				err = fmt.Errorf("restore working directory: %w", restoreErr)
			}
		}()
	}

	total := len(job.EpisodeIDs)
	if total == 0 {
		emit(ProgressEvent{
			Phase:      PhaseDone,
			Message:    "no episodes in queue",
			Level:      "info",
			QueueIndex: 0,
			QueueTotal: 0,
			Fraction:   1,
		})
		return nil
	}

	videoQuality := resolveJobVideoQuality(job.VideoQuality)
	audioQuality := resolveJobAudioQuality(job.AudioQuality)

	// Prefer job quality over RuntimeConfig defaults when set.
	if videoQuality != "" {
		cfg.VideoQuality = videoQuality
	}
	if audioQuality != "" {
		cfg.AudioQuality = audioQuality
	}
	SetRuntimeConfig(cfg)

	for i, episodeID := range job.EpisodeIDs {
		if err := ctx.Err(); err != nil {
			emit(ProgressEvent{
				Phase:      PhaseIdle,
				Message:    "cancelled",
				Level:      "warn",
				EpisodeID:  episodeID,
				QueueIndex: i,
				QueueTotal: total,
				Fraction:   -1,
			})
			return err
		}

		emit(ProgressEvent{
			Phase:      PhaseInspect,
			Message:    fmt.Sprintf("loading episode metadata (%d/%d)", i+1, total),
			Level:      "info",
			EpisodeID:  episodeID,
			QueueIndex: i,
			QueueTotal: total,
			Fraction:   -1,
		})

		info, infoErr := safeGetEpisodeInfo(episodeID)
		if infoErr != nil {
			emit(ProgressEvent{
				Phase:      PhaseDownload,
				Message:    infoErr.Error(),
				Level:      "error",
				EpisodeID:  episodeID,
				QueueIndex: i,
				QueueTotal: total,
				Fraction:   -1,
			})
			return fmt.Errorf("episode %s: %w", episodeID, infoErr)
		}

		label := episodeLabel(info)
		audioLangs, warnMsgs, langErr := resolveJobAudioLangs(job.AudioLangs, info, job.StrictLangs)
		for _, msg := range warnMsgs {
			emit(ProgressEvent{
				Phase:        PhaseDownload,
				Message:      msg,
				Level:        "warn",
				EpisodeID:    episodeID,
				EpisodeLabel: label,
				QueueIndex:   i,
				QueueTotal:   total,
				Fraction:     -1,
			})
		}
		if langErr != nil {
			emit(ProgressEvent{
				Phase:        PhaseDownload,
				Message:      langErr.Error(),
				Level:        "error",
				EpisodeID:    episodeID,
				EpisodeLabel: label,
				QueueIndex:   i,
				QueueTotal:   total,
				Fraction:     -1,
			})
			return langErr
		}
		if len(audioLangs) == 0 {
			// Non-strict path exhausted all requested audio locales.
			emit(ProgressEvent{
				Phase:        PhaseDownload,
				Message:      fmt.Sprintf("skipping episode %s: no requested audio locales available", episodeID),
				Level:        "warn",
				EpisodeID:    episodeID,
				EpisodeLabel: label,
				QueueIndex:   i,
				QueueTotal:   total,
				Fraction:     -1,
			})
			continue
		}

		subsLangs := append([]string(nil), job.SubtitleLangs...)
		ccLangs := append([]string(nil), job.CaptionLangs...)

		setJobProgress(&activeJobProgress{
			ctx:          ctx,
			emit:         emit,
			queueIndex:   i,
			queueTotal:   total,
			episodeID:    episodeID,
			episodeLabel: label,
		})

		emit(ProgressEvent{
			Phase:        PhaseDownload,
			Message:      fmt.Sprintf("downloading %s", label),
			Level:        "info",
			EpisodeID:    episodeID,
			EpisodeLabel: label,
			QueueIndex:   i,
			QueueTotal:   total,
			Fraction:     -1,
		})

		dlErr := jobDownloadEpisode(episodeID, info, audioLangs, subsLangs, ccLangs, videoQuality, audioQuality)
		clearJobProgress()

		if dlErr != nil {
			if ctx.Err() != nil {
				emit(ProgressEvent{
					Phase:        PhaseIdle,
					Message:      "cancelled",
					Level:        "warn",
					EpisodeID:    episodeID,
					EpisodeLabel: label,
					QueueIndex:   i,
					QueueTotal:   total,
					Fraction:     -1,
				})
				return ctx.Err()
			}
			if !job.StrictLangs && isMissingLocaleError(dlErr) {
				emit(ProgressEvent{
					Phase:        PhaseDownload,
					Message:      dlErr.Error(),
					Level:        "warn",
					EpisodeID:    episodeID,
					EpisodeLabel: label,
					QueueIndex:   i,
					QueueTotal:   total,
					Fraction:     -1,
				})
				continue
			}
			level := "error"
			emit(ProgressEvent{
				Phase:        PhaseDownload,
				Message:      dlErr.Error(),
				Level:        level,
				EpisodeID:    episodeID,
				EpisodeLabel: label,
				QueueIndex:   i,
				QueueTotal:   total,
				Fraction:     -1,
			})
			return fmt.Errorf("download episode %s: %w", episodeID, dlErr)
		}

		emit(ProgressEvent{
			Phase:        PhaseDownload,
			Message:      fmt.Sprintf("finished %s", label),
			Level:        "ok",
			EpisodeID:    episodeID,
			EpisodeLabel: label,
			QueueIndex:   i,
			QueueTotal:   total,
			Fraction:     float64(i+1) / float64(total),
		})
	}

	emit(ProgressEvent{
		Phase:      PhaseDone,
		Message:    "queue complete",
		Level:      "ok",
		QueueIndex: total,
		QueueTotal: total,
		Fraction:   1,
	})
	return nil
}

func safeGetEpisodeInfo(id string) (info EpisodeInfo, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = panicAsError(recovered)
		}
	}()
	info = jobGetEpisodeInfo(id)
	return info, nil
}

func episodeLabel(info EpisodeInfo) string {
	meta := info.EpisodeMetadata
	if meta.SeriesTitle != "" && meta.EpisodeNumber > 0 {
		return fmt.Sprintf("%s S%02dE%02d - %s", meta.SeriesTitle, meta.SeasonNumber, meta.EpisodeNumber, info.Title)
	}
	if info.Title != "" {
		return info.Title
	}
	return meta.SeriesTitle
}

func resolveJobVideoQuality(q string) string {
	q = strings.TrimSpace(q)
	if q == "" || strings.EqualFold(q, "max") {
		return "max"
	}
	return q
}

func resolveJobAudioQuality(q string) string {
	q = strings.TrimSpace(q)
	if q == "" || strings.EqualFold(q, "max") {
		return "max"
	}
	return q
}

// resolveJobAudioLangs picks audio locales for one episode.
// empty request → original (primary) locale; "all" is left for downloadEpisode.
func resolveJobAudioLangs(requested []string, info EpisodeInfo, strict bool) (langs []string, warnings []string, err error) {
	available := map[string]struct{}{}
	if info.EpisodeMetadata.AudioLocale != "" {
		available[info.EpisodeMetadata.AudioLocale] = struct{}{}
	}
	for _, v := range info.EpisodeMetadata.Versions {
		if v != nil && v.AudioLocale != "" {
			available[v.AudioLocale] = struct{}{}
		}
	}

	if len(requested) == 0 {
		if info.EpisodeMetadata.AudioLocale != "" {
			return []string{info.EpisodeMetadata.AudioLocale}, nil, nil
		}
		// No primary locale; pass empty and let downloadEpisode error.
		return nil, nil, fmt.Errorf("episode has no original audio locale")
	}

	if len(requested) == 1 && requested[0] == "all" {
		return []string{"all"}, nil, nil
	}

	var missing []string
	for _, locale := range requested {
		if _, ok := available[locale]; ok {
			langs = append(langs, locale)
		} else {
			missing = append(missing, locale)
		}
	}
	for _, locale := range missing {
		msg := fmt.Sprintf("audio locale %s is not available", locale)
		if strict {
			return nil, warnings, fmt.Errorf("%s for episode %v", msg, info.EpisodeMetadata.EpisodeNumber)
		}
		warnings = append(warnings, msg)
	}
	return langs, warnings, nil
}

func isMissingLocaleError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "is not available")
}

// emitSegmentProgress is called from downloadParts when a job is active.
func emitSegmentProgress(done, total int) {
	p := currentJobProgress()
	if p == nil || p.emit == nil || total <= 0 {
		return
	}
	frac := float64(done) / float64(total)
	p.emit(ProgressEvent{
		Phase:        PhaseDownload,
		Message:      fmt.Sprintf("segments %d/%d", done, total),
		Level:        "info",
		EpisodeID:    p.episodeID,
		EpisodeLabel: p.episodeLabel,
		QueueIndex:   p.queueIndex,
		QueueTotal:   p.queueTotal,
		SegmentDone:  done,
		SegmentTotal: total,
		Fraction:     frac,
	})
}

// jobDownloadCancelled reports whether the active job context was cancelled.
func jobDownloadCancelled() error {
	p := currentJobProgress()
	if p == nil || p.ctx == nil {
		return nil
	}
	return p.ctx.Err()
}
