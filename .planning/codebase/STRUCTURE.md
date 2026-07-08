# Codebase Structure

**Analysis Date:** 2026-07-08

## Directory Layout

```
crunchyroll-downloader/
├── main.go              # CLI entry point, flag parsing, URL dispatch
├── token.go             # Crunchyroll auth token acquisition
├── season.go            # Season listing and episode enumeration
├── episode.go           # Episode playback metadata and stream management
├── download.go          # Core segment download engine and orchestration
├── mpd.go               # MPD/DASH manifest parsing and representation resolution
├── drm.go               # Widevine DRM license acquisition and decryption
├── output.go            # ffmpeg MKV muxing and track management
├── http_request.go      # Centralized HTTP client with auth retry
├── utils.go             # Language name and code lookup maps
├── go.mod               # Go module definition
├── go.sum               # Dependency checksums
├── CHANGELOG.md         # Release changelog
├── README.md            # Project documentation
├── LICENSE.txt          # License file
├── .gitignore           # Git ignore rules
├── .github/             # GitHub assets
│   ├── screenshots/     # Screenshots for README
│   └── workflows/       # CI/CD workflow definitions
├── .planning/           # GSD planning artifacts
│   └── codebase/        # Codebase analysis documents
├── .opencode/           # AI agent configuration (opencode)
└── .agents/             # AI agent skills directory
```

## Directory Purposes

**Project Root (`./`):**
- Purpose: All Go source files, module definition, and documentation
- Contains: `.go` source files, `go.mod`, `go.sum`, markdown docs
- Key files: `main.go` (entry point), `go.mod` (module: `crunchyroll-downloader`)

**.github/:**
- Purpose: GitHub-specific assets and CI workflows
- Contains: Screenshots, workflow YAML definitions
- Key files: `.github/workflows/` (CI pipeline definitions)

**.planning/:**
- Purpose: GSD (Goal-oriented Software Development) planning artifacts
- Contains: Codebase analysis maps, phase plans, roadmaps
- Key files: `.planning/codebase/ARCHITECTURE.md`, `.planning/codebase/STRUCTURE.md`

## Key File Locations

**Entry Points:**
- `main.go`: CLI application entry (`func main()`, `func processUrl()`)

**Configuration:**
- `go.mod`: Go module declaration and dependency version pins
- `go.sum`: Dependency integrity checksums

**Core Logic:**
- `token.go`: `GetAccessToken()` — Crunchyroll OAuth token exchange
- `season.go`: `getSeasons()`, `getSeasonEpisodes()` — CMS data fetching
- `episode.go`: `getEpisode()`, `getEpisodeInfo()`, `deleteStream()` — playback data
- `download.go`: `downloadEpisode()`, `downloadSeason()`, `downloadParts()`, `downloadSubs()` — download orchestration and segment engine
- `mpd.go`: `parseManifest()`, `getBaseUrl()`, `expandTimeline()` — DASH manifest handling
- `drm.go`: `getLicense()`, `sendChallenge()`, `getPssh()`, `getWidevineDevice()` — DRM decryption
- `output.go`: `mergeEverything()`, `trackTitle()` — ffmpeg MKV muxing
- `http_request.go`: `DoRequest()` — HTTP client with 401 retry
- `utils.go`: `languageNames`, `languageCodes` — locale maps

**Testing:**
- None detected — the project has 0 test files

## Naming Conventions

**Files:**
- `snake_case.go`: Go source files describing their domain (`download.go`, `http_request.go`, `token.go`)
- `UPPERCASE.md`: Documentation files (`README.md`, `CHANGELOG.md`, `LICENSE.txt`)

**Functions:**
- `camelCase`: All exported and unexported functions use camelCase (`getEpisode`, `downloadParts`, `GetAccessToken`, `mergeEverything`)
- Mixed export visibility: Functions starting with uppercase are exported (`GetAccessToken`), lowercase are package-private (`downloadEpisode`, `getSeasons`)

**Variables:**
- `camelCase`: Local and package-level variables (`deviceId`, `etpRt`, `audioLangs`, `maxWorkers`)
- All-caps for constants: Not used — constants use camelCase (e.g., `maxWorkers`)

**Types:**
- `PascalCase`: Struct types (`Episode`, `Season`, `SeasonEpisode`, `EpisodeInfo`, `DubVersion`, `mediaTrack`)
- `PascalCase`: API response wrappers (`CrunchyrollTokenResponse`, `CrunchyrollWidevineLicenseResponse`, `SeasonEpisodes`, `Seasons`, `EpisodeMetadataResponse`)

**Global Variables:**
- Simple camelCase (`token`, `keys`, `deviceId`) — no naming convention to distinguish globals from locals

## Where to Add New Code

**New Feature (e.g., new streaming source):**
- Implement the new source as a new `.go` file in the project root (e.g., `funimation.go`)
- Add any new API data types as structs with JSON tags
- Wire into `processUrl()` in `main.go` if adding new URL pattern support

**New Component/Module:**
- Add a new `.go` file in the root directory for the concern
- Keep domain-specific types with their logic file
- Example: To add a new output format, extend `output.go` or create `output_mp4.go`

**Utilities:**
- Add shared helper functions to `utils.go` or create a focused utility file (e.g., `formats.go` for filename sanitization helpers)

## Special Directories

**.github/:**
- Purpose: GitHub workflows and assets
- Generated: No
- Committed: Yes

**.planning/:**
- Purpose: GSD planning and codebase analysis artifacts
- Generated: Yes (by `gsd-map-codebase` skill)
- Committed: Yes

**.opencode/ and .agents/:**
- Purpose: AI agent configuration, skills, and command definitions
- Generated: Partially (agent-managed)
- Committed: Yes

---

*Structure analysis: 2026-07-08*
