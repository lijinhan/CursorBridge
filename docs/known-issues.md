# Known Issues

> Last updated: 2026-05-14

## Fixed

### ~~1. Long-task rollback on context compaction~~

**Status**: Fixed in refactoring Phase 2–4.

Root causes and their fixes:

| Issue | Fix |
|-------|-----|
| `windowTail` could exceed `len(hist)` after truncation, causing summary to be silently dropped and old turns lost | `buildMessageHistory` now handles `windowTail >= len(hist)` by injecting the summary alone |
| Global mutable state (pending maps, compaction state) scattered across files | All maps migrated into `AgentDeps` struct with explicit locks |
| `droppedIDConv` TTL too short (30 min) — reconnect storms after long tasks couldn't find their session | TTL extended to 2 hours |
| `time.Tick` goroutines never stopped | Replaced with `AgentDeps.sweepTicker` using context cancellation |
| RecordTurn cap hardcoded at 50 | Made configurable via `SetMaxTurnsPerConversation()` |

### ~~2. Reasoning content silently discarded~~

**Status**: Fixed in Phase 4.5.

`collectSingleResponse` in `bugbot.go` previously discarded `reasoning` chunks. Now accumulated alongside content.

## Open

_None currently known._
