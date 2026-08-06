package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DiscoverCard is a browse/home/search result ready for the Home UI.
type DiscoverCard struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // series | movie_listing | episode | movie | unknown
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	PosterURL   string `json:"posterUrl,omitempty"`
	WideURL     string `json:"wideUrl,omitempty"`
	// OpenURL is a path the download UI can Inspect (e.g. https://www.crunchyroll.com/series/ID).
	OpenURL string `json:"openUrl"`
}

// DiscoverRail is a horizontal home-feed section.
type DiscoverRail struct {
	ID    string         `json:"id"`
	Title string         `json:"title"`
	Cards []DiscoverCard `json:"cards"`
}

// DiscoverHero is a top carousel slide.
type DiscoverHero struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	WideURL     string `json:"wideUrl,omitempty"`
	PosterURL   string `json:"posterUrl,omitempty"`
	OpenURL     string `json:"openUrl,omitempty"`
	ButtonText  string `json:"buttonText,omitempty"`
}

// HomeFeedPage is one page of Discover home content.
type HomeFeedPage struct {
	Heroes     []DiscoverHero `json:"heroes"`
	Rails      []DiscoverRail `json:"rails"`
	NextStart  int            `json:"nextStart"`
	PageSize   int            `json:"pageSize"`
	TotalApprox int           `json:"totalApprox,omitempty"`
}

type v2BulkRaw struct {
	Total int               `json:"total"`
	Data  []json.RawMessage `json:"data"`
}

type cmsObjectItem struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	SlugTitle   string   `json:"slug_title"`
	Images      CRImages `json:"images"`
	// Episode / series nested metadata sometimes carries series_id.
	EpisodeMetadata *struct {
		SeriesID    string `json:"series_id"`
		SeriesTitle string `json:"series_title"`
	} `json:"episode_metadata"`
}

// FetchHomeFeed loads a page of the personalized Discover home feed.
// Requires a valid token (refreshAccessToken) so GetAccountID() is non-empty.
func FetchHomeFeed(start, n int, locale string) (HomeFeedPage, error) {
	if n <= 0 {
		n = 20
	}
	if n > 50 {
		n = 50
	}
	if start < 0 {
		start = 0
	}
	locale = normalizeDiscoverLocale(locale)
	accountID := GetAccountID()
	if accountID == "" {
		return HomeFeedPage{}, fmt.Errorf("account id is not available; authenticate first")
	}

	endpoint := fmt.Sprintf(
		"https://www.crunchyroll.com/content/v2/discover/%s/home_feed?n=%d&start=%d&locale=%s",
		url.PathEscape(accountID), n, start, url.QueryEscape(locale),
	)
	body, err := discoverGET(endpoint)
	if err != nil {
		return HomeFeedPage{}, err
	}
	var bulk v2BulkRaw
	if err := json.Unmarshal(body, &bulk); err != nil {
		return HomeFeedPage{}, fmt.Errorf("decode home_feed: %w", err)
	}

	page := HomeFeedPage{
		PageSize:    n,
		NextStart:   start + len(bulk.Data),
		TotalApprox: bulk.Total,
	}

	var pendingIDs []string
	idToRail := map[string]int{} // rail index for series ids still needing cards

	for i, raw := range bulk.Data {
		var head struct {
			ResourceType string `json:"resource_type"`
			ResponseType string `json:"response_type"`
			Title        string `json:"title"`
			Description  string `json:"description"`
			ID           string `json:"id"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			continue
		}
		switch head.ResourceType {
		case "hero_carousel":
			var hero struct {
				Items []struct {
					Title       string `json:"title"`
					Description string `json:"description"`
					Link        string `json:"link"`
					ButtonText  string `json:"button_text"`
					Slug        string `json:"slug"`
					Images      struct {
						LandscapeLarge string `json:"landscape_large"`
						PortraitLarge  string `json:"portrait_large"`
					} `json:"images"`
					Panel json.RawMessage `json:"panel"`
				} `json:"items"`
			}
			if err := json.Unmarshal(raw, &hero); err != nil {
				continue
			}
			for _, it := range hero.Items {
				h := DiscoverHero{
					Title:       it.Title,
					Description: it.Description,
					WideURL:     it.Images.LandscapeLarge,
					PosterURL:   it.Images.PortraitLarge,
					ButtonText:  it.ButtonText,
					OpenURL:     normalizeOpenURL(it.Link, locale),
				}
				if h.OpenURL == "" && len(it.Panel) > 0 {
					if card, ok := cardFromCMSObject(it.Panel, locale); ok {
						h.OpenURL = card.OpenURL
						if h.PosterURL == "" {
							h.PosterURL = card.PosterURL
						}
						if h.WideURL == "" {
							h.WideURL = card.WideURL
						}
						if h.Title == "" {
							h.Title = card.Title
						}
					}
				}
				if h.Title != "" || h.OpenURL != "" {
					page.Heroes = append(page.Heroes, h)
				}
			}
		case "curated_collection":
			if head.ResponseType != "" && head.ResponseType != "series" {
				// music etc. — skip for v1
				continue
			}
			var coll struct {
				Title string   `json:"title"`
				IDs   []string `json:"ids"`
				ID    string   `json:"id"`
			}
			if err := json.Unmarshal(raw, &coll); err != nil {
				continue
			}
			if len(coll.IDs) == 0 {
				continue
			}
			railIdx := len(page.Rails)
			page.Rails = append(page.Rails, DiscoverRail{
				ID:    firstNonEmpty(coll.ID, fmt.Sprintf("rail-%d", i)),
				Title: firstNonEmpty(coll.Title, "Collection"),
			})
			for _, id := range coll.IDs {
				id = strings.TrimSpace(id)
				if id == "" {
					continue
				}
				pendingIDs = append(pendingIDs, id)
				idToRail[id] = railIdx
			}
		case "panel":
			var panelWrap struct {
				Panel json.RawMessage `json:"panel"`
			}
			if err := json.Unmarshal(raw, &panelWrap); err != nil || len(panelWrap.Panel) == 0 {
				continue
			}
			if card, ok := cardFromCMSObject(panelWrap.Panel, locale); ok {
				page.Rails = append(page.Rails, DiscoverRail{
					ID:    "panel-" + card.ID,
					Title: "Featured",
					Cards: []DiscoverCard{card},
				})
			}
		default:
			// Skip unknown / dynamic collections in v1 (recommendations etc. can be phase 2).
			continue
		}
	}

	if len(pendingIDs) > 0 {
		cards, err := ResolveObjectCards(pendingIDs, locale)
		if err != nil {
			return page, err
		}
		byID := map[string]DiscoverCard{}
		for _, c := range cards {
			byID[c.ID] = c
		}
		// Preserve order of ids per rail by walking pending in order.
		railCards := map[int][]DiscoverCard{}
		seen := map[string]struct{}{}
		for _, id := range pendingIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			card, ok := byID[id]
			if !ok {
				continue
			}
			ri := idToRail[id]
			railCards[ri] = append(railCards[ri], card)
		}
		for ri, cards := range railCards {
			if ri >= 0 && ri < len(page.Rails) {
				page.Rails[ri].Cards = cards
			}
		}
		// Drop empty rails
		filtered := page.Rails[:0]
		for _, r := range page.Rails {
			if len(r.Cards) > 0 {
				filtered = append(filtered, r)
			}
		}
		page.Rails = filtered
	}

	return page, nil
}

// BrowsePopular loads a popularity-sorted series list (fallback when home_feed fails).
func BrowsePopular(start, n int, locale string) (HomeFeedPage, error) {
	if n <= 0 {
		n = 20
	}
	if n > 50 {
		n = 50
	}
	if start < 0 {
		start = 0
	}
	locale = normalizeDiscoverLocale(locale)
	endpoint := fmt.Sprintf(
		"https://www.crunchyroll.com/content/v2/discover/browse?sort_by=popularity&type=series&n=%d&start=%d&locale=%s",
		n, start, url.QueryEscape(locale),
	)
	body, err := discoverGET(endpoint)
	if err != nil {
		return HomeFeedPage{}, err
	}
	var bulk struct {
		Total int               `json:"total"`
		Data  []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &bulk); err != nil {
		return HomeFeedPage{}, fmt.Errorf("decode browse: %w", err)
	}
	cards := make([]DiscoverCard, 0, len(bulk.Data))
	for _, raw := range bulk.Data {
		if card, ok := cardFromCMSObject(raw, locale); ok {
			cards = append(cards, card)
		}
	}
	page := HomeFeedPage{
		PageSize:    n,
		NextStart:   start + len(bulk.Data),
		TotalApprox: bulk.Total,
	}
	if len(cards) > 0 {
		page.Rails = []DiscoverRail{{
			ID:    "browse-popular",
			Title: "Popular",
			Cards: cards,
		}}
	}
	return page, nil
}

// SearchDiscover runs Crunchyroll Discover search.
func SearchDiscover(q string, start, n int, locale string) ([]DiscoverCard, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, fmt.Errorf("search query is empty")
	}
	if n <= 0 {
		n = 20
	}
	if n > 50 {
		n = 50
	}
	if start < 0 {
		start = 0
	}
	locale = normalizeDiscoverLocale(locale)
	endpoint := fmt.Sprintf(
		"https://www.crunchyroll.com/content/v2/discover/search?q=%s&n=%d&start=%d&type=top_results&locale=%s",
		url.QueryEscape(q), n, start, url.QueryEscape(locale),
	)
	body, err := discoverGET(endpoint)
	if err != nil {
		return nil, err
	}
	// Search response is often nested: data[].items or data as list of containers.
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}
	dataRaw, ok := root["data"]
	if !ok {
		return nil, fmt.Errorf("search: missing data")
	}
	var containers []struct {
		Type  string          `json:"type"`
		Items json.RawMessage `json:"items"`
		// Sometimes results are flat objects in data
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(dataRaw, &containers); err != nil {
		return nil, fmt.Errorf("decode search data: %w", err)
	}
	var cards []DiscoverCard
	var ids []string
	for _, c := range containers {
		if len(c.Items) > 0 {
			var items []json.RawMessage
			if err := json.Unmarshal(c.Items, &items); err != nil {
				continue
			}
			for _, it := range items {
				if card, ok := cardFromCMSObject(it, locale); ok {
					cards = append(cards, card)
				} else if id := peekID(it); id != "" {
					ids = append(ids, id)
				}
			}
			continue
		}
		if c.ID != "" {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) > 0 {
		resolved, err := ResolveObjectCards(ids, locale)
		if err != nil {
			return cards, err
		}
		cards = append(cards, resolved...)
	}
	return dedupeCards(cards), nil
}

// ResolveObjectCards loads CMS objects by id and maps them to DiscoverCards.
func ResolveObjectCards(ids []string, locale string) ([]DiscoverCard, error) {
	locale = normalizeDiscoverLocale(locale)
	clean := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return nil, nil
	}

	var out []DiscoverCard
	const batch = 25
	for i := 0; i < len(clean); i += batch {
		j := i + batch
		if j > len(clean) {
			j = len(clean)
		}
		chunk := clean[i:j]
		// Comma-joined ids must stay unescaped in the path (CR expects raw commas).
		endpoint := fmt.Sprintf(
			"https://www.crunchyroll.com/content/v2/cms/objects/%s?locale=%s",
			strings.Join(chunk, ","),
			url.QueryEscape(locale),
		)
		body, err := discoverGET(endpoint)
		if err != nil {
			return out, err
		}
		var bulk struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &bulk); err != nil {
			return out, fmt.Errorf("decode objects: %w", err)
		}
		for _, raw := range bulk.Data {
			if card, ok := cardFromCMSObject(raw, locale); ok {
				out = append(out, card)
			}
		}
	}
	return out, nil
}

func discoverGET(endpoint string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	req.Header.Set("Accept", "application/json")
	resp, err := DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover GET %s: HTTP %d", endpoint, resp.StatusCode)
	}
	return body, nil
}

func cardFromCMSObject(raw json.RawMessage, locale string) (DiscoverCard, bool) {
	var obj cmsObjectItem
	if err := json.Unmarshal(raw, &obj); err != nil || obj.ID == "" {
		return DiscoverCard{}, false
	}
	typ := strings.ToLower(strings.TrimSpace(obj.Type))
	if typ == "" {
		typ = "unknown"
	}
	card := DiscoverCard{
		ID:          obj.ID,
		Type:        typ,
		Title:       obj.Title,
		Description: obj.Description,
		PosterURL:   posterFromImages(obj.Images),
		WideURL:     pickImageURLFromGroups(obj.Images.PosterWide, 640),
	}
	if card.WideURL == "" {
		card.WideURL = pickImageURLFromGroups(obj.Images.Thumbnail, 640)
	}
	switch {
	case typ == "series" || strings.Contains(typ, "series"):
		card.OpenURL = seriesOpenURL(obj.ID, obj.SlugTitle, locale)
	case typ == "episode":
		card.OpenURL = watchOpenURL(obj.ID, obj.SlugTitle, locale)
		if obj.EpisodeMetadata != nil && obj.EpisodeMetadata.SeriesTitle != "" && card.Title == "" {
			card.Title = obj.EpisodeMetadata.SeriesTitle
		}
	case strings.Contains(typ, "movie"):
		card.OpenURL = watchOpenURL(obj.ID, obj.SlugTitle, locale)
	default:
		// Prefer series-style URL; Inspect accepts series or watch.
		card.OpenURL = seriesOpenURL(obj.ID, obj.SlugTitle, locale)
	}
	return card, true
}

func seriesOpenURL(id, slug, locale string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	slug = strings.Trim(slug, "/")
	// Existing Inspect parser only needs /series/{id}/…
	if slug != "" {
		return "https://www.crunchyroll.com/series/" + id + "/" + slug
	}
	return "https://www.crunchyroll.com/series/" + id
}

func watchOpenURL(id, slug, locale string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	slug = strings.Trim(slug, "/")
	if slug != "" {
		return "https://www.crunchyroll.com/watch/" + id + "/" + slug
	}
	return "https://www.crunchyroll.com/watch/" + id
}

func normalizeDiscoverLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return "pt-BR"
	}
	// Accept pt-br → pt-BR
	parts := strings.SplitN(locale, "-", 2)
	if len(parts) == 2 {
		return strings.ToLower(parts[0]) + "-" + strings.ToUpper(parts[1])
	}
	return locale
}

func normalizeOpenURL(link, locale string) string {
	link = strings.TrimSpace(link)
	if link == "" {
		return ""
	}
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link
	}
	if strings.HasPrefix(link, "/") {
		return "https://www.crunchyroll.com" + link
	}
	return "https://www.crunchyroll.com/" + link
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func peekID(raw json.RawMessage) string {
	var o struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &o)
	return strings.TrimSpace(o.ID)
}

func dedupeCards(in []DiscoverCard) []DiscoverCard {
	seen := map[string]struct{}{}
	out := make([]DiscoverCard, 0, len(in))
	for _, c := range in {
		if c.ID == "" {
			continue
		}
		if _, ok := seen[c.ID]; ok {
			continue
		}
		seen[c.ID] = struct{}{}
		out = append(out, c)
	}
	return out
}
