# Phase 5: Testing & Quality - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-09
**Phase:** 5-Testing & Quality
**Areas discussed:** Test scope & coverage targets, Mocking & testability refactors, Test data & fixture management

---

## Test Scope & Coverage Targets

| Option | Description | Selected |
|--------|-------------|----------|
| 60% with pure-logic first | Target 60% line coverage overall. Unit tests for pure functions first. Integration tests for API patterns. Lower coverage OK for FFmpeg mux paths. | |
| 70% with integration focus | Target 70%. More investment in httptest-based integration tests for processURL, episode/season flows. Accept lower coverage for exec.Command paths. | |
| Risk-based no blanket % | No universal target. 90%+ on parsing/validation, 60% on orchestrators, skip pure FFmpeg paths. Document rationale per package. | ✓ |

**User's choice:** Risk-based no blanket %
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| media + locale packages | internal/media/ (manifest, segment, mpd logic) and internal/locale/ (lang parsing). Pure logic, highest unit-test ROI. | |
| media + download packages | internal/media/ for unit tests, internal/download/ (episode, season orchestrators) for integration tests with httptest. | |
| Agents decide per package | Review each package boundary during planning. Researcher identifies testability gaps per file, planner defines per-package targets. | ✓ |

**User's choice:** Agents decide per package
**Notes:** Per-package targets delegated to planner/researcher.

| Option | Description | Selected |
|--------|-------------|----------|
| Table-driven tests (stdlib) | Go-idiomatic: struct-based test cases with t.Run subtests. No external dependencies. | ✓ |
| testify suite + assert | Use testify/assert and testify/suite for richer assertions. Adds dependency but reduces boilerplate. | |

**User's choice:** Table-driven tests (stdlib)
**Notes:** None

---

## Mocking & Testability Refactors

| Option | Description | Selected |
|--------|-------------|----------|
| Interface injection for HTTP | Define HTTP client interface, inject via constructor parameter. Use httptest.NewServer for tests. | |
| Refactor all untestable patterns | Also fix global token/keys vars, replace os.Exit, add FFmpeg interface. Higher cost but full testability. | |
| Mock at process boundary | Minimal refactoring — httptest for HTTP, exec.Command test helper for FFmpeg, recover/refactor for os.Exit. | ✓ |

**User's choice:** Mock at process boundary
**Notes:** Minimal source changes, maximum testability gain.

| Option | Description | Selected |
|--------|-------------|----------|
| Wrap in test helper | Use exec.Command test helper pattern that catches os.Exit. | |
| Refactor to return error | Replace os.Exit with error return and let main() handle exit. Small change, clean testability. | ✓ |

**User's choice:** Refactor to return error
**Notes:** Small source change with high testability payoff.

| Option | Description | Selected |
|--------|-------------|----------|
| Test helper exec wrapper | Create test helper that replaces exec.Command with a fake that validates args. No source refactoring. | |
| Interface injection | Define FFmpegRunner interface, inject into mux. Refactor for clean testability. | ✓ |

**User's choice:** Interface injection
**Notes:** Enables testing failure paths (FFmpeg crash, wrong args).

---

## Test Data & Fixture Management

| Option | Description | Selected |
|--------|-------------|----------|
| testdata/ directory per package | Go convention: testdata/ in each package. Subdirs by type: testdata/mpd/, testdata/api/. | ✓ |
| Single testdata/ at project root | All fixtures in root testdata/. Organized by source type. Simpler path resolution. | |

**User's choice:** testdata/ directory per package
**Notes:** Go convention, organized by source type subdirectories.

| Option | Description | Selected |
|--------|-------------|----------|
| Full-response fixtures | Capture real API responses as complete fixtures. Closer to real data. | ✓ |
| Minimal focused fixtures | Minimal snippets with only fields the specific test exercises. Easier to maintain. | |

**User's choice:** Full-response fixtures
**Notes:** Capture real API responses as complete JSON structures.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, testutil package | Create testutil/ with helpers to build dummy Episode, Season, MPD structs. Reduces boilerplate. | ✓ |
| No, inline data only | Construct test data inline or load from JSON. Simpler but more duplication. | |

**User's choice:** Yes, testutil package
**Notes:** Factory helpers for programmatic test data construction.

---

## the agent's Discretion

- CI Pipeline & Tooling (not discussed) — agents to select tools and trigger strategy.
- Test runner setup (Makefile vs go test directly) — agents decide.
- Specific per-file coverage targets — left to planner/researcher.

## Deferred Ideas

None — CI pipeline was not discussed but intentionally left to agent discretion.
