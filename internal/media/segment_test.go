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

	if filename, err := DownloadParts(ctx, client, &baseURL, &representationID, set, nil, 1); err == nil {
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
