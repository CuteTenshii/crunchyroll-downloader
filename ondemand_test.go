package main

import "testing"

func TestParseByteRange(t *testing.T) {
	tests := []struct {
		in        string
		wantStart int64
		wantEnd   int64
		wantErr   bool
	}{
		{"0-1837", 0, 1837, false},
		{"1838-6129", 1838, 6129, false},
		{"1708-6011", 1708, 6011, false},
		{"12345-", 0, 0, true},
		{"", 0, 0, true},
		{"abc-def", 0, 0, true},
	}

	for _, tc := range tests {
		start, end, err := parseByteRange(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseByteRange(%q) = %d,%d, nil; want error", tc.in, start, end)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseByteRange(%q) returned error: %v", tc.in, err)
			continue
		}
		if start != tc.wantStart || end != tc.wantEnd {
			t.Errorf("parseByteRange(%q) = %d,%d; want %d,%d", tc.in, start, end, tc.wantStart, tc.wantEnd)
		}
	}
}

func TestParseOnDemand(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-on-demand:2011" type="static">
  <Period>
    <AdaptationSet contentType="video" mimeType="video/mp4" segmentAlignment="true">
      <Representation id="84df326d" bandwidth="4386044" width="1280" height="720" codecs="avc1.64001F">
        <BaseURL>https://example.com/video.mp4</BaseURL>
        <SegmentBase indexRange="1838-6129">
          <Initialization range="0-1837"/>
        </SegmentBase>
      </Representation>
      <Representation id="b32ce5ea" bandwidth="16481538" width="1920" height="1080" codecs="avc1.640028">
        <BaseURL>https://example.com/video-1080.mp4</BaseURL>
        <SegmentBase indexRange="1839-6130">
          <Initialization range="0-1838"/>
        </SegmentBase>
      </Representation>
    </AdaptationSet>
    <AdaptationSet contentType="audio" mimeType="audio/mp4" segmentAlignment="true">
      <Representation id="433db54e" bandwidth="199094" codecs="mp4a.40.2" audioSamplingRate="48000">
        <BaseURL>https://example.com/audio.mp4</BaseURL>
        <SegmentBase indexRange="1708-6011">
          <Initialization range="0-1707"/>
        </SegmentBase>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`)

	sets, err := parseOnDemand(body)
	if err != nil {
		t.Fatalf("parseOnDemand returned error: %v", err)
	}
	if len(sets) != 2 {
		t.Fatalf("got %d adaptation sets, want 2", len(sets))
	}

	if !sets[0].IsVideo || sets[1].IsVideo {
		t.Fatalf("adaptation set video flags = %v,%v; want true,false", sets[0].IsVideo, sets[1].IsVideo)
	}

	if len(sets[0].Representations) != 2 {
		t.Fatalf("video has %d representations, want 2", len(sets[0].Representations))
	}
	rep := sets[0].Representations[1]
	if rep.Height != 1080 || rep.InitRange != "0-1838" || rep.IndexRange != "1839-6130" {
		t.Fatalf("video 1080p representation = %+v", rep)
	}

	audio := sets[1].Representations[0]
	if audio.Bandwidth != 199094 || audio.InitRange != "0-1707" || audio.IndexRange != "1708-6011" {
		t.Fatalf("audio representation = %+v", audio)
	}
}

func TestSelectOnDemandRepresentation(t *testing.T) {
	video := onDemandAdaptationSet{
		IsVideo: true,
		Representations: []onDemandRepresentation{
			{ID: "720p", Height: 720},
			{ID: "1080p", Height: 1080},
		},
	}
	if rep, ok := selectOnDemandRepresentation(video, "1080p", true); !ok || rep.ID != "1080p" {
		t.Fatalf("selectOnDemandRepresentation(video, 1080p) = %+v, %v", rep, ok)
	}

	audio := onDemandAdaptationSet{
		Representations: []onDemandRepresentation{
			{ID: "128k", Bandwidth: 135875},
			{ID: "192k", Bandwidth: 200315},
		},
	}
	if rep, ok := selectOnDemandRepresentation(audio, "192k", false); !ok || rep.ID != "192k" {
		t.Fatalf("selectOnDemandRepresentation(audio, 192k) = %+v, %v", rep, ok)
	}
}
