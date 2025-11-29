package main

import (
	"io"
	"net/http"
	"strings"

	"github.com/unki2aut/go-mpd"
)

func parseManifest(url string) *mpd.MPD {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	mpd := new(mpd.MPD)
	mpd.Decode(body)

	return mpd
}

func getBaseUrl(manifest *mpd.MPD, mimeType, quality string) (*string, *string) {
	set := findSet(manifest.Period[0].AdaptationSets, mimeType)
	if set == nil {
		return nil, nil
	}
	for _, representation := range set.Representations {
		// ID is something like "video/avc1/1080p-1747708204"
		if strings.Contains(*representation.ID, quality) {
			return &representation.BaseURL[0].Value, representation.ID
		}
	}
	return nil, nil
}

func expandTimeline(timeline []*mpd.SegmentTimelineS, startNumber int64) []int64 {
	var result []int64
	segNum := startNumber

	for _, s := range timeline {
		repeat := int64(0)
		if s.R != nil && *s.R > 0 {
			repeat = *s.R
		}

		total := repeat + 1 // DASH rule: total segments = r + 1

		for i := int64(0); i < total; i++ {
			result = append(result, segNum)
			segNum++
		}
	}

	return result
}
