# Phase 5: Testing & Quality - Context

**Gathered:** 2026-07-09
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers comprehensive test coverage across the codebase and CI integration. Unit tests for pure functions (parseLangs, buildUrl, expandTimeline, getFilename), integration tests with mock HTTP servers, and test infrastructure (fixtures, test utilities, mocking patterns). Also completes minimal testability refactors (FFmpeg interface injection, os.Exit removal) needed to unlock test coverage.

</domain>

<decisions>
## Implementation Decisions

### Test Scope & Coverage Targets
- **D-01:** Risk-based coverage — no blanket percentage target. 90%+ on parsing/validation logic, ~60% on orchestrators, skip pure FFmpeg exec paths. Document rationale per package in test files.
- **D-02:** Per-package coverage targets delegated to planner — researcher identifies testability gaps per file, planner defines targets during planning.
- **D-03:** Table-driven tests using Go stdlib `testing` package — no testify dependency. Struct-based test cases with `t.Run` subtests per TESTING.md recommendation.

### Mocking & Testability Refactors
- **D-04:** Mock at process boundary — httptest for HTTP mocking, exec.Command test helper for FFmpeg, refactor os.Exit to error return. Minimal source changes, maximum testability gain.
- **D-05:** os.Exit(1) in episode.go → refactor to return error. Small source change, high testability payoff. Let agents decide the exact refactoring approach.
- **D-06:** FFmpeg exec.Command → define FFmpegRunner interface, inject into mux. Enables testing failure paths (FFmpeg crash, wrong args) and simplifies integration tests.

### Test Data & Fixture Management
- **D-07:** testdata/ directories per package (Go convention). Subdirectories organized by fixture type: testdata/mpd/, testdata/api/, testdata/widevine/.
- **D-08:** Full-response fixtures — capture real API responses as complete JSON structures. Not minimal snippets. Enables testing against realistic data shapes.
- **D-09:** testutil package with factory/helper functions for constructing dummy Episode, Season, MPD structs programmatically. Reduces test boilerplate and insulates tests from struct field changes.

### the agent's Discretion
- CI Pipeline & Tooling (not discussed) — agents should select appropriate Go tooling: go vet, golangci-lint, coverage reporting. CI trigger strategy (PR vs push) left to agents.
- Test runner setup (Makefile target vs go test directly) left to agents.
- Specific per-file test coverage targets left to planner/researcher per D-02.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition and constraints
- `.planning/ROADMAP.md` — Phase 5 scope (plans 5.1/5.2), plan grouping, and requirement mapping.
- `.planning/REQUIREMENTS.md` — QOL-01 definition with acceptance criteria.
- `.planning/PROJECT.md` — Project context, validated requirements, and key decisions.

### Codebase maps
- `.planning/codebase/TESTING.md` — Existing test analysis: untestable patterns map, priority test areas, fixture recommendations, mocking approach guidance.
- `.planning/codebase/CONVENTIONS.md` — Code conventions, naming patterns, error handling patterns.
- `.planning/codebase/STRUCTURE.md` — File organization, directory layout.

### Prior phase context
- `.planning/phases/01-foundation-error-handling-http-memory/01-CONTEXT.md` — Prior error contract decisions (D-02: clean error messages, no stack traces), signal handling, HTTP transport config.
- `.planning/phases/04-user-experience-progress-output/04-CONTEXT.md` — Output abstraction pattern (interface-based, testable via mock), global singleton approach.

### Current implementation touchpoints
- `episode.go` — os.Exit(1) at line 52 (post-refactoring) — needs refactor to return error per D-05.
- `output.go` — exec.Command("ffmpeg", ...) calls — needs FFmpegRunner interface injection per D-06.
- All 10 Go source files — test coverage targets to be defined per D-02.

### No external specs
No external specs — requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Phase 1's error return patterns (all panic() replaced) — enables error-path testing without recover().
- Phase 4's output interface — testable via mock `Output` implementation for verifying log messages in tests.
- httptest from stdlib — no external dependency needed for HTTP mocking.
- sync.Once, sync.Mutex patterns already used in production code — testing patterns for concurrent code exist.

### Established Patterns
- Table-driven tests are recommended in TESTING.md as the Go-idiomatic pattern (D-03).
- Interface-based globals set in main() (Phase 3 config, Phase 4 output) — can be replaced with test implementations.
- Single-package (package main) structure — white-box tests co-located in same package.

### Integration Points
- `episode.go:52` — os.Exit(1) in error path, already returning errors elsewhere post-Phase 1.
- `output.go:91` — exec.Command("ffmpeg") in mergeEverything, needs interface injection.
- `download.go` — worker pool pattern (sync.WaitGroup + channels), test with mock HTTP.
- `http_request.go` — DoRequest with 401 retry logic, test with httptest.

### Creative Options
- Test helper exec wrapper for FFmpeg: replace exec.Command at package level in test files via init() or test helper.
- httptest.NewServer can mock all Crunchyroll API endpoints (auth, playback, CMS, license) for integration tests.

</code_context>

<specifics>
## Specific Ideas

- testutil package location: `internal/testutil/` — factory functions for EpisodeInfo, Season, MPD, DubVersion, etc.
- Table-driven test pattern to follow (from TESTING.md):
  ```go
  func TestParseLangs(t *testing.T) {
      tests := []struct {
          name string
          input string
          expected []string
      }{...}
      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) { ... })
      }
  }
  ```
- FFmpegRunner interface: `type FFmpegRunner interface { Merge(ctx context.Context, ...) error }` — single method, inject into output package.

</specifics>

<deferred>
## Deferred Ideas

- CI pipeline design (go vet, golangci-lint, coverage reporting, triggers) — not discussed, agents to decide during planning.
- `CRUNCHYROLL_CLIENT_AUTH` env var testing — belongs in CI pipeline context, not unit test scope.

</deferred>

---

*Phase: 5-Testing & Quality*
*Context gathered: 2026-07-09*
