# Discover layout parity design

**Date:** 2026-08-06  
**Status:** Approved (user: showcase liked; Top 10 chrome added; implement)  
**Stack:** Existing Wails GUI + `internal/engine`  
**Supersedes layout of:** `2026-08-06-discover-homepage-design.md` (functional v1). Auth, no-Widevine-on-Home, click→Inspect remain.

## Goal

Home should **look and organize like Crunchyroll web Discover**: feed-ordered sections, full-bleed hero, landscape Continue Watching with progress, tall poster rails, in-feed banners, Top 10 rank chrome, search, and **account switching** (cookie profiles + CR multiprofile when available). Click still opens **Download + Inspect**. No in-app video, no Widevine on Home.

## Product decisions (locked)

| Decision | Choice |
|----------|--------|
| Parity level | **B** downloader-focused, but **most** CR Home blocks |
| Architecture | **A** feed-driven: walk `home_feed` order; map blocks → UI |
| Sections in | Hero, Continue Watching, Watchlist, Recommendations / Because you watched, Curated rails, Browse entry, Search, **In-feed banners**, **Top 10 rank chrome** |
| Sections out | Music, Games, News, Store full chrome |
| Accounts | **C** cookie profiles **and** CR multiprofile (when API exposes profiles) |
| Locale | Prefs `locale`, default `pt-BR` |
| Click card | Switch to Download, set URL, auto-Inspect |

## Architecture

```
[Account menu] cookie profile / multiprofile
       ↓
Authenticate (etp_rt file → Bearer + accountId)
       ↓
GET home_feed?start&n&locale  (preserve block order)
       ↓
BlockMapper(resource_type, response_type) → HomeBlock
       ↓
Hydrators: history/playheads, watchlist, recommendations,
           similar_to, browse link, cms/objects
       ↓
HomePage { Blocks []HomeBlock, NextStart }
       ↓
Frontend renders by block.kind (hero | landscape_rail | poster_rail | banner)
       ↓
Click → Download + Inspect
```

### Ordered block model (replaces flat Heroes+Rails-only as primary)

```text
HomeBlock {
  id, kind, title,
  rankStyle: none | top10,   // Top 10 chrome when rail is ranked
  cards: []DiscoverCard,     // for rails
  heroes: []DiscoverHero,    // for hero kind
  banner: optional wide promo
}

DiscoverCard additions:
  progress: 0..1 optional     // Continue Watching
  rank: int optional          // Top 10 index 1..10
  subtitle: string optional   // e.g. S01E12
}
```

### Feed mapping

| resource / response | kind | notes |
|---------------------|------|--------|
| `hero_carousel` | `hero` | Full-bleed slides |
| `dynamic_collection` + `history` | `landscape_rail` | Hydrate history + playhead progress |
| `dynamic_collection` + `watchlist` | `poster_rail` | Watchlist API |
| `dynamic_collection` + `recommendations` | `poster_rail` | Recs API |
| `dynamic_collection` + `because_you_watched` | `poster_rail` | similar_to / source id |
| `dynamic_collection` + `browse` / `recent_episodes` | poster or landscape | Follow `link` query |
| `curated_collection` + `series` | `poster_rail` | Resolve ids via cms/objects |
| `panel` | single featured card rail or wide feature | series/movie only |
| `in_feed_banner` | `banner` | Only if open URL is series/watch |
| music / game / news / unknown | skip | activity log at debug |

**Top 10 chrome:** If rail title/id suggests Top 10 (locale-aware heuristics: "top 10", "top10", "mais populares" ranked lists when cards are ordered) **or** feed provides rank metadata, set `rankStyle=top10` and assign `rank=1..n` on cards (cap 10). Pure presentation; same open URLs.

### Accounts

1. **Cookie profiles** (prefs): list of `{ id, name, cookieFile }`; `activeProfileId`. Switch → re-auth → clear Home cache → reload. Never store raw cookie values.
2. **CR multiprofile:** After auth, if profiles API returns multiple profiles, show sub-picker; activate selected profile token/context when CR requires it; reload Home. If only one profile or API unavailable, hide multiprofile UI gracefully.

### Non-goals

- Full site nav (Manga, Games, News, Store)
- Inline streaming
- Offline catalog
- Pixel-perfect CR CSS clone (match structure, density, hierarchy)

## UI (approved showcase)

Reference mock: `.superpowers/brainstorm/*/content/home-showcase-v1.html` (+ Top 10 badges).

- Sticky header: brand, Home|Download, search, **account control**
- Full-bleed hero (~300px+), CTA, dots
- Landscape CW cards + orange progress bar
- Dense tall posters (~128×184), horizontal scroll rails
- Wide in-feed banners
- Top 10: large rank digit overlay/beside poster
- Dark cinema + Lato + solid orange (existing theme)

## Error handling

- Missing cookie: empty state + Cookie CTA
- home_feed fail: fall back browse popular as single poster rail
- Individual hydrator fail: drop that block, keep others
- Auth fail on profile switch: banner + keep previous account if safe

## Testing

- Unit: block mapper fixtures (hero, curated, dynamic history skip/hydrate stubs, banner filter, top10 rank assignment)
- Unit: locale normalize, open URL builders
- GUI: manual Home load with cookie; switch profile path; click → Inspect

## Success criteria

1. Home section order follows `home_feed` (after skips).
2. Hero full-bleed; CW landscape + progress when history hydrates.
3. Watchlist / recs / curated / browse / banners appear when feed provides them.
4. Top 10 rails show rank chrome.
5. Cookie profile switch reloads Home; multiprofile when available.
6. Card click → Download + Inspect; no Widevine on Home load.
