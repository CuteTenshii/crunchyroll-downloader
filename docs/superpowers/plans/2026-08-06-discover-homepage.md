# Discover homepage implementation plan

> **For agentic workers:** implement task-by-task; verify with `go test` + tagged GUI build.

**Goal:** In-app Home feed + search that lists Crunchyroll Discover content and opens the existing download flow.

**Architecture:** Go engine clients for home_feed/search/browse + cms/objects; Wails binds; Home view in frontend with rails/cards.

**Tech:** Same Wails stack; Bearer auth already in engine.

---

### Task 1: Engine discover API

**Files:** Create `internal/engine/discover.go`, `discover_test.go`; maybe `account.go` for `/accounts/v1/me`.

- Types: `DiscoverCard`, `DiscoverRail`, `HomeFeedPage`, `SearchResult`
- `GetAccountID() (string, error)`
- `FetchHomeFeed(start, n int, locale string) (HomeFeedPage, error)`
- `SearchDiscover(q string, start, n int, locale string) ([]DiscoverCard, error)`
- `ResolveObjectCards(ids []string, locale string) ([]DiscoverCard, error)`
- Parse feed items; resolve curated ids in batches
- Unit tests with fixture JSON (no live network)

### Task 2: App bindings

**Files:** `cmd/gui/app.go`

- `GetHomeFeed(start, n int) (HomeFeedPage, error)` — auth from cookie, locale from prefs (default pt-BR)
- `SearchTitles(q string) ([]DiscoverCard, error)`
- Prefs: add `Locale string` if missing (default pt-BR)

### Task 3: Home UI

**Files:** frontend HTML/CSS/JS

- Nav: Home | Download
- Home: search input, hero, rails, load more
- Download: existing main UI
- Card click → switch to Download, set URL, Inspect

### Task 4: Polish + docs

- README note on Home
- Build verify
- Commit
