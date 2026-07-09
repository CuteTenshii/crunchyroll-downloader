---
phase: 03
slug: usability-configuration-validation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-09
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` |
| **Config file** | none — `go test` built-in |
| **Quick run command** | `go test ./internal/config/ -short -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/config/ -short -count=1`
- **After every plan wave:** Run `go test ./internal/config/ ./internal/drm/ -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 3.1 | 1 | USAB-01, USAB-02 | — | Env var not leaked to args | unit | `go test ./internal/config/ -run TestConfigLoad` | ❌ W0 | ⬜ pending |
| 03-02-01 | 3.2 | 1 | USAB-03 | — | Output dir validated, not created | unit | `go test ./internal/... -run TestOutputDir` | ❌ W0 | ⬜ pending |
| 03-02-02 | 3.2 | 1 | USAB-04 | — | N/A (URL pattern check) | unit | `go test ./internal/... -run TestBatchURLValidation` | ❌ W0 | ⬜ pending |
| 03-03-01 | 3.3 | 1 | USAB-05 | — | N/A (LookPath check) | unit | `go test ./internal/... -run TestFfmpegCheck` | ❌ W0 | ⬜ pending |
| 03-03-02 | 3.3 | 1 | USAB-06 | — | Widevine path validated before use | unit | `go test ./internal/drm/ -run TestWidevineDevicePath` | ❌ W0 | ⬜ pending |
| 03-04-01 | 3.4 | 1 | QOL-04, QOL-05 | — | N/A (logic fix) | unit | `go test ./internal/... -run TestURLValidation` | ❌ W0 | ⬜ pending |
| 03-04-02 | 3.4 | 1 | QOL-07 | — | N/A (regex) | unit | `go test ./internal/... -run TestSanitizeFilename` | ❌ W0 | ⬜ pending |
| 03-04-03 | 3.4 | 1 | QOL-08 | — | N/A (parseLangs move) | unit | `go test ./internal/... -run TestParseLangs` | ❌ W0 | ⬜ pending |
| 03-05-01 | 3.5 | 1 | USAB-07 | — | Credential override not logged | unit | `go test ./internal/api/ -run TestClientAuth` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/config/config_test.go` — stub tests for config load/save/merge
- [ ] `internal/drm/drm_test.go` — stub tests for Widevine path resolution

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Config auto-generation on missing file | USAB-02 | Side effect (filesystem write) | Remove config file, run tool, verify skeleton appears |
| FFmpeg check at startup | USAB-05 | External dep not in CI | Remove ffmpeg from PATH, run tool, verify error message |
| CLI flag > env var > config precedence | USAB-01, USAB-02 | Integration across flag/env/config layers | Set conflicting values at each level, verify final behavior |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
