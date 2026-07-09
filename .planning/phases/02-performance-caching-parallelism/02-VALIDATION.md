---
phase: 2
slug: performance-caching-parallelism
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-08
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — standard Go test conventions |
| **Quick run command** | `go test ./internal/media/... ./internal/download/... -race -count=1 -run TestPhase2 2>&1` |
| **Full suite command** | `go test ./... -race -count=1 2>&1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/media/... ./internal/download/... -race -count=1 -run TestPhase2 2>&1`
- **After every plan wave:** Run `go test ./... -race -count=1 2>&1`
- **Before `/gsd-verify-work`:** Full suite must be green with `-race`
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01-T1 | 02-01 | 1 | PERF-04 | — | N/A | unit | `go test ./internal/media/ -run TestMPDCacheMiss -v` | ❌ W0 | ⬜ pending |
| 02-01-T2 | 02-01 | 1 | PERF-04 | — | N/A | unit | `go test ./internal/media/ -run TestMPDCacheHit -v` | ❌ W0 | ⬜ pending |
| 02-01-T3 | 02-01 | 1 | PERF-04 | — | N/A | unit | `go test ./internal/media/ -run TestMPDCacheConcurrent -race -v` | ❌ W0 | ⬜ pending |
| 02-03-T1 | 02-03 | 1 | QOL-03 | — | N/A | unit | `go test ./internal/media/ -run TestGetVideoBaseUrlEmpty -v` | ❌ W0 | ⬜ pending |
| 02-03-T2 | 02-03 | 1 | QOL-03 | — | N/A | unit | `go test ./internal/media/ -run TestGetAudioBaseUrlBandwidth -v` | ❌ W0 | ⬜ pending |
| 02-02-T1 | 02-02 | 2 | PERF-06 | — | N/A | integration | `go test ./internal/download/ -run TestEpisodeParallelAudio -race -v` | ❌ W0 | ⬜ pending |
| 02-02-T2 | 02-02 | 2 | PERF-06 | — | N/A | unit | `go test ./internal/download/ -run TestEpisodeSingleVersion -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/media/manifest_test.go` — MPD cache tests (miss, hit, concurrent, multiple keys)
- [ ] `internal/media/manifest_test.go` — GetVideoBaseUrl/GetAudioBaseUrl tests (empty set, bandwidth matching)
- [ ] `internal/download/episode_test.go` — Parallel audio test with mock HTTP, single-version edge case
- [ ] `go mod download golang.org/x/sync` — add errgroup dependency
- [ ] `go test -race` must be part of the phase gate

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Cache miss → fetch+parse+store | PERF-04 | Requires actual MPD fetch | Run episode download with debug output; verify first fetch caches, subsequent calls hit cache |
| errgroup cancels remaining on first error | PERF-06 | Integration environment needed to simulate partial failures | Inject error in one audio version's license; verify remaining goroutines cancel via context |
| GetAudioBaseUrl fallback for unknown quality | QOL-03 | Unknown quality value edge case | Call with "256k"; verify falls through to first representation |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending