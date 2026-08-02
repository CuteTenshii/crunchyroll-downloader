package engine

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/unki2aut/go-mpd"
)

var manifestHTTPClient = &http.Client{Timeout: providerHTTPTimeout}

func parseManifest(url string) *mpd.MPD {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := manifestHTTPClient.Do(req)
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

	if activeConfig.DebugManifest {
		fmt.Printf("\n%s\n", string(body))
	}

	return mpd
}

func getBaseUrl(set *mpd.AdaptationSet, isVideoSet bool, quality string) (*string, *string) {
	var bestVideo *mpd.Representation
	var bestVideoHeight uint64
	requestedVideoHeight, requestedVideoHeightErr := strconv.ParseUint(strings.TrimSuffix(quality, "p"), 10, 64)

	for representationIndex := range set.Representations {
		representation := &set.Representations[representationIndex]
		if isVideoSet {
			if representation.Height == nil || len(representation.BaseURL) == 0 || representation.ID == nil {
				continue
			}
			if requestedVideoHeightErr == nil && *representation.Height == requestedVideoHeight {
				return &representation.BaseURL[0].Value, representation.ID
			}
			if *representation.Height > bestVideoHeight {
				bestVideo = representation
				bestVideoHeight = *representation.Height
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
	if isVideoSet && bestVideo != nil {
		fmt.Printf("Video quality %s not found, deferring to %dp (%s)\n", quality, bestVideoHeight, *bestVideo.ID)
		return &bestVideo.BaseURL[0].Value, bestVideo.ID
	}
	if len(set.Representations) == 0 {
		return nil, nil
	}
	firstRep := set.Representations[0]
	if len(firstRep.BaseURL) == 0 || firstRep.ID == nil {
		return nil, nil
	}
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
