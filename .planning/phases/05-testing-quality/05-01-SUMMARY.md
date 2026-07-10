# Phase 05-01: Test Infrastructure and Pure Function Unit Tests

## Goal

Create test infrastructure (factories, fixtures), add pure function unit tests for high-priority coverage gaps, and add integration tests with HTTP mocking.

## Tasks

### Task 1: Test infrastructure — testutil factories and testdata fixtures

**Files created:**

- `testutil/factories.go` — Package `testutil` with factory functions:
  - `EpisodeInfo()` — default episode info
  - `EpisodeInfoWithVersions(locales ...string)` — episode info with dub versions
  - `SeasonEpisode(episodeNumber, seriesTitle, seasonNumber)` — season episode
  - `DummyMPD()` — minimal MPD with one Period, video and audio AdaptationSets
- `internal/media/testdata/mpd/simple-video-audio.mpd` — 1 video + 1 audio set
- `internal/media/testdata/mpd/multi-audio.mpd` — 1 video + 2 audio reps
- `internal/media/testdata/mpd/with-content-protection.mpd` — includes `<pssh>` element
- `internal/media/testdata/mpd/no-content-protection.mpd` — no ContentProtection
- `internal/api/testdata/api/episode-info-response.json` — Crunchyroll episode info response
- `internal/api/testdata/api/season-list-response.json` — season list response
- `internal/api/testdata/api/episode-playback-response.json` — playback response
- `internal/api/testdata/api/license-response.json` — Widevine license response

**Commit:** `4647915`

### Task 2: Pure function unit tests

**Files created/extended:**

- `internal/locale/locale_test.go` — `TestLanguageNames` (26 entries + missing locale), `TestLanguageCodes` (26 entries + missing locale)
- `main_test.go` — `TestParseLangs` (4 cases), `TestResolveString` (4-level hierarchy), `TestIsAllNilConfig` (all nil + 8 individual fields)
- `internal/config/config_test.go` — `TestParseDotenvEmptyFile`, `TestParseDotenvCommentsAndBlanks`, `TestParseDotenvKeyValue`, `TestParseDotenvQuotedValues`, `TestParseDotenvNoEquals`, `TestLoadDotenvFound`, `TestLoadDotenvNotFound`
- `internal/download/episode_test.go` — `TestFormatFileSize` (9 cases), `TestFormatDuration` (5 cases)
- `internal/media/segment_test.go` — `TestFormatSpeed` (9 cases), `TestFormatETAShort` (5 cases), `TestGetFilename` (4 cases)

**Commit:** `2b47065`

### Task 3: Integration tests and extended unit tests

**Files extended:**

- `internal/media/manifest_test.go` — `TestBuildUrl` (5 cases), `TestExpandTimeline` (6 cases), `TestParseManifest` (loads XML fixture), `TestParseManifestInvalidXML`
- `main_test.go` — `TestProcessURLWatchPath` (httptest.NewServer with mocked /auth/v1/token, /content/v2/cms/objects, /playback/v3/, /license/v1/license/widevine; canceled context; captureMainStderr)
- `internal/drm/drm_test.go` — `TestGetPsshWithProtection` (loads MPD with `<pssh>`, verifies non-nil), `TestGetPsshWithoutProtection` (loads MPD without CP, verifies nil)
- `internal/mux/mux_test.go` — `TestTrackTitle` (ja-JP, en-US, xx-XX), `TestTrackTitleAllLocales` (iterates all LanguageNames)
- `internal/download/season_test.go` — `TestRunSeasonEmptyEpisodes` (nil input returns nil), `TestSeasonErrorFormatting` (formats "2 of 5 episode(s) failed"), `TestFormatFailedList` (episodeError slice → semicolon-joined string)

**Commit:** `5982bd3`

## Verification

```
go build ./...  →  success
go test -count=1 -race ./...  →  all 9 packages pass
```

Test results summary:
- `crunchyroll-downloader` — ok
- `crunchyroll-downloader/internal/api` — ok
- `crunchyroll-downloader/internal/config` — ok
- `crunchyroll-downloader/internal/download` — ok
- `crunchyroll-downloader/internal/drm` — ok
- `crunchyroll-downloader/internal/locale` — ok
- `crunchyroll-downloader/internal/media` — ok
- `crunchyroll-downloader/internal/mux` — ok
- `crunchyroll-downloader/internal/output` — ok
- `crunchyroll-downloader/testutil` — no test files (factory package only)

## Notes

- All tests use Go stdlib testing only (no testify dependency)
- Table-driven tests with `t.Run` subtests throughout
- HTTP mocking via `httptest.NewServer` for integration tests
- MPD XML testdata verified parseable by `go-mpd` library
- JSON testdata mirrors real Crunchyroll API response structures
