# Desktop GUI design — Crunchyroll Downloader

**Date:** 2026-08-02  
**Status:** Draft for user review (sections 1–3 approved in chat)  
**Branch context:** `merge-all-prs` (CLI features: multi-lang, CC, index, disk streaming, longer content IDs)

## Goal

Ship a **modern, clean desktop GUI** that drives the existing Go download engine. Casual users get an Inspect → multi-select → Download flow with sensible defaults. Power users get an Advanced tab without cluttering Normal mode.

The GUI must not look like a default toolkit or “vibe-coded” purple theme. Visual direction is **dark cinema** with **warm orange/amber** accents on stone/charcoal surfaces.

## Non-goals (this iteration)

- Bypassing Premium, DRM, or account restrictions.
- Pure browser-hosted SaaS UI.
- Pure Fyne/native-widget aesthetic.
- Final release packaging decision (GUI-only vs GUI+CLI artifacts) — **deferred**; implement GUI-first and keep CLI buildable from the same engine.
- Perfect live progress for every internal phase (segment % when available; indeterminate otherwise is fine).

## Platform & architecture

### Choice: hybrid desktop (Wails v2)

| Layer | Technology |
|-------|------------|
| Window | Native OS window (Wails) — not a browser tab |
| UI | HTML/CSS/JS (dark cinema, custom widgets) |
| Engine | Existing Go package (download, inspect, DRM, index) |
| CLI | Remains buildable from the same engine |

**Rejected:** pure browser (`localhost` server as primary UX); pure Fyne (hard to match design quality).

### Process layout

1. **Extract/refactor engine** so GUI and CLI share library entry points (not “shell out to EXE” as the primary path).
2. **Wails app** binds to Go methods and events.
3. **Preferences** JSON under the user config directory (Windows: `%AppData%/crunchyroll-downloader/preferences.json`).

### Go surface (conceptual)

```text
Inspect(url, auth, options) -> Catalog
  // seasons, episodes, audio versions (CMS)
  // optional playback probe: subs, CC, video/audio qualities from MPD
  // always release playback stream token after probe

Download(job, progress) -> error
  // existing pipeline: segments, decrypt, ffmpeg merge

GetPrefs() / SavePrefs(prefs)
  // paths only for secrets (cookie, device files)
```

Progress is pushed via events/callbacks: log lines, queue item state, segment fraction when known, phase name (inspect / license / mux / idle).

## Visual design

- **Theme:** dark cinema — deep stone/charcoal (`#0c0a09`, `#141210`, `#1c1917`), soft borders, glass-like cards.
- **Accent:** warm orange (`#f97316` / `#ea580c`) — **no violet/purple “AI default” palette**.
- **Widgets:** custom checkboxes (SVG tick, centered), season chips, custom quality dropdowns (not native OS select), smooth open/close and selection transitions.
- **Typography:** system UI sans; mono only in the activity log.
- **Layout:** top bar (brand + Normal/Advanced), URL + Cookie + Inspect, then three-column Normal form; during runs, form dimmed + side panel for queue/progress/log (see § Progress & errors).

Reference mockups live under `.superpowers/brainstorm/` (local only; not required for build).

## UX: Normal mode

### Primary flow (Inspect-first)

1. Paste **URL** (series or episode/movie) and choose **cookie file** (`etp_rt`).
2. Press **Inspect**.
3. Multi-select from **live** options (when available).
4. Press **Download selected**.

### Defaults (first run and after Inspect when options exist)

| Setting | Default |
|---------|---------|
| Episodes | **S1E1** only; if URL is `/watch/…`, that single title |
| Audio | **Original** (series primary / usually `ja-JP`) |
| Subtitles | **None** |
| Closed captions | **None** (Advanced; hidden in Normal) |
| Video quality | **Max available** after probe (`(max)` policy) |
| Audio quality | **Max available** after probe |
| Output folder | last used, else `./Downloads` |

### Normal controls

- Editable URL field.
- Cookie file browse (path stored; value never stored in prefs).
- Seasons as clickable chips; episodes as multi-select list + Select all (episodes only).
- Audio multi-select; subtitles multi-select with exclusive **No subtitles**.
- Video/audio quality custom dropdowns.
- Output folder + browse.
- **Download selected** (simple solid primary button, no mandatory icon).

### Catalog vs probe

| Data | Source | Cost |
|------|--------|------|
| Seasons, episodes, titles | CMS APIs | Low — no stream |
| Audio dubs / versions | Episode metadata | Low |
| Subtitles, captions | Playback API | Probe — open then **release** |
| Video/audio quality lists | MPD after playback | Same probe |

Probe once per Inspect (or once per Advanced “probe every episode” if enabled). Do not open streams for every checkbox click.

## UX: Advanced mode

Toggle in the header (same window; panel swap with light transition). Adds:

- Closed captions multi-select (default none).
- Batch URLs (textarea and/or `.txt` file).
- Widevine paths: `.wvd` and/or `client_id.bin` + `private_key.pem` (paths only; env vars remain supported).
- Probe every selected episode (off by default).
- Debug manifest logging.
- Playback 4294 retries / backoff, circuit limit, index window.
- Subtitle index tools: `--index` / `--index-subs` equivalents (series URLs).
- Optional **strict language** behavior: stop queue instead of skipping missing locales (default remains skip + log).

Normal-mode choices remain the base of the job when Advanced is only partially used.

## Preferences

Persist on user change (debounced):

- Last URL, cookie path, output directory.
- Normal vs Advanced last tab.
- Language selections and quality policy (`max` vs fixed height/bitrate).
- Last selected season (when relevant).
- Advanced numeric/toggle settings and device paths.
- Optional window size.

**Never** persist raw `etp_rt` cookie contents or private key material — only filesystem paths.

## Progress, queue, and errors

### Progress UI

- **Activity log:** chronological, human-readable lines (CLI-equivalent info).
- **Queue:** per-item states — queued / active / done / skipped / failed.
- **Progress bar:** segment % when known; indeterminate for inspect, license, mux, backoff.
- **Cancel:** stop between episodes when safe; best-effort cancel mid-episode (stop workers, release tokens).
- Form remains visible but controls dim while a job runs.

### Error UX

| Situation | Behavior |
|-----------|----------|
| Bad/missing cookie | Error banner + choose file / retry |
| Missing Widevine on full download | Banner pointing to Advanced device paths |
| Missing selected language | Skip episode + warn log (default); strict mode stops |
| 4294 / backoff | Warn banner, countdown/indeterminate, respect Advanced retries |
| FFmpeg missing | Hard stop + install hint |
| Inspect failure | Keep form; error in log + banner |

Normal mode: no stack traces. Advanced may show more detail.

## Engine & product constraints

- Authorized Crunchyroll accounts only; Premium rules unchanged.
- Movie/watch IDs longer than 14 chars are valid (e.g. `GMEE00374450JAJP`).
- Disk streaming of encrypted segments (OOM fix) remains in the engine.
- Windows credential file mode: do not require Unix `0600` bit fidelity (existing Windows portability fix).
- Index/subtitle tooling stays CDM-free where already designed that way.

## Implementation outline (for later planning)

1. Engine API extraction + unit tests for Inspect/list qualities.
2. Wails project scaffold + shared theme CSS matching mockups.
3. Normal UI + Inspect/Download bindings + prefs.
4. Advanced UI + index entry points.
5. Progress/queue events + cancel.
6. README / build docs for GUI; CLI docs remain.

Shipping artifacts (single GUI EXE vs dual binaries) remain an open product decision.

## Approval record

| Section | Decision |
|---------|----------|
| 1 Architecture | Hybrid Wails; shared Go engine; path-only secrets |
| 2 UX / visuals / Normal+Advanced | Approved (mockups v4–v5); Inspect-first; defaults; prefs |
| 3 Progress / errors | Approved for implementation; refine when live |

## Open items (explicitly deferred)

- Exact release packaging (GUI only vs GUI+CLI).
- Whether strict-language is a Normal preference or Advanced-only (spec assumes Advanced).
- Cross-platform packaging beyond Windows-first development.
