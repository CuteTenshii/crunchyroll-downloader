package engine

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// fetchWatchHistory loads continue-watching style cards with playhead progress.
// GET /content/v2/{accountId}/watch-history + playheads.
func fetchWatchHistory(accountID, locale string, n int) ([]DiscoverCard, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account id required for watch history")
	}
	if n <= 0 {
		n = 20
	}
	locale = normalizeDiscoverLocale(locale)
	endpoint := fmt.Sprintf(
		"https://www.crunchyroll.com/content/v2/%s/watch-history?n=%d&start=0&locale=%s",
		url.PathEscape(accountID), n, url.QueryEscape(locale),
	)
	body, err := discoverGET(endpoint)
	if err != nil {
		return nil, err
	}
	cards, contentIDs, durations := parseHistoryResponse(body, locale)
	if len(cards) == 0 {
		return nil, nil
	}
	if len(contentIDs) > 0 {
		if ph, err := fetchPlayheads(accountID, contentIDs); err == nil {
			applyPlayheadProgress(cards, ph, durations)
		}
	}
	return cards, nil
}

// fetchWatchlist tries discover watchlist then account watchlist.
func fetchWatchlist(accountID, locale string, n int) ([]DiscoverCard, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account id required for watchlist")
	}
	if n <= 0 {
		n = 20
	}
	locale = normalizeDiscoverLocale(locale)
	endpoints := []string{
		fmt.Sprintf(
			"https://www.crunchyroll.com/content/v2/discover/%s/watchlist?n=%d&start=0&locale=%s",
			url.PathEscape(accountID), n, url.QueryEscape(locale),
		),
		fmt.Sprintf(
			"https://www.crunchyroll.com/content/v2/%s/watchlist?n=%d&start=0&locale=%s",
			url.PathEscape(accountID), n, url.QueryEscape(locale),
		),
	}
	var lastErr error
	for _, endpoint := range endpoints {
		body, err := discoverGET(endpoint)
		if err != nil {
			lastErr = err
			continue
		}
		cards := parsePanelListResponse(body, locale)
		if len(cards) > 0 {
			return cards, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}

// fetchRecommendations loads personalized recommendations.
// GET /content/v2/discover/{accountId}/recommendations
func fetchRecommendations(accountID, locale string, n int) ([]DiscoverCard, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account id required for recommendations")
	}
	if n <= 0 {
		n = 20
	}
	locale = normalizeDiscoverLocale(locale)
	endpoint := fmt.Sprintf(
		"https://www.crunchyroll.com/content/v2/discover/%s/recommendations?n=%d&start=0&locale=%s",
		url.PathEscape(accountID), n, url.QueryEscape(locale),
	)
	body, err := discoverGET(endpoint)
	if err != nil {
		return nil, err
	}
	return parsePanelListResponse(body, locale), nil
}

// fetchSimilarTo loads "because you watched" style rails.
// Prefers account-scoped similar_to, falls back to public similar_to.
func fetchSimilarTo(accountID, mediaID, locale string, n int) ([]DiscoverCard, error) {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return nil, fmt.Errorf("source media id required for similar_to")
	}
	if n <= 0 {
		n = 20
	}
	locale = normalizeDiscoverLocale(locale)
	var endpoints []string
	if accountID = strings.TrimSpace(accountID); accountID != "" {
		endpoints = append(endpoints, fmt.Sprintf(
			"https://www.crunchyroll.com/content/v2/discover/%s/similar_to/%s?n=%d&locale=%s",
			url.PathEscape(accountID), url.PathEscape(mediaID), n, url.QueryEscape(locale),
		))
	}
	endpoints = append(endpoints, fmt.Sprintf(
		"https://www.crunchyroll.com/content/v2/discover/similar_to/%s?n=%d&locale=%s",
		url.PathEscape(mediaID), n, url.QueryEscape(locale),
	))
	var lastErr error
	for _, endpoint := range endpoints {
		body, err := discoverGET(endpoint)
		if err != nil {
			lastErr = err
			continue
		}
		cards := parsePanelListResponse(body, locale)
		if len(cards) > 0 {
			return cards, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}

// fetchBrowseLink follows a dynamic_collection link (absolute API path, site path, or query).
func fetchBrowseLink(link, locale string, n int) ([]DiscoverCard, error) {
	if n <= 0 {
		n = 20
	}
	locale = normalizeDiscoverLocale(locale)
	endpoint, err := resolveBrowseEndpoint(link, locale, n)
	if err != nil {
		return nil, err
	}
	body, err := discoverGET(endpoint)
	if err != nil {
		return nil, err
	}
	// Browse returns CMS-like objects in data[]; some links return panel wrappers.
	cards := parsePanelListResponse(body, locale)
	if len(cards) > 0 {
		return cards, nil
	}
	var bulk struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &bulk); err != nil {
		return nil, fmt.Errorf("decode browse link: %w", err)
	}
	for _, raw := range bulk.Data {
		if card, ok := cardFromCMSObject(raw, locale); ok {
			cards = append(cards, card)
		}
	}
	return cards, nil
}

func resolveBrowseEndpoint(link, locale string, n int) (string, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return fmt.Sprintf(
			"https://www.crunchyroll.com/content/v2/discover/browse?sort_by=popularity&type=series&n=%d&start=0&locale=%s",
			n, url.QueryEscape(locale),
		), nil
	}
	// Absolute URL
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		u, err := url.Parse(link)
		if err != nil {
			return "", fmt.Errorf("parse browse link: %w", err)
		}
		q := u.Query()
		if q.Get("locale") == "" {
			q.Set("locale", locale)
		}
		if q.Get("n") == "" {
			q.Set("n", fmt.Sprintf("%d", n))
		}
		u.RawQuery = q.Encode()
		return u.String(), nil
	}
	// API-relative path e.g. /content/v2/discover/browse?...
	if strings.HasPrefix(link, "/content/") {
		u, err := url.Parse("https://www.crunchyroll.com" + link)
		if err != nil {
			return "", err
		}
		q := u.Query()
		if q.Get("locale") == "" {
			q.Set("locale", locale)
		}
		if q.Get("n") == "" {
			q.Set("n", fmt.Sprintf("%d", n))
		}
		u.RawQuery = q.Encode()
		return u.String(), nil
	}
	// Query-only fragment e.g. sort_by=popularity&type=series
	if strings.Contains(link, "=") && !strings.HasPrefix(link, "/") {
		q, err := url.ParseQuery(strings.TrimPrefix(link, "?"))
		if err != nil {
			return "", err
		}
		if q.Get("locale") == "" {
			q.Set("locale", locale)
		}
		if q.Get("n") == "" {
			q.Set("n", fmt.Sprintf("%d", n))
		}
		return "https://www.crunchyroll.com/content/v2/discover/browse?" + q.Encode(), nil
	}
	// Site path fallback → popularity browse (link is not an API catalog path)
	return fmt.Sprintf(
		"https://www.crunchyroll.com/content/v2/discover/browse?sort_by=popularity&type=series&n=%d&start=0&locale=%s",
		n, url.QueryEscape(locale),
	), nil
}

type playheadInfo struct {
	Playhead     float64 // seconds
	FullyWatched bool
}

func fetchPlayheads(accountID string, contentIDs []string) (map[string]playheadInfo, error) {
	clean := make([]string, 0, len(contentIDs))
	seen := map[string]struct{}{}
	for _, id := range contentIDs {
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
	out := map[string]playheadInfo{}
	const batch = 50
	for i := 0; i < len(clean); i += batch {
		j := i + batch
		if j > len(clean) {
			j = len(clean)
		}
		chunk := clean[i:j]
		// content_ids keep raw commas (same convention as cms/objects).
		endpoint := fmt.Sprintf(
			"https://www.crunchyroll.com/content/v2/%s/playheads?content_ids=%s",
			url.PathEscape(accountID),
			strings.Join(chunk, ","),
		)
		body, err := discoverGET(endpoint)
		if err != nil {
			return out, err
		}
		partial, err := parsePlayheadsResponse(body)
		if err != nil {
			return out, err
		}
		for k, v := range partial {
			out[k] = v
		}
	}
	return out, nil
}

// parsePlayheadsResponse maps content_id → playhead info.
// Shape: { "data": [ { "content_id", "playhead", "fully_watched" } ] }
// or object map under data.
func parsePlayheadsResponse(body []byte) (map[string]playheadInfo, error) {
	out := map[string]playheadInfo{}
	var asList struct {
		Data []struct {
			ContentID    string  `json:"content_id"`
			Playhead     float64 `json:"playhead"`
			FullyWatched bool    `json:"fully_watched"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &asList); err == nil && len(asList.Data) > 0 {
		for _, row := range asList.Data {
			id := strings.TrimSpace(row.ContentID)
			if id == "" {
				continue
			}
			out[id] = playheadInfo{Playhead: row.Playhead, FullyWatched: row.FullyWatched}
		}
		return out, nil
	}
	// Map form: data: { "GWxxx": { playhead, fully_watched } }
	var asMap struct {
		Data map[string]struct {
			Playhead     float64 `json:"playhead"`
			FullyWatched bool    `json:"fully_watched"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &asMap); err != nil {
		return nil, fmt.Errorf("decode playheads: %w", err)
	}
	for id, row := range asMap.Data {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out[id] = playheadInfo{Playhead: row.Playhead, FullyWatched: row.FullyWatched}
	}
	return out, nil
}

// parseHistoryResponse builds cards from watch-history payload.
// Returns cards, content ids for playhead lookup, and duration_ms by content id.
func parseHistoryResponse(body []byte, locale string) (cards []DiscoverCard, contentIDs []string, durations map[string]int64) {
	durations = map[string]int64{}
	var bulk struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &bulk); err != nil {
		return nil, nil, durations
	}
	for _, raw := range bulk.Data {
		card, contentID, durationMS, ok := cardFromHistoryItem(raw, locale)
		if !ok {
			continue
		}
		cards = append(cards, card)
		if contentID != "" {
			contentIDs = append(contentIDs, contentID)
			if durationMS > 0 {
				durations[contentID] = durationMS
			}
		}
	}
	return cards, contentIDs, durations
}

func cardFromHistoryItem(raw json.RawMessage, locale string) (DiscoverCard, string, int64, bool) {
	// Common shapes:
	// 1) { panel: {…cms…}, fully_watched?, parent_id? }
	// 2) flat cms object
	// 3) { id, panel, playhead? }
	var wrap struct {
		ID           string          `json:"id"`
		Panel        json.RawMessage `json:"panel"`
		FullyWatched bool            `json:"fully_watched"`
		Playhead     float64         `json:"playhead"`
		ParentID     string          `json:"parent_id"`
	}
	_ = json.Unmarshal(raw, &wrap)

	var card DiscoverCard
	var ok bool
	if len(wrap.Panel) > 0 {
		card, ok = cardFromCMSObject(wrap.Panel, locale)
	} else {
		card, ok = cardFromCMSObject(raw, locale)
	}
	if !ok {
		return DiscoverCard{}, "", 0, false
	}

	// Prefer series title for CW landscape when episode metadata is present.
	var metaHolder struct {
		Panel *struct {
			EpisodeMetadata *cmsEpisodeMetadata `json:"episode_metadata"`
			Title           string              `json:"title"`
		} `json:"panel"`
		EpisodeMetadata *cmsEpisodeMetadata `json:"episode_metadata"`
	}
	_ = json.Unmarshal(raw, &metaHolder)
	meta := metaHolder.EpisodeMetadata
	if meta == nil && metaHolder.Panel != nil {
		meta = metaHolder.Panel.EpisodeMetadata
	}
	if meta != nil {
		if sid := strings.TrimSpace(meta.SeriesID); sid != "" {
			card.SeriesID = sid
		}
		if meta.SeriesTitle != "" {
			card.Title = meta.SeriesTitle
		}
		if card.Subtitle == "" {
			card.Subtitle = episodeSubtitle(meta)
		}
	}
	if card.SeriesID == "" && strings.TrimSpace(wrap.ParentID) != "" {
		// Some history payloads put series id on parent_id.
		card.SeriesID = strings.TrimSpace(wrap.ParentID)
	}

	contentID := firstNonEmpty(wrap.ID, card.ID)
	var durationMS int64
	if meta != nil {
		durationMS = meta.DurationMS
	}
	// Inline playhead on history item if present.
	if wrap.FullyWatched {
		card.Progress = progressPtr(1)
	} else if wrap.Playhead > 0 && durationMS > 0 {
		card.Progress = progressPtr(wrap.Playhead / (float64(durationMS) / 1000.0))
	}
	return card, contentID, durationMS, true
}

func applyPlayheadProgress(cards []DiscoverCard, ph map[string]playheadInfo, durations map[string]int64) {
	if len(ph) == 0 {
		return
	}
	for i := range cards {
		// Prefer card.ID (usually episode id for history).
		info, ok := ph[cards[i].ID]
		if !ok {
			continue
		}
		if info.FullyWatched {
			cards[i].Progress = progressPtr(1)
			continue
		}
		// Skip if history already set progress and playhead is zero.
		dur := durations[cards[i].ID]
		if dur <= 0 {
			// Unknown duration: leave progress unset rather than inventing a value.
			continue
		}
		cards[i].Progress = progressPtr(info.Playhead / (float64(dur) / 1000.0))
	}
}

// parsePanelListResponse handles list endpoints that wrap CMS objects in panel or flat data.
func parsePanelListResponse(body []byte, locale string) []DiscoverCard {
	var bulk struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &bulk); err != nil {
		return nil
	}
	var cards []DiscoverCard
	for _, raw := range bulk.Data {
		if card, ok := cardFromPanelListItem(raw, locale); ok {
			cards = append(cards, card)
		}
	}
	return dedupeCards(cards)
}

func cardFromPanelListItem(raw json.RawMessage, locale string) (DiscoverCard, bool) {
	var wrap struct {
		Panel    json.RawMessage `json:"panel"`
		ID       string          `json:"id"`
		ParentID string          `json:"parent_id"`
		SeriesID string          `json:"series_id"`
	}
	_ = json.Unmarshal(raw, &wrap)

	try := func(src json.RawMessage) (DiscoverCard, bool) {
		card, ok := cardFromCMSObject(src, locale)
		if !ok {
			return DiscoverCard{}, false
		}
		if card.SeriesID == "" {
			card.SeriesID = firstNonEmpty(wrap.SeriesID, wrap.ParentID)
		}
		return card, true
	}

	if len(wrap.Panel) > 0 {
		if card, ok := try(wrap.Panel); ok {
			return card, true
		}
	}
	if card, ok := try(raw); ok {
		return card, true
	}
	var nested struct {
		Resource json.RawMessage `json:"resource"`
		Item     json.RawMessage `json:"item"`
	}
	_ = json.Unmarshal(raw, &nested)
	if len(nested.Resource) > 0 {
		if card, ok := try(nested.Resource); ok {
			return card, true
		}
	}
	if len(nested.Item) > 0 {
		if card, ok := try(nested.Item); ok {
			return card, true
		}
	}
	return DiscoverCard{}, false
}
