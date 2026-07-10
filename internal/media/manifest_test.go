package media

import (
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/unki2aut/go-mpd"
)

func TestGetBaseUrlRejectsEmptyAdaptationSet(t *testing.T) {
	baseURL, representationID := GetVideoBaseUrl(&mpd.AdaptationSet{}, "1080p")
	if baseURL != nil || representationID != nil {
		t.Fatalf("GetVideoBaseUrl(empty) = %v, %v; want nil, nil", baseURL, representationID)
	}
}

func TestGetBaseUrlSkipsMalformedRepresentation(t *testing.T) {
	baseURL, representationID := GetVideoBaseUrl(&mpd.AdaptationSet{
		Representations: []mpd.Representation{{}},
	}, "1080p")
	if baseURL != nil || representationID != nil {
		t.Fatalf("GetVideoBaseUrl(malformed) = %v, %v; want nil, nil", baseURL, representationID)
	}
}

func TestGetVideoBaseUrlMatchesHeight(t *testing.T) {
	h720 := uint64(720)
	id720 := "vid-720"
	h1080 := uint64(1080)
	id1080 := "vid-1080"

	set := &mpd.AdaptationSet{
		Representations: []mpd.Representation{
			{
				ID:     &id720,
				Height: &h720,
				BaseURL: []*mpd.BaseURL{
					{Value: "https://example.com/video/720p"},
				},
			},
			{
				ID:     &id1080,
				Height: &h1080,
				BaseURL: []*mpd.BaseURL{
					{Value: "https://example.com/video/1080p"},
				},
			},
		},
	}

	t.Run("matches 1080p by height", func(t *testing.T) {
		baseURL, repID := GetVideoBaseUrl(set, "1080p")
		if baseURL == nil || repID == nil {
			t.Fatal("GetVideoBaseUrl(1080p) = nil, nil; want non-nil")
		}
		if *baseURL != "https://example.com/video/1080p" {
			t.Fatalf("GetVideoBaseUrl(1080p) baseURL = %s; want https://example.com/video/1080p", *baseURL)
		}
		if *repID != "vid-1080" {
			t.Fatalf("GetVideoBaseUrl(1080p) repID = %s; want vid-1080", *repID)
		}
	})

	t.Run("matches 720p by height", func(t *testing.T) {
		baseURL, repID := GetVideoBaseUrl(set, "720p")
		if baseURL == nil || repID == nil {
			t.Fatal("GetVideoBaseUrl(720p) = nil, nil; want non-nil")
		}
		if *baseURL != "https://example.com/video/720p" {
			t.Fatalf("GetVideoBaseUrl(720p) baseURL = %s; want https://example.com/video/720p", *baseURL)
		}
		if *repID != "vid-720" {
			t.Fatalf("GetVideoBaseUrl(720p) repID = %s; want vid-720", *repID)
		}
	})

	t.Run("returns nil for unmatched height", func(t *testing.T) {
		baseURL, repID := GetVideoBaseUrl(set, "480p")
		if baseURL == nil || repID == nil {
			t.Fatal("GetVideoBaseUrl(480p) = nil, nil; want fallback (first rep)")
		}
		if *baseURL != "https://example.com/video/720p" {
			t.Fatalf("GetVideoBaseUrl(480p) baseURL = %s; want fallback to first rep", *baseURL)
		}
	})
}

func TestBuildUrl(t *testing.T) {
	tests := []struct {
		name             string
		base             string
		representationID string
		file             string
		partNum          *int64
		want             string
	}{
		{
			name:             "replace $Number$",
			base:             "https://example.com/",
			representationID: "video/1080p",
			file:             "seg-$Number$-$RepresentationID$.m4s",
			partNum:          int64Ptr(1),
			want:             "https://example.com/seg-00001-video/1080p.m4s",
		},
		{
			name:             "replace $Number%05d$",
			base:             "https://example.com/",
			representationID: "video/1080p",
			file:             "seg-$Number%05d$-$RepresentationID$.m4s",
			partNum:          int64Ptr(42),
			want:             "https://example.com/seg-00042-video/1080p.m4s",
		},
		{
			name:             "replace $RepresentationID$ only",
			base:             "https://example.com/",
			representationID: "audio/192k",
			file:             "init-$RepresentationID$.mp4",
			partNum:          nil,
			want:             "https://example.com/init-audio/192k.mp4",
		},
		{
			name:             "nil partNum leaves $Number$ unchanged",
			base:             "https://example.com/",
			representationID: "rep1",
			file:             "seg-$Number$-$RepresentationID$.m4s",
			partNum:          nil,
			want:             "https://example.com/seg-$Number$-rep1.m4s",
		},
		{
			name:             "empty base URL",
			base:             "",
			representationID: "rep1",
			file:             "file.mp4",
			partNum:          nil,
			want:             "file.mp4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildUrl(tt.base, tt.representationID, tt.file, tt.partNum)
			if got != tt.want {
				t.Fatalf("BuildUrl() = %q, want %q", got, tt.want)
			}
		})
	}
}

func int64Ptr(v int64) *int64 { return &v }

func TestExpandTimeline(t *testing.T) {
	t.Run("single S element R=0", func(t *testing.T) {
		timeline := []*mpd.SegmentTimelineS{{D: 1}}
		result := ExpandTimeline(timeline, 1)
		want := []int64{1}
		if !reflect.DeepEqual(result, want) {
			t.Fatalf("ExpandTimeline() = %v, want %v", result, want)
		}
	})

	t.Run("S element with R=2", func(t *testing.T) {
		r := int64(2)
		timeline := []*mpd.SegmentTimelineS{{D: 1, R: &r}}
		result := ExpandTimeline(timeline, 1)
		want := []int64{1, 2, 3}
		if !reflect.DeepEqual(result, want) {
			t.Fatalf("ExpandTimeline() = %v, want %v", result, want)
		}
	})

	t.Run("nil R treated as zero repeat", func(t *testing.T) {
		timeline := []*mpd.SegmentTimelineS{{D: 1, R: nil}}
		result := ExpandTimeline(timeline, 1)
		want := []int64{1}
		if !reflect.DeepEqual(result, want) {
			t.Fatalf("ExpandTimeline(R=nil) = %v, want %v", result, want)
		}
	})

	t.Run("empty timeline", func(t *testing.T) {
		result := ExpandTimeline(nil, 1)
		if len(result) != 0 {
			t.Fatalf("ExpandTimeline(empty) = %v, want empty slice", result)
		}
	})

	t.Run("multiple S elements", func(t *testing.T) {
		timeline := []*mpd.SegmentTimelineS{
			{D: 1},
			{D: 2},
		}
		result := ExpandTimeline(timeline, 5)
		want := []int64{5, 6}
		if !reflect.DeepEqual(result, want) {
			t.Fatalf("ExpandTimeline() = %v, want %v", result, want)
		}
	})

	t.Run("zero D value", func(t *testing.T) {
		timeline := []*mpd.SegmentTimelineS{{D: 0}}
		result := ExpandTimeline(timeline, 10)
		want := []int64{10}
		if !reflect.DeepEqual(result, want) {
			t.Fatalf("ExpandTimeline(D=0) = %v, want %v", result, want)
		}
	})
}

func TestParseManifest(t *testing.T) {
	data, err := os.ReadFile("../media/testdata/mpd/simple-video-audio.mpd")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if len(m.Period) == 0 {
		t.Fatal("ParseManifest() produced MPD with no periods")
	}
	sets := m.Period[0].AdaptationSets
	if len(sets) != 2 {
		t.Fatalf("ParseManifest() has %d adaptation sets, want 2", len(sets))
	}
}

func TestParseManifestInvalidXML(t *testing.T) {
	_, err := ParseManifest([]byte("invalid"))
	if err == nil {
		t.Fatal("ParseManifest(invalid) error = nil, want decode error")
	}
}

func TestGetAudioBaseUrlBandwidth192k(t *testing.T) {
	bw := uint64(200000)
	repID := "audio-192"
	set := &mpd.AdaptationSet{
		Representations: []mpd.Representation{
			{
				ID:        &repID,
				Bandwidth: &bw,
				BaseURL: []*mpd.BaseURL{
					{Value: "https://example.com/audio/192k"},
				},
			},
		},
	}
	baseURL, rid := GetAudioBaseUrl(set, "192k")
	if baseURL == nil || rid == nil {
		t.Fatal("GetAudioBaseUrl(192k) = nil, nil; want non-nil")
	}
	if *baseURL != "https://example.com/audio/192k" {
		t.Fatalf("GetAudioBaseUrl(192k) baseURL = %s; want https://example.com/audio/192k", *baseURL)
	}
}

func TestGetAudioBaseUrlBandwidth128k(t *testing.T) {
	bw := uint64(150000)
	repID := "audio-128"
	set := &mpd.AdaptationSet{
		Representations: []mpd.Representation{
			{
				ID:        &repID,
				Bandwidth: &bw,
				BaseURL: []*mpd.BaseURL{
					{Value: "https://example.com/audio/128k"},
				},
			},
		},
	}
	baseURL, rid := GetAudioBaseUrl(set, "128k")
	if baseURL == nil || rid == nil {
		t.Fatal("GetAudioBaseUrl(128k) = nil, nil; want non-nil")
	}
	if *baseURL != "https://example.com/audio/128k" {
		t.Fatalf("GetAudioBaseUrl(128k) baseURL = %s; want https://example.com/audio/128k", *baseURL)
	}
}

func TestGetAudioBaseUrlBandwidth96k(t *testing.T) {
	bw := uint64(100000)
	repID := "audio-96"
	set := &mpd.AdaptationSet{
		Representations: []mpd.Representation{
			{
				ID:        &repID,
				Bandwidth: &bw,
				BaseURL: []*mpd.BaseURL{
					{Value: "https://example.com/audio/96k"},
				},
			},
		},
	}
	baseURL, rid := GetAudioBaseUrl(set, "96k")
	if baseURL == nil || rid == nil {
		t.Fatal("GetAudioBaseUrl(96k) = nil, nil; want non-nil")
	}
	if *baseURL != "https://example.com/audio/96k" {
		t.Fatalf("GetAudioBaseUrl(96k) baseURL = %s; want https://example.com/audio/96k", *baseURL)
	}
}

func TestGetAudioBaseUrlFallback(t *testing.T) {
	repID := "fallback-rep"
	set := &mpd.AdaptationSet{
		Representations: []mpd.Representation{
			{
				ID: &repID,
				BaseURL: []*mpd.BaseURL{
					{Value: "https://example.com/audio/fallback"},
				},
				// No Bandwidth — won't match any switch case
			},
		},
	}

	t.Run("unmatched quality returns first representation", func(t *testing.T) {
		baseURL, rid := GetAudioBaseUrl(set, "999k")
		if baseURL == nil || rid == nil {
			t.Fatal("GetAudioBaseUrl(fallback) = nil, nil; want fallback match")
		}
		if *baseURL != "https://example.com/audio/fallback" {
			t.Fatalf("expected fallback URL, got %s", *baseURL)
		}
		if *rid != "fallback-rep" {
			t.Fatalf("expected fallback-rep, got %s", *rid)
		}
	})

	t.Run("empty set returns nil", func(t *testing.T) {
		baseURL, rid := GetAudioBaseUrl(&mpd.AdaptationSet{}, "192k")
		if baseURL != nil || rid != nil {
			t.Fatalf("GetAudioBaseUrl(empty) = %v, %v; want nil, nil", baseURL, rid)
		}
	})
}

func TestMPDCacheMiss(t *testing.T) {
	got := GetCachedManifest("cache-miss-unknown")
	if got != nil {
		t.Fatalf("GetCachedManifest(unknown) = %v, want nil", got)
	}
}

func TestMPDCacheHit(t *testing.T) {
	const key = "cache-hit-test"
	manifest := &mpd.MPD{}
	SetCachedManifest(key, manifest)
	got := GetCachedManifest(key)
	if got != manifest {
		t.Fatalf("GetCachedManifest(%s) = %v, want same pointer %v", key, got, manifest)
	}
}

func TestMPDCacheConcurrent(t *testing.T) {
	const numWorkers = 10
	var wg sync.WaitGroup

	// Pre-populate some keys
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("prepop-conc-%d", i)
		SetCachedManifest(key, &mpd.MPD{})
	}

	// errs collects failures from goroutines without calling t.Errorf across goroutines
	errs := make(chan string, numWorkers*2)

	// Concurrent readers that verify Get returns non-nil for known keys
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		i := i
		go func() {
			defer wg.Done()
			key := fmt.Sprintf("prepop-conc-%d", i%5)
			m := GetCachedManifest(key)
			if m == nil {
				errs <- fmt.Sprintf("concurrent read of %s returned nil", key)
			}
		}()
	}

	// Concurrent writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		SetCachedManifest("concurrent-write", &mpd.MPD{})
	}()

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	got := GetCachedManifest("concurrent-write")
	if got == nil {
		t.Fatal("concurrent-write key should exist after Set")
	}
}

func TestMPDCacheMultipleKeys(t *testing.T) {
	m1 := &mpd.MPD{}
	m2 := &mpd.MPD{}

	SetCachedManifest("key-a", m1)
	SetCachedManifest("key-b", m2)

	gotA := GetCachedManifest("key-a")
	gotB := GetCachedManifest("key-b")

	if gotA != m1 {
		t.Fatalf("key-a: got %v, want %v", gotA, m1)
	}
	if gotB != m2 {
		t.Fatalf("key-b: got %v, want %v", gotB, m2)
	}
	if gotA == gotB {
		t.Fatal("key-a and key-b should be different pointers")
	}
}


