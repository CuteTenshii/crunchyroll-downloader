package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/unki2aut/go-mpd"
)

// parseManifest fetches and parses an MPD, returning both the decoded struct and
// the raw XML. The raw body is needed to parse the single-file "on demand"
// manifests whose SegmentBase byte ranges go-mpd does not model.
func parseManifest(url string) (*mpd.MPD, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	mpd := new(mpd.MPD)
	mpd.Decode(body)

	if *debug {
		fmt.Printf("\n%s\n", string(body))
	}

	return mpd, body, nil
}

func getBaseUrl(set *mpd.AdaptationSet, isVideoSet bool, quality string) (*string, *string) {
	for _, representation := range set.Representations {
		if isVideoSet {
			toInt, _ := strconv.ParseInt(strings.ReplaceAll(quality, "p", ""), 10, 64)
			if *representation.Height == uint64(toInt) {
				return &representation.BaseURL[0].Value, representation.ID
			}
		} else {
			if strings.Contains(*representation.ID, "audio/") {
				if strings.Contains(*representation.ID, quality) {
					return &representation.BaseURL[0].Value, representation.ID
				}
			} else if representation.Bandwidth != nil {
				num := strings.ReplaceAll(quality, "k", "")

				// Crunchyroll MPDs are weird on the "bandwidth" value, it can be 192002 (not just 192000) on certain manifests
				if num == "192" && *representation.Bandwidth >= 192000 {
					return &representation.BaseURL[0].Value, representation.ID
				} else if num == "128" && *representation.Bandwidth >= 128000 {
					return &representation.BaseURL[0].Value, representation.ID
				} else if num == "96" && *representation.Bandwidth >= 96000 {
					return &representation.BaseURL[0].Value, representation.ID
				}
			}
		}
	}
	if len(set.Representations) == 0 {
		return nil, nil
	}
	firstRep := set.Representations[0]
	fmt.Printf("Audio quality %s not found, deferring to %s\n", quality, *firstRep.ID)
	return &firstRep.BaseURL[0].Value, firstRep.ID
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
