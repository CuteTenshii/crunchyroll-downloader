package engine

import (
	"encoding/json"
	"errors"
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

func TestCardFromCMSObjectEpisodeSubtitle(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "GWDU82Z05",
		"type": "episode",
		"title": "The Beginning",
		"slug_title": "the-beginning",
		"episode_metadata": {
			"series_id": "GYZJ43JMR",
			"series_title": "Slime",
			"season_number": 1,
			"episode": "12",
			"duration_ms": 1440000
		},
		"images": {
			"thumbnail": [[{"width": 320, "source": "https://cdn.example/ep.jpg"}]]
		}
	}`)
	card, ok := cardFromCMSObject(raw, "pt-BR")
	if !ok {
		t.Fatal("expected card")
	}
	if card.Subtitle != "S01E12" {
		t.Fatalf("subtitle %q", card.Subtitle)
	}
	if card.SeriesID != "GYZJ43JMR" {
		t.Fatalf("seriesId %q", card.SeriesID)
	}
	if card.Title != "Slime" {
		t.Fatalf("title %q want series title", card.Title)
	}
	if !strings.Contains(card.OpenURL, "/watch/GWDU82Z05") {
		t.Fatalf("url %q", card.OpenURL)
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

func TestPreferEpisodeLandscapeArt(t *testing.T) {
	in := []DiscoverCard{
		{ID: "ep1", Type: "episode", SeriesID: "S1", PosterURL: "https://cdn.example/ep.jpg"},
	}
	out := preferEpisodeLandscapeArt(in)
	if out[0].WideURL != "https://cdn.example/ep.jpg" {
		t.Fatalf("wide %q", out[0].WideURL)
	}
}

func TestDedupeCardsBySeriesKeepFirst(t *testing.T) {
	in := []DiscoverCard{
		{ID: "ep1", SeriesID: "S1", Title: "Show", Subtitle: "S01E03", EpisodeTitle: "Latest"},
		{ID: "ep2", SeriesID: "S1", Title: "Show", Subtitle: "S01E02"},
		{ID: "ep3", SeriesID: "S2", Title: "Other", Subtitle: "S01E01"},
	}
	out := dedupeCardsBySeriesKeepFirst(in)
	if len(out) != 2 {
		t.Fatalf("len %d", len(out))
	}
	if out[0].ID != "ep1" || out[0].EpisodeTitle != "Latest" {
		t.Fatalf("first %+v", out[0])
	}
	if out[1].SeriesID != "S2" {
		t.Fatalf("second %+v", out[1])
	}
}

func TestAttachRemainingLabels(t *testing.T) {
	p := 0.1
	in := []DiscoverCard{{
		Progress:   &p,
		DurationMS: 21 * 60 * 1000,
	}}
	out := attachRemainingLabels(in, "pt-BR")
	if out[0].RemainingLabel == "" || !strings.Contains(out[0].RemainingLabel, "restantes") {
		t.Fatalf("label %q", out[0].RemainingLabel)
	}
}

func TestPromoteEpisodesToSeriesCards(t *testing.T) {
	fake := &fakeFeedHydrator{
		objects: map[string]DiscoverCard{
			"SER1": {
				ID:        "SER1",
				Type:      "series",
				Title:     "Series One",
				PosterURL: "https://cdn.example/s1.jpg",
				OpenURL:   "https://www.crunchyroll.com/series/SER1",
			},
		},
	}
	in := []DiscoverCard{
		{
			ID:       "EP1",
			Type:     "episode",
			Title:    "Series One",
			SeriesID: "SER1",
			Subtitle: "S01E05",
			OpenURL:  "https://www.crunchyroll.com/watch/EP1",
			Progress: progressPtr(0.4),
		},
		{
			ID:        "SER2",
			Type:      "series",
			Title:     "Already Series",
			PosterURL: "https://cdn.example/s2.jpg",
			OpenURL:   "https://www.crunchyroll.com/series/SER2",
		},
	}
	out := promoteEpisodesToSeriesCards(in, "pt-BR", fake)
	if len(out) != 2 {
		t.Fatalf("len %d", len(out))
	}
	if out[0].ID != "SER1" || out[0].Type != "series" {
		t.Fatalf("promoted %+v", out[0])
	}
	if out[0].PosterURL != "https://cdn.example/s1.jpg" {
		t.Fatalf("poster %q", out[0].PosterURL)
	}
	if out[0].Subtitle != "S01E05" {
		t.Fatalf("subtitle %q", out[0].Subtitle)
	}
	if out[0].Progress == nil || *out[0].Progress != 0.4 {
		t.Fatalf("progress %v", out[0].Progress)
	}
	if out[1].ID != "SER2" {
		t.Fatalf("series passthrough %+v", out[1])
	}
}

func TestHeroImageURLsFlatAndPanel(t *testing.T) {
	raw := json.RawMessage(`{
		"resource_type": "hero_carousel",
		"items": [{
			"title": "Hero Title",
			"link": "/series/ABC/hero-title",
			"button_text": "Watch",
			"images": {
				"landscape_large": "https://cdn.example/wide.jpg",
				"portrait_large": "https://cdn.example/tall.jpg"
			}
		}]
	}`)
	heroes := mapHeroItems(raw, "pt-BR")
	if len(heroes) != 1 {
		t.Fatalf("len %d", len(heroes))
	}
	if heroes[0].WideURL != "https://cdn.example/wide.jpg" {
		t.Fatalf("wide %q", heroes[0].WideURL)
	}
	if heroes[0].PosterURL != "https://cdn.example/tall.jpg" {
		t.Fatalf("poster %q", heroes[0].PosterURL)
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

func TestMapHomeFeedEntryHero(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "hero-1",
		"resource_type": "hero_carousel",
		"title": "Featured",
		"items": [{
			"title": "Solo Leveling",
			"description": "Epic",
			"link": "/series/G4PH0WXVJ/solo-leveling",
			"button_text": "Watch",
			"images": {
				"landscape_large": "https://cdn.example/hero-wide.jpg",
				"portrait_large": "https://cdn.example/hero-tall.jpg"
			}
		}]
	}`)
	got := mapHomeFeedEntry(raw, 0, "pt-BR")
	if got.Skip || got.Block.Kind != HomeBlockHero {
		t.Fatalf("%+v", got)
	}
	if len(got.Block.Heroes) != 1 || got.Block.Heroes[0].Title != "Solo Leveling" {
		t.Fatalf("heroes %+v", got.Block.Heroes)
	}
	if !strings.Contains(got.Block.Heroes[0].OpenURL, "/series/G4PH0WXVJ") {
		t.Fatalf("open %q", got.Block.Heroes[0].OpenURL)
	}
	if got.Hydrate != hydrateNone {
		t.Fatalf("hydrate %q", got.Hydrate)
	}
}

func TestMapHomeFeedEntryCuratedSeries(t *testing.T) {
	raw := json.RawMessage(`{
		"resource_type": "curated_collection",
		"response_type": "series",
		"title": "Popular",
		"id": "rail-popular",
		"ids": ["AAA", "BBB"]
	}`)
	got := mapHomeFeedEntry(raw, 1, "pt-BR")
	if got.Skip || got.Hydrate != hydrateCurated {
		t.Fatalf("%+v", got)
	}
	if got.Block.Kind != HomeBlockPosterRail || got.Block.Title != "Popular" {
		t.Fatalf("%+v", got.Block)
	}
	if len(got.CuratedIDs) != 2 {
		t.Fatalf("ids %v", got.CuratedIDs)
	}
}

func TestMapHomeFeedEntryCuratedMusicSkipped(t *testing.T) {
	raw := json.RawMessage(`{
		"resource_type": "curated_collection",
		"response_type": "music",
		"title": "Songs",
		"ids": ["M1"]
	}`)
	got := mapHomeFeedEntry(raw, 0, "pt-BR")
	if !got.Skip {
		t.Fatalf("expected skip, got %+v", got)
	}
}

func TestMapHomeFeedEntryPanel(t *testing.T) {
	raw := json.RawMessage(`{
		"resource_type": "panel",
		"title": "Spotlight",
		"panel": {
			"id": "GYZJ43JMR",
			"type": "series",
			"title": "Slime",
			"slug_title": "slime",
			"images": {"poster_tall": [[{"width": 240, "source": "https://cdn.example/t.jpg"}]]}
		}
	}`)
	got := mapHomeFeedEntry(raw, 0, "pt-BR")
	if got.Skip || got.Block.Kind != HomeBlockPosterRail {
		t.Fatalf("%+v", got)
	}
	if len(got.Block.Cards) != 1 || got.Block.Cards[0].ID != "GYZJ43JMR" {
		t.Fatalf("cards %+v", got.Block.Cards)
	}
}

func TestMapHomeFeedEntryDynamicHistory(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "cw",
		"resource_type": "dynamic_collection",
		"response_type": "history",
		"title": "Continue Watching"
	}`)
	got := mapHomeFeedEntry(raw, 0, "pt-BR")
	if got.Skip || got.Hydrate != hydrateHistory {
		t.Fatalf("%+v", got)
	}
	if got.Block.Kind != HomeBlockLandscapeRail {
		t.Fatalf("kind %q", got.Block.Kind)
	}
}

func TestMapHomeFeedEntryDynamicWatchlistRecsBYW(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		hydrate string
		kind    string
	}{
		{
			name: "watchlist",
			raw: `{
				"resource_type": "dynamic_collection",
				"response_type": "watchlist",
				"title": "My List"
			}`,
			hydrate: hydrateWatchlist,
			kind:    HomeBlockPosterRail,
		},
		{
			name: "recommendations",
			raw: `{
				"resource_type": "dynamic_collection",
				"response_type": "recommendations",
				"title": "For You"
			}`,
			hydrate: hydrateRecommendations,
			kind:    HomeBlockPosterRail,
		},
		{
			name: "because_you_watched",
			raw: `{
				"resource_type": "dynamic_collection",
				"response_type": "because_you_watched",
				"title": "Because you watched Slime",
				"source_media_id": "GYZJ43JMR"
			}`,
			hydrate: hydrateBecauseYouWatched,
			kind:    HomeBlockPosterRail,
		},
		{
			name: "browse",
			raw: `{
				"resource_type": "dynamic_collection",
				"response_type": "browse",
				"title": "Explore",
				"link": "/content/v2/discover/browse?sort_by=popularity&type=series"
			}`,
			hydrate: hydrateBrowse,
			kind:    HomeBlockPosterRail,
		},
		{
			name: "recent_episodes",
			raw: `{
				"resource_type": "dynamic_collection",
				"response_type": "recent_episodes",
				"title": "New Episodes",
				"link": "/content/v2/discover/browse?type=episode"
			}`,
			hydrate: hydrateBrowse,
			kind:    HomeBlockPosterRail,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapHomeFeedEntry(json.RawMessage(tc.raw), 0, "pt-BR")
			if got.Skip {
				t.Fatal("unexpected skip")
			}
			if got.Hydrate != tc.hydrate {
				t.Fatalf("hydrate got %q want %q", got.Hydrate, tc.hydrate)
			}
			if got.Block.Kind != tc.kind {
				t.Fatalf("kind got %q want %q", got.Block.Kind, tc.kind)
			}
			if tc.name == "because_you_watched" && got.SourceMediaID != "GYZJ43JMR" {
				t.Fatalf("source %q", got.SourceMediaID)
			}
			if tc.name == "browse" && !strings.Contains(got.BrowseLink, "browse") {
				t.Fatalf("link %q", got.BrowseLink)
			}
		})
	}
}

func TestMapHomeFeedEntrySkipsMusicGameNews(t *testing.T) {
	for _, raw := range []string{
		`{"resource_type":"music_carousel","title":"Music"}`,
		`{"resource_type":"game","title":"Games"}`,
		`{"resource_type":"news_feed","title":"News"}`,
		`{"resource_type":"unknown_widget","title":"X"}`,
	} {
		got := mapHomeFeedEntry(json.RawMessage(raw), 0, "pt-BR")
		if !got.Skip {
			t.Fatalf("expected skip for %s → %+v", raw, got)
		}
	}
}

func TestMapInFeedBannerOnlyCatalogURLs(t *testing.T) {
	okBanner := json.RawMessage(`{
		"resource_type": "in_feed_banner",
		"id": "ban-1",
		"title": "Promo Series",
		"link": "/series/GYZJ43JMR/slime",
		"images": {"landscape_large": "https://cdn.example/banner.jpg"}
	}`)
	got := mapHomeFeedEntry(okBanner, 0, "pt-BR")
	if got.Skip || got.Block.Kind != HomeBlockBanner || got.Block.Banner == nil {
		t.Fatalf("%+v", got)
	}
	if !strings.Contains(got.Block.Banner.OpenURL, "/series/GYZJ43JMR") {
		t.Fatalf("open %q", got.Block.Banner.OpenURL)
	}

	badBanner := json.RawMessage(`{
		"resource_type": "in_feed_banner",
		"title": "Store sale",
		"link": "https://store.crunchyroll.com/deals",
		"images": {"landscape_large": "https://cdn.example/store.jpg"}
	}`)
	got = mapHomeFeedEntry(badBanner, 1, "pt-BR")
	if !got.Skip {
		t.Fatalf("store banner should be skipped: %+v", got)
	}

	watchBanner := json.RawMessage(`{
		"resource_type": "in_feed_banner",
		"title": "Movie night",
		"link": "/watch/G9DU8M2X4/example-movie"
	}`)
	got = mapHomeFeedEntry(watchBanner, 2, "pt-BR")
	if got.Skip || got.Block.Banner == nil {
		t.Fatalf("%+v", got)
	}
}

func TestLooksLikeTop10(t *testing.T) {
	positives := []struct{ title, id string }{
		{"Top 10 Series", "x"},
		{"Top 10 Anime", "x"},
		{"Top10 da Semana", "x"},
		{"CR Top-10", "x"},
		{"Os 10 mais assistidos", "rail-1"},
		{"10 mais populares da semana", ""},
		{"Ranking semanal", "top10-rail"},
		{"", "home_top_10_series"},
	}
	for _, p := range positives {
		if !looksLikeTop10(p.title, p.id) {
			t.Fatalf("expected top10 for title=%q id=%q", p.title, p.id)
		}
	}
	negatives := []struct{ title, id string }{
		{"Continue Watching", "history"},
		{"New Episodes", "recent"},
		{"Because you watched X", "byw"},
		// Series titles / generic popularity must not trip Top 10 chrome.
		{"Because you watched Ranking of Kings", "byw"},
		{"Ranking of Kings", "series-GY"},
		{"Mais Populares", "rail-1"},
		{"Most Popular Anime", ""},
		{"Ranking semanal", "weekly-rank"},
		{"", ""},
	}
	for _, n := range negatives {
		if looksLikeTop10(n.title, n.id) {
			t.Fatalf("unexpected top10 for title=%q id=%q", n.title, n.id)
		}
	}
}

func TestMediaIDFromSimilarToLink(t *testing.T) {
	cases := []struct {
		link string
		want string
	}{
		{"/similar_to/GYZJ43JMR", "GYZJ43JMR"},
		{"/content/v2/discover/similar_to/GYZJ43JMR", "GYZJ43JMR"},
		{"/content/v2/discover/acct/similar_to/ABC123?n=20&locale=pt-BR", "ABC123"},
		{"https://www.crunchyroll.com/content/v2/discover/similar_to/XYZ99", "XYZ99"},
		{"/content/v2/discover/browse?sort_by=popularity", ""},
		{"", ""},
		{"/similar_to/", ""},
	}
	for _, tc := range cases {
		if got := mediaIDFromSimilarToLink(tc.link); got != tc.want {
			t.Fatalf("link %q: got %q want %q", tc.link, got, tc.want)
		}
	}
}

func TestMapHomeFeedEntrySimilarToFromLink(t *testing.T) {
	// response_type similar_to with id only in link path (no source_media_id field).
	raw := json.RawMessage(`{
		"resource_type": "dynamic_collection",
		"response_type": "similar_to",
		"title": "Because you watched Slime",
		"link": "/content/v2/discover/similar_to/GYZJ43JMR"
	}`)
	got := mapHomeFeedEntry(raw, 0, "pt-BR")
	if got.Skip {
		t.Fatal("unexpected skip")
	}
	if got.Hydrate != hydrateBecauseYouWatched {
		t.Fatalf("hydrate %q", got.Hydrate)
	}
	if got.SourceMediaID != "GYZJ43JMR" {
		t.Fatalf("source %q", got.SourceMediaID)
	}

	// Unknown response_type but link contains similar_to/{id}.
	raw2 := json.RawMessage(`{
		"resource_type": "dynamic_collection",
		"response_type": "custom_row",
		"title": "More like this",
		"link": "/content/v2/discover/abc/similar_to/SERIES99"
	}`)
	got2 := mapHomeFeedEntry(raw2, 1, "pt-BR")
	if got2.Skip {
		t.Fatal("unexpected skip for link-only similar_to")
	}
	if got2.Hydrate != hydrateBecauseYouWatched {
		t.Fatalf("hydrate %q", got2.Hydrate)
	}
	if got2.SourceMediaID != "SERIES99" {
		t.Fatalf("source %q", got2.SourceMediaID)
	}
}

func TestApplyPlayheadProgressOmitsWhenDurationUnknown(t *testing.T) {
	cards := []DiscoverCard{{ID: "GWDU82Z05", Title: "Ep"}}
	ph := map[string]playheadInfo{
		"GWDU82Z05": {Playhead: 120, FullyWatched: false},
	}
	// No duration for this content id.
	applyPlayheadProgress(cards, ph, map[string]int64{})
	if cards[0].Progress != nil {
		t.Fatalf("expected nil progress when duration unknown, got %v", *cards[0].Progress)
	}

	// Fully watched still sets progress without duration.
	ph["GWDU82Z05"] = playheadInfo{Playhead: 120, FullyWatched: true}
	applyPlayheadProgress(cards, ph, map[string]int64{})
	if cards[0].Progress == nil || *cards[0].Progress != 1 {
		t.Fatalf("fully watched should set progress=1, got %v", cards[0].Progress)
	}
}

func TestApplyTop10Ranks(t *testing.T) {
	block := HomeBlock{
		ID:    "top10",
		Kind:  HomeBlockPosterRail,
		Title: "Top 10 Anime",
		Cards: make([]DiscoverCard, 12),
	}
	for i := range block.Cards {
		block.Cards[i].ID = string(rune('A' + i))
	}
	applyTop10Ranks(&block)
	if block.RankStyle != RankStyleTop10 {
		t.Fatalf("rankStyle %q", block.RankStyle)
	}
	for i := 0; i < 10; i++ {
		if block.Cards[i].Rank != i+1 {
			t.Fatalf("card %d rank %d", i, block.Cards[i].Rank)
		}
	}
	if block.Cards[10].Rank != 0 || block.Cards[11].Rank != 0 {
		t.Fatalf("ranks beyond 10 should stay 0")
	}

	normal := HomeBlock{
		ID:    "cw",
		Kind:  HomeBlockLandscapeRail,
		Title: "Continue Watching",
		Cards: []DiscoverCard{{ID: "a"}, {ID: "b"}},
	}
	applyTop10Ranks(&normal)
	if normal.RankStyle != RankStyleNone {
		t.Fatalf("expected none, got %q", normal.RankStyle)
	}
	if normal.Cards[0].Rank != 0 {
		t.Fatalf("rank set on non-top10")
	}
}

func TestDeriveCompatFromBlocks(t *testing.T) {
	page := HomeFeedPage{
		Blocks: []HomeBlock{
			{Kind: HomeBlockHero, Heroes: []DiscoverHero{{Title: "H1"}}},
			{Kind: HomeBlockPosterRail, ID: "r1", Title: "Rail", Cards: []DiscoverCard{{ID: "c1"}}},
			{Kind: HomeBlockBanner, Banner: &HomeBanner{OpenURL: "https://www.crunchyroll.com/series/X"}},
			{Kind: HomeBlockLandscapeRail, ID: "cw", Title: "CW", Cards: []DiscoverCard{{ID: "e1"}}},
		},
	}
	deriveCompatFromBlocks(&page)
	if len(page.Heroes) != 1 || page.Heroes[0].Title != "H1" {
		t.Fatalf("heroes %+v", page.Heroes)
	}
	if len(page.Rails) != 2 {
		t.Fatalf("rails %d", len(page.Rails))
	}
	if page.Rails[0].ID != "r1" || page.Rails[1].ID != "cw" {
		t.Fatalf("rails order %+v", page.Rails)
	}
}

func TestIsCatalogOpenURL(t *testing.T) {
	if !isCatalogOpenURL("https://www.crunchyroll.com/series/ABC") {
		t.Fatal("series")
	}
	if !isCatalogOpenURL("/watch/XYZ/ep") {
		t.Fatal("watch path")
	}
	if isCatalogOpenURL("https://store.crunchyroll.com/x") {
		t.Fatal("store should fail")
	}
	if isCatalogOpenURL("") {
		t.Fatal("empty")
	}
}

func TestParseHistoryResponseAndPlayheads(t *testing.T) {
	body := []byte(`{
		"data": [{
			"id": "GWDU82Z05",
			"panel": {
				"id": "GWDU82Z05",
				"type": "episode",
				"title": "Ep title",
				"slug_title": "ep-title",
				"episode_metadata": {
					"series_id": "GYZJ43JMR",
					"series_title": "Slime",
					"season_number": 2,
					"episode_number": 3,
					"duration_ms": 1000000
				},
				"images": {
					"thumbnail": [[{"width": 320, "source": "https://cdn.example/ep.jpg"}]]
				}
			}
		}]
	}`)
	cards, ids, durs := parseHistoryResponse(body, "pt-BR")
	if len(cards) != 1 || cards[0].Title != "Slime" {
		t.Fatalf("cards %+v", cards)
	}
	if cards[0].Subtitle != "S02E3" {
		t.Fatalf("subtitle %q", cards[0].Subtitle)
	}
	if len(ids) != 1 || ids[0] != "GWDU82Z05" {
		t.Fatalf("ids %v", ids)
	}
	if durs["GWDU82Z05"] != 1000000 {
		t.Fatalf("durs %v", durs)
	}

	phBody := []byte(`{
		"data": [{"content_id": "GWDU82Z05", "playhead": 250, "fully_watched": false}]
	}`)
	ph, err := parsePlayheadsResponse(phBody)
	if err != nil {
		t.Fatal(err)
	}
	applyPlayheadProgress(cards, ph, durs)
	if cards[0].Progress == nil {
		t.Fatal("expected progress")
	}
	// 250s / 1000s = 0.25
	if *cards[0].Progress < 0.24 || *cards[0].Progress > 0.26 {
		t.Fatalf("progress %v", *cards[0].Progress)
	}
}

func TestParsePanelListResponse(t *testing.T) {
	body := []byte(`{
		"data": [
			{"panel": {"id": "A1", "type": "series", "title": "One", "slug_title": "one"}},
			{"id": "B2", "type": "series", "title": "Two", "slug_title": "two"}
		]
	}`)
	cards := parsePanelListResponse(body, "en-US")
	if len(cards) != 2 {
		t.Fatalf("%d cards", len(cards))
	}
}

func TestResolveBrowseEndpoint(t *testing.T) {
	ep, err := resolveBrowseEndpoint("", "pt-BR", 10)
	if err != nil || !strings.Contains(ep, "browse") || !strings.Contains(ep, "locale=pt-BR") {
		t.Fatalf("%q %v", ep, err)
	}
	ep, err = resolveBrowseEndpoint("/content/v2/discover/browse?sort_by=newly_added", "en-US", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ep, "https://www.crunchyroll.com/content/v2/discover/browse?") {
		t.Fatalf("%q", ep)
	}
	if !strings.Contains(ep, "sort_by=newly_added") || !strings.Contains(ep, "locale=en-US") {
		t.Fatalf("%q", ep)
	}
	ep, err = resolveBrowseEndpoint("sort_by=popularity&type=series", "pt-BR", 8)
	if err != nil || !strings.Contains(ep, "sort_by=popularity") {
		t.Fatalf("%q %v", ep, err)
	}
}

// fakeFeedHydrator is an in-memory hydrator for buildHomeBlocks tests.
type fakeFeedHydrator struct {
	history    []DiscoverCard
	watchlist  []DiscoverCard
	recs       []DiscoverCard
	similar    map[string][]DiscoverCard
	browse     []DiscoverCard
	objects    map[string]DiscoverCard
	failHist   bool
	failWatch  bool
	callHist   int
	callWatch  int
	callRecs   int
	callSim    int
	callBrowse int
	callObj    int
}

func (f *fakeFeedHydrator) History(accountID, locale string, n int) ([]DiscoverCard, error) {
	f.callHist++
	if f.failHist {
		return nil, errors.New("history boom")
	}
	return f.history, nil
}
func (f *fakeFeedHydrator) Watchlist(accountID, locale string, n int) ([]DiscoverCard, error) {
	f.callWatch++
	if f.failWatch {
		return nil, errors.New("watchlist boom")
	}
	return f.watchlist, nil
}
func (f *fakeFeedHydrator) Recommendations(accountID, locale string, n int) ([]DiscoverCard, error) {
	f.callRecs++
	return f.recs, nil
}
func (f *fakeFeedHydrator) SimilarTo(accountID, mediaID, locale string, n int) ([]DiscoverCard, error) {
	f.callSim++
	return f.similar[mediaID], nil
}
func (f *fakeFeedHydrator) Browse(link, locale string, n int) ([]DiscoverCard, error) {
	f.callBrowse++
	return f.browse, nil
}
func (f *fakeFeedHydrator) Objects(ids []string, locale string) ([]DiscoverCard, error) {
	f.callObj++
	var out []DiscoverCard
	for _, id := range ids {
		if c, ok := f.objects[id]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

func TestBuildHomeBlocksOrderAndHydratorFailIsolation(t *testing.T) {
	fake := &fakeFeedHydrator{
		failHist:  true, // history block dropped
		watchlist: []DiscoverCard{{ID: "W1", Title: "Watch Me", OpenURL: "https://www.crunchyroll.com/series/W1"}},
		objects: map[string]DiscoverCard{
			"AAA": {ID: "AAA", Title: "A", OpenURL: "https://www.crunchyroll.com/series/AAA"},
			"BBB": {ID: "BBB", Title: "B", OpenURL: "https://www.crunchyroll.com/series/BBB"},
		},
	}
	data := []json.RawMessage{
		json.RawMessage(`{
			"id": "hero",
			"resource_type": "hero_carousel",
			"items": [{"title": "Hero", "link": "/series/H1/h"}]
		}`),
		json.RawMessage(`{
			"id": "hist",
			"resource_type": "dynamic_collection",
			"response_type": "history",
			"title": "Continue Watching"
		}`),
		json.RawMessage(`{
			"id": "wl",
			"resource_type": "dynamic_collection",
			"response_type": "watchlist",
			"title": "Watchlist"
		}`),
		json.RawMessage(`{
			"id": "top10-pt",
			"resource_type": "curated_collection",
			"response_type": "series",
			"title": "Top 10 da Semana",
			"ids": ["AAA", "BBB"]
		}`),
		json.RawMessage(`{
			"resource_type": "in_feed_banner",
			"title": "Store",
			"link": "https://store.crunchyroll.com/x"
		}`),
		json.RawMessage(`{
			"resource_type": "music_carousel",
			"title": "Music"
		}`),
	}
	blocks := buildHomeBlocks(data, "acct", "pt-BR", fake)
	// hero, watchlist, top10 curated — history/store/music dropped
	if len(blocks) != 3 {
		t.Fatalf("blocks=%d %+v", len(blocks), blockKinds(blocks))
	}
	if blocks[0].Kind != HomeBlockHero {
		t.Fatalf("0 %s", blocks[0].Kind)
	}
	if blocks[1].Kind != HomeBlockPosterRail || blocks[1].Title != "Watchlist" {
		t.Fatalf("1 %+v", blocks[1])
	}
	if blocks[2].RankStyle != RankStyleTop10 {
		t.Fatalf("expected top10 on curated, got %q title=%q", blocks[2].RankStyle, blocks[2].Title)
	}
	if len(blocks[2].Cards) != 2 || blocks[2].Cards[0].Rank != 1 || blocks[2].Cards[1].Rank != 2 {
		t.Fatalf("ranks %+v", blocks[2].Cards)
	}
	if fake.callHist != 1 || fake.callWatch != 1 || fake.callObj != 1 {
		t.Fatalf("calls hist=%d watch=%d obj=%d", fake.callHist, fake.callWatch, fake.callObj)
	}
}

func TestBuildHomeBlocksBrowsePopularShape(t *testing.T) {
	// Simulate BrowsePopular-style single poster rail via mapper path for curated-less page.
	page := HomeFeedPage{}
	block := HomeBlock{
		ID:        "browse-popular",
		Kind:      HomeBlockPosterRail,
		Title:     "Popular",
		RankStyle: RankStyleNone,
		Cards:     []DiscoverCard{{ID: "P1", Title: "Pop"}},
	}
	applyTop10Ranks(&block)
	page.Blocks = []HomeBlock{block}
	deriveCompatFromBlocks(&page)
	if len(page.Blocks) != 1 || page.Blocks[0].Kind != HomeBlockPosterRail {
		t.Fatalf("%+v", page.Blocks)
	}
	if len(page.Rails) != 1 || page.Rails[0].ID != "browse-popular" {
		t.Fatalf("compat rails %+v", page.Rails)
	}
}

func blockKinds(blocks []HomeBlock) []string {
	out := make([]string, len(blocks))
	for i, b := range blocks {
		out[i] = b.Kind + ":" + b.Title
	}
	return out
}
