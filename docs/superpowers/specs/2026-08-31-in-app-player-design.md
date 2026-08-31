# In-app forced-quality player + playhead sync

**Date:** 2026-08-31  
**Status:** Approved direction (user locked overlay UI; skip ±10/30 chips **out**)  
**Stack:** Existing Wails GUI + `internal/engine` download/decrypt pipeline + **libmpv** HWND in this window  

## Goal

Watch Crunchyroll episodes **inside this app** at a **locked quality** (same representation pick as download), with **ASS via libass**, and **sync Continue Watching** to the account on pause / close / finish only — so the user does not have to stay on crunchyroll.com for history.

Not a Chrome extension. Not wrapping crunchyroll.com (that keeps ABR). Not launching a separate mpv window.

## Product decisions (locked)

| Decision | Choice |
|----------|--------|
| Player engine | **libmpv in-window** (native video pane + ASS), HTML chrome overlay |
| Quality | Forced; one DASH representation; **never ABR** |
| Progressive stream | **Target:** download/decrypt while watching |
| Completed MKV | Same Play surface; skip workers if file already exists |
| Playhead write | **Pause, close, finish only** (no periodic ticks) |
| UI | Cinema overlay (CR/Netflix-like), mock: `.superpowers/brainstorm/*/play-page-mockup-v3.html` |
| Skip ±10/30 chips | **Out** (extension chrome; user does not want them) |
| Chrome extension max-quality | **Out** (unreliable vs our pipeline) |

## Architecture

```
[Play overlay in this window]
  HTML: back, title, 1080p locked, timeline, play/pause, vol, time, next, CC, 1x, settings, fullscreen
  Native: libmpv HWND = full-bleed video + ASS (not HTML <video>)

Pipeline:
  inspect episode → pick forced video/audio (+ fetch ASS sidecar when possible)
  → license/CDM as download today
  → workers fetch only chosen representations
  → decrypt → progressive buffer (segments / fMP4)
  → libmpv plays buffer
  pause / leave Play / ended → POST playheads
```

Phases (same spec, implement in order):

1. Play overlay + libmpv + **completed MKV** + playhead POST  
2. **Progressive forced-quality** session (same overlay)

## Buffering and seek

- Ready after a short contiguous decrypt (not 100% of episode).  
- UI timeline: **gray = buffered**, **orange = position**.  
- Seek **inside** buffer: immediate.  
- Seek **ahead of** buffer: retarget workers to that segment index; wait; **do not** drop quality.  
- On Play, GET playheads → **Resume** vs **Start over** (default Resume).  
- Finish: position ≥ ~95% duration or `ended` → POST playhead ≈ duration (optional `mark_as_watched`).

## Playhead contract

```
POST /content/v2/{accountId}/playheads?locale=&preferred_audio_language=
Authorization: Bearer <existing token>
Content-Type: application/json
{ "content_id": "<episodeId>", "playhead": <seconds> }
```

| Event | playhead |
|--------|----------|
| Pause | current seconds |
| Close / leave Play | last known seconds |
| Finish | duration seconds |

Debounce: one write per user event; pause-then-close → single POST. Failure: Activity log, retry once on close, do not stop local playback.

GET playheads / watch-history already used on Home.

Optional later: `DELETE /playback/v1/token/...` if we open a CR play-service session (not required for v1 history).

## Overlay UI (locked)

- Full-bleed black; video fills the Play page (letterbox if needed).  
- ASS on the picture (libass).  
- Chrome over video, fades on idle.  
- Top: back, series · SxxExx, episode title, **quality locked** badge.  
- Bottom: thin orange progress, replay, play/pause, volume, time, next, subtitles, speed, settings, fullscreen.  
- **No** 30/10/30 skip buttons.  
- Keyboard: space pause, arrows seek, F fullscreen (mpv defaults OK).

Entry: Downloads row or Home Continue Watching → Play (progressive if no complete MKV).  
Exit: back / other sidebar page = close → playhead POST, stop workers.

## Errors

- No libmpv: error in overlay (“player library missing”), **do not** open another app.  
- License fail: same as download.  
- Stall: freeze at buffer end; retry same quality.  

## Testing

- Unit: playhead JSON body, finish threshold, seek-ahead queue retarget.  
- Manual: pause/close updates Continue Watching after Home refresh; ASS visible; quality label matches chosen representation.

## Non-goals

- Wrapping crunchyroll.com  
- Browser EME against live encrypted CR DASH  
- Chrome extension ABR hacks  
- Realtime playhead ticks  
- Skip intro/credits v1 (can use skip-events later)  

## Success

1. Play stays in this window; looks like overlay cinema, not a utility card.  
2. Quality never ABR-drops.  
3. ASS looks like libass, not W11 Movies & TV.  
4. Pause/close/finish updates CR Continue Watching.  
5. Progressive play starts before the full episode is on disk.
