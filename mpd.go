package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	return mpd, body, nil
}

func getBaseUrl(set *mpd.AdaptationSet, isVideoSet bool, quality string) (*string, *string) {
	if set == nil {
		return nil, nil
	}
	if isVideoSet || quality == "best" {
		bestIndex := -1
		var bestHeight, bestBandwidth uint64

		for i := range set.Representations {
			representation := &set.Representations[i]

			if representation.ID == nil || len(representation.BaseURL) == 0 || strings.TrimSpace(representation.BaseURL[0].Value) == "" {
				continue
			}
			height, bandwidth := uintValue(representation.Height), uintValue(representation.Bandwidth)
			if preferRepresentation(quality, isVideoSet, height, bandwidth, bestHeight, bestBandwidth, bestIndex >= 0) {
				bestIndex, bestHeight, bestBandwidth = i, height, bandwidth
			}
		}

		if bestIndex != -1 {
			bestRepresentation := &set.Representations[bestIndex]

			return &bestRepresentation.BaseURL[0].Value, bestRepresentation.ID
		}

		return nil, nil
	}

	// Audio
	for _, representation := range set.Representations {
		if representation.ID == nil || len(representation.BaseURL) == 0 {
			continue
		}
		if strings.Contains(*representation.ID, "audio/") {
			if strings.Contains(*representation.ID, quality) {
				return &representation.BaseURL[0].Value, representation.ID
			}
		} else if representation.Bandwidth != nil {
			num := strings.ReplaceAll(quality, "k", "")

			// Crunchyroll MPDs are weird on the "bandwidth" value,
			// it can be 192002 (not just 192000) on certain manifests
			if num == "192" && *representation.Bandwidth >= 192000 {
				return &representation.BaseURL[0].Value, representation.ID
			} else if num == "128" && *representation.Bandwidth >= 128000 {
				return &representation.BaseURL[0].Value, representation.ID
			} else if num == "96" && *representation.Bandwidth >= 96000 {
				return &representation.BaseURL[0].Value, representation.ID
			}
		}
	}

	if len(set.Representations) == 0 {
		return nil, nil
	}

	firstRep := set.Representations[0]
	if firstRep.ID == nil || len(firstRep.BaseURL) == 0 {
		return nil, nil
	}
	fmt.Printf("Audio quality %s not found, deferring to %s\n", quality, *firstRep.ID)

	return &firstRep.BaseURL[0].Value, firstRep.ID
}

// Shared ranking for SegmentTemplate and SegmentBase. Unknown height/bandwidth
// cannot establish a best choice; explicit video heights still allow zero bandwidth.
func preferRepresentation(quality string, video bool, height, bandwidth, bestHeight, bestBandwidth uint64, found bool) bool {
	if quality == "best" {
		if bandwidth == 0 || (video && height == 0) {
			return false
		}
		if video && found && height != bestHeight {
			return height > bestHeight
		}
		return !found || bandwidth > bestBandwidth
	}
	if !video {
		return false
	}
	target, err := strconv.ParseUint(strings.TrimSuffix(quality, "p"), 10, 64)
	return err == nil && target > 0 && height == target && (!found || bandwidth > bestBandwidth)
}

func uintValue(value *uint64) uint64 {
	if value == nil {
		return 0
	}
	return *value
}

// Omit URL credentials, query parameters and fragments from diagnostics.
// The original, unmodified URL is still used for the request.
func videoDiagnosticURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[invalid URL omitted]"
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	return u.String()
}

func logSelectedVideo(width, height, bandwidth uint64, rawURL string) {
	if !*debug {
		return
	}
	fmt.Printf("Selected video representation:\nResolution: %dx%d\nBandwidth: %d\nURL (credentials/query/fragment omitted): %s\n",
		width, height, bandwidth, videoDiagnosticURL(rawURL))
}

func logSelectedAudio(locale []string, bandwidth uint64, codec string) {
	if !*debug {
		return
	}
	language := "unknown"
	if len(locale) > 0 {
		language = locale[0]
	}
	fmt.Printf("Selected audio representation:\nLanguage: %s\nBandwidth: %d\nCodec: %s\n", language, bandwidth, codec)
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
