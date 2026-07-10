---
quick_id: 260709-v3w
status: complete
date: 2026-07-10
---

# Quick Task 260709-v3w: Fix speed display unit in Bps()

## Changes

- **internal/output/speed.go** — Fixed `SpeedTracker.Bps()`: removed `if i == 0 { oldestTime = ... }` which incorrectly set `oldestTime` to the newest sample timestamp. Now tracks the minimum timestamp among valid samples inside the validity check. This caused `elapsed` to be near-zero, producing unrealistic speeds like `129.4 GB/s` instead of correct values like `9.98 MB/s`.

## Verification

- `go build ./...` — passes
- `go vet ./...` — passes
- `go test ./internal/output/... -run Speed` — all 7 speed tests pass
  - `TestSpeedTrackerWithMultipleRecords`: `9.98 MB/s` (correct for 5×1MB over ~500ms)
  - `TestSpeedTrackerETAClamping`: first/second ETA both `10s` (correct)
- `go test ./...` — all tests pass
