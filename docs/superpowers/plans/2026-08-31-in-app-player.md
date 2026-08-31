# In-app libmpv player Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** In-window cinema Play overlay (libmpv + ASS) with forced quality, progressive decrypt-while-watching as the target, and CR playhead POST on pause/close/finish only.

**Architecture:** Engine writes playheads and runs a PlaySession (reuse download/decrypt, one representation). GUI hosts a native libmpv HWND under HTML overlay chrome (mock `play-page-mockup-v3.html`). Phase 1 plays completed MKV; Phase 2 grows a buffer and retargets workers on seek-ahead. Never launch a separate mpv window; never ABR.

**Tech Stack:** Go 1.25, Wails v2 WebView2, existing `internal/engine` DRM/download, libmpv on Windows (`libmpv-2.dll` or embed), HTML/CSS/JS overlay.

**Spec:** `docs/superpowers/specs/2026-08-31-in-app-player-design.md`

**Do not:** wrap crunchyroll.com, Chrome extensions, skip ±10/30 chips, realtime playhead ticks.

---

## File map

| File | Responsibility |
|------|----------------|
| `internal/engine/playhead.go` | GET (reuse) + POST playheads; finish threshold |
| `internal/engine/playhead_test.go` | JSON body, finish %, debounce |
| `internal/engine/play_session.go` | PlaySession start/stop, MKV vs progressive, buffer range, seek retarget |
| `internal/engine/play_session_test.go` | Queue retarget, ready threshold |
| `cmd/gui/player_windows.go` | HWND + libmpv load/play/pause/seek/time (build tag windows) |
| `cmd/gui/player_stub.go` | Non-windows stub: error “player library missing” |
| `cmd/gui/app.go` | `StartPlay`, `StopPlay`, `PlayControl`, events `play-state` |
| `cmd/gui/frontend/index.html` | `#page-play` overlay |
| `cmd/gui/frontend/css/app.css` | Cinema overlay (no skip chips) |
| `cmd/gui/frontend/js/app.js` | Play page, wire episode → Play, playhead events |
| `README.md` | Play + libmpv runtime note |

---

### Task 1: Playhead POST + finish helper

**Files:**
- Create: `internal/engine/playhead.go`
- Create: `internal/engine/playhead_test.go`
- Modify: `internal/engine/discover_hydrate.go` — export `PlayheadInfo` if still unexported (`playheadInfo` → keep GET via `fetchPlayheads`; add `GetPlayheads` wrapper)

- [ ] **Step 1: Failing tests**

```go
package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlayheadPOSTBody(t *testing.T) {
	var got struct {
		ContentID string  `json:"content_id"`
		Playhead  float64 `json:"playhead"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/playheads") {
			t.Fatalf("path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	// Test will inject playheadPOSTURL or pass base URL via seam.
	if err := PostPlayheadWithBase(srv.URL, "acct", "GWep", 372.5, "pt-BR", "ja-JP"); err != nil {
		t.Fatal(err)
	}
	if got.ContentID != "GWep" || got.Playhead != 372.5 {
		t.Fatalf("%+v", got)
	}
}

func TestFinishPlayheadSeconds(t *testing.T) {
	if !IsPlayFinished(23.0, 24.0) { // >= 95%
		t.Fatal("expected finish")
	}
	if IsPlayFinished(10.0, 24.0) {
		t.Fatal("mid episode is not finish")
	}
	if s := FinishPlayheadSeconds(24.1); s != 24.1 {
		t.Fatalf("%v", s)
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL** (`PostPlayheadWithBase` / `IsPlayFinished` undefined)

```
go test ./internal/engine/ -count=1 -run "TestPlayheadPOSTBody|TestFinishPlayheadSeconds"
```

- [ ] **Step 3: Implement**

```go
package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const playFinishRatio = 0.95

func IsPlayFinished(positionSec, durationSec float64) bool {
	if durationSec <= 0 {
		return false
	}
	return positionSec >= durationSec*playFinishRatio
}

func FinishPlayheadSeconds(durationSec float64) float64 {
	if durationSec < 0 {
		return 0
	}
	return durationSec
}

type playheadPOSTBody struct {
	ContentID string  `json:"content_id"`
	Playhead  float64 `json:"playhead"`
}

func PostPlayhead(accountID, contentID string, playheadSec float64, locale, audioLang string) error {
	return PostPlayheadWithBase("https://www.crunchyroll.com", accountID, contentID, playheadSec, locale, audioLang)
}

func PostPlayheadWithBase(base, accountID, contentID string, playheadSec float64, locale, audioLang string) error {
	accountID = strings.TrimSpace(accountID)
	contentID = strings.TrimSpace(contentID)
	if accountID == "" || contentID == "" {
		return fmt.Errorf("playhead requires account and content id")
	}
	if playheadSec < 0 {
		playheadSec = 0
	}
	locale = normalizeDiscoverLocale(locale)
	q := url.Values{}
	if locale != "" {
		q.Set("locale", locale)
	}
	if strings.TrimSpace(audioLang) != "" {
		q.Set("preferred_audio_language", audioLang)
	}
	endpoint := strings.TrimRight(base, "/") + "/content/v2/" + url.PathEscape(accountID) + "/playheads"
	if enc := q.Encode(); enc != "" {
		endpoint += "?" + enc
	}
	payload, err := json.Marshal(playheadPOSTBody{ContentID: contentID, Playhead: playheadSec})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := DoRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("playhead POST HTTP %d", resp.StatusCode)
	}
	return nil
}
```

Use `httptest` with `PostPlayheadWithBase` so tests never hit production. Production GUI calls `PostPlayhead`.

Also export:

```go
func GetPlayheads(accountID string, contentIDs []string) (map[string]playheadInfo, error) {
	return fetchPlayheads(accountID, contentIDs)
}
```

- [ ] **Step 4:** `go test ./internal/engine/ -count=1 -run "TestPlayhead|TestFinish"` — PASS

- [ ] **Step 5: Commit**

```
git add internal/engine/playhead.go internal/engine/playhead_test.go internal/engine/discover_hydrate.go
git commit -m "feat(engine): POST Crunchyroll playheads on pause/close/finish"
```

---

### Task 2: Play overlay HTML/CSS/JS (no native video yet)

**Files:**
- Modify: `cmd/gui/frontend/index.html` — add `#page-play` full-bleed overlay
- Modify: `cmd/gui/frontend/css/app.css` — cinema overlay (copy structure from mock `play-page-mockup-v3.html`)
- Modify: `cmd/gui/frontend/js/app.js` — `setShellPage("play")`, overlay idle fade, no skip chips

**UI must include:** back, series · SxxExx, episode title, `1080p locked` badge, orange timeline + gray buffer, replay, play/pause, volume, time, next, CC, 1x, settings, fullscreen. **Must not** include 30/10/30 skip buttons.

- [ ] **Step 1:** Add `#page-play.page-play` in `index.html` with those controls (IDs: `play-stage`, `btn-play-back`, `btn-play-toggle`, `play-timeline`, `play-buf`, `play-prog`, `play-time`, `play-title`, `play-lock`).

- [ ] **Step 2:** CSS: `page-play` `position:fixed; inset:0; z-index:80; background:#000` so it covers the shell like CR/Netflix. Chrome fades with `.is-idle`.

- [ ] **Step 3:** JS `openPlayOverlay(meta)` / `closePlayOverlay()`:
  - `openPlayOverlay` calls `setShellPage("play")` or just shows `#page-play` without destroying Home.
  - Back and sidebar other pages call `closePlayOverlay` (later StopPlay).
  - Space toggles play button visually.

Until Task 3, `play-stage` is a black letterbox (label “libmpv surface”).

- [ ] **Step 4:** Wire episode list and CW `openDiscoverCard` to offer Play later; for this task add a **Play** control on Downloads after Inspect: button `#btn-play-selected` next to Download Selected — click opens overlay with selected episode meta from catalog.

- [ ] **Step 5: Commit**

```
git add cmd/gui/frontend/index.html cmd/gui/frontend/css/app.css cmd/gui/frontend/js/app.js
git commit -m "feat(gui): cinema Play overlay chrome without skip chips"
```

---

### Task 3: libmpv HWND host (Windows) + stub

**Files:**
- Create: `cmd/gui/player_windows.go` (`//go:build windows`)
- Create: `cmd/gui/player_stub.go` (`//go:build !windows`)
- Modify: `cmd/gui/app.go` — player lifecycle

**Approach (Windows):** do **not** `exec` a visible `mpv.exe` window. Load `libmpv-2.dll` (document: place next to the GUI exe, same as CDM discovery). Create a child HWND covering `#play-stage` screen rect (Win32 `CreateWindowEx`), attach mpv `wid` to that HWND. On resize/fullscreen, move/size the HWND. If DLL missing, return error string `player library missing` — GUI shows it in overlay; **never** ShellExecute mpv.

Stub `StartMpv(hwnd uintptr, file string) error` returns `fmt.Errorf("player library missing")`.

- [ ] **Step 1:** Define in `cmd/gui/player.go` (all OS):

```go
type MpvHost interface {
	Attach(hwnd uintptr) error
	LoadFile(path string) error
	Pause(p bool) error
	Seek(seconds float64) error
	Position() (float64, error)
	Duration() (float64, error)
	SetVolume(percent int) error
	Destroy() error
}
```

- [ ] **Step 2:** Windows implementation using `github.com/gen2brain/go-mpv` **or** cgo `libmpv` if that module fails to build — prefer a **pure LoadLibrary** wrapper of `mpv_create` / `mpv_set_option_string("wid", ...)` / `mpv_command(["loadfile", path])` to avoid extra C compiler if possible. If CGO is required, note in README: GUI Play needs TDM-GCC **or** prebuilt `libmpv-2.dll` + generated stub.

**Pragmatic path if cgo is painful on this machine:** embed **hidden** mpv with `--wid=<hwnd>` via `exec.Command("mpv", "--wid="+strconv.FormatUint(uint64(hwnd), 10), "--no-border", "--idle=yes", "--input-ipc-server=...")` so video still draws **in our HWND**, not a second app window. IPC JSON for pause/seek/time. Still **not** a separate player window.

- [ ] **Step 3:** `App.playHost MpvHost`; `StartPlay(path string) error`; EventsEmit `play-state` `{position, duration, paused, eof}`.

- [ ] **Step 4:** Manual: Play overlay + local sample MKV if present under `Downloads/`; if no libmpv, overlay shows error, no extra window.

- [ ] **Step 5: Commit**

```
git commit -m "feat(gui): embed libmpv video pane in Play overlay"
```

---

### Task 4: Bind Play session for completed MKV + playhead events

**Files:**
- Modify: `cmd/gui/app.go`
- Modify: `cmd/gui/frontend/js/app.js`

**Bindings:**

```go
type PlayRequest struct {
	EpisodeID   string `json:"episodeId"`
	SeriesTitle string `json:"seriesTitle"`
	EpisodeTitle string `json:"episodeTitle"`
	SeasonNumber int   `json:"seasonNumber"`
	EpisodeNumber int  `json:"episodeNumber"`
	FilePath    string `json:"filePath"` // empty = resolve from output dir
	AudioLang   string `json:"audioLang"`
	Locale      string `json:"locale"`
}

func (a *App) StartPlay(req PlayRequest) error
func (a *App) StopPlay() error
func (a *App) PlayPause() error
func (a *App) PlaySeek(seconds float64) error
func (a *App) PlaySetVolume(pct int) error
```

On `StartPlay`:
1. Auth from cookie prefs.
2. `GetPlayheads` for `EpisodeID` → return start position via event `play-ready` `{resumeSeconds}`.
3. If `FilePath` empty, glob output dir for existing `*SxxExx*.mkv` matching season/episode; if missing, error `no local file` (Task 5 adds progressive).
4. Attach mpv, load file, seek resume unless user chose start-over (JS confirm).
5. Tick goroutine every 250ms emit `play-state` (position/duration/paused/eof) — **do not POST playheads on tick**.

On Pause (JS play toggle when going paused), StopPlay (back/sidebar), eof:
- `PostPlayhead(accountId, episodeId, seconds, locale, audio)`
- eof / `IsPlayFinished` → `FinishPlayheadSeconds(duration)`
- Debounce: `lastPost` mutex; skip duplicate if same second within 1s.

- [ ] **Step 1:** Tests for debounce helper in engine:

```go
func TestPlayheadDebounceSameSecond(t *testing.T) {
	d := NewPlayheadDebouncer(time.Second)
	if !d.ShouldPost("id", 10) {
		t.Fatal("first should post")
	}
	if d.ShouldPost("id", 10) {
		t.Fatal("duplicate within window")
	}
}
```

Implement `PlayheadDebouncer` in `playhead.go`.

- [ ] **Step 2:** Wire JS: overlay buttons call `go.main.App.PlayPause` etc. Back → `StopPlay` + hide overlay. `EventsOn("play-state")` updates time + orange width.

- [ ] **Step 3:** `go test ./internal/engine/ -count=1` and tagged GUI build.

- [ ] **Step 4: Commit**

```
git commit -m "feat(gui): play completed MKV and POST playheads on pause/close/finish"
```

---

### Task 5: Progressive forced-quality session

**Files:**
- Create: `internal/engine/play_session.go`
- Create: `internal/engine/play_session_test.go`
- Modify: `internal/engine/download.go` only if extracting `downloadParts` for one representation without full mux
- Modify: `cmd/gui/app.go` `StartPlay` when no MKV

**PlaySession:**

```go
type PlaySession struct {
	EpisodeID string
	VideoQuality string
	AudioQuality string
	BufferEndSec float64 // contiguous from 0
	Dir string           // temp session dir
}

func StartProgressivePlay(ctx context.Context, episodeID string, cfg RuntimeConfig, emit func(PlayProgress)) (*PlaySession, error)
```

Reuse: `getEpisode` / manifest parse / `getLicense` / `downloadParts` **per representation**, but:
- Write decrypted fragments into `sessionDir/seg-NNNN.m4s` plus init.
- After init + first K segments, concatenate or build **fMP4** `session.mp4` that mpv can play (or `edl://` list). Simplest v1: **append decrypted samples into `playing.mp4` after ftyp+moov from init** if the CR fragments allow; if not, generate an **mkv** incrementally with ffmpeg `-f concat` when each segment lands (heavier). Prefer **mpv playlist of fragments** if concat is unsafe.

**Ready threshold:** `BufferEndSec >= 4` or 3 media segments, whichever first → emit `play-ready`.

**Seek ahead:** `SeekTarget(sec)` maps time → segment index (duration/segment count from MPD), **prioritize that index in the worker queue** (channel of jobs already exists in `downloadParts` — extract a `priorityHeap` or skip-to-index then fill gaps).

**Never** switch representation on stall.

Tests (fake MPD + fake hydrator):

```go
func TestSeekAheadRetargetsIndex(t *testing.T) {
	q := newSegmentQueue(100)
	q.Retarget(40)
	if q.Next() != 40 {
		t.Fatalf("want 40")
	}
}
```

- [ ] **Step 1:** Queue retarget tests + impl.  
- [ ] **Step 2:** Progressive StartPlay path when MKV missing.  
- [ ] **Step 3:** Timeline gray width = `bufferEnd/duration`. Seek in JS: if `sec > bufferEnd` show waiting, call `PlaySeek` which retargets.  
- [ ] **Step 4:** `go test ./internal/engine/ -count=1`  
- [ ] **Step 5: Commit**

```
git commit -m "feat(engine): progressive forced-quality play session with seek retarget"
```

---

### Task 6: Entry points, next episode, README

**Files:**
- Modify: `cmd/gui/frontend/js/app.js` — Home CW Play uses series catalog + `episodeId`; Downloads **Play** on selected episode  
- Modify: `README.md` — Play, libmpv DLL, playheads  
- Sidebar: optional **Play** item or only overlay (spec: overlay covers app; back returns)

- [ ] **Step 1:** CW click: keep Inspect for Download; add Play action that `StartPlay({episodeId, filePath: ""})` (progressive).  
- [ ] **Step 2:** Next button: if next catalog episode exists, StopPlay + StartPlay next id.  
- [ ] **Step 3:** README section **GUI Play**.  
- [ ] **Step 4:** `go test ./internal/engine/ -count=1` and `go build -tags "desktop,production" -ldflags "-w -s -H windowsgui" -o crunchyroll-downloader-gui.exe ./cmd/gui`  
- [ ] **Step 5: Commit**

```
git commit -m "feat(gui): wire Play from catalog and document libmpv"
```

---

## Spec coverage

| Spec item | Task |
|-----------|------|
| libmpv in-window, no extra app | 3 |
| Cinema overlay, no 30/10/30 | 2 |
| Playhead POST pause/close/finish | 1, 4 |
| GET resume | 4 |
| Forced quality / no ABR | 5 |
| Progressive decrypt-while-watch | 5 |
| Completed MKV same surface | 4 |
| Seek inside buffer / ahead retarget | 5 |
| Finish 95% | 1, 4 |
| Missing libmpv error | 3 |
| Home/Downloads entry | 6 |

## Execution

After compact, a new thread should read this plan + the spec path above. Default: **subagent-driven** unless the user picks inline.
