# Layer 6: MCP Server

**Theme:** Expose kanban-md as a Model Context Protocol server, making it a first-class tool for AI agent frameworks. MCP is the emerging standard for AI tool integration, adopted by Anthropic (November 2024), OpenAI (March 2025), and Google.

**Target release:** v1.0.0 (marquee feature for 1.0)
**Estimated effort:** ~5 days
**Prerequisites:** Layers 3-5 (all features should be stable before exposing via MCP)

---

## Background: Why MCP

MCP (Model Context Protocol) is an open standard that allows AI agents to discover and use external tools through a structured protocol. Instead of agents parsing CLI output or constructing shell commands, they call typed tool functions with validated parameters and receive structured responses.

**Current state:** kanban-md is already agent-friendly via `--json` output, but agents must:
1. Know how to construct CLI commands (string manipulation)
2. Parse stdout/stderr and exit codes
3. Handle shell escaping and quoting

**With MCP:** Agents call `kanban_create(title="Fix bug", priority="high")` and receive a typed response. No shell, no string parsing, no escaping.

### Reference Implementations

- **kanban-mcp** (github.com/eyalzh/kanban-mcp) -- SQLite-based, 7 tools, web UI. Written in TypeScript.
- **TaskBoardAI** (github.com/TuckerTucker/TaskBoardAI) -- JSON file storage, subtasks, dependencies. TypeScript.
- **Backlog.md** (github.com/MrLesk/Backlog.md) -- Markdown files, MCP integration via separate process. Python.

kanban-md would be the first **Go-native, single-binary MCP Kanban server** with file-based storage.

---

## 6.1 MCP Server Core (L)

### What

New `mcp` subcommand that starts a stdio-based MCP server. The server exposes all board operations as MCP tools with JSON Schema-typed parameters and responses. It runs as a long-lived process communicating over stdin/stdout using the MCP JSON-RPC protocol.

### CLI Interface

```bash
# Start MCP server (stdio transport -- used by AI clients)
kanban-md mcp --dir ./kanban

# The server reads JSON-RPC requests from stdin, writes responses to stdout.
# All logging goes to stderr.
```

### Configuration Examples

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "kanban": {
      "command": "kanban-md",
      "args": ["mcp", "--dir", "/Users/you/project/kanban"]
    }
  }
}
```

**Claude Code** (`.claude/settings.json` or via CLI):
```bash
claude mcp add kanban -- kanban-md mcp --dir ./kanban
```

**Cursor** (`.cursor/mcp.json`):
```json
{
  "mcpServers": {
    "kanban": {
      "command": "kanban-md",
      "args": ["mcp", "--dir", "./kanban"]
    }
  }
}
```

### MCP Tool Definitions

Each tool maps to an existing CLI command. The tool name is prefixed with `kanban_` to avoid collisions with other MCP servers.

#### `kanban_list`

```json
{
  "name": "kanban_list",
  "description": "List tasks with optional filtering and sorting. Returns an array of task objects.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "status": {
        "type": "string",
        "description": "Filter by status (comma-separated for multiple)"
      },
      "priority": {
        "type": "string",
        "description": "Filter by priority (comma-separated for multiple)"
      },
      "assignee": {
        "type": "string",
        "description": "Filter by assignee"
      },
      "tag": {
        "type": "string",
        "description": "Filter by tag"
      },
      "sort": {
        "type": "string",
        "enum": ["id", "status", "priority", "created", "updated", "due"],
        "description": "Sort field (default: id)"
      },
      "reverse": {
        "type": "boolean",
        "description": "Reverse sort order"
      },
      "limit": {
        "type": "integer",
        "description": "Max results (0 = unlimited)"
      },
      "unblocked": {
        "type": "boolean",
        "description": "Only show tasks with all dependencies completed"
      },
      "blocked": {
        "type": "boolean",
        "description": "Only show manually blocked tasks"
      }
    }
  }
}
```

#### `kanban_show`

```json
{
  "name": "kanban_show",
  "description": "Show full details of a task including its body, dependencies, and metadata.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "id": {
        "type": "integer",
        "description": "Task ID"
      }
    },
    "required": ["id"]
  }
}
```

#### `kanban_create`

```json
{
  "name": "kanban_create",
  "description": "Create a new task. Returns the created task object with its assigned ID.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "title": {"type": "string", "description": "Task title (required)"},
      "status": {"type": "string", "description": "Initial status (default: config default)"},
      "priority": {"type": "string", "description": "Priority level (default: config default)"},
      "assignee": {"type": "string", "description": "Person assigned"},
      "tags": {"type": "string", "description": "Comma-separated tags"},
      "due": {"type": "string", "description": "Due date (YYYY-MM-DD)"},
      "estimate": {"type": "string", "description": "Time estimate (e.g., 4h, 2d)"},
      "body": {"type": "string", "description": "Task description (markdown)"},
      "depends_on": {"type": "string", "description": "Comma-separated dependency IDs"},
      "parent": {"type": "integer", "description": "Parent task ID"}
    },
    "required": ["title"]
  }
}
```

#### `kanban_edit`

```json
{
  "name": "kanban_edit",
  "description": "Modify an existing task. Only specified fields are changed.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "id": {"type": "integer", "description": "Task ID (required)"},
      "title": {"type": "string"},
      "status": {"type": "string"},
      "priority": {"type": "string"},
      "assignee": {"type": "string"},
      "add_tag": {"type": "string", "description": "Tags to add (comma-separated)"},
      "remove_tag": {"type": "string", "description": "Tags to remove (comma-separated)"},
      "due": {"type": "string", "description": "Due date (YYYY-MM-DD)"},
      "clear_due": {"type": "boolean"},
      "estimate": {"type": "string"},
      "body": {"type": "string"},
      "block": {"type": "string", "description": "Block reason (sets blocked=true)"},
      "unblock": {"type": "boolean", "description": "Clear blocked state"}
    },
    "required": ["id"]
  }
}
```

#### `kanban_move`

```json
{
  "name": "kanban_move",
  "description": "Change a task's status. Use 'status' for direct move, 'next'/'prev' for relative move.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "id": {"type": "integer", "description": "Task ID (required)"},
      "status": {"type": "string", "description": "Target status"},
      "next": {"type": "boolean", "description": "Move to next status"},
      "prev": {"type": "boolean", "description": "Move to previous status"},
      "force": {"type": "boolean", "description": "Override WIP limits"}
    },
    "required": ["id"]
  }
}
```

#### `kanban_delete`

```json
{
  "name": "kanban_delete",
  "description": "Delete a task. Always force-deletes (no confirmation prompt in MCP mode).",
  "inputSchema": {
    "type": "object",
    "properties": {
      "id": {"type": "integer", "description": "Task ID (required)"}
    },
    "required": ["id"]
  }
}
```

#### `kanban_board`

```json
{
  "name": "kanban_board",
  "description": "Get board summary: task counts per status, WIP utilization, blocked/overdue counts.",
  "inputSchema": {
    "type": "object",
    "properties": {}
  }
}
```

#### `kanban_metrics`

```json
{
  "name": "kanban_metrics",
  "description": "Get flow metrics: throughput, cycle time, lead time, aging work items.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "since": {"type": "string", "description": "Start date (YYYY-MM-DD)"},
      "by_status": {"type": "boolean", "description": "Include per-status breakdown"}
    }
  }
}
```

### Architecture

```
cmd/mcp.go
  └── Creates MCP server, registers tools
       └── internal/mcp/server.go
            ├── NewServer(kanbanDir) -- initializes server with board directory
            ├── Registers tools via mcp-go SDK
            └── Routes tool calls to handlers

internal/mcp/
  ├── server.go    -- Server creation, tool registration, lifecycle
  ├── tools.go     -- Tool name/description/schema definitions
  └── handlers.go  -- Handler functions (one per tool)
        ├── handleList(params) -> calls board.List()
        ├── handleShow(params) -> calls task.FindByID() + task.Read()
        ├── handleCreate(params) -> calls task.Write() + config.Save()
        ├── handleEdit(params) -> calls task.FindByID() + task.Write()
        ├── handleMove(params) -> calls handleEdit with status change
        ├── handleDelete(params) -> calls os.Remove()
        ├── handleBoard(params) -> calls board.Summary()
        └── handleMetrics(params) -> calls board.Metrics()
```

**Key design principle:** MCP handlers call the same internal packages (`board`, `task`, `config`) that CLI commands use. They do NOT shell out to the binary. This makes the MCP server fast (no process spawn per request) and testable (unit tests on handlers directly).

### Dependency: mcp-go

**Library:** `github.com/mark3labs/mcp-go` (the standard Go MCP SDK)

Evaluation:
- MIT licensed
- Implements the MCP 2025-11-25 specification
- Supports stdio transport (what we need)
- Used by 100+ Go MCP servers
- Well-maintained, active development
- Minimal dependency footprint

```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

func newMCPServer(kanbanDir string) *server.MCPServer {
    s := server.NewMCPServer(
        "kanban-md",
        "1.0.0",
        server.WithToolCapabilities(true),
        server.WithResourceCapabilities(true, false),
    )

    s.AddTool(listTool, makeListHandler(kanbanDir))
    s.AddTool(showTool, makeShowHandler(kanbanDir))
    // ... register all tools

    return s
}
```

### MCP Delete Behavior

In MCP mode, `kanban_delete` always force-deletes (no confirmation prompt). The agent has explicitly called the delete tool -- that's sufficient intent. This matches how other MCP tools work (no interactive confirmations in machine-to-machine protocols).

### Error Handling in MCP

MCP tool calls return errors as structured responses (not exceptions). The mcp-go SDK handles this:

```go
func makeShowHandler(dir string) server.ToolHandlerFunc {
    return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        id, err := getIntParam(request, "id")
        if err != nil {
            return mcp.NewToolResultError("invalid task ID"), nil
        }

        cfg, err := config.Load(dir)
        if err != nil {
            return mcp.NewToolResultError("board not found: " + err.Error()), nil
        }

        t, err := task.FindByID(cfg.TasksPath(), id)
        if err != nil {
            return mcp.NewToolResultError("task not found"), nil
        }

        data, _ := json.Marshal(t)
        return mcp.NewToolResultText(string(data)), nil
    }
}
```

### Testing Strategy

1. **Unit tests for handlers** (`internal/mcp/handlers_test.go`): Test each handler function directly with mock parameters and a temp board directory. No MCP protocol involved.

2. **Integration test** (`e2e/mcp_test.go`): Start the MCP server as a subprocess, send JSON-RPC requests via stdin, validate responses from stdout. Tests the full protocol stack.

3. **Manual testing**: Use Claude Desktop or Claude Code with the MCP config to verify end-to-end agent interaction.

### Files Affected

- New: `cmd/mcp.go` -- `mcp` subcommand, server startup
- New: `internal/mcp/server.go` -- Server creation and tool registration
- New: `internal/mcp/tools.go` -- Tool definitions (names, descriptions, schemas)
- New: `internal/mcp/handlers.go` -- Handler functions calling internal packages
- New: `internal/mcp/handlers_test.go` -- Unit tests
- `go.mod` -- Add `github.com/mark3labs/mcp-go`

---

## 6.2 MCP Resources & Prompts (M)

### What

In addition to tools (functions agents can call), MCP supports:
- **Resources**: Read-only data that agents can access (like files or API endpoints)
- **Prompts**: Pre-built prompt templates that agents can use for common workflows

### MCP Resources

Resources provide a way for agents to read board state without calling a tool. They're analogous to REST GET endpoints.

```
kanban://board          -> Board summary (same as kanban_board tool output)
kanban://tasks          -> Full task list as JSON array
kanban://task/{id}      -> Individual task detail
kanban://config         -> Board configuration
kanban://log            -> Recent activity log entries (last 50)
```

Implementation:

```go
s.AddResource(
    mcp.NewResource("kanban://board", "Board summary", "application/json"),
    makeBoardResourceHandler(kanbanDir),
)

s.AddResource(
    mcp.NewResourceTemplate("kanban://task/{id}", "Task detail", "application/json"),
    makeTaskResourceHandler(kanbanDir),
)
```

### MCP Prompts

Prompts are pre-built templates that agents can request and fill in. They're useful for common workflows.

#### `plan-project`

```json
{
  "name": "plan-project",
  "description": "Decompose a project goal into kanban tasks with priorities and dependencies",
  "arguments": [
    {
      "name": "goal",
      "description": "The project goal or feature to decompose",
      "required": true
    },
    {
      "name": "max_tasks",
      "description": "Maximum number of tasks to create (default: 10)",
      "required": false
    }
  ]
}
```

Returns a prompt like:
```
You are a project planner. Given the following goal, decompose it into concrete, actionable kanban tasks.

Goal: {goal}

Current board state:
{board_summary}

Available statuses: {statuses}
Available priorities: {priorities}

Create up to {max_tasks} tasks. For each task, provide:
- Title (imperative, specific)
- Priority (from the available list)
- Dependencies (which other tasks must complete first)
- Estimate (time estimate like "2h", "1d")

Output as a list of kanban-md create commands.
```

#### `daily-standup`

```json
{
  "name": "daily-standup",
  "description": "Generate a daily standup summary from recent board activity",
  "arguments": [
    {
      "name": "days",
      "description": "Lookback period in days (default: 1)",
      "required": false
    }
  ]
}
```

Returns a prompt with recent activity data injected.

### Files Affected

- New: `internal/mcp/resources.go` -- Resource handlers
- New: `internal/mcp/prompts.go` -- Prompt template handlers
- `internal/mcp/server.go` -- Register resources and prompts

---

## Implementation Order

1. **6.1 MCP Server Core** -- Build server with all tools, test thoroughly
2. **6.2 Resources & Prompts** -- Add after core tools are working

The MCP server should be the last major feature before declaring v1.0.0, as it exposes the entire tool surface and any API instability would be amplified.
