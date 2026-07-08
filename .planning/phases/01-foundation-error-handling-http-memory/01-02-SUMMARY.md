---
phase: 01-foundation-error-handling-http-memory
plan: 01-02
subsystem: api
tags: [go, http, context, retry, tests]
requires:
  - phase: 01-foundation-error-handling-http-memory
    provides: "01-01 established error-returning download and mux paths that this plan extends with cancellable network behavior."
provides:
  - Shared HTTP client with configured transport, keep-alive, idle connection reuse, and explicit timeouts.
  - Context-aware API, license, segment, subtitle, episode, and season request paths.
  - Bounded 401 handling that refreshes once and retries once before returning an error.
  - Tests covering retry budget, cancellation, request routing, license body retry, and transport configuration.
affects: [api, download, drm, media, phase-01]
tech-stack:
  added: []
  patterns:
    - "Root context is created in main and threaded through download/API entrypoints."
    - "api.Client.Do is the single authenticated HTTP execution path with a one-shot refresh budget."
    - "media downloads accept an HTTP doer so segment/subtitle requests use the configured client."
key-files:
  created:
    - internal/api/client_test.go
    - internal/api/episode_test.go
    - internal/api/license_test.go
    - internal/api/season_test.go
  modified:
    - main.go
    - internal/api/auth.go
    - internal/api/client.go
    - internal/api/episode.go
    - internal/api/license.go
    - internal/api/manifest.go
    - internal/api/season.go
    - internal/download/episode.go
    - internal/download/season.go
    - internal/drm/drm.go
    - internal/media/segment.go
    - .planning/STATE.md
    - .planning/ROADMAP.md
    - .planning/REQUIREMENTS.md
key-decisions:
  - "Kept 401 recovery centralized in api.Client.Do and bounded it to one refresh plus one retry."
  - "Used context-aware method signatures instead of background contexts in API, download, DRM, and media paths."
  - "Updated media segment and subtitle helpers to accept an HTTP doer so they use the configured client without importing API internals."
patterns-established:
  - "Network methods accept context.Context as their first argument."
  - "Retryable request bodies rely on http.NewRequestWithContext GetBody support for bytes.Reader/strings.Reader inputs."
  - "Local httptest coverage verifies behavior that future phases depend on."
requirements-completed: [PERF-02, PERF-03, PERF-07, QOL-06]
coverage:
  - id: D1
    description: "Shared HTTP client uses a configured transport with keep-alive, idle connection reuse, and explicit timeout behavior."
    requirement: PERF-02
    verification:
      - kind: unit
        ref: "internal/api/client_test.go#TestConfiguredHTTPClientUsesTimeoutsAndKeepAlive"
        status: pass
      - kind: other
        ref: "command: GOCACHE=/tmp/go-build go test ./internal/api ./internal/download"
        status: pass
    human_judgment: false
  - id: D2
    description: "HTTP calls carry context from main through API, download, DRM, segment, and subtitle request paths."
    requirement: PERF-03
    verification:
      - kind: unit
        ref: "internal/api/client_test.go#TestNewRequestPropagatesCancellation"
        status: pass
      - kind: unit
        ref: "internal/api/episode_test.go#TestGetEpisodeInfoUsesClientRequestPath"
        status: pass
      - kind: other
        ref: "command: GOCACHE=/tmp/go-build go test ./..."
        status: pass
    human_judgment: false
  - id: D3
    description: "Segment and subtitle downloads use the configured HTTP client path instead of http.DefaultClient."
    requirement: PERF-07
    verification:
      - kind: other
        ref: "command: rg \"http\\.DefaultClient|http\\.Get\\(|http\\.Post\\(\" internal main.go"
        status: pass
      - kind: other
        ref: "command: GOCACHE=/tmp/go-build go test ./internal/api ./internal/download"
        status: pass
    human_judgment: false
  - id: D4
    description: "401 handling refreshes once, retries once, and returns an error if the retry is still unauthorized."
    requirement: QOL-06
    verification:
      - kind: unit
        ref: "internal/api/client_test.go#TestDoRefreshesTokenOnceAndRetries"
        status: pass
      - kind: unit
        ref: "internal/api/client_test.go#TestDoReturnsErrorAfterOneFailedRefreshRetry"
        status: pass
      - kind: unit
        ref: "internal/api/license_test.go#TestSendChallengeRetriesRewindableBodyAfterRefresh"
        status: pass
    human_judgment: false
duration: 7 min
completed: 2026-07-08
status: complete
---

# Phase 01 Plan 01-02: HTTP Client Context and Retry Summary

**Configured HTTP transport, cancellable request flow, and one-shot 401 refresh now cover API, license, segment, and subtitle downloads.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-08T22:26:55Z
- **Completed:** 2026-07-08T22:33:49Z
- **Tasks:** 3 completed
- **Files modified:** 21

## Accomplishments

- `main` now owns a root context and passes it through episode and season download orchestration.
- `api.Client` now builds a configured `http.Client` with dial timeout, overall timeout, keep-alive, idle connection reuse, and response-header timeout.
- API, license, DRM, manifest, segment, subtitle, episode, and season request paths now accept context and use the shared client path.
- 401 handling is explicit and bounded: one refresh and one retry, then an error if authorization still fails.
- API tests cover configured transport settings, retry budget, cancellation propagation, request routing, and license POST body retry.

## Task Commits

Each task was committed atomically:

1. **Task 1: Root context and download orchestration wiring** - `188e483` (feat)
2. **Task 2: Shared HTTP client, context-aware API/media paths, and bounded 401 retry** - `c5ba1f9` (feat)
3. **Task 3: HTTP retry, cancellation, and transport tests** - `fbc9c77` (test)

**Plan metadata:** committed separately with this summary.

## Files Created/Modified

- `main.go` - Creates the root context, initializes the API client with it, and passes it through URL processing.
- `internal/api/client.go` - Adds configured transport, timeout policy, context-aware request construction, and one-shot 401 retry handling.
- `internal/api/auth.go` - Fetches access tokens through a context-aware request.
- `internal/api/episode.go` - Uses context-aware request construction for episode info, playback, and stream deletion.
- `internal/api/season.go` - Uses context-aware request construction for season and episode listing calls.
- `internal/api/manifest.go` - Fetches manifests with caller-provided context.
- `internal/api/license.go` - Sends Widevine challenges with context and rewindable request bodies for retry.
- `internal/download/episode.go` - Threads context into playback, manifest, subtitle, segment, license, and cleanup calls.
- `internal/download/season.go` - Threads context through season orchestration and test injection.
- `internal/drm/drm.go` - Passes context into the license challenge request.
- `internal/media/segment.go` - Uses an injected HTTP doer for segment/subtitle downloads and honors cancellation during retry backoff.
- `internal/api/client_test.go` - Covers transport configuration, one-shot retry, second-401 failure, and cancellation.
- `internal/api/episode_test.go` - Covers context-aware episode info request routing and auth headers.
- `internal/api/season_test.go` - Covers context-aware season request routing and auth headers.
- `internal/api/license_test.go` - Covers license challenge retry with a rewindable POST body.
- `internal/download/episode_test.go` - Updates episode tests for context-aware signatures.
- `internal/download/season_test.go` - Updates season tests for context-aware signatures.
- `.planning/STATE.md` - Records plan completion and next action.
- `.planning/ROADMAP.md` - Marks HTTP/context/retry work complete for Phase 1.
- `.planning/REQUIREMENTS.md` - Marks PERF-02, PERF-03, PERF-07, and QOL-06 complete.
- `.planning/phases/01-foundation-error-handling-http-memory/01-02-SUMMARY.md` - This summary.

## Decisions Made

- Kept token refresh in `api.Client.Do` so callers do not duplicate retry policy.
- Returned an error after the second 401 rather than returning an unauthorized response body to callers.
- Used a small `httpDoer` interface in `internal/media` so media downloads can use the shared client without coupling the package to API implementation details.
- Preserved `api.New(etpRt)` as a backward-compatible wrapper while adding `api.NewWithContext(ctx, etpRt)` for root-context initialization.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Extended shared-client wiring into media and DRM paths**
- **Found during:** Task 2 (shared HTTP client and context propagation)
- **Issue:** The task file list did not include `internal/media/segment.go` or `internal/drm/drm.go`, but PERF-07 requires `DownloadPart()` and `DownloadSubs()` to stop using `http.DefaultClient`, and license requests need the same context chain.
- **Fix:** Updated media helpers to accept an HTTP doer and context, updated DRM license calls to accept context, and routed episode downloads through those signatures.
- **Files modified:** `internal/media/segment.go`, `internal/drm/drm.go`, `internal/download/episode.go`
- **Verification:** `rg "http\\.DefaultClient|http\\.Get\\(|http\\.Post\\(" internal main.go` returned no matches; `GOCACHE=/tmp/go-build go test ./...` passed.
- **Committed in:** `c5ba1f9`

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** The deviation was required to satisfy PERF-07 and full context propagation. No unrelated behavior was added.

## Issues Encountered

- The restricted sandbox cannot bind `httptest` localhost servers. Tests that require local servers were rerun with approved escalation.
- The default Go build cache path is not writable in the sandbox. Verification used `GOCACHE=/tmp/go-build`.

## Known Stubs

None. Stub scan found only normal nil/error checks and existing user-facing availability messages.

## Threat Flags

None. This plan refactored existing outbound HTTP surfaces and did not add new network endpoints, auth entrypoints, file-access trust boundaries, or schema changes.

## Verification

- `GOCACHE=/tmp/go-build go test ./internal/api ./internal/download` - passed
- `GOCACHE=/tmp/go-build go test ./...` - passed
- `rg "http\\.DefaultClient|http\\.Get\\(|http\\.Post\\(" internal main.go` - no matches
- `TestDoRefreshesTokenOnceAndRetries` - verifies exactly one refresh and retry on initial 401.
- `TestDoReturnsErrorAfterOneFailedRefreshRetry` - verifies no recursive or repeated 401 retry.
- `TestNewRequestPropagatesCancellation` - verifies request cancellation is propagated.
- `TestConfiguredHTTPClientUsesTimeoutsAndKeepAlive` - verifies timeout and idle connection configuration.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Ready for the next Phase 1 plan. HTTP requests now have a single configured execution path with cancellation and bounded auth retry behavior, so follow-on memory/streaming and worker-count changes can reuse the same client path.

## Self-Check: PASSED

- Created files exist: `internal/api/client_test.go`, `internal/api/episode_test.go`, `internal/api/license_test.go`, `internal/api/season_test.go`, and this summary.
- Task commits exist: `188e483`, `c5ba1f9`, `fbc9c77`.
- Required verification passed with `GOCACHE=/tmp/go-build go test ./internal/api ./internal/download`.
- Plan-level verification passed with `GOCACHE=/tmp/go-build go test ./...`.

---
*Phase: 01-foundation-error-handling-http-memory*
*Completed: 2026-07-08*
