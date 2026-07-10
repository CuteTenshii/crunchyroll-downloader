package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unki2aut/go-mpd"
)

type fakeDoer func(*http.Request) (*http.Response, error)

func (f fakeDoer) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func okResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestDownloadPartUsesInjectedClient(t *testing.T) {
	var called bool
	client := fakeDoer(func(req *http.Request) (*http.Response, error) {
		called = true
		if req.URL.String() != "https://media.example/segment.m4s" {
			t.Fatalf("request URL = %q, want injected segment URL", req.URL.String())
		}
		if got := req.Header.Get("Origin"); got != "https://static.crunchyroll.com" {
			t.Fatalf("Origin header = %q, want Crunchyroll static origin", got)
		}
		return okResponse("segment-data"), nil
	})

	data, err := DownloadPart(context.Background(), client, "https://media.example/segment.m4s")
	if err != nil {
		t.Fatalf("DownloadPart() error = %v", err)
	}
	if !called {
		t.Fatal("DownloadPart() did not use injected client")
	}
	if string(data) != "segment-data" {
		t.Fatalf("DownloadPart() data = %q, want segment-data", string(data))
	}
}

func TestDownloadPartStopsRetryBackoffWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	client := fakeDoer(func(req *http.Request) (*http.Response, error) {
		calls++
		cancel()
		return nil, errors.New("temporary network failure")
	})

	_, err := DownloadPart(ctx, client, "https://media.example/segment.m4s")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DownloadPart() error = %v, want context.Canceled", err)
	}
	if calls != 1 {
		t.Fatalf("client calls = %d, want one attempt before canceled backoff", calls)
	}
}

func TestDownloadSubsUsesInjectedClientAndTempFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)

	var called bool
	client := fakeDoer(func(req *http.Request) (*http.Response, error) {
		called = true
		if req.URL.String() != "https://subs.example/subs.ass" {
			t.Fatalf("request URL = %q, want subtitle URL", req.URL.String())
		}
		return okResponse("[Script Info]\n"), nil
	})

	filename, err := DownloadSubs(context.Background(), client, "https://subs.example/subs.ass")
	if err != nil {
		t.Fatalf("DownloadSubs() error = %v", err)
	}
	defer os.Remove(filename)

	if !called {
		t.Fatal("DownloadSubs() did not use injected client")
	}
	if filepath.Dir(filename) != tempDir {
		t.Fatalf("DownloadSubs() filename dir = %q, want %q", filepath.Dir(filename), tempDir)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("reading subtitle temp file: %v", err)
	}
	if string(data) != "[Script Info]\n" {
		t.Fatalf("subtitle file data = %q, want [Script Info]", string(data))
	}
}

func TestGetFilenameReturnsCreateTempError(t *testing.T) {
	missingTempDir := filepath.Join(t.TempDir(), "missing")
	t.Setenv("TMPDIR", missingTempDir)

	if filename, err := getFilename(nil); err == nil {
		t.Fatalf("getFilename() = %q, nil error; want CreateTemp error", filename)
	}
}

func TestGetFilenameRejectsEmptyAdaptationSet(t *testing.T) {
	if filename, err := getFilename(&mpd.AdaptationSet{}); err == nil {
		t.Fatalf("getFilename() = %q, nil error; want empty adaptation set error", filename)
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		name string
		bps  float64
		want string
	}{
		{name: "zero bps", bps: 0, want: "0 B/s"},
		{name: "bytes per second", bps: 500, want: "500 B/s"},
		{name: "KB/s boundary", bps: 1024, want: "1 KB/s"},
		{name: "KB/s", bps: 2048, want: "2 KB/s"},
		{name: "MB/s boundary", bps: 1048576, want: "1.0 MB/s"},
		{name: "MB/s", bps: 5242880, want: "5.0 MB/s"},
		{name: "GB/s boundary", bps: 1073741824, want: "1.0 GB/s"},
		{name: "GB/s", bps: 2147483648, want: "2.0 GB/s"},
		{name: "negative bps", bps: -100, want: "-100 B/s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSpeed(tt.bps)
			if got != tt.want {
				t.Fatalf("formatSpeed(%f) = %q, want %q", tt.bps, got, tt.want)
			}
		})
	}
}

func TestFormatETAShort(t *testing.T) {
	tests := []struct {
		name string
		secs int
		want string
	}{
		{name: "zero seconds", secs: 0, want: "0s"},
		{name: "thirty seconds", secs: 30, want: "30s"},
		{name: "one minute", secs: 60, want: "1m 0s"},
		{name: "two minutes thirty", secs: 150, want: "2m 30s"},
		{name: "negative seconds", secs: -1, want: "0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatETAShort(tt.secs)
			if got != tt.want {
				t.Fatalf("formatETAShort(%d) = %q, want %q", tt.secs, got, tt.want)
			}
		})
	}
}

func TestGetFilename(t *testing.T) {
	height := uint64(1080)
	bandwidth := uint64(192000)
	videoRepID := "video/1080p"
	audioRepID := "audio/192k"

	t.Run("nil set uses subs pattern", func(t *testing.T) {
		filename, err := getFilename(nil)
		if err != nil {
			t.Fatalf("getFilename(nil) error = %v", err)
		}
		if !strings.Contains(filename, "crdl-subs-") {
			t.Fatalf("getFilename(nil) = %q, want crdl-subs- prefix", filename)
		}
	})

	t.Run("video representation uses video pattern", func(t *testing.T) {
		set := &mpd.AdaptationSet{
			Representations: []mpd.Representation{
				{ID: &videoRepID, Height: &height},
			},
		}
		filename, err := getFilename(set)
		if err != nil {
			t.Fatalf("getFilename(video) error = %v", err)
		}
		if !strings.Contains(filename, "crdl-video-") {
			t.Fatalf("getFilename(video) = %q, want crdl-video- prefix", filename)
		}
	})

	t.Run("audio representation uses audio pattern", func(t *testing.T) {
		set := &mpd.AdaptationSet{
			Representations: []mpd.Representation{
				{ID: &audioRepID, Bandwidth: &bandwidth},
			},
		}
		filename, err := getFilename(set)
		if err != nil {
			t.Fatalf("getFilename(audio) error = %v", err)
		}
		if !strings.Contains(filename, "crdl-audio-") {
			t.Fatalf("getFilename(audio) = %q, want crdl-audio- prefix", filename)
		}
	})

	t.Run("empty set returns error", func(t *testing.T) {
		_, err := getFilename(&mpd.AdaptationSet{})
		if err == nil {
			t.Fatal("getFilename(empty) error = nil, want error")
		}
	})
}

func TestDownloadPartsRemovesEncryptedTempOnSegmentFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseURL := "https://media.example/"
	representationID := "video/1080p"
	initPattern := "init-$RepresentationID$.mp4"
	mediaPattern := "seg-$Number%05d$-$RepresentationID$.m4s"
	height := uint64(1080)
	set := &mpd.AdaptationSet{
		SegmentTemplate: &mpd.SegmentTemplate{
			Initialization: &initPattern,
			Media:          &mediaPattern,
			SegmentTimeline: &mpd.SegmentTimeline{
				S: []*mpd.SegmentTimelineS{{D: 1}},
			},
		},
		Representations: []mpd.Representation{{Height: &height}},
	}

	client := fakeDoer(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "init-") {
			return okResponse("init"), nil
		}
		cancel()
		return nil, errors.New("segment failed")
	})

	if filename, err := DownloadParts(ctx, client, &baseURL, &representationID, set, nil, 1, "test"); err == nil {
		t.Fatalf("DownloadParts() = %q, nil error; want segment failure", filename)
	}

	matches, err := filepath.Glob(filepath.Join(tempDir, "crdl-encrypted-*.mp4"))
	if err != nil {
		t.Fatalf("glob encrypted temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("encrypted temp files left behind: %v", matches)
	}
}
