# Phase 1: Foundation — Error Handling, HTTP, Memory - Context

**Gathered:** 2026-07-08
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase hardens the download pipeline: replace panic-driven failures with explicit errors, configure the shared HTTP path with timeouts and cancellation, stream segment assembly through temp files to cut RAM usage, cache the Widevine device per process, cleanly handle interrupts and ffmpeg failures, and add initial tests around the most fragile behavior.

</domain>

<decisions>
## Implementation Decisions

### Error Contract
- **D-01:** Batch and season flows should continue past an episode failure after cleaning up resources for that episode.
- **D-02:** User-facing errors should be short, clear, and actionable, with no stack traces in normal output.
- **D-03:** Cleanup failures such as temp-file removal or `DeleteStream` failures should be warnings when the main download/mux succeeded.
- **D-04:** Expected control flow should not use `panic()`; internal functions should return `error` and `main` should decide whether to continue or exit.

### HTTP + Context Behavior
- **D-05:** The shared HTTP client should use a configured transport with keep-alive, `MaxIdleConnsPerHost >= 10`, and request/context timeouts.
- **D-06:** A `401` response should trigger exactly one token refresh and one retry before returning a clean error.
- **D-07:** Segment and subtitle downloads should use the same configured client path instead of `http.DefaultClient`.
- **D-08:** `context.Context` should propagate from `main` through API/media/workers so Ctrl+C can cancel active work.

### Streaming Segment Assembly
- **D-09:** `DownloadParts` should write init + segment data into a temp file first, then decrypt from that file instead of buffering the whole payload in memory.
- **D-10:** Any partial segment failure should fail the download and clean up all temporaries.
- **D-11:** Progress should stay segment-based.
- **D-12:** `media.DownloadParts` should own temp-file creation, write, decrypt, and cleanup.

### Widevine Device Lifetime
- **D-13:** The Widevine device should be loaded once per process and reused for all license requests.
- **D-14:** Device paths should come from environment variables loaded from `.env` for this phase, preserving compatibility with the current CLI surface.
- **D-15:** `client_id.bin` and `private_key.pem` should be treated as an all-or-nothing pair.
- **D-16:** If both `.wvd` and the raw pair exist, `.wvd` should win.
- **D-17:** Missing device configuration should produce one clear error that lists accepted formats and mentions the `.env`-based source.

### Signal Cleanup + Resource Ownership
- **D-18:** Ctrl+C/SIGTERM should cancel active work, stop workers, remove temporaries, and release streams.
- **D-19:** ffmpeg should be killed on interruption and any partial output should be removed.
- **D-20:** Temporary files should always be removed on interruption.
- **D-21:** `DeleteStream` failures should not block a clean local shutdown.
- **D-22:** Any partial episode artifacts should be removed after interruption.

### Test Scope for Phase 1
- **D-23:** Test investment should focus on the most fragile points in this phase instead of chasing aggressive coverage.
- **D-24:** The first test batch should cover the error contract, HTTP timeout/retry, `DownloadParts`, Widevine discovery, cleanup/interruption, and URL parsing.
- **D-25:** This phase should set a minimal useful coverage bar, not a strict high target.

### the agent's Discretion
None — the user made explicit choices for the discussed gray areas.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition and constraints
- `.planning/ROADMAP.md` — Phase 1 scope, plan ordering, and dependency boundaries.
- `.planning/REQUIREMENTS.md` — Locked requirements for PERF-01/02/03/05/07/08, UX-01/05/07, and QOL-01/02/06/09/10/11/12/13.
- `.planning/PROJECT.md` — Project context, validated requirements, current milestone, and existing package layout.

### Codebase maps
- `.planning/codebase/STACK.md` — Go 1.25 CLI stack, ffmpeg dependency, and current config surface.
- `.planning/codebase/ARCHITECTURE.md` — Current package boundaries, data flow, and error-handling constraints.
- `.planning/codebase/CONCERNS.md` — Known bugs and fragility points that this phase is meant to address.

### Current implementation touchpoints
- `main.go` — CLI entry, URL parsing, and batch dispatch.
- `internal/api/client.go` — Shared client, token refresh, and request construction.
- `internal/api/license.go` — Widevine challenge/response request path.
- `internal/download/episode.go` — Episode orchestration, subtitle/audio download loop, and cleanup.
- `internal/media/segment.go` — Segment download, temp-file naming, and current memory-heavy assembly path.
- `internal/drm/drm.go` — Widevine device discovery and license acquisition.
- `internal/mux/mux.go` — ffmpeg merge path and cleanup behavior.

### No external specs
No external specs — requirements are fully captured in the decisions above and the phase roadmap.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/api.Client` already centralizes auth token handling and request retries; it is the natural anchor for the configured HTTP behavior.
- `internal/media.DownloadParts` already owns segment download and decryption, so it is the right place to switch from RAM buffering to temp-file assembly.
- `internal/drm.GetLicense` already returns Widevine keys instead of storing them globally, which fits a per-process device cache.
- `internal/download.Episode` already owns episode-level orchestration and cleanup, so it is the right boundary for cancel propagation and episode-level failure handling.

### Established Patterns
- The codebase already prefers small package-local responsibilities in `internal/` rather than reintroducing a monolith.
- Request paths already set explicit headers and retry on auth failure, so the next step is tightening transport, timeout, and cancellation behavior rather than redesigning the flow.
- Cleanup is already performed in a deferred episode-level closure, which can be extended to support interruption semantics.

### Integration Points
- `main.go` is the entry point for wiring `context.Context` into the rest of the application.
- `internal/api/client.go` needs the transport/timeouts/token-refresh policy.
- `internal/media/segment.go` needs the temp-file streaming rewrite and the shared HTTP client path.
- `internal/drm/drm.go` needs the one-time device cache and `.env`-backed lookup.
- `internal/mux/mux.go` needs error returns instead of `panic()` and cleanup warnings.

</code_context>

<specifics>
## Specific Ideas

- The user wants Widevine device paths sourced from environment variables loaded from `.env` during this phase.
- The user wants `DownloadParts` to stay segment-progress based even after switching to temp-file streaming.
- The user wants `media.DownloadParts` to own the whole temp-file lifecycle instead of pushing that responsibility upward.

</specifics>

<deferred>
## Deferred Ideas

None — the discussion stayed within Phase 1 scope.

</deferred>

---

*Phase: 1-Foundation — Error Handling, HTTP, Memory*
*Context gathered: 2026-07-08*
