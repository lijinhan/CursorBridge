# CursorBridge — Project Guide

## Build & Run

```bash
go build ./...                    # Go build check
cd frontend && npm run build      # Frontend build
wails3 dev                        # Dev mode (Go + Vue hot reload)
wails3 build                      # Production build
```

## Test

```bash
go test ./internal/agent/...      # Agent package tests (21 tests)
go test ./...                     # All Go tests
```

## Architecture

Go module: `cursorbridge`

### Key Files

- `internal/agent/runsse.go` — HandleRunSSE orchestrator + types
- `internal/agent/runsse_setup.go` — Session init + keepalive
- `internal/agent/runsse_loop.go` — Main streaming loop
- `internal/agent/runsse_finalize.go` — Persistence + cleanup
- `internal/agent/toolbuilder.go` — Tool builder registry + dispatch
- `internal/agent/toolbuilder_mcp.go` — MCP tool builders
- `internal/agent/toolbuilder_fs.go` — File system tool builders (StrReplace, Read, Write, Delete)
- `internal/agent/toolbuilder_exec.go` — Shell, Glob, Grep, ReadLints builders
- `internal/agent/deps.go` — AgentDeps (global mutable state)
- `internal/mitm/proxy.go` — MITM proxy + path routing
- `internal/relay/gateway.go` — Request/response rewriting
- `internal/bridge/proxy_service.go` — Wails frontend bindings

### Frontend

- `frontend/src/types.ts` — TypeScript type definitions
- `frontend/src/i18n.ts` — i18n module (zh-CN + en)
- `frontend/src/stores/proxy.ts` — Pinia store
- `frontend/src/components/ProxyDashboard.vue` — Main dashboard (being decomposed)

## Key Patterns

- **AdapterResolver** is an interface `{ Resolve() []AdapterTarget }`, not a func type
- **toolBuilder** is a func type `func(pc PendingToolCall) (tool, errMsg)` registered in `toolBuilderRegistry`
- **AgentDeps** holds all package-level mutable state; `DefaultDeps` is the singleton
- **setupResult/loopResult** structs carry intermediate state between decomposition phases
- Unix timestamps (int64) in setupResult, not time.Time (for serializable state)
- `parseToolArgs[T]` / `parseToolArgsPartial[T]` are generic helpers for tool arg unmarshaling

## Conventions

- Go: no comments unless WHY is non-obvious
- Commit messages: Conventional Commits (refactor/feat/fix/docs/test)
- Frontend: Vue 3 + TypeScript, Pinia for state, scoped styles