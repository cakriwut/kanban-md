# kanban-md Feature Roadmap: Layers 3-7

Layers 0-2 are complete and shipping (v0.3.1). This roadmap defines the next 5 layers of development, each independently useful and shippable.

**Research basis:** Kanban methodology (David J. Anderson, Jim Benson), AI agent task management (Claude Code, Cursor, Devin, OpenHands), competitive analysis (Backlog.md, kanban-mcp, Vibe Kanban, Linear's agent API).

## Layer Overview

| Layer | Theme | Marquee Feature | Effort | Proposal |
|-------|-------|-----------------|--------|----------|
| [3](layer-3-kanban-discipline.md) | Kanban Discipline | WIP limits + dependencies | ~5 days | [Details](layer-3-kanban-discipline.md) |
| [4](layer-4-metrics-observability.md) | Metrics & Observability | Cycle time, throughput, activity log | ~5 days | [Details](layer-4-metrics-observability.md) |
| [5](layer-5-agent-integration.md) | Agent Integration | Structured errors, batch ops, context gen | ~5 days | [Details](layer-5-agent-integration.md) |
| [6](layer-6-mcp-server.md) | MCP Server | Model Context Protocol integration | ~5 days | [Details](layer-6-mcp-server.md) |
| [7](layer-7-multi-agent-orchestration.md) | Multi-Agent Orchestration | Claim/release, pick, classes of service | ~6 days | [Details](layer-7-multi-agent-orchestration.md) |

## Release Strategy

| Release | Layer | Marquee Feature |
|---------|-------|-----------------|
| v0.4.0 | 3 | WIP limits + dependencies |
| v0.5.0 | 4 | Metrics + activity log |
| v0.6.0 | 5 | Agent-native (structured errors, batch ops) |
| v1.0.0 | 6 | MCP server |
| v1.1.0 | 7 | Multi-agent orchestration |

## What NOT to Build

These features don't fit the tool's philosophy ("files are the API, single binary, zero dependencies"):

| Feature | Why not |
|---------|---------|
| **Web UI / TUI board view** | Rendering is a separate concern. A `kanban-md-tui` project using Bubbletea could exist, but not in the core binary. |
| **Database backend** | Breaks "files are the API." Tasks must stay grep-able, diff-able, git-tracked. |
| **Sync / real-time** | Git is the sync mechanism. No WebSockets, no file watchers, no daemons. |
| **Notifications / webhooks** | Requires a running process. Use git hooks or CI to trigger notifications. |
| **Auth / access control** | No server = no auth. File permissions are OS-level. |
| **Timer / time tracking** | Requires a running process. External tools can read task files. |
| **Import from Jira/Linear** | API clients for moving targets. Better as a separate `kanban-md-import` script. |
| **Plugin system** | Conflicts with single-binary goal. JSON output enables composition via pipes. |
| **Per-column policies** | Config complexity for marginal value. Better as documented team conventions or CI scripts. |
| **Watch mode / event streaming** | File watchers add platform-specific complexity and a long-running process. Activity log serves the audit trail. |

## Critical Files (Most Impacted Across All Layers)

- `internal/config/config.go` -- Nearly every layer adds config fields
- `internal/task/task.go` -- Task struct gains fields in layers 3, 4, 7
- `internal/board/filter.go` -- Filter logic expands in layers 3, 5, 7
- `cmd/move.go` -- WIP checks, idempotency, claim checks, auto-timestamps, log entries converge here
- `cmd/edit.go` -- Blocked state, claim/release, dependencies, class of service route through here

## Verification (Per Layer)

1. `go test -race ./...` -- all tests pass (unit + e2e)
2. `golangci-lint run ./...` -- zero issues
3. E2e tests added for each new command and feature
4. Manual workflow test of the full layer
5. README updated for user-facing changes
