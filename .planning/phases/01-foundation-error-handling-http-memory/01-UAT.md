---
status: testing
phase: 01-foundation-error-handling-http-memory
source:
  - 01-VERIFICATION.md
started: 2026-07-09T07:37:03Z
updated: 2026-07-09T07:37:03Z
---

## Current Test

number: 1
name: SIGINT/SIGTERM end-to-end cancellation
expected: |
  A running download exits cleanly without panic, cancels active HTTP/media/mux work,
  stops ffmpeg if running, and leaves no partial local artifacts.
awaiting: user response

## Tests

### 1. SIGINT/SIGTERM end-to-end cancellation
expected: |
  Start a real or mocked long-running download and send SIGINT, then SIGTERM.
  Active work should be canceled, ffmpeg should stop if running, the process
  should exit cleanly, and partial local artifacts should be removed.
result: pending
source: human

### 2. Interrupted shutdown with DeleteStream failure
expected: |
  Force the Crunchyroll stream cleanup request to fail or hang during interrupted
  shutdown. The failure should be reported or bounded, local cleanup should still
  complete, and shutdown should not block indefinitely.
result: pending
source: human

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
