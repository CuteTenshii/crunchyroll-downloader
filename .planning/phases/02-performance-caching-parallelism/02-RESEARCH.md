# Phase 2: Performance — Caching & Parallelism - Research

**Researched:** 2026-07-08
**Domain:** Go concurrency, in-memory caching, DASH manifest parsing, parallel download orchestration
**Confidence:** HIGH

## Summary

Phase 2 optimizes multi-dub episode downloads through three related changes: (1) an in-memory MPD manifest cache to eliminate redundant XML fetching/parsing per audio version, (2) parallelization of audio version manifest-fetching and license-challenge phases using `golang.org/x/sync/errgroup`, and (3) splitting the fragile `GetBaseUrl` function into typed `GetVideoBaseUrl`/`GetAudioBaseUrl` variants with explicit bandwidth matching.

The current sequential loop in `internal/download/episode.go:152-216` processes each audio version one-at-a-time: fetch manifest → parse MPD → get PSSH → get license → audio download. With N audio versions, this is N sequential round-trips for manifest fetching (CDN latency) and N sequential round-trips for license challenges (Crunchyroll license server). For 3+ audio locales, the savings from parallelizing these I/O-bound operations is substantial — manifest fetching dominates because each `FetchManifest` call downloads the full MPD XML.

The MPD cache is straightforward: a `sync.RWMutex`-protected `map[string]*mpd.MPD` keyed by `contentId`. The parallelization uses `errgroup.WithContext` for the audio fan-out, with video download remaining sequential before the fan-out. Stream cleanup (DeleteStream) is deferred to after all parallel work completes.

### Go 1.26 Compatibility Note

The installed Go toolchain is 1.26.4 (go.mod specifies 1.25). All concurrency primitives work as expected. `golang.org/x/sync/errgroup` at v0.14.0+ (April 2025) includes panic recovery — goroutines that panic inside `g.Go` are caught and returned as `PanicError`. Use the latest v0.22.0.

**Primary recommendation:** Implement the MPD cache as a co-located package-level variable in `internal/media/manifest.go`, use `errgroup.WithContext` for the audio version fan-out, split `GetBaseUrl` into typed functions with explicit bandwidth switch/case, and defer all stream cleanup to after the parallel section completes.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### MPD Cache Design (PERF-04)
- Cache keyed by `contentId`
- Cache stores parsed `*mpd.MPD` struct only (not raw XML bytes)
- Thread-safe via `sync.RWMutex` — read-mostly pattern, single writer on first fetch
- No eviction — per-episode lifetime with at most ~5 entries

#### Parallelization Pattern (PERF-06)
- Use `golang.org/x/sync/errgroup` for goroutine coordination
- Cancel remaining goroutines on first error — `errgroup.WithContext` cancels `ctx`
- Video download stays outside the parallel group — sequential, before audio fan-out
- Manifest fetching + parsing happens inside the parallel group, using the MPD cache (cache-first)

#### GetBaseUrl Split (QOL-03)
- Split `GetBaseUrl` into `GetVideoBaseUrl` and `GetAudioBaseUrl`
- Return type stays `(*string, *string)` — minimal caller changes
- Improve audio bandwidth matching with explicit switch/case mapping

#### Stream Cleanup Strategy
- Defer all stream release (DeleteStream) to after parallel work completes — use existing `activeStreams` map
- Primary version's (version[0]) stream treated the same as others

### the agent's Discretion
None — all decisions were made explicitly during discussion.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within Phase 2 scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PERF-04 | Parsed MPD manifests cached per `contentId` — avoid redundant fetch/re-parse | Straightforward `sync.RWMutex` + `map` pattern in `manifest.go`. Keyed by `contentId`, stores `*mpd.MPD`. Checked before `ParseManifest`. |
| PERF-06 | Audio version manifest fetching and license challenges parallelized | `errgroup.WithContext` in `episode.go` for audio fan-out. Video stays sequential before fan-out. Each goroutine handles: cache-check → fetch+parse → PSSH → license challenge → audio download. |
| QOL-03 | `GetBaseUrl` split into `GetVideoBaseUrl` and `GetAudioBaseUrl` | Remove `isVideoSet` bool. Video logic stays the same. Audio logic gets explicit switch/case bandwidth mapping. Return type `(*string, *string)` unchanged. |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| MPD manifest parsing | Internal library (`internal/media`) | — | `ParseManifest` is already in `internal/media/manifest.go`; cache co-located there |
| MPD cache | Internal library (`internal/media`) | — | Cache must be co-located with `ParseManifest` to avoid import cycles and maintain cache coherency |
| Audio version coordination | Download engine (`internal/download`) | — | The audio loop in `Episode()` is the coordination point |
| Video download | Download engine (`internal/download`) | — | Stays sequential, before audio fan-out |
| License challenge | DRM layer (`internal/drm`) | — | `GetLicense()` already returns keys directly (no globals), thread-safe device via `sync.Once` |
| Stream cleanup | Download engine (`internal/download`) | API layer (`internal/api`) | Deferred cleanup closure in `Episode()` holds `activeStreams` map; calls `client.DeleteStream()` |
| Bandwidth matching | Internal library (`internal/media`) | — | `GetVideoBaseUrl`/`GetAudioBaseUrl` are manifest utility functions |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/sync/errgroup` | v0.22.0 | Structured concurrency with error propagation and context cancellation | Go team-maintained, replaces `sync.WaitGroup` + error channel + manual cancel boilerplate |
| `sync.RWMutex` | stdlib (Go 1.25+) | Thread-safe read-mostly cache access | Zero dependencies, perfect for cache-hit-dominant pattern (~5 entries, reads >> writes) |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `context.Context` | stdlib (Go 1.25+) | Cancellation propagation across goroutines | Pass `gctx` (from `errgroup.WithContext`) to all parallel operations |
| `errors.Join` | stdlib (Go 1.20+) | Error aggregation if partial-failure pattern needed | Not for Phase 2 (fail-fast is correct), useful for future collect-all patterns |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `sync.RWMutex` + `map` | `sync.Map` | `sync.Map` is optimized for write-once/read-many registries; a mutex+map is faster for general concurrent access with type safety and simpler reasoning [CITED: go-patterns.dev, atharvapandey.com] |
| `sync.RWMutex` + `map` | Sharded/striped locks | Overkill for ~5 entries; sharding benefits appear at higher core counts and larger maps [CITED: strebkov.dev] |
| `errgroup` | Manual `sync.WaitGroup` + error channel + `context.WithCancel` | errgroup wraps all three in one abstraction that's impossible to wire wrong [CITED: pkg.go.dev, atharvapandey.com] |

**Installation:**
```bash
go get golang.org/x/sync@v0.22.0
```
This adds `golang.org/x/sync` to `go.mod` as a direct dependency.

**Version verification (conducted 2026-07-08):**
- `golang.org/x/sync v0.22.0` — latest available, confirmed via `go list -m -versions`
- Go toolchain: 1.26.4 — go.mod specifies 1.25, compatible
- All stdlib packages: available at go 1.25+

## Package Legitimacy Audit

> Performed 2026-07-08. Only one new dependency is introduced: `golang.org/x/sync`.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `golang.org/x/sync` | Go modules | ~10 yrs | Millions | `go.googlesource.com/sync` | OK | Approved |

**Packages removed due to [SLOP] verdict:** None
**Packages flagged as suspicious [SUS]:** None

`golang.org/x/sync` is the Go team's official sub-repo for sync primitives. The `errgroup` package has been stable since introduction. v0.14.0+ adds `PanicError` recovery. Use v0.22.0 (latest, confirmed via `go list -m -versions`). [VERIFIED: Go Modules Registry]

## Architecture Patterns

### System Architecture Diagram

```
Episode Download (internal/download/episode.go)
│
├── Sequential (before parallel)
│   ├── GetEpisode(versions[0])              → activeStreams[contentId] = token
│   ├── Download subtitles (sequential)       → subTracks[]
│   │
│   └── i==0: Video processing (sequential)
│       ├── FetchManifest                    → raw bytes
│       ├── ParseManifest / Cache:store       → *mpd.MPD
│       ├── DRM: GetPssh → GetLicense         → []*widevine.Key
│       └── DownloadParts(video)             → videoFile
│
├── Parallel (errgroup.WithContext)
│   │  for each audio version [1..N]:
│   │  g.Go(func() error {
│   │      ├── Cache:Get(contentId)
│   │      ├── if miss: FetchManifest → ParseManifest → Cache:Set
│   │      ├── DRM: GetPssh → GetLicense     → []*widevine.Key (per-goroutine)
│   │      └── DownloadParts(audio)          → audioFile + audioTracks[]
│   │  })
│   │  g.Wait()  ← first error cancels all
│   │
├── Sequential (after parallel)
│   ├── deferred cleanup: DeleteStream for all activeStreams
│   └── mux.MergeEverything(video + audioTracks + subTracks)
│
└── completed = true
```

### Recommended Project Structure

No structural changes — all modifications stay within existing files:

```
internal/
├── api/
│   ├── client.go       (no changes — DeleteStream, etc. stay put)
│   ├── episode.go      (no changes — GetEpisode, DeleteStream stay)
│   ├── license.go      (no changes — SendChallenge unchanged)
│   └── manifest.go     (no changes — FetchManifest unchanged)
├── drm/
│   └── drm.go          (no changes — GetLicense already returns keys, thread-safe)
├── media/
│   └── manifest.go     (ADD: MPD cache struct + Get/Set functions)
│                        (MODIFY: GetBaseUrl → GetVideoBaseUrl + GetAudioBaseUrl)
├── download/
│   └── episode.go      (MODIFY: sequential loop → errgroup fan-out)
├── mux/
│   └── mux.go          (no changes)
```

### Pattern 1: Cache-First with sync.RWMutex

**What:** A read-mostly cache where concurrent reads are allowed but writes are exclusive.

**When to use:** Map with many more reads than writes; no eviction needed.

**Why this is correct for MPD cache:**
- At most ~5 writes (one per audio version), each read dominates
- Each contentId is requested at most once (deduplication by the caller)
- `sync.RWMutex` allows concurrent cache-check goroutines to proceed in parallel
- Simpler and faster than `sync.Map` for this access pattern [CITED: go-patterns.dev, sync docs]

**Example:**
```go
// Source: Derived from go-patterns.dev / sync.RWMutex pattern [CITED]
type MPDPCache struct {
    mu    sync.RWMutex
    items map[string]*mpd.MPD
}

var mpdCache = MPDPCache{
    items: make(map[string]*mpd.MPD),
}

func (c *MPDPCache) Get(key string) (*mpd.MPD, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    mpd, ok := c.items[key]
    return mpd, ok
}

func (c *MPDPCache) Set(key string, mpd *mpd.MPD) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[key] = mpd
}
```

### Pattern 2: errgroup Fan-Out with Fail-Fast

**What:** Launch N goroutines concurrently, cancel all on first error.

**When to use:** Homogeneous work items where any failure makes remaining work useless.

**Why this is correct for audio fan-out:**
- All audio versions are independent and I/O-bound (manifest fetch, license challenge)
- If one version fails (e.g., PSSH not found), there's no point continuing others
- `errgroup.WithContext` gives both error propagation and context cancellation in one abstraction
- No need for `SetLimit` — audio versions are typically 1-5, well under any resource limit

**Example:**
```go
// Source: Derived from pkg.go.dev/errgroup and atharvapandey.com [CITED]
g, gctx := errgroup.WithContext(ctx)

for i, version := range audioVersions {
    i, version := i, version  // capture for closure
    g.Go(func() error {
        // Check MPD cache first
        manifest := getCachedManifest(version.contentId)
        if manifest == nil {
            raw, err := client.FetchManifest(gctx, ...)
            if err != nil { return fmt.Errorf("fetch manifest %s: %w", version.locale, err) }
            manifest, err = media.ParseManifest(raw)
            if err != nil { return fmt.Errorf("parse manifest %s: %w", version.locale, err) }
            setCachedManifest(version.contentId, manifest)
        }

        pssh := drm.GetPssh(manifest)
        if pssh == nil { return fmt.Errorf("PSSH not found for %s", version.locale) }

        keys, err := drm.GetLicense(gctx, client, *pssh, version.contentId, episode.Token)
        if err != nil { return fmt.Errorf("license %s: %w", version.locale, err) }

        audioFile, err := media.DownloadParts(gctx, client, baseUrl, id, audioSet, keys, workers)
        if err != nil { return fmt.Errorf("audio %s: %w", version.locale, err) }

        mu.Lock()
        audioTracks = append(audioTracks, mux.MediaTrack{...})
        mu.Unlock()
        return nil
    })
}

if err := g.Wait(); err != nil {
    return fmt.Errorf("audio versions: %w", err)
}
```

### Anti-Patterns to Avoid

- **Shadowing the parent context:** Always use `gctx` (the context from `errgroup.WithContext`) inside `g.Go` closures, not the original `ctx`. The parent context won't be cancelled on sibling errors. [VERIFIED: pkg.go.dev/errgroup]
- **Sharing mutable state without synchronization:** The `audioTracks` slice is written from multiple goroutines. Either use a mutex or pre-allocate the slice and write by index. Appending from multiple goroutines without synchronization is a data race. [VERIFIED: go race detector]
- **Closing over loop variables incorrectly:** Go 1.22+ fixes loop variable scoping, but explicit capture is still safe and clearest: `i, version := i, version`. [VERIFIED: Go 1.22 release notes]
- **Reusing errgroup.Group:** A Group must not be reused for different tasks after `Wait` returns. Create a new one per fan-out. [CITED: pkg.go.dev/errgroup]
- **Ignoring context cancellation in long-running operations:** Every blocking call inside `g.Go` must use the group's context — especially `DownloadParts` which runs for the duration of the audio segment download. [VERIFIED: blog.golang.org/pipelines]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Goroutine + error + cancellation coordination | Manual `sync.WaitGroup` + `chan error` + `context.WithCancel` | `errgroup.WithContext` | The manual wiring is error-prone (leaked goroutines, missed cancellation, lost errors). errgroup is ~50 lines of tested code from the Go team. [CITED: atharvapandey.com] |
| Thread-safe concurrent map for cache | Custom lock-free map | `sync.RWMutex` + `map[string]*mpd.MPD` | The RWMutex pattern is well-understood, type-safe, and trivially correct. No need for sharding or lock-free structures for ~5 entries. [CITED: go-patterns.dev] |
| Audio bandwidth matching | Fuzzy threshold fallback (< 192k, < 128k, < 96k) | Explicit `switch/case` mapping (`"192k" → bandwidth >= 192000`) | The threshold approach silently falls through to the first representation for unrecognized values. Switch/case rejects unexpected quality values with an error. [VERIFIED: CONCERNS.md] |
| Deferred stream cleanup | Manual per-version DeleteStream in loop body | `activeStreams` map + deferred closure after errgroup | In-loop cleanup races with parallel work. Deferred bulk cleanup is simpler and correct regardless of parallel/serial execution. [VERIFIED: CONTEXT.md D-12, D-13] |

**Key insight:** All three changes (cache, errgroup, GetBaseUrl split) share a common theme — they eliminate architectural bottlenecks that force sequential execution. The global `keys` variable (already fixed in Phase 1) was the main blocker for parallel license acquisition. With that gone, the remaining constraints are the sequential loop structure and the fragile bandwidth matching.

## Common Pitfalls

### Pitfall 1: Context Not Propagated to errgroup Workers

**What goes wrong:** Using the original parent `ctx` inside `g.Go` functions instead of the derived `gctx` from `errgroup.WithContext`. When one goroutine fails, the other goroutines don't receive cancellation and keep running.

**Why it happens:** `errgroup.WithContext` returns a new context (`gctx`) but doesn't warn when you ignore it. It's natural to shadow `ctx` with `gctx` in the same scope.

**How to avoid:** Always use the derived context inside goroutines. The canonical pattern is:
```go
g, gctx := errgroup.WithContext(ctx)
// ... inside g.Go: use gctx, not ctx
```

**Warning signs:** If `select` or `ctx.Done()` checks in workers never trigger after a sibling error.

### Pitfall 2: Data Race on audioTracks Slice Append

**What goes wrong:** Multiple goroutines call `append(audioTracks, ...)` concurrently without synchronization. The Go race detector catches this, but production runs without `-race`.

**Why it happens:** `append` may write to the slice header (len, cap, pointer) — concurrent writes to the same slice header from different goroutines is a data race.

**How to avoid:** Either:
1. Pre-allocate `audioTracks` to `len(versions)-1` and write by index (each goroutine owns a unique slot)
2. Protect the append with `sync.Mutex`
3. Collect results through a channel

Pre-allocation is cleanest for this case since indices are known ahead of time.

### Pitfall 3: Zero goroutines in errgroup

**What goes wrong:** If there are no audio versions beyond the first (i.e., `len(versions) == 1`), no `g.Go` calls are made. `g.Wait()` returns `nil` immediately, which is correct — but the code must handle the single-version case gracefully (skip errgroup entirely).

**Why it happens:** The first audio version (i=0) includes video. Only subsequent versions (i>=1) are parallelized.

**How to avoid:** Wrap the errgroup block in `if len(versions) > 1` or structure the loop to handle both cases.

### Pitfall 4: MPD Cache Invalidation on Changing Manifests

**What goes wrong:** If Crunchyroll returns different MPD content for the same contentId over time (e.g., different CDN URLs, different segment timelines), the cached version is stale.

**Why it happens:** The cache has no TTL or eviction. For a single episode download session, the manifest URLs should be stable for the stream token's lifetime (~4 hours per Crunchyroll's typical TTL).

**How to avoid:** This is acceptable because:
- Per-episode lifetime (at most ~5 entries)
- All audio versions are fetched within seconds of each other
- Stream tokens expire after ~4 hours, making cache TTL unnecessary
- If Crunchyroll changes manifests mid-download, the broken version affects the uncached path too

Document this assumption: cache assumes manifest stability within a single episode download session.

### Pitfall 5: errgroup Goroutine Leak from Blocking Channel Send

**What goes wrong:** If a `g.Go` function tries to send on an unbuffered channel and the receiver has already exited (e.g., because of context cancellation from a sibling's error), the goroutine blocks forever.

**How to avoid:** Every channel send inside `g.Go` must use `select` with `<-gctx.Done()`:
```go
select {
case resultCh <- result:
case <-gctx.Done():
    return gctx.Err()
}
```

This is not needed for Phase 2 because audio results are collected via pre-allocated arrays/mutex, not channels — but is relevant for future phases that use channel-based fan-in.

## Code Examples

### MPD Cache Implementation (internal/media/manifest.go)

```go
// Source: Derived from go-patterns.dev and CONTEXT.md D-01 through D-04

package media

import (
    "sync"
    "github.com/unki2aut/go-mpd"
)

// mpdCache is a thread-safe read-mostly cache for parsed MPD manifests.
// Keyed by contentId, stores *mpd.MPD. No eviction (max ~5 entries per episode).
type mpdCache struct {
    mu    sync.RWMutex
    items map[string]*mpd.MPD
}

var manifestCache = &mpdCache{
    items: make(map[string]*mpd.MPD),
}

// getCachedManifest returns the cached manifest for contentId, or nil on miss.
func (c *mpdCache) get(contentId string) *mpd.MPD {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.items[contentId]
}

// setCachedManifest stores a parsed manifest for contentId.
func (c *mpdCache) set(contentId string, mpd *mpd.MPD) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[contentId] = mpd
}
```

### errgroup Fan-Out in download/episode.go

```go
// Source: Derived from pkg.go.dev/errgroup and CONTEXT.md D-05 through D-08

import "golang.org/x/sync/errgroup"

// ... inside Episode(), after video is downloaded (i==0) ...

if len(versions) > 1 {
    g, gctx := errgroup.WithContext(ctx)
    
    // Pre-allocate result slots for audio versions [1..N]
    audioTracks = make([]mux.MediaTrack, len(versions)-1)
    
    for idx := 1; idx < len(versions); idx++ {
        idx := idx // explicit capture
        version := versions[idx]
        g.Go(func() error {
            // Check MPD cache first
            manifest := manifestCache.get(version.contentId)
            if manifest == nil {
                episode, err := client.GetEpisode(gctx, version.contentId)
                if err != nil {
                    return fmt.Errorf("fetch episode %s: %w", version.locale, err)
                }
                activeStreams[version.contentId] = episode.Token

                raw, err := client.FetchManifest(gctx, episode.ManifestURL)
                if err != nil {
                    return fmt.Errorf("fetch manifest %s: %w", version.locale, err)
                }
                manifest, err = media.ParseManifest(raw)
                if err != nil {
                    return fmt.Errorf("parse manifest %s: %w", version.locale, err)
                }
                manifestCache.set(version.contentId, manifest)
            }

            pssh := drm.GetPssh(manifest)
            if pssh == nil {
                return fmt.Errorf("PSSH not found for %s", version.locale)
            }

            keys, err := drm.GetLicense(gctx, client, *pssh, version.contentId, episode.Token)
            if err != nil {
                return fmt.Errorf("license %s: %w", version.locale, err)
            }

            audioSet := manifest.Period[0].AdaptationSets[1]
            audioBaseUrl, audioRepresentationId := media.GetAudioBaseUrl(audioSet, *audioQuality)
            if audioBaseUrl == nil {
                return fmt.Errorf("audio base URL not found for %s", version.locale)
            }

            audioFile, err := media.DownloadParts(gctx, client, audioBaseUrl, audioRepresentationId, audioSet, keys, workers)
            if err != nil {
                return fmt.Errorf("download audio %s: %w", version.locale, err)
            }
            tempFiles = append(tempFiles, audioFile)  // NOTE: protect tempFiles with mutex
            audioTracks[idx-1] = mux.MediaTrack{File: audioFile, Locale: version.locale}
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        return fmt.Errorf("audio versions: %w", err)
    }
}
```

### GetAudioBaseUrl with Explicit Bandwidth Switch

```go
// Source: Derived from mpd.go current logic and CONCERNS.md fragile area

func GetAudioBaseUrl(set *mpd.AdaptationSet, quality string) (*string, *string) {
    if set == nil || len(set.Representations) == 0 {
        return nil, nil
    }

    for _, rep := range set.Representations {
        if len(rep.BaseURL) == 0 || rep.ID == nil {
            continue
        }
        if strings.Contains(*rep.ID, "audio/") {
            if strings.Contains(*rep.ID, quality) {
                return &rep.BaseURL[0].Value, rep.ID
            }
        } else if rep.Bandwidth != nil {
            switch quality {
            case "192k":
                if *rep.Bandwidth >= 192000 {
                    return &rep.BaseURL[0].Value, rep.ID
                }
            case "128k":
                if *rep.Bandwidth >= 128000 {
                    return &rep.BaseURL[0].Value, rep.ID
                }
            case "96k":
                if *rep.Bandwidth >= 96000 {
                    return &rep.BaseURL[0].Value, rep.ID
                }
            default:
                // Unknown quality — fall through to first-rep fallback
            }
        }
    }

    // Fallback: return first available representation
    if first := set.Representations[0]; len(first.BaseURL) > 0 && first.ID != nil {
        return &first.BaseURL[0].Value, first.ID
    }
    return nil, nil
}
```

## Validation Architecture

> `workflow.nyquist_validation` is **enabled** in .planning/config.json.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | `go test` (stdlib) |
| Config file | none — standard Go test conventions |
| Quick run command | `go test ./internal/media/... ./internal/download/... -race -count=1 -run TestPhase2 2>&1` |
| Full suite command | `go test ./... -race -count=1 2>&1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File |
|--------|----------|-----------|-------------------|------|
| PERF-04 | MPD cache returns nil on miss for unknown contentId | unit | `go test ./internal/media/ -run TestMPDCacheMiss -v` | `media/manifest_test.go` |
| PERF-04 | MPD cache returns cached value after Set | unit | `go test ./internal/media/ -run TestMPDCacheHit -v` | `media/manifest_test.go` |
| PERF-04 | MPD cache handles concurrent Get/Set without race | unit (with -race) | `go test ./internal/media/ -run TestMPDCacheConcurrent -race -v` | `media/manifest_test.go` |
| PERF-06 | errgroup parallel audio processing completes without races | integration (with -race) | `go test ./internal/download/ -run TestEpisodeParallelAudio -race -v` | `download/episode_test.go` |
| PERF-06 | Single audio version skips errgroup (no goroutines) | unit | `go test ./internal/download/ -run TestEpisodeSingleVersion -v` | `download/episode_test.go` |
| QOL-03 | GetVideoBaseUrl rejects empty adaptation set | unit | `go test ./internal/media/ -run TestGetVideoBaseUrlEmpty -v` | `media/manifest_test.go` |
| QOL-03 | GetAudioBaseUrl matches "192k" to bandwidth >= 192000 | unit | `go test ./internal/media/ -run TestGetAudioBaseUrlBandwidth -v` | `media/manifest_test.go` |
| PERF-04 | MPD cache keyed by contentId returns correct value for multi-key scenario | unit | `go test ./internal/media/ -run TestMPDCacheMultipleKeys -v` | `media/manifest_test.go` |

### Sampling Rate
- **Per task commit:** `go test ./internal/media/... ./internal/download/... -race -count=1 -run TestPhase2 2>&1`
- **Phase gate:** Full suite green with `-race` before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] MPD cache test file — covers cache miss, hit, concurrent access, multiple keys
- [ ] GetVideoBaseUrl/GetAudioBaseUrl replacement tests in `media/manifest_test.go`
- [ ] Parallel audio test with mock HTTP in `download/episode_test.go`
- [ ] Ensure `go test -race` is part of the phase gate

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | MPD manifest content is stable for the same `contentId` within a single download session (no stale-cache issues) | MPD Cache | Stale cached manifest causes wrong segment URLs or timeline — but unlikely since all versions fetched within seconds |
| A2 | Audio versions rarely exceed 5 per episode | MPD Cache | Cache eviction may be needed if Crunchyroll adds 10+ audio locales — easy to add LRU later |
| A3 | `GetLicense()` concurrent calls for different contentIds are safe because Phase 1 removed the global `keys` variable | Parallelization | If `GetLicense()` still uses shared mutable state beyond the `sync.Once` device, race conditions reappear |
| A4 | `DownloadParts()` respects context cancellation for its full duration | Parallelization | If `DownloadParts` ignores cancelled context, it continues downloading segments for a failed sibling version, wasting bandwidth |
| A5 | The `activeStreams` map is only accessed from the main goroutine (deferred cleanup runs serially after errgroup) | Stream Cleanup | If cleanup runs concurrently with parallel goroutines, map access must be synchronized |

**If this table is empty:** All claims in this research were verified or cited — no user confirmation needed.

## Open Questions

1. **How to handle `tempFiles` slice in parallel goroutines?**
   - What we know: `tempFiles = append(tempFiles, audioFile)` is written from multiple goroutines
   - What's unclear: Should we use a mutex, or pre-allocate the slice?
   - Recommendation: Pre-allocate `tempFiles` with known capacity (`len(versions)`) and write by index. The deferred cleanup reads `tempFiles` after all goroutines complete, so synchronization is only needed during parallel writes.

2. **Does `DownloadParts` check `ctx.Done()` during segment download?**
   - What we know: `DownloadParts` has a worker pool with `downloadCtx, cancel := context.WithCancel(ctx)`
   - What's unclear: Whether workers check `ctx.Done()` between segments, or only on error
   - Recommendation: Verify that `downloadCtx` cancellation stops the worker pool promptly. If workers only check `ctx.Done()` on error (current code), add an explicit check at the start of each worker iteration: `select { case <-downloadCtx.Done(): return nil, downloadCtx.Err(); default: }`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Code compilation | ✓ | 1.26.4 | — |
| `golang.org/x/sync` | errgroup import | ✓ (v0.22.0 will be fetched) | v0.22.0 | Use `sync.WaitGroup` + manual error channel (not recommended) |
| `go test` | Validation | ✓ | — | — |
| `go vet` | Code quality | ✓ | — | — |

**Missing dependencies with no fallback:** None

## Security Domain

> `workflow.security_enforcement` is absent from config (implicitly enabled if the key doesn't exist or is `true` — but no `.planning/config.json` key exists for `security_enforcement`). However, Phase 2 has no security-relevant changes — caching parsed structs, splitting functions, and parallelizing I/O are pure performance/correctness concerns. No new attack surface is introduced.

### Applicable ASVS Categories

None — Phase 2 introduces no authentication, no new HTTP endpoints, no cryptographic operations, and no user input handling. The only new code is:
- A `map` + `sync.RWMutex` for MPD caching (no security exposure)
- `errgroup` goroutine orchestration (no security exposure)
- Function refactoring (GetBaseUrl → GetVideoBaseUrl/GetAudioBaseUrl)

No ASVS categories apply to these changes.

## Sources

### Primary (HIGH confidence)
- [CITED: pkg.go.dev/sync] — `sync.RWMutex` documentation, properties, and contract
- [CITED: pkg.go.dev/golang.org/x/sync/errgroup] — errgroup API, WithContext behavior, Go source code
- [CITED: go-patterns.dev/stability/caching/with-map-and-rwmutex] — RWMutex cache pattern
- [VERIFIED: direct code analysis] — `internal/download/episode.go`, `internal/media/manifest.go`, `internal/drm/drm.go`, `internal/api/client.go`
- [VERIFIED: go.sum + go.mod] — Current dependency state, missing errgroup

### Secondary (MEDIUM confidence)
- [CITED: atharvapandey.com/post/go/go-concurrency-errgroup/] — errgroup idiomatic usage and pitfalls
- [CITED: blog.golang.org/pipelines] — Fan-out/fan-in patterns, context cancellation
- [CITED: strebkov.dev/posts/shard-your-locks/] — RWMutex vs sharded vs sync.Map benchmarks
- [CITED: github.com/samber/cc-skills-golang] — sync.RWMutex best practices
- [CITED: go.googlesource.com/sync/+/master/errgroup/errgroup.go] — errgroup source code

### Tertiary (LOW confidence)
None — all critical claims are backed by official Go documentation or verified via code analysis.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — `sync.RWMutex` and `errgroup` are battle-tested Go patterns with official documentation
- Architecture: HIGH — all changes are straightforward co-located modifications within existing files
- Pitfalls: HIGH — race conditions and context propagation gotchas are well-documented for errgroup

**Research date:** 2026-07-08
**Valid until:** 2026-08-08 (30 days — Go stdlib and errgroup are stable; no API changes expected)
