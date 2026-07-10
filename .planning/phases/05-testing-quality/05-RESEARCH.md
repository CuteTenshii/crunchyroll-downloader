# Phase 5: Testing & Quality - Research

**Researched:** 2026-07-09
**Domain:** Go test infrastructure, coverage analysis, CI pipeline
**Confidence:** HIGH

## Summary

The project has already undergone significant refactoring from a flat `package main` into 8 internal packages (`api/`, `config/`, `download/`, `drm/`, `locale/`, `media/`, `mux/`, `output/`). Fourteen test files exist with established patterns (table-driven tests, `httptest`, `fakeDoer` interface, `exec.Command` test helper, stdout/stderr capture). However, **no `testdata/` directories exist**, the `internal/locale/` package has zero test files, and several critical pure functions remain untested (`parseLangs`, `BuildUrl`, `ExpandTimeline`, `ParseManifest`).

Current aggregate coverage is **47.5%**. The main untested areas are: `internal/locale/` (0 files), `main.go` pure functions (parseLangs at 0%, resolveString at 0%, isAllNilConfig at 0%), and error-path branches in `internal/download/` and `internal/api/`. The existing test patterns (D-03 table-driven, D-04 httptest, D-06 FFmpegRunner interface injection already implemented via `ffmpegCommand` package var) are mature and should be followed for all new tests.

**Primary recommendation:** Close per-package coverage gaps following existing patterns, add `testdata/` directories with full-response fixtures, implement CI workflow with `go vet`, `golangci-lint`, and coverage reporting. All `os.Exit(1)` refactors (D-05) and `FFmpegRunner` interface injection (D-06) are **already complete** — no source changes needed.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| QOL-01 | Test suite with unit tests for parseLangs, sanitizeFilename, ExpandTimeline, GetBaseUrl, BuildUrl; integration tests for processURL (mock HTTP); coverage target >=60% for internal/media/, internal/download/, internal/locale/ | All functions identified with current coverage gaps. Existing patterns documented. Testability refactors already complete. |

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Risk-based coverage — no blanket percentage target. 90%+ on parsing/validation, ~60% on orchestrators, skip FFmpeg exec paths.
- **D-02:** Per-package coverage targets delegated to planner/researcher.
- **D-03:** Table-driven tests with Go stdlib testing — no testify dependency.
- **D-04:** Mock at process boundary — httptest for HTTP, exec.Command test helper for FFmpeg, refactor os.Exit to error return.
- **D-05:** os.Exit(1) in episode.go → refactor to return error.
- **D-06:** FFmpeg exec.Command → define FFmpegRunner interface, inject into mux.
- **D-07:** testdata/ directories per package (Go convention).
- **D-08:** Full-response fixtures — capture real API responses as complete JSON structures.
- **D-09:** testutil package with factory/helper functions for constructing dummy structs.

### the agent's Discretion
- CI Pipeline & Tooling — agents select Go tooling, trigger strategy.
- Test runner setup (Makefile target vs go test directly).
- Specific per-file coverage targets per D-02.

### Deferred Ideas (OUT OF SCOPE)
- CI pipeline design (go vet, golangci-lint, coverage reporting, triggers) — agents to decide during planning.
- CRUNCHYROLL_CLIENT_AUTH env var testing — belongs in CI pipeline context, not unit test scope.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Unit tests (pure functions) | Code-level | — | Tests co-located in same package, white-box access to unexported functions |
| Integration tests (HTTP mock) | Code-level | — | httptest.NewServer replaces Crunchyroll API endpoints, tests flow through internal/api |
| FFmpeg exec tests | Code-level | — | exec.Command test helper pattern (already implemented) replaces real FFmpeg |
| Coverage reporting | CI pipeline | — | `go test -coverprofile` runs in CI, outputs to job summary or coverage service |
| Linting | CI pipeline | — | golangci-lint runs in CI, blocks on new issues |
| go vet | CI pipeline | — | Built-in Go tool, runs before lint in CI |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go `testing` | stdlib (1.25) | Test framework | D-03 locked decision, no external deps |
| `net/http/httptest` | stdlib (1.25) | HTTP mock server | D-04 locked decision |
| `go test` | stdlib (1.25) | Test runner | No external dep needed, D-03 |
| `go vet` | stdlib (1.25) | Static analysis | Built-in, zero config |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golangci-lint` | v2.0+ | Multi-linter runner | CI pipeline — but verify install (currently absent) |
| `go tool cover` | stdlib | Coverage HTML reporting | Local dev, CI artifact |
| `github.com/wacul/ptr` | external | Pointer helpers for test data | Optional — reduces boilerplate in test factories |
| `gotest.tools/v3/golden` | external | Golden file testing | Optional — for full-response fixture comparison |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| golangci-lint | staticcheck alone | golangci-lint bundles staticcheck, govet, ineffassign, etc. One config file instead of many tools |
| go test directly | Makefile target | Both valid — Makefile target adds discoverability (`make test`, `make coverage`) |
| golden files for fixtures | Manual inline structs | Golden files are easier to update (`-update` flag) but less explicit. Use manual structs for unit tests, golden for full API response integration tests |

**Installation:**
```bash
# golangci-lint (for CI)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# No other test dependencies (stdlib only per D-03)
```

**Version verification:**
```bash
go version          # go version go1.25.0 linux/amd64
golangci-lint version  # (check after install)
```

## Package Legitimacy Audit

> **Required** whenever this phase installs external packages.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| golangci-lint | N/A (Go tool) | ~7 yrs | 1.6B+ total | github.com/golangci/golangci-lint | [OK] | Approved for CI |
| github.com/wacul/ptr | Go module | ~6 yrs | moderate | github.com/wacul/ptr | [OK] | Optional — planner may skip |
| gotest.tools/v3/golden | Go module | ~7 yrs | high | gotest.tools/gotest.tools | [OK] | Optional — planner may skip |

**Packages removed due to [SLOP] verdict:** None
**Packages flagged as suspicious [SUS]:** None

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        Test Suite Overview                       │
│                                                                  │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────────┐    │
│  │ Unit Tests    │   │ Integration  │   │ CI Pipeline       │    │
│  │ (pure funcs)  │──▶│ Tests (HTTP) │   │ (GitHub Actions)  │    │
│  └──────┬───────┘   └──────┬───────┘   └────────┬─────────┘    │
│         │                  │                     │              │
│         ▼                  ▼                     ▼              │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────────┐    │
│  │ internal/*   │   │ internal/api │   │ go test => vet   │    │
│  │ whitebox     │   │ httptest.New │   │ => lint => cov   │    │
│  │ table-driven │   │ Server mock  │   │ => report        │    │
│  │ t.Run        │   │ API responses│   │                  │    │
│  └──────────────┘   └──────────────┘   └──────────────────┘    │
│                                                                  │
│  ┌──────────────┐   ┌──────────────┐                             │
│  │ Test Helpers │   │ Fixtures     │                             │
│  │ internal/*/  │──▶│ testdata/    │                             │
│  │ *_test.go    │   │ (per pkg)    │                             │
│  └──────────────┘   └──────────────┘                             │
└─────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure (Test Additions Only)
```
crunchyroll-downloader/
├── internal/
│   ├── locale/
│   │   └── locale_test.go       # NEW — tests for LanguageNames, LanguageCodes
│   ├── media/
│   │   ├── testdata/             # NEW
│   │   │   ├── mpd/             # full MPD XML fixtures
│   │   │   └── segment/         # sample init/media segments
│   │   └── manifest_test.go     # EXTEND — ExpandTimeline, ParseManifest, BuildUrl
│   ├── api/
│   │   ├── testdata/             # NEW
│   │   │   └── api/             # full JSON responses (episode, season, auth)
│   │   └── client_test.go       # EXTEND — FetchManifest, GetEpisode, DeleteStream
│   ├── config/
│   │   └── config_test.go       # EXTEND — LoadDotenv, parseDotenv tests
│   ├── download/
│   │   ├── testdata/             # NEW
│   │   └── episode_test.go      # EXTEND — formatFileSize, formatDuration
│   └── mux/
│       └── mux_test.go           # EXTEND — TrackTitle, additional error paths
├── main_test.go                  # EXTEND — parseLangs, resolveString, isAllNilConfig
├── testutil/                     # NEW — factory functions per D-09
│   └── factories.go
├── .golangci.yml                 # NEW — lint configuration
├── .github/workflows/
│   └── ci.yml                    # NEW — CI pipeline with test+lint+coverage
└── Makefile                      # EXTEND — add test/coverage/lint targets
```

### Pattern 1: Table-Driven Tests (Already Established)
**What:** Struct-based test cases with `t.Run` subtests using Go stdlib.
**When to use:** All pure-function unit tests.
**Example:**
```go
// From internal/download/episode_test.go:141
func TestSanitizeFilenameCollapsesMultiUnderscore(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string
    }{
        {name: "single underscore unchanged", input: "a_b", want: "a_b"},
        {name: "double underscore collapses", input: "a__b", want: "a_b"},
        {name: "empty returns Unknown", input: "", want: "Unknown"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := sanitizeFilename(tt.input)
            if got != tt.want {
                t.Fatalf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

### Pattern 2: httptest for HTTP Mocking (Already Established)
**What:** `httptest.NewServer` with per-endpoint routing.
**When to use:** All API integration tests.
**Example:**
```go
// From internal/api/client_test.go:49
func TestDoRefreshesTokenOnceAndRetries(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/auth/v1/token":
            w.Header().Set("Content-Type", "application/json")
            io.WriteString(w, `{"access_token":"refreshed-token"}`)
        case "/resource":
            http.Error(w, "expired", http.StatusUnauthorized)
        }
    }))
    defer server.Close()
    client := newTestClient(server.URL)
    // ... test body
}
```

### Pattern 3: fakeDoer Interface (Already Established)
**What:** Minimal `httpDoer` interface with function-based fake.
**When to use:** When testing functions that accept an HTTP interface.
**Example:**
```go
// From internal/media/segment_test.go:16
type fakeDoer func(*http.Request) (*http.Response, error)
func (f fakeDoer) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestDownloadPartUsesInjectedClient(t *testing.T) {
    client := fakeDoer(func(req *http.Request) (*http.Response, error) {
        return okResponse("segment-data"), nil
    })
    data, err := DownloadPart(context.Background(), client, "https://media.example/segment.m4s")
    // ... assertions
}
```

### Pattern 4: exec.Command Test Helper (Already Established)
**What:** Replace `ffmpegCommand` package var with test helper that execs test binary.
**When to use:** FFmpeg exec-path testing in `internal/mux/`.
**Example:**
```go
// From internal/mux/mux_test.go:77
func restoreFFmpegCommand(t *testing.T, exitCode, stderr string) {
    original := ffmpegCommand
    ffmpegCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
        cs := []string{"-test.run=TestHelperProcess", "--", command}
        cs = append(cs, args...)
        cmd := exec.CommandContext(ctx, os.Args[0], cs...)
        cmd.Env = append(os.Environ(),
            "GO_WANT_HELPER_PROCESS=1",
            "GO_HELPER_EXIT_CODE="+exitCode,
            "GO_HELPER_STDERR="+stderr,
        )
        return cmd
    }
    t.Cleanup(func() { ffmpegCommand = original })
}
```

### Anti-Patterns to Avoid
- **Using testify/assert:** Locked by D-03 — use `t.Fatalf`/`t.Errorf` with descriptive messages instead.
- **Hand-rolling HTTP clients in tests:** Use `httptest.NewServer` — never create test fixtures that make real network calls.
- **Parallel test corruption:** Tests that modify `ffmpegCommand` or `widevineDeviceLoader` package vars must use `t.Cleanup` to restore originals. Tests that modify `os.Stdout`/`os.Stderr` must use `os.Pipe` and restore. Tests that call `t.Chdir` must use `t.Cleanup` to restore CWD.
- **Testing with real credentials:** Never use real `etp_rt` cookies — use `httptest.NewServer` and `NewTestClient` instead.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP mocking | Custom http.Client | httptest.NewServer | stdlib, well-tested, realistic |
| FFmpeg exec testing | os/exec wrappers | exec.Command test helper pattern | Already implemented in mux_test.go |
| Test data construction | Inline structs everywhere | testutil factories | D-09, reduces boilerplate |
| Gold file comparison | Manual diff | gotest.tools/v3/golden | (Optional) cleaner update workflow |
| CI pipeline | Self-hosted CI | GitHub Actions | Already has `.github/workflows/`, free for public repos |

**Key insight:** The project already has all the test patterns it needs. The biggest gap is **coverage of pure functions** in `internal/locale/` (0 files) and `main.go` (parseLangs at 0%). No new test infrastructure is needed — just write the tests following established patterns.

## Coverage Analysis

### Per-Package Current State
| Package | Current | Target | Gap | Strategy |
|---------|---------|--------|-----|----------|
| `internal/output` | 86.7% | ~90% | ✓ Near target | Minor edge cases (clampETA, min) |
| `internal/mux` | 76.0% | ~80% | ✓ Near target | TrackTitle, additional Merge paths |
| `internal/media` | 55.0% | 90%+ | 35% | ExpandTimeline, BuildUrl, ParseManifest, formatSpeed, formatETAShort, createTempFilename |
| `internal/api` | 50.8% | 60%+ | 9% | FetchManifest, GetEpisode, DeleteStream, GetSeasonEpisodes |
| `internal/drm` | 47.4% | 60% | 13% | GetPssh, GetLicense, loadWidevineDevice error paths |
| `internal/config` | 42.7% | 90%+ | 47% | LoadDotenv, parseDotenv — high priority per D-01 |
| `internal/download` | 30.8% | 60% | 29% | formatFileSize, formatDuration, Season wrapper |
| `internal/locale` | 0% (no file) | 90%+ | 90% | LanguageNames map coverage, LanguageCodes map coverage |
| `main` (root) | 23.8% | 60%+ | 36% | parseLangs (0%), resolveString (0%), isAllNilConfig (0%) |

### Easy Wins (High Coverage ROI, Low Effort)
1. `parseLangs()` — table-driven test, pure string function
2. `BuildUrl()` — table-driven test, pure string interpolation
3. `ExpandTimeline()` — table-driven with various S element patterns
4. `formatFileSize()` — table-driven with byte boundary values
5. `formatDuration()` — table-driven with time edge cases
6. `LanguageNames`/`LanguageCodes` maps — two simple map lookup tests
7. `LoadDotenv()`/`parseDotenv()` — temp file-based tests
8. `resolveString()` — table-driven with precedence hierarchy
9. `isAllNilConfig()` — table-driven with nil/non-nil field combinations

### Medium Difficulty
1. `ParseManifest()` — needs minimal valid MPD XML fixture
2. `GetPssh()` — needs MPD with/without ContentProtection
3. `GetLicense()` — needs mock `client.SendChallenge`, Widevine device
4. `FetchManifest()` — already tested indirectly via other tests
5. `Season()` — delegates to `runSeason`, which is already tested
6. `processURL()` — integration test with httptest covering watch/series/dispatch paths

### Hard (Skip per D-01)
1. `main()` — flag parsing, env reading, complex orchestration
2. `DownloadParts()` — full end-to-end with real encrypted segments
3. `GetLicense()` — full path requiring Widevine CDM
4. `downloadEpisode()` — full integration with real download flow

## Fixture and Test Data Plan

### testdata/ Structure
```
internal/media/testdata/
├── mpd/
│   ├── simple-video-audio.mpd       # minimal MPD with one video + one audio AdaptationSet
│   ├── multi-audio.mpd               # MPD with 3 audio versions (192k, 128k, 96k)
│   ├── with-content-protection.mpd   # MPD with Widevine PSSH data
│   └── no-content-protection.mpd     # MPD without any ContentProtection

internal/api/testdata/
├── api/
│   ├── episode-info-response.json    # Full GetEpisodeInfo response
│   ├── season-list-response.json     # Full GetSeasons response
│   ├── episode-playback-response.json# Full GetEpisode playback response
│   └── license-response.json         # Full SendChallenge response

internal/download/testdata/
└── fixtures.go                       # Helper functions (no testdata dir needed if using factories)
```

### Fixture Strategy (per D-08)
- **Full-response fixtures:** Capture real API responses as JSON files in `testdata/api/`. These are actual (sanitized) responses from Crunchyroll's API. Use `os.ReadFile` + `json.Unmarshal` in tests to load them.
- **MPD fixtures:** Create minimal but valid MPD XML files that exercise specific parsing paths (with/without PSSH, single/multiple representations, various bandwidth values).
- **No large binary fixtures:** Segment data and Widevine challenge/response are too large to commit. Use `testutil` factory functions for those (D-09).

### testutil Package (per D-09)
```go
// testutil/factories.go
package testutil

import "crunchyroll-downloader/internal/api"

func EpisodeInfo() *api.EpisodeInfo {
    return &api.EpisodeInfo{
        EpisodeMetadata: api.EpisodeMetadata{
            SeriesTitle:   "Test Series",
            SeasonNumber:  1,
            EpisodeNumber: 1,
            AudioLocale:   "ja-JP",
        },
        Title: "Test Episode",
    }
}

func EpisodeInfoWithVersions(locales ...string) *api.EpisodeInfo {
    info := EpisodeInfo()
    for _, l := range locales {
        info.EpisodeMetadata.Versions = append(info.EpisodeMetadata.Versions, &api.DubVersion{
            AudioLocale: l,
            GUID:        l + "-guid",
        })
    }
    return info
}
```

## D-05 and D-06 Refactor Status

### D-05: os.Exit(1) Refactor — ALREADY COMPLETE
- `internal/download/episode.go` — **No `os.Exit` calls found.** The function returns `error` throughout.
- `internal/download/season.go` — **No `os.Exit` calls.** Returns `error`.
- `main.go` — `os.Exit(1)` is only in `func main()` (lines 268, 319, 335, 341, 348, 367) — these are correct per Go convention (only `main()` should call `os.Exit`).
- **Verdict: No refactoring needed.** The decision D-05 was already implemented.

### D-06: FFmpegRunner Interface — ALREADY COMPLETE
- `internal/mux/mux.go:20` — `var ffmpegCommand = exec.CommandContext` is a package-level variable that tests replace via `restoreFFmpegCommand`.
- `internal/mux/mux_test.go:77` — the `restoreFFmpegCommand` helper cleanly replaces it.
- **Verdict: No refactoring needed.** The replacement pattern is already in place. The interface is implicit (`func(context.Context, string, ...string) *exec.Cmd`). An explicit `FFmpegRunner` interface is not needed — the existing pattern works for all test scenarios.

## Common Pitfalls

### Pitfall 1: Package-Level State Pollution Between Tests
**What goes wrong:** Tests that modify `ffmpegCommand`, `widevineDeviceLoader`, `manifestCache`, or `os.Stdout`/`os.Stderr` leak state to other tests, causing order-dependent failures.
**Why it happens:** Go tests within the same package share package-level variables. Tests clean up only if explicitly programmed to.
**How to avoid:** Always use `t.Cleanup(func() { /* restore original */ })` after modifying package-level state. The existing tests in `drm_test.go` and `mux_test.go` demonstrate this pattern correctly.
**Warning signs:** Tests pass in isolation (`go test -run TestName`) but fail in a batch (`go test ./package`).

### Pitfall 2: httptest Server Leakage
**What goes wrong:** httptest.NewServer creates a goroutine that leaks if not closed.
**Why it happens:** The `server.Close()` deferred call may be skipped on `t.Fatal`.
**How to avoid:** Always `defer server.Close()` immediately after creating the server. Use `t.Cleanup(server.Close)` for safety.
**Warning signs:** Test suite memory growth, goroutine leaks in `go test -race`.

### Pitfall 3: Flaky SpeedTracker Tests
**What goes wrong:** `SpeedTracker` tests that depend on `time.Sleep` sometimes fail on CI due to timing variability.
**Why it happens:** Tests use real time to simulate download speed patterns. CI runners may be slower than dev machines.
**How to avoid:** Use `t.Skip` when Bps is zero (unmeasurable), not `t.Error`. The existing tests in `output_test.go` already use `t.Skipf` for this case. Structure speed tests to be tolerant of timing variance (wide tolerances, `t.Logf` for observation, not `t.Errorf` for strict values).

### Pitfall 4: Cross-Package Test Client Construction
**What goes wrong:** Tests in `internal/download/` need an `*api.Client` but `api.New()` requires a real `etp_rt` cookie and makes a real HTTP call.
**Why it happens:** The constructor has side effects (token fetch).
**How to avoid:** Use `api.NewTestClient(nil, url, token)` which bypasses auth. Already available at `internal/api/client_helper.go:13`.
**Warning signs:** Tests that hang or fail with "etp_rt cookie is required".

### Pitfall 5: Test Parallelization with Shared State
**What goes wrong:** Tests that use `t.Chdir(t.TempDir())` affect the CWD for all tests running in parallel.
**Why it happens:** `os.Chdir` changes the process-wide working directory, not a per-goroutine value.
**How to avoid:** Either avoid `t.Chdir` (use absolute paths: `filepath.Join(t.TempDir(), ...)`) or use `t.Parallel()` only in tests that don't change CWD. Current tests in `download/episode_test.go` use `t.Chdir` without `t.Parallel`, which is correct.

## Code Examples

### Test for parseLangs (Table-Driven)
```go
func TestParseLangs(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  []string
    }{
        {name: "single locale", input: "ja-JP", want: []string{"ja-JP"}},
        {name: "multiple locales", input: "ja-JP,en-US", want: []string{"ja-JP", "en-US"}},
        {name: "with whitespace", input: " ja-JP , en-US ", want: []string{"ja-JP", "en-US"}},
        {name: "empty input", input: "", want: nil},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := parseLangs(tt.input)
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("parseLangs(%q) = %v, want %v", tt.input, got, tt.want)
            }
        })
    }
}
```

### Test for BuildUrl (Table-Driven)
```go
func TestBuildUrl(t *testing.T) {
    partNum := int64(5)
    tests := []struct {
        name            string
        base, repID, file string
        partNum         *int64
        want            string
    }{
        {name: "no part number", base: "https://example.com/", repID: "vid-1080", file: "init.mp4", partNum: nil,
            want: "https://example.com/init.mp4"},
        {name: "with part number", base: "https://example.com/", repID: "vid-1080", file: "seg-$Number%05d$.m4s", partNum: &partNum,
            want: "https://example.com/seg-00005.m4s"},
        {name: "representation ID substitution", base: "https://example.com/", repID: "vid-1080", file: "$RepresentationID$/init.mp4", partNum: nil,
            want: "https://example.com/vid-1080/init.mp4"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := BuildUrl(tt.base, tt.repID, tt.file, tt.partNum)
            if got != tt.want {
                t.Errorf("BuildUrl(%q, %q, %q, %v) = %q, want %q", tt.base, tt.repID, tt.file, tt.partNum, got, tt.want)
            }
        })
    }
}
```

### Test for ExpandTimeline (Table-Driven)
```go
func TestExpandTimeline(t *testing.T) {
    rNonZero := uint64(2)
    tests := []struct {
        name        string
        timeline    []*mpd.SegmentTimelineS
        startNumber int64
        want        []int64
    }{
        {name: "single S element", timeline: []*mpd.SegmentTimelineS{{D: 1}},
            startNumber: 1, want: []int64{1}},
        {name: "S element with repeat 2",
            timeline: []*mpd.SegmentTimelineS{{D: 1, R: &rNonZero}},
            startNumber: 1, want: []int64{1, 2, 3}},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ExpandTimeline(tt.timeline, tt.startNumber)
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("ExpandTimeline(%v, %d) = %v, want %v", tt.timeline, tt.startNumber, got, tt.want)
            }
        })
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Flat `package main` with panic() | 8 internal packages with error returns | Phase 1-4 (Jul 2026) | Testability unlocked — functions return errors, interfaces injectable |
| Full HTTP integration | `httptest.NewServer` + `NewTestClient` | Phase 1.6 (Jul 2026) | HTTP tests don't need real credentials |
| No test infrastructure | 14 test files with established patterns | Phase 1.6 (Jul 2026) | Patterns exist to follow |
| os.Exit(1) in download code | error returns only (main.go handles exit) | Phase 1-4 | Tests no longer terminate process |
| FFmpeg hardcoded exec.Command | ffmpegCommand package variable, replaceable in test | Phase 4+ | FFmpeg failure paths testable |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | golangci-lint v2.0+ is available and stable for Go 1.25 | Standard Stack | Minor — fallback is to use `go vet` only and add lint later |
| A2 | GitHub Actions `ubuntu-latest` includes Go 1.25 | CI Pipeline | Low — the release workflow already pins `actions/setup-go@v5` with `go-version: '1.25'` |
| A3 | No `os.Exit(1)` calls remain outside `main()` | D-05 Status | MEDIUM — if found, the planner must add a refactor plan before test writing |

## Open Questions (RESOLVED)

1. **Golden file vs inline fixtures for testdata?** (RESOLVED)
   - What we know: D-08 requires full-response fixtures. `gotest.tools/golden` is standard for golden file testing.
   - What's unclear: Whether to use golden file assertions or load JSON fixtures with manual assertions.
   - Recommendation: Start with manual assertions (simpler, no extra dep). Add golden file support only if fixture count exceeds ~10.
   - Resolution: Plans follow recommendation — manual assertions for testdata fixtures.

2. **testutil package location?** (RESOLVED)
   - What we know: D-09 calls for a testutil package. CONTEXT.md suggests `internal/testutil/`.
   - What's unclear: Whether the root `main` package can import from `internal/testutil/` (white-box test in root package vs cross-package test).
   - Recommendation: `testutil/` at project root (not internal) for maximum visibility. Root-package tests in `main_test.go` can import it as `crunchyroll-downloader/testutil`.
   - Resolution: Plans use project-root `testutil/` per recommendation.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25 | All testing | ✓ | 1.25 | — |
| go vet | CI linting | ✓ (go tool) | 1.25 | — |
| golangci-lint | CI linting | ✗ | — | go vet only (reduce lint scope) |
| ffmpeg | Integration tests | ✗ | — | exec.Command test helper (already in place) |
| GitHub Actions | CI pipeline | ✓ (infra) | — | — |

**Missing dependencies with no fallback:**
- None — `golangci-lint` is optional (vet-only CI is acceptable)

**Missing dependencies with fallback:**
- golangci-lint: Use `go vet` + `go test` only as minimal CI
- ffmpeg: Tests already use the exec.Command test helper pattern

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no external deps per D-03) |
| Config file | None — not needed |
| Quick run command | `go test ./internal/media/ ./internal/locale/ ./internal/config/` |
| Full suite command | `go test -count=1 -race ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| QOL-01 | parseLangs unit test | unit | `go test -run TestParseLangs .` | ❌ Wave 0 |
| QOL-01 | BuildUrl unit test | unit | `go test -run TestBuildUrl ./internal/media/` | ❌ Wave 0 |
| QOL-01 | ExpandTimeline unit test | unit | `go test -run TestExpandTimeline ./internal/media/` | ❌ Wave 0 |
| QOL-01 | GetBaseUrl unit test | unit | `go test -run TestGetVideoBaseUrl ./internal/media/` | ✅ existing |
| QOL-01 | GetAudioBaseUrl unit test | unit | `go test -run TestGetAudioBaseUrl ./internal/media/` | ✅ existing |
| QOL-01 | sanitizeFilename unit test | unit | `go test -run TestSanitizeFilename ./internal/download/` | ✅ existing |
| QOL-01 | processURL integration test | integration | `go test -run TestProcessURL .` | ❌ Wave 0 |
| QOL-01 | locale package tests | unit | `go test ./internal/locale/` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./package/being-modified/` (targeted)
- **Per wave merge:** `go test -count=1 -race ./...` (full suite)
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/locale/locale_test.go` — covers locale map lookups
- [ ] `main_test.go` — parseLangs table-driven tests
- [ ] `internal/media/manifest_test.go` — extend with BuildUrl, ExpandTimeline, ParseManifest
- [ ] `internal/config/config_test.go` — extend with LoadDotenv, parseDotenv
- [ ] `internal/download/episode_test.go` — extend with formatFileSize, formatDuration
- [ ] `internal/mux/mux_test.go` — extend with TrackTitle
- [ ] Framework install: none needed (Go stdlib)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Auth is tested via httptest + Client tests |
| V3 Session Management | no | Token refresh tested in client_test.go |
| V4 Access Control | no | No access control logic in this tool |
| V5 Input Validation | yes | parseLangs, validateURL, sanitizeFilename — 90%+ coverage target per D-01 |
| V6 Cryptography | no | Widevine is external, tested through license_test.go |

### Known Threat Patterns for {stack}
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| URL injection in BuildUrl | Tampering | Table-driven tests with special characters, path traversal patterns |
| Filename injection in sanitizeFilename | Tampering | Table-driven tests with illegal chars, empty string, unicode |

## Sources

### Primary (HIGH confidence)
- [Context7: Go stdlib testing patterns] — table-driven tests, httptest
- [Context7: Go exec.Command test helper pattern] — helper_process pattern used in mux_test.go
- [Official Go testing docs: golang.org/pkg/testing] — go test, coverage, subtests

### Secondary (MEDIUM confidence)
- [Codebase analysis: All .go files read and verified] — current test coverage, existing patterns, refactor status
- [golangci-lint docs: golangci-lint.run] — lint configuration, v2 compatibility

### Tertiary (LOW confidence)
- [GitHub Actions setup-go documentation] — cache, go-version, coverage upload — check specific version compatibility

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All stdlib, no external test deps needed
- Architecture: HIGH - Complete codebase verification performed
- Pitfalls: HIGH - Issues observed in existing test patterns and verified against codebase
- D-05/D-06 status: HIGH - Verified by reading source files; both refactors already complete

**Research date:** 2026-07-09
**Valid until:** 2026-08-09 (30 days for stable tooling)

## Key Findings Summary

1. **D-05 and D-06 are already complete.** No `os.Exit(1)` outside `main()`. The `ffmpegCommand` package var replacement pattern is in place. No refactoring source changes needed.

2. **14 test files exist with mature patterns.** Table-driven tests, httptest, fakeDoer, exec.Command helper, stdout/stderr capture — all established and reusable.

3. **No `testdata/` directories exist.** Per D-07, each package needs `testdata/` with subdirectories by fixture type. Create `internal/media/testdata/mpd/`, `internal/api/testdata/api/`.

4. **`internal/locale/` has zero test files.** This is the top priority gap — pure map lookups, easy 90%+ coverage.

5. **Current aggregate coverage is 47.5%.** Pure functions in `main.go` (parseLangs at 0%, resolveString at 0%) and `internal/media/` (BuildUrl, ExpandTimeline untested) are the highest-ROI targets.

6. **CI needs a new workflow.** Only `release.yml` exists (runs on tag publish). Add `ci.yml` for PR/push trigger with `go test`, `go vet`, optional `golangci-lint`, and coverage reporting.

7. **Makefile needs test/coverage/lint targets.** Currently only build/run/deps/clean targets exist.
