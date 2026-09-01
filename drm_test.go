package main

import (
	"bytes"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	gompd "github.com/unki2aut/go-mpd"
)

var playReadySystemID = mp4.UUID{0x9a, 0x04, 0xf0, 0x79, 0x98, 0x40, 0x42, 0x86, 0xab, 0x92, 0xe6, 0x5b, 0xe0, 0x88, 0x5f, 0x95}

func buildPssh(t *testing.T, systemID mp4.UUID, data []byte) []byte {
	t.Helper()
	box := &mp4.PsshBox{SystemID: systemID, Data: data}
	var buf bytes.Buffer
	if err := box.Encode(&buf); err != nil {
		t.Fatalf("encode PSSH box: %v", err)
	}
	return buf.Bytes()
}

func TestToWidevinePssh(t *testing.T) {
	payload := []byte{0x08, 0x01, 0x12, 0x10, 0xaa, 0xbb, 0xcc, 0xdd}

	t.Run("widevine system id is unchanged", func(t *testing.T) {
		original := buildPssh(t, widevineSystemID, payload)
		got, err := toWidevinePssh(original)
		if err != nil {
			t.Fatalf("toWidevinePssh returned error: %v", err)
		}
		if !bytes.Equal(got, original) {
			t.Fatal("toWidevinePssh altered an already-widevine PSSH")
		}
	})

	t.Run("playready system id is rewritten", func(t *testing.T) {
		in := buildPssh(t, playReadySystemID, payload)
		got, err := toWidevinePssh(in)
		if err != nil {
			t.Fatalf("toWidevinePssh returned error: %v", err)
		}

		box, err := mp4.DecodeBox(0, bytes.NewReader(got))
		if err != nil {
			t.Fatalf("decode result: %v", err)
		}
		pssh, ok := box.(*mp4.PsshBox)
		if !ok {
			t.Fatalf("result is %T, want *mp4.PsshBox", box)
		}
		if !bytes.Equal(pssh.SystemID, widevineSystemID) {
			t.Fatalf("system id = %s, want widevine", pssh.SystemID)
		}
		if !bytes.Equal(pssh.Data, payload) {
			t.Fatalf("payload changed: got %x, want %x", pssh.Data, payload)
		}
	})
}

func TestGetPsshPrefersWidevine(t *testing.T) {
	widevineURI := "urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed"
	playReadyURI := "urn:uuid:9a04f079-9840-4286-ab92-e65be0885f95"
	widevinePssh := "widevine-pssh"
	playReadyPssh := "playready-pssh"

	t.Run("widevine preferred over playready", func(t *testing.T) {
		m := &gompd.MPD{
			Period: []*gompd.Period{
				{AdaptationSets: []*gompd.AdaptationSet{
					{ContentProtections: []gompd.Descriptor{
						{SchemeIDURI: &playReadyURI, CencPSSH: &playReadyPssh},
						{SchemeIDURI: &widevineURI, CencPSSH: &widevinePssh},
					}},
				}},
			},
		}
		if got := getPssh(m); got == nil || *got != widevinePssh {
			t.Fatalf("getPssh() = %v, want the widevine PSSH", got)
		}
	})

	t.Run("falls back when no widevine scheme", func(t *testing.T) {
		m := &gompd.MPD{
			Period: []*gompd.Period{
				{AdaptationSets: []*gompd.AdaptationSet{
					{ContentProtections: []gompd.Descriptor{
						{SchemeIDURI: &playReadyURI, CencPSSH: &playReadyPssh},
					}},
				}},
			},
		}
		if got := getPssh(m); got == nil || *got != playReadyPssh {
			t.Fatalf("getPssh() = %v, want the fallback PSSH", got)
		}
	})

	t.Run("nil when no pssh", func(t *testing.T) {
		m := &gompd.MPD{
			Period: []*gompd.Period{
				{AdaptationSets: []*gompd.AdaptationSet{
					{ContentProtections: []gompd.Descriptor{
						{SchemeIDURI: &widevineURI},
					}},
				}},
			},
		}
		if got := getPssh(m); got != nil {
			t.Fatalf("getPssh() = %v, want nil", got)
		}
	})

	t.Run("representation level widevine pssh", func(t *testing.T) {
		m := &gompd.MPD{
			Period: []*gompd.Period{
				{AdaptationSets: []*gompd.AdaptationSet{
					{Representations: []gompd.Representation{
						{ContentProtections: []gompd.Descriptor{
							{SchemeIDURI: &playReadyURI, CencPSSH: &playReadyPssh},
							{SchemeIDURI: &widevineURI, CencPSSH: &widevinePssh},
						}},
					}},
				}},
			},
		}
		if got := getPssh(m); got == nil || *got != widevinePssh {
			t.Fatalf("getPssh() = %v, want the widevine PSSH", got)
		}
	})
}
