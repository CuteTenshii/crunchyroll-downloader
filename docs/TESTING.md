<!-- generated-by: gsd-doc-writer -->
# Testing

## Test Framework and Setup

The project uses Go's built-in `testing` package (Go 1.25.0). No external test runner or assertion library is required — all tests use standard `testing.T` methods for control flow and assertions.

No global test setup is needed beyond a working Go installation. Run `make deps` (or `go mod download`) to ensure dependencies are available before executing tests.

## Running Tests

All test commands are available through the `Makefile` and directly via `go test`.

| Command | Description |
|---|---|
| `make test` | Run the full test suite with the race detector enabled (`go test -race -count=1 ./...`) |
| `go test ./...` | Run all tests in all packages |
| `go test ./internal/api/...` | Run tests for a specific package |
| `go test -run TestDoRefreshesTokenOnceAndRetries ./internal/api/...` | Run a single test function by name |
| `go test -v ./...` | Run all tests with verbose (detailed) output |
| `make coverage` | Run tests with coverage profiling and generate an HTML report (`coverage.html`) |
| `make vet` | Run `go vet ./...` for static analysis |
| `make lint` | Run `golangci-lint` across the project |
| `make ci` | Run vet, tests, and lint sequentially (simulates CI pipeline) |

### Race Detector

All tests in CI and the `make test` target run with `-race` enabled. This detects data races at runtime. When writing tests locally, it is recommended to include `-race` to catch concurrency bugs early.

### Coverage

To generate a coverage report:

```bash
make coverage
```

This produces `coverage.html` in the project root, which can be opened in a browser. The raw coverage profile is written to `coverage.out`.

To view per-function coverage in the terminal:

```bash
go tool cover -func=coverage.out
```

## Writing New Tests

### File Naming Convention

Test files follow Go's convention: `*_test.go` placed alongside the source file they test. Examples from the project:

| Source | Test file |
|---|---|
| `internal/api/client.go` | `internal/api/client_test.go` |
| `internal/config/config.go` | `internal/config/config_test.go` |
| `internal/download/episode.go` | `internal/download/episode_test.go` |
| `main.go` | `main_test.go` |

Test functions use CamelCase names starting with `Test`:

```go
func TestProcessURLRejectsInvalidContentIDLength(t *testing.T) {
    // ...
}
```

### Package Choice

Tests use **same-package** (`package api`, `package config`) or **external-package** (`package main` for `main_test.go`) conventions depending on the testing needs. The `main_test.go` file uses `package main` (external-package style) to test exported entry points.

### Test Helpers

The project provides several test utility patterns:

- **`testutil/factories.go`** — Factory functions that create realistic test data (episodes, seasons, MPD manifests):
  - `testutil.EpisodeInfo()` — Returns a minimal `*api.EpisodeInfo` with default values
  - `testutil.EpisodeInfoWithVersions(locales ...string)` — Episode info with dub version overrides
  - `testutil.SeasonEpisode(episodeNumber, seriesTitle, seasonNumber)` — A single season episode
  - `testutil.DummyMPD()` — A complete `*mpd.MPD` with video and audio adaptation sets

- **Internal HTTP test servers** — Tests in `internal/api/` use `net/http/httptest.NewServer` to mock Crunchyroll API endpoints:

  ```go
  server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      switch r.URL.Path {
      case "/auth/v1/token":
          w.Header().Set("Content-Type", "application/json")
          _, _ = io.WriteString(w, `{"access_token":"test-token"}`)
      // ...
      }
  }))
  defer server.Close()
  ```

- **`api.NewTestClient()`** — Constructs a test API client pointed at a mock server URL (used in `main_test.go`).

- **Output capture helpers** (`main_test.go`):
  - `captureMainStderr(t, fn)` — Captures stderr output during a function call
  - `captureMainStdout(t, fn)` — Captures stdout output during a function call

- **Table-driven tests** — Used extensively for testing functions with multiple input/output cases:

  ```go
  tests := []struct {
      name  string
      input string
      want  []string
  }{
      {name: "single locale", input: "ja-JP", want: []string{"ja-JP"}},
      {name: "empty string", input: "", want: nil},
  }
  for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
          got := parseLangs(tt.input)
          if !reflect.DeepEqual(got, tt.want) {
              t.Fatalf("parseLangs(%q) = %v, want %v", tt.input, got, tt.want)
          }
      })
  }
  ```

### Test Data Fixtures

JSON and MPD test fixtures are stored in `testdata` directories:

- `internal/api/testdata/api/` — JSON response fixtures for API responses:
  - `episode-info-response.json`
  - `episode-playback-response.json`
  - `license-response.json`
  - `season-list-response.json`
- `internal/media/testdata/mpd/` — MPD manifest fixtures:
  - `simple-video-audio.mpd`
  - `multi-audio.mpd`
  - `with-content-protection.mpd`
  - `no-content-protection.mpd`

## Coverage Requirements

No minimum coverage thresholds are currently configured in CI or in any configuration file. Coverage data is collected and uploaded as a CI artifact for manual review, but the pipeline does not enforce a coverage gate.

Coverage is measured at the package level using Go's `-covermode=atomic` mode. The `make coverage` command produces both a raw profile (`coverage.out`) and an HTML report (`coverage.html`) for local review.

## CI Integration

Tests run in CI via the **CI** workflow (`.github/workflows/ci.yml`).

**Triggers:**
- Push to the `main` branch
- Pull requests targeting the `main` branch

**Test-related steps (in order):**

1. `go mod tidy` — Verify `go.mod` and `go.sum` are clean (fails if they differ from committed versions)
2. `go vet ./...` — Static analysis
3. `golangci-lint` — Linting via the `golangci/golangci-lint-action@v6` (continues on error — does not block the workflow)
4. `go test -race -count=1 ./...` — Full test suite with race detection
5. `go test -coverprofile=coverage.out -covermode=atomic ./...` — Coverage profile generation
6. `go tool cover -func=coverage.out` — Display per-function coverage in CI output
7. Upload `coverage.out` as a build artifact

Linting is set to `continue-on-error: true`, so lint warnings do not block PRs — they are informational. Test failures and vet errors, however, will fail the CI run.
