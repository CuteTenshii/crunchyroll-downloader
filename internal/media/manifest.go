package media

import (
	"strconv"
	"strings"
	"sync"

	"github.com/unki2aut/go-mpd"
)

// mpdCache is a thread-safe read-mostly cache for parsed MPD manifests.
// Keyed by contentId, stores *mpd.MPD. No eviction (max ~5 entries per episode).
type mpdCache struct {
	mu    sync.RWMutex
	items map[string]*mpd.MPD
}

var manifestCache = &mpdCache{
	items: make(map[string]*mpd.MPD),
}

// GetCachedManifest returns the cached manifest for contentId, or nil on miss.
func GetCachedManifest(contentId string) *mpd.MPD {
	manifestCache.mu.RLock()
	defer manifestCache.mu.RUnlock()
	return manifestCache.items[contentId]
}

// SetCachedManifest stores a parsed manifest for contentId.
func SetCachedManifest(contentId string, manifest *mpd.MPD) {
	manifestCache.mu.Lock()
	defer manifestCache.mu.Unlock()
	manifestCache.items[contentId] = manifest
}

func ParseManifest(data []byte) (*mpd.MPD, error) {
	mpd := new(mpd.MPD)
	if err := mpd.Decode(data); err != nil {
		return nil, err
	}
	return mpd, nil
}

func GetBaseUrl(set *mpd.AdaptationSet, isVideoSet bool, quality string) (*string, *string) {
	if set == nil || len(set.Representations) == 0 {
		return nil, nil
	}

	for _, representation := range set.Representations {
		if isVideoSet {
			if representation.Height == nil || len(representation.BaseURL) == 0 || representation.ID == nil {
				continue
			}
			toInt, _ := strconv.ParseInt(strings.ReplaceAll(quality, "p", ""), 10, 64)
			if *representation.Height == uint64(toInt) {
				return &representation.BaseURL[0].Value, representation.ID
			}
		} else {
			if len(representation.BaseURL) == 0 || representation.ID == nil {
				continue
			}
			if strings.Contains(*representation.ID, "audio/") {
				if strings.Contains(*representation.ID, quality) {
					return &representation.BaseURL[0].Value, representation.ID
				}
			} else if representation.Bandwidth != nil {
				num := strings.ReplaceAll(quality, "k", "")
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
	firstRep := set.Representations[0]
	if len(firstRep.BaseURL) == 0 || firstRep.ID == nil {
		return nil, nil
	}
	return &firstRep.BaseURL[0].Value, firstRep.ID
}

func ExpandTimeline(timeline []*mpd.SegmentTimelineS, startNumber int64) []int64 {
	var result []int64
	segNum := startNumber

	for _, s := range timeline {
		repeat := int64(0)
		if s.R != nil && *s.R > 0 {
			repeat = *s.R
		}

		total := repeat + 1
		for i := int64(0); i < total; i++ {
			result = append(result, segNum)
			segNum++
		}
	}

	return result
}
