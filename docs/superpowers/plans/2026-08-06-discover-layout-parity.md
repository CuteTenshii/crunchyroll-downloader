# Discover layout parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make GUI Home match approved CR-like Discover layout (feed-ordered blocks, CW, watchlist, recs, banners, Top 10 chrome, cookie + multiprofile accounts).

**Architecture:** Feed-driven `HomeBlock` list from `home_feed` + hydrators; Wails bindings; frontend showcase layout; prefs for cookie profiles.

**Tech Stack:** Go engine, Wails v2, existing HTML/CSS/JS GUI.

---

### Task 1: Engine HomeBlock model + mapper + hydrators

**Files:**
- Modify: `internal/engine/discover.go`
- Modify: `internal/engine/discover_test.go`
- Maybe create: `internal/engine/discover_hydrate.go`

- [ ] Extend types: `HomeBlock`, card `Progress`/`Rank`/`Subtitle`, `HomeFeedPage.Blocks`
- [ ] Map all approved resource types; hydrate dynamic_collection
- [ ] Top 10 rankStyle heuristic
- [ ] Unit tests with fixtures
- [ ] `go test ./internal/engine/ -count=1`

### Task 2: Accounts prefs + App bindings

**Files:**
- Modify: `internal/engine/prefs.go`
- Modify: `cmd/gui/app.go`

- [ ] CookieProfiles + ActiveProfileId in Preferences
- [ ] ListProfiles / SwitchCookieProfile / ListCRProfiles / SwitchCRProfile (best-effort multiprofile)
- [ ] GetHomeFeed returns new block shape

### Task 3: Frontend Home layout + Top 10 + accounts UI

**Files:**
- Modify: `cmd/gui/frontend/index.html`, `css/app.css`, `js/app.js`

- [ ] Render blocks like showcase
- [ ] Account menu
- [ ] Top 10 rank chrome CSS

### Task 4: Build, test, commit, README

- [ ] `go test` + GUI build
- [ ] README note
- [ ] Commit
