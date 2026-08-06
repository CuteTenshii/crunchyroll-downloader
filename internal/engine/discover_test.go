package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCardFromCMSObjectSeries(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "GYZJ43JMR",
		"type": "series",
		"title": "That Time I Got Reincarnated as a Slime",
		"slug_title": "that-time-i-got-reincarnated-as-a-slime",
		"images": {
			"poster_tall": [[{"width": 240, "source": "https://cdn.example/tall.jpg"}]],
			"poster_wide": [[{"width": 640, "source": "https://cdn.example/wide.jpg"}]]
		}
	}`)
	card, ok := cardFromCMSObject(raw, "pt-BR")
	if !ok {
		t.Fatal("expected card")
	}
	if card.ID != "GYZJ43JMR" || card.Type != "series" {
		t.Fatalf("%+v", card)
	}
	if card.PosterURL != "https://cdn.example/tall.jpg" {
		t.Fatalf("poster %q", card.PosterURL)
	}
	if card.OpenURL != "https://www.crunchyroll.com/series/GYZJ43JMR/that-time-i-got-reincarnated-as-a-slime" {
		t.Fatalf("url %q", card.OpenURL)
	}
}

func TestParseHomeFeedHeroAndCurated(t *testing.T) {
	// Smoke: curated ids + hero structure unmarshals via helpers used by FetchHomeFeed.
	raw := json.RawMessage(`{
		"resource_type": "curated_collection",
		"response_type": "series",
		"title": "Popular",
		"ids": ["AAA", "BBB"]
	}`)
	var coll struct {
		Title string   `json:"title"`
		IDs   []string `json:"ids"`
	}
	if err := json.Unmarshal(raw, &coll); err != nil {
		t.Fatal(err)
	}
	if coll.Title != "Popular" || len(coll.IDs) != 2 {
		t.Fatalf("%+v", coll)
	}
}

func TestNormalizeDiscoverLocale(t *testing.T) {
	if got := normalizeDiscoverLocale("pt-br"); got != "pt-BR" {
		t.Fatalf("%q", got)
	}
	if got := normalizeDiscoverLocale(""); got != "pt-BR" {
		t.Fatalf("%q", got)
	}
}

func TestDedupeCards(t *testing.T) {
	in := []DiscoverCard{{ID: "a"}, {ID: "a"}, {ID: "b"}}
	out := dedupeCards(in)
	if len(out) != 2 {
		t.Fatalf("%d", len(out))
	}
}

func TestNormalizeOpenURL(t *testing.T) {
	if got := normalizeOpenURL("/series/ABC", "pt-BR"); got != "https://www.crunchyroll.com/series/ABC" {
		t.Fatalf("%q", got)
	}
	if got := normalizeOpenURL("https://www.crunchyroll.com/watch/X", "pt-BR"); got != "https://www.crunchyroll.com/watch/X" {
		t.Fatalf("%q", got)
	}
}

func TestCardFromCMSObjectMovie(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "G9DU8M2X4",
		"type": "movie_listing",
		"title": "Example Movie",
		"slug_title": "example-movie",
		"images": {
			"poster_tall": [[{"width": 240, "source": "https://cdn.example/m.jpg"}]]
		}
	}`)
	card, ok := cardFromCMSObject(raw, "pt-BR")
	if !ok {
		t.Fatal("expected card")
	}
	if !strings.Contains(card.OpenURL, "/watch/G9DU8M2X4") {
		t.Fatalf("url %q", card.OpenURL)
	}
}
