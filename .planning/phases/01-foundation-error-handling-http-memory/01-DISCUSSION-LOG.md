# Phase 1: Foundation — Error Handling, HTTP, Memory - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-08
**Phase:** 1-Foundation — Error Handling, HTTP, Memory
**Areas discussed:** Error Contract, HTTP + Context Behavior, Streaming Segment Assembly, Widevine Device Lifetime, Signal Cleanup + Resource Ownership, Test Scope for Phase 1

---

## Error Contract

| Option | Description | Selected |
|--------|-------------|----------|
| Continue the batch/season | Fail only the current episode, clean resources, report the error, and continue. | ✓ |
| Stop on first error | Any failure interrupts everything immediately. | |
| Depends on the error | Recoverable failures continue; configuration failures stop everything. | |

**User's choice:** Continue the batch/season
**Notes:** Episode-level failures should not derail a season or batch run.

| Option | Description | Selected |
|--------|-------------|----------|
| Clean and actionable | No stack trace; short context and a next step when obvious. | ✓ |
| Clean plus technical detail in debug | Simple by default; detailed cause in debug mode. | |
| Detailed always | Always show full error detail. | |

**User's choice:** Clean and actionable
**Notes:** The user wanted normal output to stay human-readable.

| Option | Description | Selected |
|--------|-------------|----------|
| Warning, not failure | Cleanup failure does not override a successful main result. | ✓ |
| Failure if remote resource is left open | Stream cleanup can change the outcome. | |
| Always fail | Any cleanup failure becomes a hard error. | |

**User's choice:** Warning, not failure
**Notes:** Cleanup should not erase a successful download.

| Option | Description | Selected |
|--------|-------------|----------|
| No panic in expected flow | Internal functions return errors; main decides continue/exit. | ✓ |
| Panic only for impossible invariants | Allow panic only for truly unreachable bugs. | |
| Recover panics at the top | Convert panics at the episode boundary. | |

**User's choice:** No panic in expected flow
**Notes:** The user wanted the refactor to remove panic-driven control flow from normal paths.

## HTTP + Context Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Simple overall timeout | Single `http.Client.Timeout` value. | |
| Configured transport plus layered timeouts | Keep-alive, idle conn reuse, connect timeout, request/context timeout. | ✓ |
| Context-only timeout | No `Client.Timeout`, only explicit contexts. | |

**User's choice:** Configured transport plus layered timeouts
**Notes:** Shared client configuration should be explicit and reused.

| Option | Description | Selected |
|--------|-------------|----------|
| One refresh, then error | Retry auth once, then return a clean failure. | ✓ |
| Refresh while necessary | Continue refreshing as long as `401` appears. | |
| Do not refresh automatically | Surface any `401` to the user. | |

**User's choice:** One refresh, then error
**Notes:** The retry budget stays intentionally small.

| Option | Description | Selected |
|--------|-------------|----------|
| Everything through configured client | Remove `http.DefaultClient` bypasses everywhere. | ✓ |
| Same transport, no auth | Derive a no-auth client with the same transport. | |
| Separate configured clients | Use distinct clients for API and CDN. | |

**User's choice:** Everything through configured client
**Notes:** Segment and subtitle paths should stop bypassing the shared client.

| Option | Description | Selected |
|--------|-------------|----------|
| Context from main to API/media | Propagate cancellation through the full stack. | ✓ |
| Context only for long downloads | Limit context to the expensive paths. | |
| No explicit context yet | Rely only on client timeouts. | |

**User's choice:** Context from main to API/media
**Notes:** Ctrl+C should be able to cancel active work cleanly.

## Streaming Segment Assembly

| Option | Description | Selected |
|--------|-------------|----------|
| Temp file then decrypt | Download init + segments to disk and decrypt from there. | ✓ |
| Pipe/stream direct to decrypt | Feed the decrypt step as a stream. | |
| Separate temp file per segment | Keep each segment as its own file. | |

**User's choice:** Temp file then decrypt
**Notes:** The main requirement was to remove the in-memory `[][]byte` assembly.

| Option | Description | Selected |
|--------|-------------|----------|
| Fail and clean everything | Partial segment failure invalidates the output and cleans temporaries. | ✓ |
| Keep partial artifacts | Leave artifacts behind for inspection. | |
| Tolerate missing segments | Continue with a degraded output. | |

**User's choice:** Fail and clean everything
**Notes:** No partial output should survive a failed download.

| Option | Description | Selected |
|--------|-------------|----------|
| Progress by segment | Keep the existing segment counter style. | ✓ |
| Progress by bytes | Track bytes instead of segment count. | |
| No progress | Drop progress reporting during the refactor. | |

**User's choice:** Progress by segment
**Notes:** Keep the current mental model while changing the storage path.

| Option | Description | Selected |
|--------|-------------|----------|
| DownloadParts owns the cycle | Create, write, decrypt, and clean in one place. | ✓ |
| Caller owns the cycle | Let the caller manage temporary files. | |
| Separate explicit helpers | Split creation/writing/cleanup into separate helpers. | |

**User's choice:** DownloadParts owns the cycle
**Notes:** The temp-file lifecycle should stay localized.

## Widevine Device Lifetime

| Option | Description | Selected |
|--------|-------------|----------|
| Once per process | Load once and reuse across all license requests. | ✓ |
| Once per episode | Reload per episode. | |
| Lazy on first license | Delay until the first license request. | |

**User's choice:** Once per process
**Notes:** The device cache should be process-wide.

| Option | Description | Selected |
|--------|-------------|----------|
| Keep current discovery for now | Preserve the current filesystem scan. | |
| Explicit paths now | Add explicit CLI paths in this phase. | |
| Environment variables via `.env` | Read device paths from environment variables loaded from `.env`. | ✓ |

**User's choice:** Environment variables via `.env`
**Notes:** The user explicitly asked for `.env`-backed configuration.

| Option | Description | Selected |
|--------|-------------|----------|
| Require both or none | `client_id.bin` and `private_key.pem` must appear together. | ✓ |
| Accept one if possible | Use whichever file exists. | |
| Fallback silently | Ignore incomplete input and search elsewhere. | |

**User's choice:** Require both or none
**Notes:** Incomplete raw-pair config should fail clearly.

| Option | Description | Selected |
|--------|-------------|----------|
| Prefer `.wvd` | Use the single packaged device first. | ✓ |
| Prefer raw pair | Use `client_id.bin` + `private_key.pem` first. | |
| Error on ambiguity | Force the user to choose one source. | |

**User's choice:** Prefer `.wvd`
**Notes:** When both exist, the simpler packaged format wins.

| Option | Description | Selected |
|--------|-------------|----------|
| Single clear error | List accepted formats and point to the `.env` source. | ✓ |
| Technical detailed error | Include search order and attempted paths. | |
| Fallback to nil | Let the failure happen later. | |

**User's choice:** Single clear error
**Notes:** Missing-device feedback should be concise and obvious.

## Signal Cleanup + Resource Ownership

| Option | Description | Selected |
|--------|-------------|----------|
| Cancel and clean everything | Cancel work, stop workers, remove temps, release streams. | ✓ |
| Cancel only the current download | Stop the active episode only. | |
| Try to finish the episode | Ignore the signal until the episode ends. | |

**User's choice:** Cancel and clean everything
**Notes:** Interrupts should terminate the whole active flow.

| Option | Description | Selected |
|--------|-------------|----------|
| Kill ffmpeg and clean partial output | Stop ffmpeg and delete any partial file. | ✓ |
| Let ffmpeg finish | Let the mux step complete after interruption. | |
| Wait for a graceful timeout | Allow a short graceful shutdown window. | |

**User's choice:** Kill ffmpeg and clean partial output
**Notes:** The output file should not be left half-written.

| Option | Description | Selected |
|--------|-------------|----------|
| Always delete temps | Remove temps even if debug artifacts are lost. | ✓ |
| Preserve temps on error | Keep temps for later inspection. | |
| Controlled by debug flag | Make temp retention conditional. | |

**User's choice:** Always delete temps
**Notes:** Cleanup wins over artifact preservation.

| Option | Description | Selected |
|--------|-------------|----------|
| Prioritize local exit | Ignore `DeleteStream` failure during shutdown. | ✓ |
| Report as warning | Warn, but still treat it as notable. | |
| Treat as final error | Fail shutdown because remote cleanup failed. | |

**User's choice:** Prioritize local exit
**Notes:** Remote cleanup must not block local shutdown.

| Option | Description | Selected |
|--------|-------------|----------|
| Remove everything | No partial artifacts survive cancelation. | ✓ |
| Keep the downloaded video | Leave completed pieces behind. | |
| Keep everything | Preserve all partial work. | |

**User's choice:** Remove everything
**Notes:** Cancelation should leave a clean workspace.

## Test Scope for Phase 1

| Option | Description | Selected |
|--------|-------------|----------|
| Focal and practical | Test the most fragile points without chasing aggressive coverage. | ✓ |
| Broader base | Add more coverage across the touched packages. | |
| Scaffolding only | Create the test harness and defer real coverage. | |

**User's choice:** Focal and practical
**Notes:** The user wanted practical tests rather than a broad coverage push.

| Option | Description | Selected |
|--------|-------------|----------|
| Critical flow of this phase | Error contract, HTTP retry, `DownloadParts`, Widevine discovery, cleanup/interruption, URL parsing. | ✓ |
| Pure unit tests only | Avoid integration-style tests. | |
| Integration with mock HTTP plus units | Add a stronger mock-server harness now. | |

**User's choice:** Critical flow of this phase
**Notes:** The first batch should validate the phase’s risky behavior.

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal useful coverage | Set a practical floor instead of a strict high target. | ✓ |
| Moderate coverage | Push a clearer percentage target for touched packages. | |
| High coverage now | Aim aggressively in phase 1. | |

**User's choice:** Minimal useful coverage
**Notes:** Coverage is a floor, not the main objective for this phase.

## the agent's Discretion
None.

## Deferred Ideas

None.
