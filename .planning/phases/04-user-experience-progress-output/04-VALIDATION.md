---
phase: 4
slug: user-experience-progress-output
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-09
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (`go test`) |
| **Config file** | None — standard `_test.go` convention |
| **Quick run command** | `go test ./internal/output/... -count=1 -v -short` |
| **Full suite command** | `go test ./internal/output/... -count=1 -v` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/output/... -count=1 -v -short`
- **After every plan wave:** Run `go test ./internal/output/... -count=1 -v`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 04-03-01 | 03 | 1 | UX-04, UX-08 | T-04-01 / T-04-02 / T-04-03 | N/A — output writes to stdout/stderr only, no parsing | unit | `go test ./internal/output/... -run TestHumanOutput -v` | ❌ W0 | ⬜ pending |
| 04-03-02 | 03 | 1 | UX-04, UX-08 | — | N/A — CLI flags don't process untrusted input | integration | `go build .` | ❌ W0 | ⬜ pending |
| 04-03-03 | 03 | 1 | UX-04, UX-08 | — | N/A — output replacement preserves existing error handling | integration | `go build ./...` | ❌ W0 | ⬜ pending |
| 04-01-01 | 01 | 2 | UX-02, UX-06 | T-04-04 / T-04-05 | N/A — episode titles already visible in output | integration | `go test ./internal/output/... -run TestSeasonProgress -v` | ❌ W0 | ⬜ pending |
| 04-01-02 | 01 | 2 | UX-02, UX-06 | — | N/A — error accumulation is program logic | integration | `go test ./internal/output/... -run TestSeasonErrors -v` | ❌ W0 | ⬜ pending |
| 04-02-01 | 02 | 3 | UX-03 | T-04-06 / T-04-07 | N/A — speed values are derived from segment byte counts | unit | `go test ./internal/output/... -run TestSpeedTracker -v` | ❌ W0 | ⬜ pending |
| 04-02-02 | 02 | 3 | UX-03 | — | N/A — SpeedTracker tests don't touch I/O | unit | `go test ./internal/output/... -run TestSpeedTracker -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/output/output_test.go` — test stubs for all output modes (human, JSON, quiet)
- [ ] `internal/output/output_test.go` — SpeedTracker unit tests (Record, Bps, ETA, clamping)
- [ ] `internal/output/output_test.go` — NDJSON event stream tests (capture stdout, validate JSON)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Season progress displays colored ✓/✗ on terminal | UX-02 | ANSI color rendering depends on terminal emulator — cannot assert exact escape code output in portable tests | Run `go run . --help`, verify no ANSI codes in piped output; run `go run .` with a season URL, verify colored output |
| Speed/ETA formatting in human mode | UX-03 | Formatting depends on runtime byte counts and timing — unit tests validate SpeedTracker algorithm but live formatting is end-to-end | Run season download, observe progress line updates with speed/ETA once per second |
| Season summary with failed episode details | UX-06 | Requires actual errors during download — unit tests validate structure but live output is end-to-end | Simulate network failure during season download, verify error listing at end |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
