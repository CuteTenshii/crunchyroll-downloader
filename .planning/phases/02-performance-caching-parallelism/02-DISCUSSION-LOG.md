# Phase 2: Performance — Caching & Parallelism - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-08
**Phase:** 2-Performance — Caching & Parallelism
**Areas discussed:** MPD Cache Design, Parallelization Pattern, GetBaseUrl Split, Stream Cleanup Strategy

---

## MPD Cache Design

| Option | Description | Selected |
|--------|-------------|----------|
| contentId | Matches PERF-04 spec. Natural map key, already available in audio version loop. | ✓ |
| Manifest URL | Guarantees unique per manifest. Slightly more robust if Crunchyroll reuses URLs across contentIds. | |
| Parsed \*mpd.MPD only | Cache the decoded struct. Avoids re-parsing. Simple. | ✓ |
| Parsed + raw bytes | Cache both. Useful if error recovery needs re-parse or debug-manifest flag dumps XML. | |
| sync.RWMutex | Read-mostly pattern. Multiple goroutines read, single writer on first fetch. | ✓ |
| sync.Map | Built-in concurrent map. Optimized for write-once, read-many. | |
| No eviction | Simple map. Per-episode lifetime — far fewer entries than any eviction threshold. | ✓ |
| Simple LRU | Over-engineered for 3-5 entries. Useful if caching across episodes. | |

**User's choice:** contentId, parsed struct only, sync.RWMutex, no eviction
**Notes:** All four sub-questions answered with recommended options.

---

## Parallelization Pattern

| Option | Description | Selected |
|--------|-------------|----------|
| errgroup | Built-in context cancellation on first error. Idiomatic for parallel subtask fan-out. | ✓ |
| sync.WaitGroup + channel | Manual error collection via channel. More boilerplate. | |
| Cancel remaining | errgroup.WithContext cancels ctx on first error. Faster failure. | ✓ |
| Let all complete | Continue downloading all audio versions. Accumulate errors. | |
| Outside parallel group | Download video separately, then fan-out audio parallel. Simpler. | ✓ |
| Inside parallel group | Treat video as one task in errgroup. Cleaner but needs special-casing. | |
| Fetch in parallel, cache-first | Each goroutine fetches via cache (miss→fetch+parse→cache, hit→return). | ✓ |
| Pre-fetch manifests sequentially | Fetch and cache all manifests first, then parallel audio. Adds sequential bottleneck. | |

**User's choice:** errgroup, cancel on first error, video outside parallel group, cache-first manifest fetching
**Notes:** All four sub-questions answered with recommended options.

---

## GetBaseUrl Split

| Option | Description | Selected |
|--------|-------------|----------|
| Split + improve matching | Same refactor but also add explicit quality→bandwidth mapping. | ✓ |
| Pure mechanical split | Keep bandwidth logic identical. Move video/audio to separate functions. | |
| Keep (\*string, \*string) | Minimal caller changes. Nil pointers for not-found. | ✓ |
| New BaseURLResult struct | Named fields with error field. More refactoring. | |
| Simple switch/case map | Map quality string to minimum bandwidth. Plus fallback. | ✓ |
| Configurable mapping | Make bandwidth thresholds configurable. Over-engineered. | |

**User's choice:** Split + improve matching, keep (*string, *string) return, simple switch/case map
**Notes:** Wants the bandwidth matching improved in the same refactor.

---

## Stream Cleanup Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Defer all to post-parallel | After all parallel audio downloads complete, release all streams together. | ✓ |
| Release per-goroutine | Each goroutine releases its own stream when done. Tighter timing. | |
| Defer all to end (v0 same as others) | Track all stream tokens including version[0] in activeStreams. Release everything in deferred cleanup. | ✓ |
| Cleanup version[0] early | Release version[0]'s stream right after video download. Tighter lifetime. | |

**User's choice:** Defer all to post-parallel, version[0] treated same as others
**Notes:** Simplicity wins — track all tokens, cleanup at end.

---

## the agent's Discretion

None — all decisions made explicitly.

## Deferred Ideas

None — discussion stayed within Phase 2 scope.
