package media

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iyear/gowidevine"
	"github.com/unki2aut/go-mpd"
)

const defaultWorkers = 10

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type segmentJob struct {
	index int
	url   string
}

type segmentResult struct {
	index int
	data  []byte
}

func BuildUrl(base, representationId, file string, partNum *int64) string {
	if partNum != nil {
		file = strings.ReplaceAll(file, "$Number$", fmt.Sprintf("%05d", *partNum))
		file = strings.ReplaceAll(file, "$Number%05d$", fmt.Sprintf("%05d", *partNum))
	}
	return base + strings.ReplaceAll(file, "$RepresentationID$", representationId)
}

func DownloadPart(ctx context.Context, client httpDoer, url string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Origin", "https://static.crunchyroll.com")
		req.Header.Set("Referer", "https://static.crunchyroll.com/")
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")

		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxRetries-1 {
				continue
			}
			return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			if attempt < maxRetries-1 {
				continue
			}
			return nil, fmt.Errorf("failed after %d retries, status: %d", maxRetries, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if attempt < maxRetries-1 {
				continue
			}
			return nil, fmt.Errorf("failed reading body after %d retries: %w", maxRetries, err)
		}
		return body, nil
	}
	return nil, fmt.Errorf("failed after %d retries", maxRetries)
}

func getFilename(set *mpd.AdaptationSet) string {
	if set == nil {
		f, _ := os.CreateTemp("", "crdl-subs-*.ass")
		return f.Name()
	}
	for _, representation := range set.Representations {
		if representation.Height != nil {
			f, _ := os.CreateTemp("", "crdl-video-*.mp4")
			return f.Name()
		} else if representation.Bandwidth != nil {
			f, _ := os.CreateTemp("", "crdl-audio-*.mp3")
			return f.Name()
		}
	}
	return ""
}

func DownloadParts(ctx context.Context, client httpDoer, baseUrl, representationId *string, set *mpd.AdaptationSet, keys []*widevine.Key, workers int) (string, error) {
	if workers <= 0 {
		workers = defaultWorkers
	}
	if ctx == nil {
		ctx = context.Background()
	}

	initUrl := BuildUrl(*baseUrl, *representationId, *set.SegmentTemplate.Initialization, nil)
	initData, err := DownloadPart(ctx, client, initUrl)
	if err != nil {
		return "", err
	}

	encryptedFile, err := os.CreateTemp("", "crdl-encrypted-*.mp4")
	if err != nil {
		return "", err
	}
	encryptedFilename := encryptedFile.Name()
	defer os.Remove(encryptedFilename)

	if _, err := encryptedFile.Write(initData); err != nil {
		encryptedFile.Close()
		return "", err
	}

	timeline := ExpandTimeline(set.SegmentTemplate.SegmentTimeline.S, 1)
	total := len(timeline)
	results := make(chan segmentResult, total)
	var downloadErr error
	var errOnce sync.Once
	var done atomic.Int64
	downloadCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan segmentJob, total)
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				data, err := DownloadPart(downloadCtx, client, job.url)
				if err != nil {
					errOnce.Do(func() {
						downloadErr = err
						cancel()
					})
					return
				}
				results <- segmentResult{index: job.index, data: data}
				count := done.Add(1)
				fmt.Printf("\rDownloaded %v of %v segments (%v%%)", count, total, (100*count)/int64(total))
			}
		}()
	}

	for i, item := range timeline {
		url := BuildUrl(*baseUrl, *representationId, *set.SegmentTemplate.Media, &item)
		jobs <- segmentJob{index: i, url: url}
	}
	close(jobs)
	wg.Wait()
	close(results)

	if downloadErr != nil {
		encryptedFile.Close()
		return "", downloadErr
	}

	nextIndex := 0
	pending := make(map[int][]byte)
	for result := range results {
		pending[result.index] = result.data
		for {
			data, ok := pending[nextIndex]
			if !ok {
				break
			}
			if _, err := encryptedFile.Write(data); err != nil {
				encryptedFile.Close()
				return "", err
			}
			delete(pending, nextIndex)
			nextIndex++
		}
	}
	if nextIndex != total {
		encryptedFile.Close()
		return "", fmt.Errorf("downloaded %d of %d segments", nextIndex, total)
	}
	if err := encryptedFile.Close(); err != nil {
		return "", err
	}

	fmt.Println("\nFinished downloading!")

	filename := getFilename(set)
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Failed to close output file %s: %v\n", filename, err)
		}
	}()

	encryptedInput, err := os.Open(encryptedFilename)
	if err != nil {
		os.Remove(filename)
		return "", err
	}
	defer encryptedInput.Close()

	if err := widevine.DecryptMP4Auto(encryptedInput, keys, file); err != nil {
		os.Remove(filename)
		return "", fmt.Errorf("widevine.DecryptMP4Auto: %w", err)
	}

	return filename, nil
}

func DownloadSubs(ctx context.Context, client httpDoer, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	filename := getFilename(nil)
	if err := os.WriteFile(filename, body, 0644); err != nil {
		return "", err
	}

	return filename, nil
}
