package media

import (
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
