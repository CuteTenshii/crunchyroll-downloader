package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Block kinds for HomeBlock.Kind.
const (
	HomeBlockHero          = "hero"
	HomeBlockLandscapeRail = "landscape_rail"
	HomeBlockPosterRail    = "poster_rail"
	HomeBlockBanner        = "banner"
)

// Rank styles for HomeBlock.RankStyle.
const (
	RankStyleNone  = "none"
	RankStyleTop10 = "top10"
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
	OpenURL  string   `json:"openUrl"`
	Progress *float64 `json:"progress,omitempty"` // 0..1 Continue Watching
	Rank     int      `json:"rank,omitempty"`     // 1..10 Top 10 chrome
	Subtitle string   `json:"subtitle,omitempty"` // e.g. S01E12 / E3
	// SeriesID is set for episode-like cards so rails can promote to series posters.
	SeriesID string `json:"seriesId,omitempty"`
	// EpisodeTitle is the episode name (CW layout); Title is the series name.
	EpisodeTitle string `json:"episodeTitle,omitempty"`
	// RemainingLabel is overlay text like "21m remaining" / "21m restantes".
	RemainingLabel string `json:"remainingLabel,omitempty"`
	// DurationMS is the episode duration when known (for remaining-time label).
	DurationMS int64 `json:"durationMs,omitempty"`
}

// DiscoverRail is a horizontal home-feed section (backward-compat view of a rail block).
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

// HomeBanner is a wide in-feed promo (only series/watch open targets).
type HomeBanner struct {
	ID      string `json:"id,omitempty"`
	Title   string `json:"title,omitempty"`
	WideURL string `json:"wideUrl,omitempty"`
	OpenURL string `json:"openUrl,omitempty"`
}

// HomeBlock is one ordered section of the Discover home feed.
type HomeBlock struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"` // hero | landscape_rail | poster_rail | banner
	Title     string         `json:"title,omitempty"`
	RankStyle string         `json:"rankStyle"` // none | top10
	Cards     []DiscoverCard `json:"cards,omitempty"`
	Heroes    []DiscoverHero `json:"heroes,omitempty"`
	Banner    *HomeBanner    `json:"banner,omitempty"`
}

// HomeFeedPage is one page of Discover home content.
// Blocks is the primary ordered model; Heroes/Rails are derived for backward compatibility.
type HomeFeedPage struct {
	Blocks      []HomeBlock    `json:"blocks"`
	Heroes      []DiscoverHero `json:"heroes"`
	Rails       []DiscoverRail `json:"rails"`
	NextStart   int            `json:"nextStart"`
	PageSize    int            `json:"pageSize"`
	TotalApprox int            `json:"totalApprox,omitempty"`
}

// Hydrate kinds for dynamic / curated feed entries that need network follow-up.
const (
	hydrateNone              = ""
	hydrateHistory           = "history"
	hydrateWatchlist         = "watchlist"
	hydrateRecommendations   = "recommendations"
	hydrateBecauseYouWatched = "because_you_watched"
	hydrateBrowse            = "browse"
	hydrateCurated           = "curated"
)

// feedMapResult is the pure mapping outcome for one home_feed data item.
type feedMapResult struct {
	Block         HomeBlock
	Hydrate       string
	CuratedIDs    []string
	SourceMediaID string
	BrowseLink    string
	Skip          bool
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
	EpisodeMetadata *cmsEpisodeMetadata `json:"episode_metadata"`
}

type cmsEpisodeMetadata struct {
	SeriesID      string          `json:"series_id"`
	SeriesTitle   string          `json:"series_title"`
	SeasonNumber  int             `json:"season_number"`
	Episode       json.RawMessage `json:"episode"`
	EpisodeNumber json.RawMessage `json:"episode_number"`
	DurationMS    int64           `json:"duration_ms"`
}

type feedItemHead struct {
	ResourceType  string `json:"resource_type"`
	ResponseType  string `json:"response_type"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	ID            string `json:"id"`
	Link          string `json:"link"`
	SourceMediaID string `json:"source_media_id"`
}

// FetchHomeFeed loads a page of the personalized Discover home feed.
// Requires a valid token (refreshAccessToken) so GetAccountID() is non-empty.
// Catalog only — no Widevine. Individual hydrator failures drop that block only.
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
	page.Blocks = buildHomeBlocks(bulk.Data, accountID, locale, defaultFeedHydrator{})
	deriveCompatFromBlocks(&page)
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
		block := HomeBlock{
			ID:        "browse-popular",
			Kind:      HomeBlockPosterRail,
			Title:     "Popular",
			RankStyle: RankStyleNone,
			Cards:     cards,
		}
		applyTop10Ranks(&block)
		page.Blocks = []HomeBlock{block}
	}
	deriveCompatFromBlocks(&page)
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

// feedHydrator loads dynamic/curated card payloads. Tests inject fakes.
type feedHydrator interface {
	History(accountID, locale string, n int) ([]DiscoverCard, error)
	Watchlist(accountID, locale string, n int) ([]DiscoverCard, error)
	Recommendations(accountID, locale string, n int) ([]DiscoverCard, error)
	SimilarTo(accountID, mediaID, locale string, n int) ([]DiscoverCard, error)
	Browse(link, locale string, n int) ([]DiscoverCard, error)
	Objects(ids []string, locale string) ([]DiscoverCard, error)
}

type defaultFeedHydrator struct{}

func (defaultFeedHydrator) History(accountID, locale string, n int) ([]DiscoverCard, error) {
	return fetchWatchHistory(accountID, locale, n)
}
func (defaultFeedHydrator) Watchlist(accountID, locale string, n int) ([]DiscoverCard, error) {
	return fetchWatchlist(accountID, locale, n)
}
func (defaultFeedHydrator) Recommendations(accountID, locale string, n int) ([]DiscoverCard, error) {
	return fetchRecommendations(accountID, locale, n)
}
func (defaultFeedHydrator) SimilarTo(accountID, mediaID, locale string, n int) ([]DiscoverCard, error) {
	return fetchSimilarTo(accountID, mediaID, locale, n)
}
func (defaultFeedHydrator) Browse(link, locale string, n int) ([]DiscoverCard, error) {
	return fetchBrowseLink(link, locale, n)
}
func (defaultFeedHydrator) Objects(ids []string, locale string) ([]DiscoverCard, error) {
	return ResolveObjectCards(ids, locale)
}

// buildHomeBlocks walks home_feed data in order, maps approved types, hydrates dynamics.
// Dynamic hydrators run in parallel; hydrator errors drop that block only.
func buildHomeBlocks(data []json.RawMessage, accountID, locale string, h feedHydrator) []HomeBlock {
	const cardLimit = 20

	type slot struct {
		mapped feedMapResult
		// For hydrateCurated we keep the empty block skeleton and fill later.
		block HomeBlock
		// Dynamic hydrate job index into dynJobs, or -1.
		dynIdx int
	}

	type dynJob struct {
		hydrate       string
		sourceMediaID string
		browseLink    string
		// result filled by worker
		cards []DiscoverCard
		ok    bool
	}

	var slots []slot
	var dynJobs []dynJob
	var curatedIDs []string
	type curatedRef struct {
		slotIdx int
		ids     []string
	}
	var curatedRefs []curatedRef

	for i, raw := range data {
		mapped := mapHomeFeedEntry(raw, i, locale)
		if mapped.Skip {
			continue
		}
		s := slot{mapped: mapped, block: mapped.Block, dynIdx: -1}
		switch mapped.Hydrate {
		case hydrateNone:
			// static
		case hydrateCurated:
			curatedRefs = append(curatedRefs, curatedRef{slotIdx: len(slots), ids: mapped.CuratedIDs})
			curatedIDs = append(curatedIDs, mapped.CuratedIDs...)
		case hydrateHistory, hydrateWatchlist, hydrateRecommendations, hydrateBecauseYouWatched, hydrateBrowse:
			if mapped.Hydrate == hydrateBecauseYouWatched && mapped.SourceMediaID == "" {
				continue
			}
			s.dynIdx = len(dynJobs)
			dynJobs = append(dynJobs, dynJob{
				hydrate:       mapped.Hydrate,
				sourceMediaID: mapped.SourceMediaID,
				browseLink:    mapped.BrowseLink,
			})
		default:
			continue
		}
		slots = append(slots, s)
	}

	// Parallel dynamic hydrates (main latency source on first Home load).
	if len(dynJobs) > 0 {
		var wg sync.WaitGroup
		for i := range dynJobs {
			wg.Add(1)
			go func(j *dynJob) {
				defer wg.Done()
				var cards []DiscoverCard
				var err error
				switch j.hydrate {
				case hydrateHistory:
					cards, err = h.History(accountID, locale, cardLimit)
				case hydrateWatchlist:
					cards, err = h.Watchlist(accountID, locale, cardLimit)
				case hydrateRecommendations:
					cards, err = h.Recommendations(accountID, locale, cardLimit)
				case hydrateBecauseYouWatched:
					cards, err = h.SimilarTo(accountID, j.sourceMediaID, locale, cardLimit)
				case hydrateBrowse:
					cards, err = h.Browse(j.browseLink, locale, cardLimit)
				}
				if err != nil || len(cards) == 0 {
					return
				}
				// Continue Watching: episode thumbs, one card per series (most recent first).
				// My List / New Releases / recs / similar: promote episodes → series posters.
				if j.hydrate == hydrateHistory {
					cards = preferEpisodeLandscapeArt(cards)
					cards = attachRemainingLabels(cards, locale)
					// History API is newest-first → keep first card per series.
					cards = dedupeCardsBySeriesKeepFirst(cards)
				} else {
					cards = promoteEpisodesToSeriesCards(cards, locale, h)
					// Browse/new releases can repeat the same series/episode.
					cards = dedupeCards(cards)
				}
				j.cards = cards
				j.ok = len(cards) > 0
			}(&dynJobs[i])
		}
		wg.Wait()
	}

	// Single batched cms/objects for all curated rails.
	byID := map[string]DiscoverCard{}
	if len(curatedIDs) > 0 {
		if cards, err := h.Objects(curatedIDs, locale); err == nil {
			for _, c := range cards {
				byID[c.ID] = c
			}
		}
	}
	for _, ref := range curatedRefs {
		if ref.slotIdx < 0 || ref.slotIdx >= len(slots) {
			continue
		}
		cards := make([]DiscoverCard, 0, len(ref.ids))
		seen := map[string]struct{}{}
		for _, id := range ref.ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			if c, ok := byID[id]; ok {
				cards = append(cards, c)
			}
		}
		slots[ref.slotIdx].block.Cards = promoteEpisodesToSeriesCards(cards, locale, h)
	}

	// Assemble in feed order.
	var out []HomeBlock
	for _, s := range slots {
		block := s.block
		if s.dynIdx >= 0 {
			job := dynJobs[s.dynIdx]
			if !job.ok {
				continue
			}
			block.Cards = job.cards
		}
		if !blockHasContent(block) {
			continue
		}
		applyTop10Ranks(&block)
		out = append(out, block)
	}
	return out
}

// promoteEpisodesToSeriesCards rewrites episode-like cards to series cards so rails
// show anime posters/titles (My List, New Releases, Continue Watching). Keeps
// episode subtitle + progress. Resolves series objects in one batch when needed.
func promoteEpisodesToSeriesCards(cards []DiscoverCard, locale string, h feedHydrator) []DiscoverCard {
	if len(cards) == 0 {
		return cards
	}
	locale = normalizeDiscoverLocale(locale)
	need := make([]string, 0)
	seenNeed := map[string]struct{}{}
	for _, c := range cards {
		sid := strings.TrimSpace(c.SeriesID)
		if sid == "" {
			continue
		}
		if !isEpisodeLikeCard(c) {
			continue
		}
		if _, ok := seenNeed[sid]; ok {
			continue
		}
		seenNeed[sid] = struct{}{}
		need = append(need, sid)
	}
	seriesByID := map[string]DiscoverCard{}
	if len(need) > 0 && h != nil {
		if resolved, err := h.Objects(need, locale); err == nil {
			for _, c := range resolved {
				seriesByID[c.ID] = c
			}
		}
	}

	out := make([]DiscoverCard, 0, len(cards))
	for _, c := range cards {
		if !isEpisodeLikeCard(c) || strings.TrimSpace(c.SeriesID) == "" {
			out = append(out, c)
			continue
		}
		sid := strings.TrimSpace(c.SeriesID)
		progress := c.Progress
		subtitle := c.Subtitle
		if subtitle == "" && c.Title != "" && (c.Type == "episode" || strings.Contains(c.Type, "episode")) {
			// Keep episode title as subtitle when we only had episode name.
			subtitle = c.Title
		}
		if s, ok := seriesByID[sid]; ok {
			s.Progress = progress
			if subtitle != "" {
				s.Subtitle = subtitle
			}
			s.SeriesID = sid
			out = append(out, s)
			continue
		}
		// Soft fallback without CMS series object.
		c.ID = sid
		c.Type = "series"
		c.OpenURL = seriesOpenURL(sid, "", locale)
		c.Progress = progress
		c.Subtitle = subtitle
		out = append(out, c)
	}
	return out
}

func isEpisodeLikeCard(c DiscoverCard) bool {
	typ := strings.ToLower(strings.TrimSpace(c.Type))
	if typ == "episode" || strings.Contains(typ, "episode") {
		return true
	}
	// History sometimes leaves type empty but series id set.
	if strings.TrimSpace(c.SeriesID) != "" && strings.Contains(strings.ToLower(c.OpenURL), "/watch/") {
		return true
	}
	return false
}

// preferEpisodeLandscapeArt ensures CW landscape cards use episode stills
// (thumbnail/wide) rather than tall series posters when both exist.
func preferEpisodeLandscapeArt(cards []DiscoverCard) []DiscoverCard {
	for i := range cards {
		c := &cards[i]
		if !isEpisodeLikeCard(*c) {
			continue
		}
		// Landscape rail uses wide first; if wide is empty but poster is a tall
		// series art, keep whatever we have. History parser already prefers thumbs.
		if c.WideURL == "" && c.PosterURL != "" {
			c.WideURL = c.PosterURL
		}
	}
	return cards
}

// dedupeCardsBySeriesKeepFirst keeps the first card per series (or id).
// Watch-history is newest-first, so this is the most recently watched episode per show.
func dedupeCardsBySeriesKeepFirst(cards []DiscoverCard) []DiscoverCard {
	if len(cards) == 0 {
		return cards
	}
	seen := map[string]struct{}{}
	out := make([]DiscoverCard, 0, len(cards))
	for _, c := range cards {
		key := strings.TrimSpace(c.SeriesID)
		if key == "" {
			key = strings.TrimSpace(c.ID)
		}
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(c.Title))
		}
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}

// attachRemainingLabels fills RemainingLabel from progress + duration when possible.
func attachRemainingLabels(cards []DiscoverCard, locale string) []DiscoverCard {
	locale = strings.ToLower(normalizeDiscoverLocale(locale))
	pt := strings.HasPrefix(locale, "pt")
	for i := range cards {
		c := &cards[i]
		if c.RemainingLabel != "" {
			continue
		}
		if c.Progress == nil || c.DurationMS <= 0 {
			continue
		}
		p := *c.Progress
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		if p >= 0.98 {
			if pt {
				c.RemainingLabel = "Concluído"
			} else {
				c.RemainingLabel = "Completed"
			}
			continue
		}
		remainSec := int((float64(c.DurationMS) / 1000.0) * (1.0 - p))
		if remainSec < 60 {
			if pt {
				c.RemainingLabel = "<1m restante"
			} else {
				c.RemainingLabel = "<1m left"
			}
			continue
		}
		mins := remainSec / 60
		if pt {
			c.RemainingLabel = fmt.Sprintf("%dm restantes", mins)
		} else {
			c.RemainingLabel = fmt.Sprintf("%dm left", mins)
		}
	}
	return cards
}

// mapHomeFeedEntry is a pure mapper from one home_feed item to a block skeleton + hydrate hint.
func mapHomeFeedEntry(raw json.RawMessage, index int, locale string) feedMapResult {
	var head feedItemHead
	if err := json.Unmarshal(raw, &head); err != nil {
		return feedMapResult{Skip: true}
	}
	resType := strings.ToLower(strings.TrimSpace(head.ResourceType))
	respType := strings.ToLower(strings.TrimSpace(head.ResponseType))
	id := firstNonEmpty(head.ID, fmt.Sprintf("block-%d", index))
	title := strings.TrimSpace(head.Title)

	switch resType {
	case "hero_carousel":
		heroes := mapHeroItems(raw, locale)
		if len(heroes) == 0 {
			return feedMapResult{Skip: true}
		}
		return feedMapResult{
			Block: HomeBlock{
				ID:        id,
				Kind:      HomeBlockHero,
				Title:     title,
				RankStyle: RankStyleNone,
				Heroes:    heroes,
			},
			Hydrate: hydrateNone,
		}

	case "curated_collection":
		if respType != "" && respType != "series" {
			return feedMapResult{Skip: true}
		}
		var coll struct {
			Title string   `json:"title"`
			IDs   []string `json:"ids"`
			ID    string   `json:"id"`
		}
		if err := json.Unmarshal(raw, &coll); err != nil || len(coll.IDs) == 0 {
			return feedMapResult{Skip: true}
		}
		return feedMapResult{
			Block: HomeBlock{
				ID:        firstNonEmpty(coll.ID, id),
				Kind:      HomeBlockPosterRail,
				Title:     firstNonEmpty(coll.Title, title, "Collection"),
				RankStyle: RankStyleNone,
			},
			Hydrate:    hydrateCurated,
			CuratedIDs: coll.IDs,
		}

	case "panel":
		var panelWrap struct {
			Panel json.RawMessage `json:"panel"`
			Title string          `json:"title"`
			ID    string          `json:"id"`
		}
		if err := json.Unmarshal(raw, &panelWrap); err != nil || len(panelWrap.Panel) == 0 {
			return feedMapResult{Skip: true}
		}
		card, ok := cardFromCMSObject(panelWrap.Panel, locale)
		if !ok {
			return feedMapResult{Skip: true}
		}
		return feedMapResult{
			Block: HomeBlock{
				ID:        firstNonEmpty(panelWrap.ID, "panel-"+card.ID),
				Kind:      HomeBlockPosterRail,
				Title:     firstNonEmpty(panelWrap.Title, title, "Featured"),
				RankStyle: RankStyleNone,
				Cards:     []DiscoverCard{card},
			},
			Hydrate: hydrateNone,
		}

	case "in_feed_banner":
		banner, ok := mapInFeedBanner(raw, head, locale)
		if !ok {
			return feedMapResult{Skip: true}
		}
		return feedMapResult{
			Block: HomeBlock{
				ID:        firstNonEmpty(banner.ID, id),
				Kind:      HomeBlockBanner,
				Title:     firstNonEmpty(banner.Title, title),
				RankStyle: RankStyleNone,
				Banner:    &banner,
			},
			Hydrate: hydrateNone,
		}

	case "dynamic_collection":
		return mapDynamicCollection(raw, head, id, title, respType)

	case "music_carousel", "music", "game", "games", "news_feed", "news", "store":
		return feedMapResult{Skip: true}

	default:
		// Unknown resource types are skipped (music/game/news often appear with other names).
		if strings.Contains(resType, "music") || strings.Contains(resType, "game") ||
			strings.Contains(resType, "news") || strings.Contains(resType, "manga") {
			return feedMapResult{Skip: true}
		}
		return feedMapResult{Skip: true}
	}
}

func mapDynamicCollection(raw json.RawMessage, head feedItemHead, id, title, respType string) feedMapResult {
	// Pull optional fields that may only appear on the full object.
	var extra struct {
		SourceMediaID string `json:"source_media_id"`
		Link          string `json:"link"`
		// Nested source sometimes used by because_you_watched
		SourceMedia struct {
			ID string `json:"id"`
		} `json:"source_media"`
	}
	_ = json.Unmarshal(raw, &extra)
	sourceID := firstNonEmpty(head.SourceMediaID, extra.SourceMediaID, extra.SourceMedia.ID)
	link := firstNonEmpty(head.Link, extra.Link)
	// similar_to / because_you_watched often only carry the media id in the link path.
	if sourceID == "" {
		sourceID = mediaIDFromSimilarToLink(link)
	}

	switch respType {
	case "history", "continue_watching", "watch_history":
		return feedMapResult{
			Block: HomeBlock{
				ID:        id,
				Kind:      HomeBlockLandscapeRail,
				Title:     firstNonEmpty(title, "Continue Watching"),
				RankStyle: RankStyleNone,
			},
			Hydrate: hydrateHistory,
		}
	case "watchlist":
		return feedMapResult{
			Block: HomeBlock{
				ID:        id,
				Kind:      HomeBlockPosterRail,
				Title:     firstNonEmpty(title, "Watchlist"),
				RankStyle: RankStyleNone,
			},
			Hydrate: hydrateWatchlist,
		}
	case "recommendations", "recommendation":
		return feedMapResult{
			Block: HomeBlock{
				ID:        id,
				Kind:      HomeBlockPosterRail,
				Title:     firstNonEmpty(title, "Recommended"),
				RankStyle: RankStyleNone,
			},
			Hydrate: hydrateRecommendations,
		}
	case "because_you_watched", "similar_to":
		if sourceID == "" {
			return feedMapResult{Skip: true}
		}
		return feedMapResult{
			Block: HomeBlock{
				ID:        id,
				Kind:      HomeBlockPosterRail,
				Title:     firstNonEmpty(title, "Because You Watched"),
				RankStyle: RankStyleNone,
			},
			Hydrate:       hydrateBecauseYouWatched,
			SourceMediaID: sourceID,
		}
	case "browse", "recent_episodes", "recent_episode":
		// New releases / browse: series posters (episodes promoted after hydrate).
		kind := HomeBlockPosterRail
		// Prefer explicit link; empty link still hydrates default browse popularity.
		return feedMapResult{
			Block: HomeBlock{
				ID:        id,
				Kind:      kind,
				Title:     firstNonEmpty(title, "Browse"),
				RankStyle: RankStyleNone,
			},
			Hydrate:    hydrateBrowse,
			BrowseLink: link,
		}
	default:
		// Some dynamic rows only set link without a known response_type.
		if link != "" && (strings.Contains(link, "browse") || strings.Contains(link, "watchlist") ||
			strings.Contains(link, "recommendations") || strings.Contains(link, "similar_to")) {
			hydrate := hydrateBrowse
			kind := HomeBlockPosterRail
			switch {
			case strings.Contains(link, "watchlist"):
				hydrate = hydrateWatchlist
			case strings.Contains(link, "recommendations"):
				hydrate = hydrateRecommendations
			case strings.Contains(link, "similar_to"):
				hydrate = hydrateBecauseYouWatched
			case strings.Contains(link, "watch-history") || strings.Contains(link, "history"):
				hydrate = hydrateHistory
				kind = HomeBlockLandscapeRail
			}
			return feedMapResult{
				Block: HomeBlock{
					ID:        id,
					Kind:      kind,
					Title:     firstNonEmpty(title, "Collection"),
					RankStyle: RankStyleNone,
				},
				Hydrate:       hydrate,
				BrowseLink:    link,
				SourceMediaID: sourceID,
			}
		}
		return feedMapResult{Skip: true}
	}
}

// mediaIDFromSimilarToLink extracts a media id from a path containing similar_to/{id}.
// Examples: /similar_to/GYZJ43JMR, /content/v2/discover/similar_to/ID?n=20
// Returns empty when the segment is missing or unparseable.
func mediaIDFromSimilarToLink(link string) string {
	link = strings.TrimSpace(link)
	if link == "" {
		return ""
	}
	if i := strings.IndexAny(link, "?#"); i >= 0 {
		link = link[:i]
	}
	// Prefer path-only when an absolute URL is provided.
	if u, err := url.Parse(link); err == nil && u.Path != "" {
		link = u.Path
	}
	parts := strings.Split(strings.Trim(link, "/"), "/")
	for i, p := range parts {
		if !strings.EqualFold(p, "similar_to") {
			continue
		}
		if i+1 >= len(parts) {
			return ""
		}
		id := strings.TrimSpace(parts[i+1])
		if id == "" || strings.EqualFold(id, "similar_to") {
			return ""
		}
		return id
	}
	return ""
}

func mapHeroItems(raw json.RawMessage, locale string) []DiscoverHero {
	var hero struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &hero); err != nil {
		return nil
	}
	var out []DiscoverHero
	for _, itemRaw := range hero.Items {
		h := mapOneHeroItem(itemRaw, locale)
		if h.Title != "" || h.OpenURL != "" || h.WideURL != "" || h.PosterURL != "" {
			out = append(out, h)
		}
	}
	return out
}

func mapOneHeroItem(raw json.RawMessage, locale string) DiscoverHero {
	var it struct {
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Link        string          `json:"link"`
		URL         string          `json:"url"`
		ButtonText  string          `json:"button_text"`
		Slug        string          `json:"slug"`
		ImageURL    string          `json:"image_url"`
		Images      json.RawMessage `json:"images"`
		Panel       json.RawMessage `json:"panel"`
	}
	_ = json.Unmarshal(raw, &it)

	wide, poster := heroImageURLs(it.Images)
	if wide == "" {
		wide = strings.TrimSpace(it.ImageURL)
	}
	h := DiscoverHero{
		Title:       it.Title,
		Description: it.Description,
		WideURL:     absoluteAssetURL(wide),
		PosterURL:   absoluteAssetURL(poster),
		ButtonText:  it.ButtonText,
		OpenURL:     normalizeOpenURL(firstNonEmpty(it.Link, it.URL), locale),
	}

	// Always merge panel CMS art when present (CR often only puts art on panel).
	if len(it.Panel) > 0 {
		// Flexible panel image scrape (not only typed CRImages).
		pw, pp := heroImagesFromPanelRaw(it.Panel)
		if h.WideURL == "" {
			h.WideURL = absoluteAssetURL(pw)
		}
		if h.PosterURL == "" {
			h.PosterURL = absoluteAssetURL(pp)
		}
		if card, ok := cardFromCMSObject(it.Panel, locale); ok {
			if h.OpenURL == "" {
				h.OpenURL = card.OpenURL
			}
			if h.PosterURL == "" {
				h.PosterURL = absoluteAssetURL(card.PosterURL)
			}
			if h.WideURL == "" {
				h.WideURL = absoluteAssetURL(firstNonEmpty(card.WideURL, card.PosterURL))
			}
			if h.Title == "" {
				h.Title = card.Title
			}
			if h.Description == "" {
				h.Description = card.Description
			}
		}
		// Last resort: walk the whole panel JSON for any image URL (imgsrv, etc.).
		if h.WideURL == "" {
			h.WideURL = scrapeAnyImageURL(it.Panel)
		}
	}
	if h.WideURL == "" && len(it.Images) > 0 {
		h.WideURL = scrapeAnyImageURL(it.Images)
	}
	if h.WideURL == "" {
		h.WideURL = h.PosterURL
	}
	// Last resort: walk the entire hero item.
	if h.WideURL == "" {
		h.WideURL = scrapeAnyImageURL(raw)
	}
	return h
}

// heroImageURLs extracts wide/poster URLs from CR hero image objects that may be
// flat string maps, single URL strings, or nested CMS image groups.
func heroImageURLs(raw json.RawMessage) (wide, poster string) {
	if len(raw) == 0 {
		return "", ""
	}
	// Single string URL
	var single string
	if err := json.Unmarshal(raw, &single); err == nil && strings.TrimSpace(single) != "" {
		return strings.TrimSpace(single), ""
	}
	// Flat string map (common on hero_carousel items).
	var flat map[string]json.RawMessage
	if err := json.Unmarshal(raw, &flat); err == nil && len(flat) > 0 {
		pickStr := func(keys ...string) string {
			for _, k := range keys {
				v, ok := flat[k]
				if !ok || len(v) == 0 {
					continue
				}
				var s string
				if json.Unmarshal(v, &s) == nil && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
				// Nested groups under this key
				if u := pickImageURLFromRawGroups(v, 1280); u != "" {
					return u
				}
			}
			return ""
		}
		wide = pickStr(
			"landscape_large", "desktop_large", "desktop_wide", "desktop_hero_wide",
			"poster_wide", "wide", "banner", "background", "background_image",
			"image", "src", "url",
		)
		poster = pickStr(
			"portrait_large", "poster_tall", "tall", "poster", "mobile_poster",
		)
		if wide != "" || poster != "" {
			return wide, poster
		}
	}
	// Nested CRImages groups.
	var imgs CRImages
	if err := json.Unmarshal(raw, &imgs); err == nil {
		wide = firstNonEmpty(
			pickImageURLFromGroups(imgs.PosterWide, 1280),
			pickImageURLFromGroups(imgs.Thumbnail, 960),
		)
		poster = pickImageURLFromGroups(imgs.PosterTall, 480)
		return wide, poster
	}
	return "", ""
}

func heroImagesFromPanelRaw(panel json.RawMessage) (wide, poster string) {
	var obj struct {
		Images json.RawMessage `json:"images"`
	}
	if json.Unmarshal(panel, &obj) != nil {
		return "", ""
	}
	return heroImageURLs(obj.Images)
}

func pickImageURLFromRawGroups(raw json.RawMessage, maxWidth int) string {
	var groups [][]CRImage
	if json.Unmarshal(raw, &groups) != nil {
		return ""
	}
	return pickImageURLFromGroups(groups, maxWidth)
}

func absoluteAssetURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	if strings.HasPrefix(u, "/") {
		// Crunchyroll static CDN paths sometimes appear root-relative under imgsrv.
		if strings.Contains(u, "catalog/") || strings.Contains(u, "cdn-cgi/") {
			return "https://imgsrv.crunchyroll.com" + u
		}
		return "https://www.crunchyroll.com" + u
	}
	return u
}

// scrapeAnyImageURL walks arbitrary JSON for the first usable image URL
// (source fields, imgsrv hosts, common extensions). Last-resort for hero art.
func scrapeAnyImageURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		s = strings.TrimSpace(s)
		if looksLikeImageURL(s) {
			return absoluteAssetURL(s)
		}
		return ""
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		for _, el := range arr {
			if u := scrapeAnyImageURL(el); u != "" {
				return u
			}
		}
		return ""
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	// Prefer explicit source keys first.
	for _, key := range []string{"source", "url", "src", "image", "image_url", "poster_wide", "landscape_large", "desktop_large"} {
		if v, ok := obj[key]; ok {
			if u := scrapeAnyImageURL(v); u != "" {
				return u
			}
		}
	}
	for _, v := range obj {
		if u := scrapeAnyImageURL(v); u != "" {
			return u
		}
	}
	return ""
}

func looksLikeImageURL(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}
	if strings.Contains(s, "imgsrv.crunchyroll.com") || strings.Contains(s, "img1.ak.crunchyroll.com") {
		return true
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "//") {
		return strings.Contains(s, ".jpg") || strings.Contains(s, ".png") ||
			strings.Contains(s, ".webp") || strings.Contains(s, ".jpeg") ||
			strings.Contains(s, "cdn-cgi/image") || strings.Contains(s, "/catalog/")
	}
	return false
}

// mapInFeedBanner extracts a banner only when the open URL is series/watch catalog content.
func mapInFeedBanner(raw json.RawMessage, head feedItemHead, locale string) (HomeBanner, bool) {
	var b struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Link        string `json:"link"`
		URL         string `json:"url"`
		Description string `json:"description"`
		Images      struct {
			LandscapeLarge string `json:"landscape_large"`
			DesktopWide    string `json:"desktop_wide"`
			MobileWide     string `json:"mobile_wide"`
			PosterWide     string `json:"poster_wide"`
		} `json:"images"`
		Panel json.RawMessage `json:"panel"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return HomeBanner{}, false
	}
	open := normalizeOpenURL(firstNonEmpty(b.Link, b.URL, head.Link), locale)
	wide := firstNonEmpty(b.Images.LandscapeLarge, b.Images.DesktopWide, b.Images.MobileWide, b.Images.PosterWide)
	title := firstNonEmpty(b.Title, head.Title)
	if open == "" && len(b.Panel) > 0 {
		if card, ok := cardFromCMSObject(b.Panel, locale); ok {
			open = card.OpenURL
			if wide == "" {
				wide = card.WideURL
			}
			if title == "" {
				title = card.Title
			}
		}
	}
	if !isCatalogOpenURL(open) {
		return HomeBanner{}, false
	}
	return HomeBanner{
		ID:      firstNonEmpty(b.ID, head.ID),
		Title:   title,
		WideURL: wide,
		OpenURL: open,
	}, true
}

// isCatalogOpenURL reports whether url points at a series or watch title we can Inspect.
func isCatalogOpenURL(open string) bool {
	open = strings.TrimSpace(strings.ToLower(open))
	if open == "" {
		return false
	}
	// Absolute or path.
	if strings.Contains(open, "/series/") || strings.Contains(open, "/watch/") {
		return true
	}
	return false
}

// looksLikeTop10 uses title/id heuristics (locale-aware) for Top 10 chrome.
// Only explicit top-10 rank-list phrases match — bare "ranking" / "most popular"
// false-positive on series titles like "Ranking of Kings".
func looksLikeTop10(title, id string) bool {
	s := strings.ToLower(strings.TrimSpace(title + " " + id))
	if s == "" {
		return false
	}
	// Normalize separators so top-10 / top_10 become "top 10".
	compact := strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(s)
	compact = strings.Join(strings.Fields(compact), " ")
	needles := []string{
		"top 10",
		"top10",
		"top ten",
		"os 10 mais",
		"10 mais",
	}
	for _, n := range needles {
		if strings.Contains(s, n) || strings.Contains(compact, n) {
			return true
		}
	}
	return false
}

// applyTop10Ranks sets rankStyle and card ranks when the rail looks like a Top 10 list.
func applyTop10Ranks(block *HomeBlock) {
	if block == nil {
		return
	}
	if block.RankStyle == "" {
		block.RankStyle = RankStyleNone
	}
	if block.Kind != HomeBlockPosterRail && block.Kind != HomeBlockLandscapeRail {
		return
	}
	if !looksLikeTop10(block.Title, block.ID) {
		return
	}
	block.RankStyle = RankStyleTop10
	n := len(block.Cards)
	if n > 10 {
		n = 10
	}
	for i := 0; i < n; i++ {
		block.Cards[i].Rank = i + 1
	}
}

func blockHasContent(b HomeBlock) bool {
	switch b.Kind {
	case HomeBlockHero:
		return len(b.Heroes) > 0
	case HomeBlockBanner:
		return b.Banner != nil && b.Banner.OpenURL != ""
	case HomeBlockPosterRail, HomeBlockLandscapeRail:
		return len(b.Cards) > 0
	default:
		return false
	}
}

// deriveCompatFromBlocks fills Heroes/Rails from Blocks for older GUI consumers.
func deriveCompatFromBlocks(page *HomeFeedPage) {
	if page == nil {
		return
	}
	page.Heroes = nil
	page.Rails = nil
	for _, b := range page.Blocks {
		switch b.Kind {
		case HomeBlockHero:
			page.Heroes = append(page.Heroes, b.Heroes...)
		case HomeBlockPosterRail, HomeBlockLandscapeRail:
			if len(b.Cards) == 0 {
				continue
			}
			page.Rails = append(page.Rails, DiscoverRail{
				ID:    b.ID,
				Title: b.Title,
				Cards: b.Cards,
			})
		}
	}
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
		card.SeriesID = obj.ID
	case typ == "episode" || strings.Contains(typ, "episode"):
		card.OpenURL = watchOpenURL(obj.ID, obj.SlugTitle, locale)
		if obj.EpisodeMetadata != nil {
			card.SeriesID = strings.TrimSpace(obj.EpisodeMetadata.SeriesID)
			if obj.EpisodeMetadata.SeriesTitle != "" {
				// Keep series name as primary title for rail cards.
				card.Title = obj.EpisodeMetadata.SeriesTitle
			}
			card.Subtitle = episodeSubtitle(obj.EpisodeMetadata)
			if card.PosterURL == "" {
				card.PosterURL = thumbnailFromImages(obj.Images)
			}
			if card.WideURL == "" {
				card.WideURL = thumbnailFromImages(obj.Images)
			}
		}
	case strings.Contains(typ, "movie"):
		card.OpenURL = watchOpenURL(obj.ID, obj.SlugTitle, locale)
	default:
		// Prefer series-style URL; Inspect accepts series or watch.
		card.OpenURL = seriesOpenURL(obj.ID, obj.SlugTitle, locale)
	}
	return card, true
}

func episodeSubtitle(meta *cmsEpisodeMetadata) string {
	if meta == nil {
		return ""
	}
	ep := firstNonEmpty(rawJSONString(meta.Episode), rawJSONString(meta.EpisodeNumber))
	if meta.SeasonNumber > 0 && ep != "" {
		return fmt.Sprintf("S%02dE%s", meta.SeasonNumber, ep)
	}
	if ep != "" {
		return "E" + ep
	}
	if meta.SeasonNumber > 0 {
		return fmt.Sprintf("S%02d", meta.SeasonNumber)
	}
	return ""
}

func rawJSONString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return strings.TrimSpace(n.String())
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		if f == float64(int64(f)) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return ""
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

// progressPtr returns a heap-backed *float64 clamped to [0,1].
func progressPtr(v float64) *float64 {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p := v
	return &p
}
