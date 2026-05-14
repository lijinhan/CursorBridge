# Debug Log Capture Standard Process

## Overview

cursor-byok uses a build-tag-based debug logging system. Diagnostic logs are compiled into debug builds only (`-tags=debug`) and completely eliminated from release builds, ensuring zero runtime overhead in production.

## Build Commands

### Debug Build (with diagnostic logs)

```bash
go build -tags=debug -o cursor-byok-debug.exe .
```

### Release Build (no diagnostic logs)

```bash
go build -o cursor-byok.exe .
```

## Capturing Logs

### Step 1: Build Debug Version

```bash
go build -tags=debug -o cursor-byok-debug.exe .
```

### Step 2: Run with stderr Redirected to Log File

```bash
cursor-byok-debug.exe 2>tool_debug_stderr.log
```

All `[TOOL-EXEC]`, `[SESSION]`, `[RUNSSE]`, `[HISTORY]`, `[HISTORY-DISK]`, and `[DEBUG]` prefixed logs are written to stderr via `log.Printf`, so redirecting stderr captures everything.

### Step 3: Reproduce the Problem

Use Cursor IDE normally until the issue occurs. The debug log will record:
- Tool call registration and result delivery (`[TOOL-EXEC]`)
- Shell stream milestones (Start/Exit with stdout/stderr lengths)
- Session lifecycle (`[SESSION]`)
- SSE request/response flow (`[RUNSSE]`)
- History persistence (`[HISTORY]`, `[HISTORY-DISK]`)
- System prompt extraction (`[DEBUG]`)

### Step 4: Analyze the Log

```bash
# Filter by specific subsystem
grep "\[TOOL-EXEC\]" tool_debug_stderr.log
grep "\[SESSION\]" tool_debug_stderr.log
grep "\[RUNSSE\]" tool_debug_stderr.log

# Find critical events
grep "NO WAITER\|TIMED OUT\|DROPPING" tool_debug_stderr.log
grep "deliverToolResult\|waitForToolResult" tool_debug_stderr.log

# Find Shell stream milestones (Start/Exit only, not per-chunk)
grep "ShellStream Start\|ShellStream Exit" tool_debug_stderr.log
```

## Log Prefix Reference

| Prefix | Source File | Description |
|--------|-------------|-------------|
| `[TOOL-EXEC]` | `tool_exec.go` | Tool call registration, result routing, delivery, Shell stream milestones |
| `[SESSION]` | `session.go` | Session creation, lookup, cloning |
| `[RUNSSE]` | `runsse.go` | SSE request/response flow, model interaction |
| `[HISTORY]` | `history.go` | Conversation history lookup, turn recording |
| `[HISTORY-DISK]` | `history_disk.go` | Disk persistence of conversation turns |
| `[DEBUG]` | `bidi.go` | System prompt extraction, BidiAppend message routing |

## Architecture

The `internal/debuglog` package provides conditional compilation:

- `debuglog.go` (`//go:build !debug`) — Release: `Printf` is a no-op, compiler eliminates call + arguments
- `debuglog_debug.go` (`//go:build debug`) — Debug: `Printf` calls `log.Printf` with `[DEBUG]` output

All diagnostic logging throughout the codebase uses `debuglog.Printf()` instead of `log.Printf()`, ensuring logs are only present when needed for debugging.