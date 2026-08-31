package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunDownloadJobEmptyEpisodes(t *testing.T) {
	var events []ProgressEvent
	err := RunDownloadJob(context.Background(), DownloadJob{}, DefaultRuntimeConfig(), func(ev ProgressEvent) {
		events = append(events, ev)
	})
	if err != nil {
		t.Fatalf("empty queue should succeed, got %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one progress event")
	}
	last := events[len(events)-1]
	if last.Phase != PhaseDone {
		t.Fatalf("last phase = %q, want %q", last.Phase, PhaseDone)
	}
	if last.QueueTotal != 0 {
		t.Fatalf("QueueTotal = %d, want 0", last.QueueTotal)
	}
}

func TestRunDownloadJobRespectsCancelBeforeStart(t *testing.T) {
	originalInfo := jobGetEpisodeInfo
	originalDL := jobDownloadEpisode
	defer func() {
		jobGetEpisodeInfo = originalInfo
		jobDownloadEpisode = originalDL
	}()

	var infoCalls, dlCalls atomic.Int32
	jobGetEpisodeInfo = func(string) EpisodeInfo {
		infoCalls.Add(1)
		return EpisodeInfo{}
	}
	jobDownloadEpisode = func(string, EpisodeInfo, []string, []string, []string, string, string) error {
		dlCalls.Add(1)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var events []ProgressEvent
	err := RunDownloadJob(ctx, DownloadJob{
		EpisodeIDs: []string{"GY19Q77GR", "GYOTHER01"},
	}, DefaultRuntimeConfig(), func(ev ProgressEvent) {
		events = append(events, ev)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if infoCalls.Load() != 0 || dlCalls.Load() != 0 {
		t.Fatalf("cancelled job still called info=%d download=%d", infoCalls.Load(), dlCalls.Load())
	}
	if len(events) == 0 || events[0].Level != "warn" {
		t.Fatalf("expected cancel warn event, got %#v", events)
	}
}

func TestRunDownloadJobQueueOrderAndProgress(t *testing.T) {
	originalInfo := jobGetEpisodeInfo
	originalDL := jobDownloadEpisode
	defer func() {
		jobGetEpisodeInfo = originalInfo
		jobDownloadEpisode = originalDL
	}()

	var seenIDs []string
	jobGetEpisodeInfo = func(id string) EpisodeInfo {
		return EpisodeInfo{
			EpisodeMetadata: EpisodeMetadata{
				SeriesTitle:   "Test Series",
				SeasonNumber:  1,
				EpisodeNumber: len(seenIDs) + 1,
				AudioLocale:   "ja-JP",
				Versions:      []*DubVersion{{AudioLocale: "ja-JP", GUID: id}},
			},
			Title: "Ep",
		}
	}
	jobDownloadEpisode = func(id string, _ EpisodeInfo, audio, subs, cc []string, vq, aq string) error {
		seenIDs = append(seenIDs, id)
		if vq != "max" {
			t.Errorf("video quality = %q, want max", vq)
		}
		if aq != "max" {
			t.Errorf("audio quality = %q, want max", aq)
		}
		if len(audio) != 1 || audio[0] != "ja-JP" {
			t.Errorf("audio langs = %v", audio)
		}
		if len(subs) != 0 || len(cc) != 0 {
			t.Errorf("subs/cc should be empty by default, got subs=%v cc=%v", subs, cc)
		}
		return nil
	}

	var events []ProgressEvent
	err := RunDownloadJob(context.Background(), DownloadJob{
		EpisodeIDs:   []string{"ID1", "ID2"},
		VideoQuality: "max",
		AudioQuality: "max",
	}, DefaultRuntimeConfig(), func(ev ProgressEvent) {
		events = append(events, ev)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(seenIDs, ","); got != "ID1,ID2" {
		t.Fatalf("download order = %q", got)
	}
	var sawInspect, sawDone bool
	for _, ev := range events {
		if ev.Phase == PhaseInspect {
			sawInspect = true
		}
		if ev.Phase == PhaseDone && ev.Level == "ok" {
			sawDone = true
		}
	}
	if !sawInspect || !sawDone {
		t.Fatalf("missing phases inspect=%v done=%v events=%#v", sawInspect, sawDone, events)
	}
}

func TestRunDownloadJobStrictLangsMissingAudio(t *testing.T) {
	originalInfo := jobGetEpisodeInfo
	originalDL := jobDownloadEpisode
	defer func() {
		jobGetEpisodeInfo = originalInfo
		jobDownloadEpisode = originalDL
	}()

	jobGetEpisodeInfo = func(string) EpisodeInfo {
		return EpisodeInfo{
			EpisodeMetadata: EpisodeMetadata{AudioLocale: "ja-JP", EpisodeNumber: 1},
			Title:           "Ep",
		}
	}
	jobDownloadEpisode = func(string, EpisodeInfo, []string, []string, []string, string, string) error {
		t.Fatal("download should not run when strict audio is missing")
		return nil
	}

	err := RunDownloadJob(context.Background(), DownloadJob{
		EpisodeIDs:  []string{"EP1"},
		AudioLangs:  []string{"en-US"},
		StrictLangs: true,
	}, DefaultRuntimeConfig(), nil)
	if err == nil || !strings.Contains(err.Error(), "en-US") {
		t.Fatalf("want strict missing-locale error, got %v", err)
	}
}

func TestRunDownloadJobNonStrictSkipsMissingAudio(t *testing.T) {
	originalInfo := jobGetEpisodeInfo
	originalDL := jobDownloadEpisode
	defer func() {
		jobGetEpisodeInfo = originalInfo
		jobDownloadEpisode = originalDL
	}()

	var dlCalls int
	jobGetEpisodeInfo = func(string) EpisodeInfo {
		return EpisodeInfo{
			EpisodeMetadata: EpisodeMetadata{AudioLocale: "ja-JP", EpisodeNumber: 1},
			Title:           "Ep",
		}
	}
	jobDownloadEpisode = func(string, EpisodeInfo, []string, []string, []string, string, string) error {
		dlCalls++
		return nil
	}

	var warns int
	err := RunDownloadJob(context.Background(), DownloadJob{
		EpisodeIDs:  []string{"EP1"},
		AudioLangs:  []string{"en-US"},
		StrictLangs: false,
	}, DefaultRuntimeConfig(), func(ev ProgressEvent) {
		if ev.Level == "warn" {
			warns++
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if dlCalls != 0 {
		t.Fatalf("expected skip when no audio remains, dlCalls=%d", dlCalls)
	}
	if warns == 0 {
		t.Fatal("expected warn events for missing audio")
	}
}

func TestRunDownloadJobCancelBetweenEpisodes(t *testing.T) {
	originalInfo := jobGetEpisodeInfo
	originalDL := jobDownloadEpisode
	defer func() {
		jobGetEpisodeInfo = originalInfo
		jobDownloadEpisode = originalDL
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var dlCalls atomic.Int32
	jobGetEpisodeInfo = func(string) EpisodeInfo {
		return EpisodeInfo{
			EpisodeMetadata: EpisodeMetadata{AudioLocale: "ja-JP", EpisodeNumber: 1},
			Title:           "Ep",
		}
	}
	jobDownloadEpisode = func(string, EpisodeInfo, []string, []string, []string, string, string) error {
		dlCalls.Add(1)
		cancel()
		return nil
	}

	err := RunDownloadJob(ctx, DownloadJob{
		EpisodeIDs: []string{"A", "B"},
	}, DefaultRuntimeConfig(), nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want canceled after first episode", err)
	}
	if dlCalls.Load() != 1 {
		t.Fatalf("download calls = %d, want 1", dlCalls.Load())
	}
}

func TestRunDownloadJobOutputDirChdirRestore(t *testing.T) {
	originalInfo := jobGetEpisodeInfo
	originalDL := jobDownloadEpisode
	defer func() {
		jobGetEpisodeInfo = originalInfo
		jobDownloadEpisode = originalDL
	}()

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()

	var sawWD string
	jobGetEpisodeInfo = func(string) EpisodeInfo {
		return EpisodeInfo{
			EpisodeMetadata: EpisodeMetadata{AudioLocale: "ja-JP"},
			Title:           "Ep",
		}
	}
	jobDownloadEpisode = func(string, EpisodeInfo, []string, []string, []string, string, string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		sawWD = wd
		return nil
	}

	err = RunDownloadJob(context.Background(), DownloadJob{
		EpisodeIDs: []string{"EP1"},
		OutputDir:  out,
	}, DefaultRuntimeConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Resolve for fair compare on Windows.
	want, _ := filepath.EvalSymlinks(out)
	got, _ := filepath.EvalSymlinks(sawWD)
	if !strings.EqualFold(want, got) {
		t.Fatalf("download cwd = %q, want %q", sawWD, out)
	}
	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if after != origWD {
		t.Fatalf("cwd not restored: got %q want %q", after, origWD)
	}
}

func TestRunDownloadJobMissingLocaleErrorFromDownloadNonStrict(t *testing.T) {
	originalInfo := jobGetEpisodeInfo
	originalDL := jobDownloadEpisode
	defer func() {
		jobGetEpisodeInfo = originalInfo
		jobDownloadEpisode = originalDL
	}()

	jobGetEpisodeInfo = func(string) EpisodeInfo {
		return EpisodeInfo{
			EpisodeMetadata: EpisodeMetadata{AudioLocale: "ja-JP", EpisodeNumber: 3},
			Title:           "Ep",
		}
	}
	jobDownloadEpisode = func(string, EpisodeInfo, []string, []string, []string, string, string) error {
		return errors.New("subtitle locale en-US is not available for episode 3")
	}

	var warns int
	err := RunDownloadJob(context.Background(), DownloadJob{
		EpisodeIDs:    []string{"EP1"},
		SubtitleLangs: []string{"en-US"},
		StrictLangs:   false,
	}, DefaultRuntimeConfig(), func(ev ProgressEvent) {
		if ev.Level == "warn" {
			warns++
		}
	})
	if err != nil {
		t.Fatalf("non-strict should continue past missing sub: %v", err)
	}
	if warns == 0 {
		t.Fatal("expected warn for missing subtitle locale")
	}
}

func TestJobDownloadCancelledRespectsActiveContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	setJobProgress(&activeJobProgress{ctx: ctx})
	defer clearJobProgress()

	if err := jobDownloadCancelled(); err != nil {
		t.Fatalf("unexpected cancel: %v", err)
	}
	cancel()
	// Give cancel a moment to propagate (usually immediate).
	time.Sleep(time.Millisecond)
	if err := jobDownloadCancelled(); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want canceled", err)
	}
}
