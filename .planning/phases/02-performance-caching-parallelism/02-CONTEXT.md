# Phase 2: Performance — Caching & Parallelism - Context

**Gathered:** 2026-07-08
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase optimizes multi-dub downloads within a single episode: cache parsed MPD manifests per `contentId` to avoid redundant XML parsing, parallelize manifest fetching and license challenges across audio versions using `errgroup`, and refactor `GetBaseUrl` into separate video/audio functions with improved bandwidth matching.

</domain>

<decisions>
## Implementation Decisions

### MPD Cache Design (PERF-04)
- **D-01:** Cache keyed by `contentId` (as specified in PERF-04 requirement).
- **D-02:** Cache stores parsed `*mpd.MPD` struct only (not raw XML bytes).
- **D-03:** Thread-safe via `sync.RWMutex` — read-mostly pattern, single writer on first fetch.
- **D-04:** No eviction — per-episode lifetime with at most ~5 entries.

### Parallelization Pattern (PERF-06)
- **D-05:** Use `golang.org/x/sync/errgroup` for goroutine coordination — idiomatic context cancellation on first error.
- **D-06:** Cancel remaining goroutines on first error — `errgroup.WithContext` cancels `ctx`, remaining goroutines exit via `ctx.Done()`.
- **D-07:** Video download stays outside the parallel group — sequential, before audio fan-out.
- **D-08:** Manifest fetching + parsing happens inside the parallel group, using the MPD cache (cache-first: miss → fetch+parse → cache).

### GetBaseUrl Split (QOL-03)
- **D-09:** Split `GetBaseUrl` into `GetVideoBaseUrl` and `GetAudioBaseUrl`.
- **D-10:** Return type stays `(*string, *string)` — minimal caller changes.
- **D-11:** Improve audio bandwidth matching with explicit switch/case mapping instead of fragile threshold fallback (noted in CONCERNS.md).

### Stream Cleanup Strategy
- **D-12:** Defer all stream release (DeleteStream) to after parallel work completes — use existing `activeStreams` map + deferred cleanup closure.
- **D-13:** Primary version's (version[0]) stream treated the same as others — defer to end rather than cleaning up early.

### the agent's Discretion
None — all decisions were made explicitly during discussion.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition and constraints
- `.planning/ROADMAP.md` — Phase 2 scope (plans 2.1/2.2/2.3), dependency ordering, and requirement mapping.
- `.planning/REQUIREMENTS.md` — Full definitions for PERF-04, PERF-06, QOL-03 with acceptance criteria.
- `.planning/PROJECT.md` — Project context, validated requirements, and architecture decisions.

### Codebase maps
- `.planning/codebase/ARCHITECTURE.md` — Current data flow, sequential audio loop, global state patterns.
- `.planning/codebase/CONCERNS.md` — Known performance bottlenecks (MPD re-parsing, sequential audio, global `keys` anti-pattern).
- `.planning/codebase/STACK.md` — Go 1.25 stack, gowidevine and go-mpd dependencies.

### Prior phase context
- `.planning/phases/01-foundation-error-handling-http-memory/01-CONTEXT.md` — Prior decisions on HTTP transport, context propagation, Widevine device caching, error contracts.

### Current implementation touchpoints
- `internal/download/episode.go` — Main audio version loop (lines 152-216) — parallelization target.
- `internal/media/manifest.go` — `ParseManifest`, `GetBaseUrl` — cache and split targets.
- `internal/media/segment.go` — `DownloadParts`, `DownloadSubs` — concurrency targets.
- `internal/drm/drm.go` — `GetLicense`, `GetWidevineDevice`, `GetPssh` — license acquisition path.
- `internal/api/client.go` — `FetchManifest`, `SendChallenge`, `DeleteStream` — HTTP API calls.

### No external specs
No external specs — requirements are fully captured in the decisions above and the roadmap.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `errgroup` from `golang.org/x/sync/errgroup` — needs to be added as dependency; idiomatic parallel fan-out with context cancellation.
- `sync.RWMutex` — stdlib, already used elsewhere; natural fit for MPD cache concurrent access.
- `internal/download.Episode`'s existing `activeStreams` map and deferred cleanup closure — can be extended to track all stream tokens during parallel work.

### Established Patterns
- Phase 1 already fixed global `keys` — `GetLicense()` returns `[]*widevine.Key` and callers pass them to `DownloadParts`. This unblocks parallel audio.
- Widevine device cached via `sync.Once` in `GetWidevineDevice()` — already thread-safe, parallel license requests work.
- `context.Context` propagates from `main` through the entire pipeline — `errgroup.WithContext` can derive from this.
- The codebase uses `internal/` packages with clear file-level responsibilities — cache goes in `internal/media/manifest.go` (co-located with `ParseManifest`).

### Integration Points
- `internal/download/episode.go:152-216` — Replace sequential `for` loop with parallel fan-out via `errgroup`. Video download stays sequential at first-iteration position; audio versions fan out after.
- `internal/media/manifest.go` — Add MPD cache (package-level `sync.RWMutex` + `map[string]*mpd.MPD`), add cache-check before `ParseManifest`. Split `GetBaseUrl` into `GetVideoBaseUrl` and `GetAudioBaseUrl`.
- `internal/download/episode.go:210-215` — Move `DeleteStream` calls out of the per-version loop into deferred post-parallel cleanup.

</code_context>

<specifics>
## Specific Ideas

- The MPD cache should be co-located with `ParseManifest` in `internal/media/manifest.go` — a simple `sync.RWMutex` + `map[string]*mpd.MPD`, keyed by `contentId`.
- The cache is checked before every `ParseManifest` call. On miss, fetch+parse normally and store the result. On hit, return the cached parsed struct.
- `GetVideoBaseUrl` keeps the exact video-height-matching logic. `GetAudioBaseUrl` replaces the bandwidth threshold chain with an explicit `switch/case` mapping (e.g., `"192k" → bandwidth >= 192000`).

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within Phase 2 scope.

</deferred>

---

*Phase: 2-Performance — Caching & Parallelism*
*Context gathered: 2026-07-08*
