# Testing Patterns

**Analysis Date:** 2026-07-08

## Test Framework

**Status: No tests exist in this codebase.**

**Runner:** Not configured — no test framework, test runner config, or test files detected.

**Assertion Library:** Not applicable.

**Configuration files checked (not found):**
- `*_test.go` — no test files found via `glob`
- `jest.config.*`, `vitest.config.*` — not applicable (Go project)
- No `Makefile` with test targets
- No CI step for running tests (see `release.yml` — only `go build`)

## Test File Organization

**Location:** No test files exist. By Go convention, tests should be placed:
- Co-located in the same package (`package main`) for white-box tests: `main_test.go`, `download_test.go`, `episode_test.go`, etc.
- Files should follow the `*_test.go` naming pattern.

**Structure:** Not applicable — no tests present.

## Test Structure

**Not applicable** — no existing tests to analyze.

**Recommended patterns (Go standard):**
```go
// Unit test for a pure function
func TestParseLangs(t *testing.T) {
    tests := []struct {
        name string
        input string
        expected []string
    }{
        {"single locale", "ja-JP", []string{"ja-JP"}},
        {"multiple locales", "ja-JP,en-US", []string{"ja-JP", "en-US"}},
        {"with whitespace", " ja-JP , en-US ", []string{"ja-JP", "en-US"}},
        {"empty input", "", []string{}},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := parseLangs(tt.input)
            if !reflect.DeepEqual(got, tt.expected) {
                t.Errorf("parseLangs(%q) = %v, want %v", tt.input, got, tt.expected)
            }
        })
    }
}
```

## Mocking

**Framework:** Not used — no tests exist.

**What would need mocking:**
- HTTP responses from Crunchyroll API (`auth/v1/token`, `playback/v3/...`, `content/v2/cms/...`, `license/v1/license/widevine`)
- Widevine CDM device files (`.wvd`, `client_id.bin`, `private_key.pem`)
- FFmpeg execution (`exec.Command("ffmpeg", ...)`)
- File system operations (`os.Create`, `os.Open`, `os.ReadDir`, `os.Remove`)

**Recommended approach:**
- Interface-based HTTP client injection (currently hardcodes `http.DefaultClient` and raw `&http.Client{}`)
- Use `httptest.NewServer` for HTTP mocking in integration tests
- Consider `testify/mock` or interface-based stubs for Widevine/FFmpeg interactions

## Fixtures and Factories

**Not applicable** — no test fixtures exist.

**Recommended fixture location:**
- `testdata/` directory at project root (Go convention)
- Possible fixtures: sample MPD manifests, sample episode JSON responses, sample Widevine challenge/response data

## Coverage

**Requirements:** None enforced — no CI coverage step, no coverage badge.

**To view coverage once tests exist:**
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Test Types

**Unit Tests:**
- Current state: None
- Good candidates for unit testing (pure functions with no I/O):
  - `parseLangs()` — `main.go:24`
  - `buildUrl()` — `download.go:22`
  - `getFilename()` — `download.go:72`
  - `expandTimeline()` — `mpd.go:74`
  - `trackTitle()` — `output.go:18`
  - `parseLangs()` branching

**Integration Tests:**
- Current state: None
- Would require HTTP mocking for Crunchyroll API endpoints
- `DoRequest()` auth refresh logic needs testing

**E2E Tests:**
- Not applicable for this CLI tool (requires live Crunchyroll account)

## Common Patterns to Establish

**No existing test patterns to follow.** The codebase needs initial test infrastructure. Recommended starting point:

1. Add a `Makefile` with a `test` target:
```makefile
.PHONY: test
test:
    go test -v -race -count=1 ./...
```

2. Add `go vet` and linting to CI alongside tests.

3. Use table-driven tests (Go idiomatic) for pure functions.

## Untestable Patterns to Address

**Hard dependencies that complicate testing:**
| Pattern | File | Issue |
|---------|------|-------|
| Global `token` variable | `main.go:12` | Mutable state shared across packages |
| Global `keys` slice | `drm.go:18` | Mutable state mutated by side-effect |
| `http.DefaultClient` usage | multiple files | Cannot inject mock in tests |
| Direct `panic(err)` calls | multiple files | Cannot recover errors gracefully in tests |
| `os.Exit(1)` in non-main function | `episode.go:52` | Terminates test process |
| `exec.Command("ffmpeg", ...)` | `output.go:91` | Requires system FFmpeg binary |
| File-based Widevine device detection | `drm.go:79` | Depends on filesystem state |

## Priority Areas for Test Coverage

**High priority (pure logic, no I/O):**
- `parseLangs` in `main.go` — string splitting and whitespace trimming
- `buildUrl` in `download.go` — URL template substitution
- `expandTimeline` in `mpd.go` — segment timeline expansion logic
- `getFilename` in `download.go` — adaptation set type detection
- `trackTitle` in `output.go` — locale to display name mapping

**Medium priority (I/O boundaries, auth logic):**
- `DoRequest` in `http_request.go` — token refresh on 401
- `deleteStream` in `episode.go` — HTTP DELETE success detection
- `getBaseUrl` in `mpd.go` — quality matching logic

---

*Testing analysis: 2026-07-08*
