---
phase: 5
slug: testing-quality
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-09
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (no external deps per D-03) |
| **Config file** | None — not needed |
| **Quick run command** | `go test ./internal/media/ ./internal/locale/ ./internal/config/` |
| **Full suite command** | `go test -count=1 -race ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./package/being-modified/` (targeted run)
- **After every plan wave:** Run `go test -count=1 -race ./...` (full suite)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 5.1 | 1 | QOL-01 | — | N/A (test infrastructure) | N/A | `go test ./...` | ❌ W0 | ⬜ pending |
| 05-01-02 | 5.1 | 1 | QOL-01 | V5 | URL injection in BuildUrl — table-driven tests with special characters, path traversal patterns | unit | `go test -run TestBuildUrl ./internal/media/` | ❌ W0 | ⬜ pending |
| 05-01-02 | 5.1 | 1 | QOL-01 | V5 | Filename injection in sanitizeFilename — table-driven tests with illegal chars, empty string, unicode | unit | `go test -run TestSanitizeFilename ./internal/download/` | ✅ | ⬜ pending |
| 05-01-02 | 5.1 | 1 | QOL-01 | — | parseLangs unit test | unit | `go test -run TestParseLangs .` | ❌ W0 | ⬜ pending |
| 05-01-02 | 5.1 | 1 | QOL-01 | — | ExpandTimeline unit test | unit | `go test -run TestExpandTimeline ./internal/media/` | ❌ W0 | ⬜ pending |
| 05-01-02 | 5.1 | 1 | QOL-01 | — | GetBaseUrl unit test | unit | `go test -run TestGetVideoBaseUrl ./internal/media/` | ✅ | ⬜ pending |
| 05-01-02 | 5.1 | 1 | QOL-01 | — | GetAudioBaseUrl unit test | unit | `go test -run TestGetAudioBaseUrl ./internal/media/` | ✅ | ⬜ pending |
| 05-01-02 | 5.1 | 1 | QOL-01 | — | LanguageNames/LanguageCodes (locale package) | unit | `go test ./internal/locale/` | ❌ W0 | ⬜ pending |
| 05-01-03 | 5.1 | 1 | QOL-01 | — | processURL integration with httptest | integration | `go test -run TestProcessURL .` | ❌ W0 | ⬜ pending |
| 05-01-03 | 5.1 | 1 | QOL-01 | — | ParseManifest unit test | unit | `go test -run TestParseManifest ./internal/media/` | ❌ W0 | ⬜ pending |
| 05-01-03 | 5.1 | 1 | QOL-01 | — | GetPssh unit test (drm) | unit | `go test -run TestGetPssh ./internal/drm/` | ❌ W0 | ⬜ pending |
| 05-01-03 | 5.1 | 1 | QOL-01 | — | TrackTitle unit test (mux) | unit | `go test -run TestTrackTitle ./internal/mux/` | ✅ | ⬜ pending |
| 05-02-01 | 5.2 | 1 | QOL-01 | — | CI workflow file exists | N/A | `test -f .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| 05-02-02 | 5.2 | 1 | QOL-01 | — | Makefile targets exist | N/A | `grep -q '^test:' Makefile && grep -q '^coverage:' Makefile && grep -q '^lint:' Makefile` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/locale/locale_test.go` — first test file for locale package (pure map lookups)
- [ ] `main_test.go` — parseLangs, sanitizeFilename, resolveString, isAllNilConfig table-driven tests
- [ ] `internal/media/manifest_test.go` — extend with BuildUrl, ExpandTimeline, ParseManifest
- [ ] `internal/config/config_test.go` — extend with LoadDotenv, parseDotenv
- [ ] `internal/download/episode_test.go` — extend with formatFileSize, formatDuration
- [ ] `internal/mux/mux_test.go` — extend with TrackTitle
- [ ] Framework install: none needed (Go stdlib)

*Existing infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| CI pipeline triggers on PR/push | QOL-01 | Requires actual GitHub PR event | Open a PR and verify `.github/workflows/ci.yml` triggers |
| golangci-lint configuration | QOL-01 | Linter config correctness validated by CI run | Push `.golangci.yml` and verify CI lint step passes |

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
