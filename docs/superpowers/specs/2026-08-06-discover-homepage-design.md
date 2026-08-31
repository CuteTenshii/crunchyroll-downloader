# Discover homepage design

**Date:** 2026-08-06  
**Status:** Approved to implement (user: “Proceed!” after API research)  
**Stack:** Existing Wails GUI (HTML/CSS/JS + Go engine)

## Goal

Add an in-app **Discover / Home** experience close to  
`https://www.crunchyroll.com/pt-br/discover`, so the user can browse series/movies, open a title, and download without opening a full browser.

## Product defaults (locked)

| Choice | Default |
|--------|---------|
| Feed | **Personalized** `home_feed` when cookie/token works; on failure show error + retry / fall back to **browse** popular |
| Locale | Prefs field, default **`pt-BR`** (match user’s Discover URL); can change later in settings |
| v1 scope | **Home rails + search** (no full category tree, no music/games/news) |
| Click title | Switch to **Download** view with URL filled + auto-**Inspect** |

## Architecture

```
[Home UI] --Wails--> App.GetHomeFeed / Search / ResolveObjects
                --> engine (Bearer token, same etp_rt auth)
[Card click] --> set URL --> Inspect --> existing download UI
```

- **No new browser process.** Same WebView.
- **No Widevine** for home/search/browse (catalog only).
- Pagination: load first page of feed; “Load more” for additional `start` offsets.
- Bulk resolve IDs via `GET /content/v2/cms/objects/{ids}` (batched, e.g. ≤20–50 ids).

## API map (v1)

| Action | Endpoint |
|--------|----------|
| Account id | `GET /accounts/v1/me` (or parse from token if already available) |
| Home | `GET /content/v2/discover/{accountId}/home_feed?n=&start=&locale=` |
| Search | `GET /content/v2/discover/search?q=&n=&start=&locale=` |
| Browse fallback | `GET /content/v2/discover/browse?sort_by=popularity&type=series&n=&start=&locale=` |
| Metadata | `GET /content/v2/cms/objects/{commaIds}?locale=` |

### Home feed item shapes (subset)

Parse `resource_type` (+ `response_type` where needed):

- `hero_carousel` → hero slides (`items[]` with title, link, images, optional panel)
- `curated_collection` + `response_type=series` → rail title + `ids[]`
- `panel` → single featured series
- `dynamic_collection` with browse/recommendations → optional v1: skip or map to secondary rails if easy
- Unknown types → skip silently (log in activity)

### Card model (UI)

```text
DiscoverCard {
  id, type (series|movie|episode|unknown),
  title, description?,
  posterUrl,  // poster_tall preferred
  wideUrl?,   // poster_wide / landscape for hero
  seriesUrl,  // absolute or path we can hand to Inspect
}
```

URL construction:

- series → `https://www.crunchyroll.com/series/{id}`
- movie/episode watch → `https://www.crunchyroll.com/watch/{id}`
- Prefer locale-prefixed path only if we already store locale; absolute `/series/` works with existing parser.

## UI layout (v1)

### Navigation

Top chrome gains a mode or tab: **Home | Download** (or logo opens Home).

- **Home:** Discover  
- **Download:** existing inspect/download UI (current main content)

### Home screen

1. Optional **search** bar under chrome (or in Home body).  
2. **Hero** carousel (first 1–5 `hero_carousel` items) — large wide art, title, “Open” / click.  
3. **Rails:** horizontal scroll rows — section title + poster cards.  
4. **Load more** for feed pages if needed.

### Interaction

- Click card → navigate to Download view, set URL, run Inspect, keep cookie.  
- Long description optional on hover/detail later (YAGNI for v1).

### Visual language

Same dark cinema + Lato + solid orange accents. Cards: tall poster, title under. Rails: horizontal scroll with same dark scrollbar language as workspace.

## Risks & mitigations

| Risk | Mitigation |
|------|------------|
| API shape drift | Tolerant JSON parse; skip unknown rows |
| Rate limits | Paginate modestly; cache last feed in memory for session |
| Missing account id | Call `/accounts/v1/me`; surface clear auth error |
| Movies vs series | Use object `type` from CMS when resolving objects |

## Non-goals (v1)

- Full category browser parity  
- Music, games, news  
- Inline video playback  
- Offline catalog  
- Multi-profile picker  

## Success criteria

1. With valid cookie, Home shows hero + at least one rail of series posters.  
2. Search returns series/movies as cards.  
3. Click opens Download + successful Inspect path for a known series.  
4. No playback/license calls on Home load.
