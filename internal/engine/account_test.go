package engine

import (
	"encoding/json"
	"testing"
)

func TestParseCRProfilesItemsShape(t *testing.T) {
	body := []byte(`{
		"items": [
			{"profile_id": "p1", "profile_name": "Kids", "is_selected": false},
			{"profile_id": "p2", "profile_name": "Adult", "is_selected": true}
		]
	}`)
	got := parseCRProfiles(body)
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
	if got[0].ID != "p1" || got[0].Name != "Kids" || got[0].IsSelected {
		t.Fatalf("first: %#v", got[0])
	}
	if got[1].ID != "p2" || !got[1].IsSelected {
		t.Fatalf("second: %#v", got[1])
	}
}

func TestParseCRProfilesDataAndAltKeys(t *testing.T) {
	body := []byte(`{
		"data": [
			{"id": "x1", "name": "One", "isSelected": true},
			{"external_id": "x2", "username": "Two"}
		]
	}`)
	got := parseCRProfiles(body)
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
	if got[0].ID != "x1" || got[1].ID != "x2" || got[1].Name != "Two" {
		t.Fatalf("%#v", got)
	}
}

func TestParseCRProfilesNestedMultiprofiles(t *testing.T) {
	body := []byte(`{
		"multiprofiles": [
			{"profileId": "n1", "profileName": "Nested"}
		]
	}`)
	got := parseCRProfiles(body)
	if len(got) != 1 || got[0].ID != "n1" || got[0].Name != "Nested" {
		t.Fatalf("%#v", got)
	}
}

func TestParseCRProfilesEmpty(t *testing.T) {
	if got := parseCRProfiles([]byte(`{}`)); len(got) != 0 {
		t.Fatalf("want empty, got %#v", got)
	}
	if got := parseCRProfiles([]byte(`not-json`)); len(got) != 0 {
		t.Fatalf("want empty on bad json, got %#v", got)
	}
}

func TestParseMeAsSingleProfile(t *testing.T) {
	body := []byte(`{"external_id":"acct-1","email":"user@example.com","profile_name":"Main"}`)
	p, ok := parseMeAsSingleProfile(body)
	if !ok || p.ID != "acct-1" || p.Name != "Main" || !p.IsSelected {
		t.Fatalf("got ok=%v %#v", ok, p)
	}
}

func TestDecodeOneCRProfileRequiresID(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"name": "NoID"})
	if _, ok := decodeOneCRProfile(raw); ok {
		t.Fatal("expected reject without id")
	}
}
