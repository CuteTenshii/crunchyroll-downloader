package engine

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	widevine "github.com/iyear/gowidevine"
	"github.com/unki2aut/go-mpd"
)

func TestSeekAheadRetargetsIndex(t *testing.T) {
	q := newSegmentQueue(100)
	q.Retarget(40)
	if q.Next() != 40 {
		t.Fatalf("want 40")
	}
}

func TestSeekAheadThenFillsLowestMissing(t *testing.T) {
	// After Retarget(3), Next is 3 once, then lowest missing: 0,1,2,4.
	// playing.mp4 / BufferEndSec are contiguous from 0, so prefix fill is required.
	q := newSegmentQueue(5)
	q.Retarget(3)
	if q.Next() != 3 {
		t.Fatalf("want 3")
	}
	got := []int{q.Next(), q.Next(), q.Next(), q.Next()}
	want := []int{0, 1, 2, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestPlayBufferReadyThreshold(t *testing.T) {
	if playBufferReady(0, 0) {
		t.Fatal("empty buffer is not ready")
	}
	if !playBufferReady(0.5, 0) {
		t.Fatal("0.5s contiguous is ready")
	}
	if !playBufferReady(0.1, 1) {
		t.Fatal("first media segment is ready")
	}
	if playBufferReady(0.4, 0) {
		t.Fatal("under both thresholds is not ready")
	}
}

func TestIndexForTime(t *testing.T) {
	durs := []float64{2, 2, 2, 2}
	if got := indexForTime(durs, 0); got != 0 {
		t.Fatalf("0s → %d", got)
	}
	if got := indexForTime(durs, 3.9); got != 1 {
		t.Fatalf("3.9s → %d", got)
	}
	if got := indexForTime(durs, 4); got != 2 {
		t.Fatalf("4s → %d", got)
	}
	if got := indexForTime(durs, 99); got != 3 {
		t.Fatalf("past end → %d", got)
	}
}

func TestMediaAfterInit(t *testing.T) {
	ftyp := mp4Box("ftyp", []byte("isom"))
	moov := mp4Box("moov", nil)
	moof := mp4Box("moof", []byte{1})
	mdat := mp4Box("mdat", []byte{2, 3})
	all := concatBytes(ftyp, moov, moof, mdat)
	got := mediaAfterInit(all)
	want := concatBytes(moof, mdat)
	if !bytes.Equal(got, want) {
		t.Fatalf("got %d bytes want %d", len(got), len(want))
	}
}

func TestStartProgressivePlayRequiresExplicitWidevine(t *testing.T) {
	t.Setenv("CRUNCHYROLL_WIDEVINE_DEVICE_FILE", "")
	t.Setenv("CRUNCHYROLL_WIDEVINE_CLIENT_ID_FILE", "")
	t.Setenv("CRUNCHYROLL_WIDEVINE_PRIVATE_KEY_FILE", "")

	cwd := t.TempDir()
	t.Chdir(cwd)
	if err := os.WriteFile(filepath.Join(cwd, "device.wvd"), []byte("planted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "client_id.bin"), []byte("planted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "private_key.pem"), []byte("planted"), 0o600); err != nil {
		t.Fatal(err)
	}

	origOpen := playOpenPlayback
	origLic := playGetLicense
	opens, licenses := 0, 0
	playOpenPlayback = func(string) Episode {
		opens++
		t.Fatal("play must not open playback without explicit Widevine env")
		return Episode{}
	}
	playGetLicense = func(string, string, string) error {
		licenses++
		t.Fatal("play must not call getLicense without explicit Widevine env")
		return nil
	}
	t.Cleanup(func() {
		playOpenPlayback = origOpen
		playGetLicense = origLic
	})

	_, err := StartProgressivePlay(context.Background(), "GWep", DefaultRuntimeConfig(), nil)
	if err == nil || !strings.Contains(err.Error(), "no authorized Widevine device configured") {
		t.Fatalf("err=%v", err)
	}
	if opens != 0 || licenses != 0 {
		t.Fatalf("opened=%d licenses=%d", opens, licenses)
	}
}

func TestStartProgressivePlayFailsWhenDownloadRunning(t *testing.T) {
	setJobProgress(&activeJobProgress{ctx: context.Background()})
	t.Cleanup(clearJobProgress)
	_, err := StartProgressivePlay(context.Background(), "GWep", DefaultRuntimeConfig(), nil)
	if err == nil || !strings.Contains(err.Error(), "download already running") {
		t.Fatalf("err=%v", err)
	}
}

func TestStartProgressivePlayEmitsReady(t *testing.T) {
	restore := stubPlayPipeline(t, 5, 2)
	defer restore()

	var mu sync.Mutex
	var events []PlayProgress
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, err := StartProgressivePlay(ctx, "GWep", DefaultRuntimeConfig(), func(p PlayProgress) {
		mu.Lock()
		events = append(events, p)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	if sess.BufferedEnd() < 4 {
		t.Fatalf("not ready: buffer=%.2f", sess.BufferedEnd())
	}
	path := sess.PlayingPath()
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		t.Fatalf("playing file: %v size=%v", err, st)
	}
	if _, err := os.Stat(filepath.Join(sess.Dir, "init.mp4")); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sess.Dir, "seg-0000.m4s")); err != nil {
		t.Fatalf("seg-0000: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	sawReady := false
	for _, ev := range events {
		if ev.Ready {
			sawReady = true
		}
	}
	if !sawReady {
		t.Fatal("expected ready emit")
	}

	sess.SeekTarget(6) // must not panic; workers may already have drained the queue
}

func TestStartProgressivePlayAlreadyRunning(t *testing.T) {
	restore := stubPlayPipeline(t, 5, 2)
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, err := StartProgressivePlay(ctx, "GWep", DefaultRuntimeConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	_, err = StartProgressivePlay(context.Background(), "GWep", DefaultRuntimeConfig(), nil)
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("err=%v", err)
	}
}

func mp4Box(typ string, payload []byte) []byte {
	n := 8 + len(payload)
	b := make([]byte, n)
	binary.BigEndian.PutUint32(b[0:4], uint32(n))
	copy(b[4:8], typ)
	copy(b[8:], payload)
	return b
}

func concatBytes(parts ...[]byte) []byte {
	var buf bytes.Buffer
	for _, p := range parts {
		buf.Write(p)
	}
	return buf.Bytes()
}

func fakeFragment() []byte {
	return concatBytes(
		mp4Box("ftyp", []byte("isom")),
		mp4Box("moov", nil),
		mp4Box("moof", []byte{1}),
		mp4Box("mdat", []byte{2, 3, 4, 5}),
	)
}

func stubPlayPipeline(t *testing.T, segs int, secPerSeg float64) (restore func()) {
	t.Helper()
	t.Setenv("CRUNCHYROLL_WIDEVINE_DEVICE_FILE", filepath.Join(t.TempDir(), "device.wvd"))
	origOpen := playOpenPlayback
	origParse := playParseManifest
	origLic := playGetLicense
	origDL := playDownloadPart
	origDec := playDecryptFragment
	origClose := playClosePlayback
	playOpenPlayback = func(string) Episode {
		return Episode{ManifestURL: "https://example.test/manifest.mpd", Token: "tok"}
	}
	playParseManifest = func(string) *mpd.MPD {
		return fakePlayMPD(segs, secPerSeg)
	}
	playGetLicense = func(string, string, string) error { return nil }
	playDownloadPart = func(string) ([]byte, error) { return []byte("enc"), nil }
	playDecryptFragment = func([]byte, []byte, []*widevine.Key) ([]byte, error) {
		return fakeFragment(), nil
	}
	playClosePlayback = func(string, string) bool { return true }
	return func() {
		playOpenPlayback = origOpen
		playParseManifest = origParse
		playGetLicense = origLic
		playDownloadPart = origDL
		playDecryptFragment = origDec
		playClosePlayback = origClose
	}
}

func fakePlayMPD(segs int, secPerSeg float64) *mpd.MPD {
	if segs < 1 {
		segs = 1
	}
	timescale := uint64(1000)
	dur := uint64(secPerSeg * float64(timescale))
	r := int64(segs - 1)
	pssh := "AAAA"
	vid := "video/avc1/1080p"
	aid := "audio/aac/192k"
	height := uint64(1080)
	bw := uint64(192000)
	vInit := "init.mp4"
	vMedia := "seg_$Number$.m4s"
	aInit := "audio_init.mp4"
	aMedia := "audio_$Number$.m4s"
	vBase := "https://example.test/v/"
	aBase := "https://example.test/a/"
	video := &mpd.AdaptationSet{
		SegmentTemplate: &mpd.SegmentTemplate{
			Timescale:      &timescale,
			Initialization: &vInit,
			Media:          &vMedia,
			SegmentTimeline: &mpd.SegmentTimeline{
				S: []*mpd.SegmentTimelineS{{D: dur, R: &r}},
			},
		},
		ContentProtections: []mpd.Descriptor{{CencPSSH: &pssh}},
		Representations: []mpd.Representation{{
			ID:      &vid,
			Height:  &height,
			BaseURL: []*mpd.BaseURL{{Value: vBase}},
		}},
	}
	audio := &mpd.AdaptationSet{
		SegmentTemplate: &mpd.SegmentTemplate{
			Timescale:      &timescale,
			Initialization: &aInit,
			Media:          &aMedia,
			SegmentTimeline: &mpd.SegmentTimeline{
				S: []*mpd.SegmentTimelineS{{D: dur, R: &r}},
			},
		},
		Representations: []mpd.Representation{{
			ID:        &aid,
			Bandwidth: &bw,
			BaseURL:   []*mpd.BaseURL{{Value: aBase}},
		}},
	}
	return &mpd.MPD{Period: []*mpd.Period{{
		AdaptationSets: []*mpd.AdaptationSet{video, audio},
	}}}
}
