<!-- generated-by: gsd-doc-writer -->
# Development

This document describes how to set up a development environment, run builds, follow code style, and submit changes for the Crunchyroll Downloader project.

---

## Local Setup

1. **Fork and clone** the repository:
   ```bash
   git clone https://github.com/<your-username>/crunchyroll-downloader.git
   cd crunchyroll-downloader
   ```

2. **Copy the environment file** and fill in your credentials:
   ```bash
   cp .env.example .env
   ```
   See [CONFIGURATION.md](CONFIGURATION.md) for details on each variable.

3. **Verify dependencies** — the project requires Go 1.25+ and FFmpeg:
   ```bash
   make deps
   ```
   This checks that `go` and `ffmpeg` / `ffprobe` are available on `$PATH`.

4. **Build the binary** (optional, the first run step will also build):
   ```bash
   make build
   ```

The built binary is placed at `dist/crunchyroll-downloader`.

---

## Build Commands

| Command | Description |
|---|---|
| `make build` | Tidy Go modules and compile the binary to `dist/crunchyroll-downloader` |
| `make run ARGS="--url <URL>"` | Build and execute with the given arguments |
| `make deps` | Verify that Go, FFmpeg, and FFprobe are installed |
| `make clean` | Remove the `dist/` build directory |
| `make test` | Run all tests with the race detector enabled |
| `make coverage` | Run tests with coverage profiling and generate an HTML report |
| `make vet` | Run `go vet` on all packages |
| `make lint` | Run `golangci-lint` (requires `golangci-lint` to be installed) |
| `make ci` | Run vet, test, and lint sequentially (simulates the CI pipeline) |

---

## Code Style

The project uses the following tools to enforce code quality:

- **go vet** — Static analysis for suspicious constructs. Run manually with `make vet` or `go vet ./...`.
- **golangci-lint** — Meta-linter configured at `.golangci.yml` in the project root. Enables:
  - `govet`, `staticcheck`, `ineffassign`, `unused`, `errorlint`, `gosimple`
  - Run with `make lint` or `golangci-lint run ./... --timeout=5m`.
- **gofmt** (implicit) — Standard Go formatting. CI does not enforce it explicitly, but all Go code should follow `gofmt` conventions.

CI runs `go vet` and `golangci-lint` on every push and pull request to `main`.

---

## Branch Conventions

- The default branch is `master`.
- No formal branch naming convention is documented. Contributors are encouraged to use descriptive names (e.g., `fix/retry-backoff`, `feat/multi-audio`).

---

## PR Process

1. Create a feature branch from `main`:
   ```bash
   git checkout -b my-feature-branch
   ```
2. Make your changes and commit with descriptive messages.
3. Ensure the full test suite passes:
   ```bash
   make ci
   ```
4. Push your branch and open a pull request against `main`.
5. CI automatically runs `go vet`, `golangci-lint`, tests with the race detector, and coverage on the PR.
6. Reviewers will check for:
   - Correctness and test coverage
   - Code style compliance
   - No regressions in the test suite

There is no formal pull request template — include a summary of changes and any relevant context in the PR description.
