package main

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// segmentURLs builds n placeholder urls whose index is recoverable by indexOf.
func segmentURLs(n int) []string {
	urls := make([]string, n)
	for i := range urls {
		urls[i] = fmt.Sprintf("seg-%d", i)
	}
	return urls
}

func indexOf(t *testing.T, url string) int {
	t.Helper()
	i, err := strconv.Atoi(strings.TrimPrefix(url, "seg-"))
	if err != nil {
		t.Fatalf("unparsable url %q: %v", url, err)
	}
	return i
}

// segmentBody makes each segment a distinct, variable-length payload so that a
// misordered or truncated write cannot coincidentally produce the right bytes.
func segmentBody(i int) []byte {
	return bytes.Repeat([]byte{byte('a' + i%26)}, 1+i%7)
}

func wantConcatenated(n int) []byte {
	var want []byte
	for i := 0; i < n; i++ {
		want = append(want, segmentBody(i)...)
	}
	return want
}

// runStream fails the test rather than hanging forever if streamSegments
// deadlocks, which is the failure mode these tests exist to catch.
func runStream(t *testing.T, w *lockedBuffer, urls []string, fetch func(string) ([]byte, error)) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- streamSegments(w, urls, fetch, nil) }()
	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("streamSegments deadlocked")
		return nil
	}
}

// lockedBuffer serialises writes only for race-detector cleanliness; the writer
// goroutine is the sole caller.
type lockedBuffer struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	onWrite  func()
	failFrom int
	writes   int
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.onWrite != nil {
		b.onWrite()
	}
	b.writes++
	if b.failFrom > 0 && b.writes >= b.failFrom {
		return 0, errors.New("disk full")
	}
	return b.buf.Write(p)
}

func (b *lockedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

func TestStreamSegmentsWritesInIndexOrder(t *testing.T) {
	const total = 200
	var w lockedBuffer

	// Jittered latencies make segments finish out of order, so a writer that
	// merely appends completions would produce scrambled output.
	fetch := func(url string) ([]byte, error) {
		i := indexOf(t, url)
		time.Sleep(time.Duration((total-i)%13) * time.Millisecond)
		return segmentBody(i), nil
	}

	if err := runStream(t, &w, segmentURLs(total), fetch); err != nil {
		t.Fatalf("streamSegments returned %v, want nil", err)
	}
	if got, want := w.Bytes(), wantConcatenated(total); !bytes.Equal(got, want) {
		t.Fatalf("output is not the segments concatenated in index order (got %d bytes, want %d)", len(got), len(want))
	}
}

// TestStreamSegmentsSurvivesSlowLeadSegment reproduces the deadlock that a
// naive semaphore introduces: if later segments can take every slot while
// segment 0 is still downloading, the writer waits on 0 forever and segment 0's
// worker waits on a slot forever.
func TestStreamSegmentsSurvivesSlowLeadSegment(t *testing.T) {
	total := maxBufferedSegments * 4
	var w lockedBuffer

	fetch := func(url string) ([]byte, error) {
		if indexOf(t, url) == 0 {
			time.Sleep(500 * time.Millisecond)
		}
		return segmentBody(indexOf(t, url)), nil
	}

	if err := runStream(t, &w, segmentURLs(total), fetch); err != nil {
		t.Fatalf("streamSegments returned %v, want nil", err)
	}
	if got, want := w.Bytes(), wantConcatenated(total); !bytes.Equal(got, want) {
		t.Fatalf("output mismatch: got %d bytes, want %d", len(got), len(want))
	}
}

// TestStreamSegmentsBoundsResidentSegments is the regression test for the OOM
// itself: the number of payloads held in memory must not scale with the number
// of segments.
func TestStreamSegmentsBoundsResidentSegments(t *testing.T) {
	total := maxBufferedSegments * 20
	var resident, peak atomic.Int64

	bump := func() {
		cur := resident.Add(1)
		for {
			old := peak.Load()
			if cur <= old || peak.CompareAndSwap(old, cur) {
				break
			}
		}
	}

	w := lockedBuffer{onWrite: func() { resident.Add(-1) }}

	fetch := func(url string) ([]byte, error) {
		i := indexOf(t, url)
		// Stall the lead segment so the workers have every chance to run ahead.
		if i == 0 {
			time.Sleep(300 * time.Millisecond)
		}
		bump()
		return segmentBody(i), nil
	}

	if err := runStream(t, &w, segmentURLs(total), fetch); err != nil {
		t.Fatalf("streamSegments returned %v, want nil", err)
	}
	if got := peak.Load(); got > maxBufferedSegments {
		t.Fatalf("held %d segments in memory at once, want at most %d", got, maxBufferedSegments)
	}
	if got, want := w.Bytes(), wantConcatenated(total); !bytes.Equal(got, want) {
		t.Fatalf("output mismatch: got %d bytes, want %d", len(got), len(want))
	}
}

func TestStreamSegmentsPropagatesFetchError(t *testing.T) {
	sentinel := errors.New("segment 7 is gone")
	var w lockedBuffer

	fetch := func(url string) ([]byte, error) {
		i := indexOf(t, url)
		if i == 7 {
			return nil, sentinel
		}
		return segmentBody(i), nil
	}

	err := runStream(t, &w, segmentURLs(maxBufferedSegments*4), fetch)
	if !errors.Is(err, sentinel) {
		t.Fatalf("streamSegments returned %v, want %v", err, sentinel)
	}
}

// A failing writer must abort the download rather than let the workers keep
// buffering segments that will never be drained.
func TestStreamSegmentsPropagatesWriteError(t *testing.T) {
	w := lockedBuffer{failFrom: 3}

	fetch := func(url string) ([]byte, error) {
		return segmentBody(indexOf(t, url)), nil
	}

	err := runStream(t, &w, segmentURLs(maxBufferedSegments*4), fetch)
	if err == nil {
		t.Fatal("streamSegments returned nil, want the writer's error")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("streamSegments returned %v, want it to wrap \"disk full\"", err)
	}
}

func TestStreamSegmentsNoURLs(t *testing.T) {
	var w lockedBuffer
	if err := runStream(t, &w, nil, func(string) ([]byte, error) {
		t.Fatal("fetch called with no urls")
		return nil, nil
	}); err != nil {
		t.Fatalf("streamSegments returned %v, want nil", err)
	}
	if len(w.Bytes()) != 0 {
		t.Fatalf("wrote %d bytes for an empty url list", len(w.Bytes()))
	}
}

func TestStreamSegmentsReportsProgress(t *testing.T) {
	const total = 50
	var w lockedBuffer
	var mu sync.Mutex
	var seen []int64

	err := streamSegments(&w, segmentURLs(total),
		func(url string) ([]byte, error) { return segmentBody(indexOf(t, url)), nil },
		func(fetched int64) {
			mu.Lock()
			defer mu.Unlock()
			seen = append(seen, fetched)
		})
	if err != nil {
		t.Fatalf("streamSegments returned %v, want nil", err)
	}
	if len(seen) != total {
		t.Fatalf("progress reported %d times, want %d", len(seen), total)
	}
	// The counter is shared, so the reported values must be exactly 1..total.
	mu.Lock()
	defer mu.Unlock()
	got := make(map[int64]bool, len(seen))
	for _, v := range seen {
		got[v] = true
	}
	for i := int64(1); i <= total; i++ {
		if !got[i] {
			t.Fatalf("progress never reported %d", i)
		}
	}
}

func TestBuildGuidByLocale(t *testing.T) {
	tests := []struct {
		name   string
		info   EpisodeInfo
		baseID string
		want   map[string]string
	}{
		{
			name: "versions are authoritative",
			info: EpisodeInfo{EpisodeMetadata: EpisodeMetadata{
				AudioLocale: "it-IT",
				Versions: []*DubVersion{
					{AudioLocale: "ja-JP", GUID: "ja-guid"},
					{AudioLocale: "it-IT", GUID: "it-guid"},
				},
			}},
			baseID: "base-id",
			want:   map[string]string{"ja-JP": "ja-guid", "it-IT": "it-guid"},
		},
		{
			name: "preferred dub absent from versions is not mapped",
			info: EpisodeInfo{EpisodeMetadata: EpisodeMetadata{
				AudioLocale: "it-IT",
				Versions: []*DubVersion{
					{AudioLocale: "ja-JP", GUID: "ja-guid"},
				},
			}},
			baseID: "base-id",
			want:   map[string]string{"ja-JP": "ja-guid"},
		},
		{
			name: "falls back to content id when no versions",
			info: EpisodeInfo{EpisodeMetadata: EpisodeMetadata{
				AudioLocale: "ja-JP",
			}},
			baseID: "base-id",
			want:   map[string]string{"ja-JP": "base-id"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildGuidByLocale(tc.info, tc.baseID)
			if len(got) != len(tc.want) {
				t.Fatalf("buildGuidByLocale() = %v, want %v", got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("buildGuidByLocale()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestMergeSubtitleAndCaptions(t *testing.T) {
	// Reproduces issue #48: en-US closed captions exist only on the English
	// dub (second version), not on the first (zh-CN) version.
	episodes := []Episode{
		{
			Subtitles: map[string]*Subtitle{
				"en-US": {Language: "en-US", Format: "ass", URL: "sub-en"},
			},
			Captions: map[string]*Subtitle{
				"zh-CN": {Language: "zh-CN", Format: "vtt", URL: "cc-zh"},
			},
		},
		{
			Subtitles: map[string]*Subtitle{
				"en-US": {Language: "en-US", Format: "ass", URL: "sub-en"},
			},
			Captions: map[string]*Subtitle{
				"en-US": {Language: "en-US", Format: "vtt", URL: "cc-en"},
			},
		},
	}

	subtitles, captions := mergeSubtitleAndCaptions(episodes)

	if subtitles["en-US"] == nil {
		t.Fatalf("en-US subtitle missing from merged map")
	}
	if captions["en-US"] == nil {
		t.Fatalf("en-US caption missing from merged map; should be pulled from the second version")
	}
	if captions["zh-CN"] == nil {
		t.Fatalf("zh-CN caption missing from merged map")
	}
}

func TestFilterAvailableLangs(t *testing.T) {
	available := map[string]*Subtitle{
		"en-US":  {},
		"es-419": {},
	}
	got := filterAvailableLangs([]string{"en-US", "hi-IN"}, available, "Subtitle", 0)
	if len(got) != 1 || got[0] != "en-US" {
		t.Fatalf("filterAvailableLangs() = %v, want [en-US]", got)
	}

	if got := filterAvailableLangs(nil, available, "Subtitle", 0); got != nil {
		t.Fatalf("filterAvailableLangs(nil) = %v, want nil", got)
	}
}
