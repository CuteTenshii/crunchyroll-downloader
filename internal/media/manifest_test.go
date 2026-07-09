package media

import (
	"fmt"
	"sync"
	"testing"

	"github.com/unki2aut/go-mpd"
)

func TestGetBaseUrlRejectsEmptyAdaptationSet(t *testing.T) {
	baseURL, representationID := GetBaseUrl(&mpd.AdaptationSet{}, true, "1080p")
	if baseURL != nil || representationID != nil {
		t.Fatalf("GetBaseUrl(empty) = %v, %v; want nil, nil", baseURL, representationID)
	}
}

func TestGetBaseUrlSkipsMalformedRepresentation(t *testing.T) {
	baseURL, representationID := GetBaseUrl(&mpd.AdaptationSet{
		Representations: []mpd.Representation{{}},
	}, true, "1080p")
	if baseURL != nil || representationID != nil {
		t.Fatalf("GetBaseUrl(malformed) = %v, %v; want nil, nil", baseURL, representationID)
	}
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


