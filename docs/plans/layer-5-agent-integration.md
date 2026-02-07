# Layer 5: Agent Integration

**Theme:** Bridge from "agent-compatible" (JSON output exists) to "agent-native" (structured errors, schema introspection, batch operations, context generation). Based on patterns from Claude Code's task system, Linear's agent API, and the InfoQ article "Keep the Terminal Relevant: Patterns for AI Agent Driven CLIs."

**Target release:** v0.6.0
**Estimated effort:** ~5 days
**Prerequisites:** Layers 3-4 (error codes reference WIP limits, metrics command)

---

## 5.1 Structured Error Output (M)

### What

When `--json` is active, errors are written to stdout as JSON objects with a stable `code` field instead of plain text to stderr. Error codes are machine-readable strings that agents can switch on programmatically. This is the single most impactful change for agent interoperability.

### CLI Interface

```bash
kanban-md show 999 --json
# stdout: {"error":"task not found","code":"TASK_NOT_FOUND","details":{"id":999}}
# exit code: 1

kanban-md move 5 in-progress --json
# stdout: {"error":"WIP limit reached for \"in-progress\" (3/3)","code":"WIP_LIMIT_EXCEEDED","details":{"status":"in-progress","limit":3,"current":3}}
# exit code: 1

kanban-md create "" --json
# stdout: {"error":"title is required","code":"INVALID_INPUT","details":{"field":"title"}}
# exit code: 1

kanban-md list --status nonexistent --json
# stdout: {"error":"invalid status \"nonexistent\"","code":"INVALID_STATUS","details":{"status":"nonexistent","allowed":["backlog","todo","in-progress","review","done"]}}
# exit code: 1
```

### Error Code Taxonomy

All error codes are uppercase, underscore-separated, stable across minor versions.

| Code | Exit Code | When |
|------|-----------|------|
| `TASK_NOT_FOUND` | 1 | Task ID does not exist |
| `BOARD_NOT_FOUND` | 1 | No kanban board found in directory tree |
| `BOARD_ALREADY_EXISTS` | 1 | `init` when board already exists |
| `INVALID_INPUT` | 1 | Missing required input (empty title, missing ID) |
| `INVALID_STATUS` | 1 | Status not in configured list |
| `INVALID_PRIORITY` | 1 | Priority not in configured list |
| `INVALID_DATE` | 1 | Date not in YYYY-MM-DD format |
| `INVALID_TASK_ID` | 1 | Non-numeric task ID argument |
| `WIP_LIMIT_EXCEEDED` | 1 | Target column is at WIP capacity |
| `DEPENDENCY_NOT_FOUND` | 1 | Referenced dependency ID doesn't exist |
| `SELF_REFERENCE` | 1 | Task depends on or parents itself |
| `NO_CHANGES` | 1 | `edit` called with no change flags |
| `BOUNDARY_ERROR` | 1 | `--next` at last status or `--prev` at first |
| `STATUS_CONFLICT` | 1 | Both `--next`/`--prev` and explicit status |
| `CONFIRMATION_REQUIRED` | 1 | Non-TTY delete without `--force` |
| `INTERNAL_ERROR` | 2 | Unexpected filesystem/YAML errors |

### Go Implementation

Error response type in `internal/output/json.go`:

```go
type ErrorResponse struct {
    Error   string         `json:"error"`
    Code    string         `json:"code"`
    Details map[string]any `json:"details,omitempty"`
}

func JSONError(code string, msg string, details map[string]any) error {
    resp := ErrorResponse{Error: msg, Code: code, Details: details}
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    return enc.Encode(resp)
}
```

Typed error in commands:

```go
type CLIError struct {
    Code    string
    Message string
    Details map[string]any
}

func (e *CLIError) Error() string { return e.Message }
```

In `cmd/root.go`, intercept errors from `RunE`:

```go
// In Execute(), after cmd.ExecuteC() returns an error:
if jsonOutput {
    var cliErr *CLIError
    if errors.As(err, &cliErr) {
        output.JSONError(cliErr.Code, cliErr.Message, cliErr.Details)
    } else {
        output.JSONError("INTERNAL_ERROR", err.Error(), nil)
    }
    os.Exit(1)
}
```

### Files Affected

- `internal/output/json.go` -- `ErrorResponse`, `JSONError()`
- `cmd/root.go` -- Error interception in Execute()
- All `cmd/*.go` -- Return `*CLIError` instead of `fmt.Errorf`
- `internal/task/validate.go` -- Typed validation errors
- `internal/board/filter.go` -- Typed filter errors

---

## 5.2 Schema Command (S)

### What

New `schema` command that outputs JSON Schema definitions for command outputs. Agents use these to validate responses and understand the data contract without reading documentation.

### CLI Interface

```bash
kanban-md schema task
# {
#   "$schema": "https://json-schema.org/draft/2020-12/schema",
#   "type": "object",
#   "properties": {
#     "id": {"type": "integer"},
#     "title": {"type": "string"},
#     "status": {"type": "string"},
#     "priority": {"type": "string"},
#     "created": {"type": "string", "format": "date-time"},
#     "updated": {"type": "string", "format": "date-time"},
#     "assignee": {"type": "string"},
#     "tags": {"type": "array", "items": {"type": "string"}},
#     "due": {"type": "string", "format": "date"},
#     "estimate": {"type": "string"},
#     "parent": {"type": ["integer", "null"]},
#     "depends_on": {"type": "array", "items": {"type": "integer"}},
#     "blocked": {"type": "boolean"},
#     "block_reason": {"type": "string"},
#     "body": {"type": "string"},
#     "file": {"type": "string"}
#   },
#   "required": ["id", "title", "status", "priority", "created", "updated"]
# }

kanban-md schema list     # array of task objects
kanban-md schema error    # error response schema
kanban-md schema config   # config schema
kanban-md schema board    # board summary schema
kanban-md schema metrics  # metrics output schema
```

### Implementation

Static schemas stored as Go constants or embedded JSON files. No dynamic generation needed -- the schemas are maintained manually and tested against actual output in e2e tests.

```go
var schemas = map[string]string{
    "task":    taskSchema,
    "list":    listSchema,
    "error":   errorSchema,
    "config":  configSchema,
    "board":   boardSchema,
    "metrics": metricsSchema,
}
```

### Files Affected

- New: `cmd/schema.go` -- `schema` command
- New: `internal/output/schema.go` -- Static JSON Schema definitions

---

## 5.3 Batch Operations (M)

### What

`move`, `edit`, and `delete` accept comma-separated IDs for bulk operations. Operations execute per-task with partial success semantics -- if some tasks succeed and some fail, the successful operations are persisted and the failures are reported.

### CLI Interface

```bash
# Move multiple tasks
kanban-md move 1,2,3 in-progress
# Moved 3 tasks to "in-progress"

# Edit multiple tasks
kanban-md edit 1,2,3 --priority high --assignee alice
# Updated 3 tasks

# Delete multiple tasks
kanban-md delete 1,2,3 --force
# Deleted 3 tasks

# Mixed results (JSON output)
kanban-md move 1,2,3 in-progress --json
# [
#   {"id": 1, "status": "in-progress", "ok": true},
#   {"id": 2, "status": "in-progress", "ok": true},
#   {"id": 3, "ok": false, "error": "WIP limit reached", "code": "WIP_LIMIT_EXCEEDED"}
# ]
```

### Batch Result Type

```go
type BatchResult struct {
    ID    int    `json:"id"`
    OK    bool   `json:"ok"`
    Error string `json:"error,omitempty"`
    Code  string `json:"code,omitempty"`
}
```

### Semantics

- **Partial success:** Each task is processed independently. Failures don't roll back successes.
- **Exit code:** 0 if all succeed, 1 if any fail. This lets scripts check `$?` for full success.
- **Order:** Tasks are processed in the order specified.
- **WIP limits:** Checked per-task in order. If moving tasks 1,2,3 to a column with WIP limit 2 and 1 slot open, task 1 succeeds, tasks 2 and 3 fail.
- **Table output:** Summary line ("Moved 2/3 tasks") plus individual failure messages.

### ID Parsing

Comma-separated IDs are parsed in the argument handling:

```go
func parseIDs(arg string) ([]int, error) {
    parts := strings.Split(arg, ",")
    ids := make([]int, 0, len(parts))
    for _, p := range parts {
        id, err := strconv.Atoi(strings.TrimSpace(p))
        if err != nil {
            return nil, fmt.Errorf("invalid task ID %q", p)
        }
        ids = append(ids, id)
    }
    return ids, nil
}
```

### Backward Compatibility

Single IDs work exactly as before. The change is purely additive -- `move 1 done` and `move 1,2,3 done` use the same code path.

### Files Affected

- `cmd/move.go` -- Multi-ID parsing, loop with result collection
- `cmd/edit.go` -- Multi-ID parsing, loop with result collection
- `cmd/delete.go` -- Multi-ID parsing, loop with result collection
- `internal/output/json.go` -- `BatchResult` type

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestBatchMoveAll` | Move 3 tasks, all succeed |
| `TestBatchMovePartialFailure` | WIP limit causes partial failure |
| `TestBatchEditMultiple` | Edit 3 tasks with same flags |
| `TestBatchDeleteMultiple` | Delete 3 tasks with --force |
| `TestBatchExitCode` | Exit 0 on full success, 1 on any failure |
| `TestBatchSingleIDBackcompat` | Single ID still works as before |

---

## 5.4 Context Generation (S)

### What

New `context` command that generates a markdown summary of the current board state. Designed for inclusion in `CLAUDE.md`, `AGENTS.md`, or any file where an AI agent needs to understand the project's task landscape.

### CLI Interface

```bash
kanban-md context
# ## kanban-md Board: My Project
#
# ### Summary
# 24 tasks total | 4 in-progress (3/3 WIP) | 2 blocked | 1 overdue
#
# ### In Progress
# - #4 "Auth flow" — high, alice, 2 days old
# - #5 "Database migration" — critical, bob, 1 day old
# - #7 "API integration" — medium, alice, BLOCKED: waiting on keys
#
# ### Blocked
# - #7 "API integration" (in-progress) — Waiting on API keys from vendor
#
# ### Ready to Start (unblocked, in todo)
# - #8 "Write unit tests" — high
# - #9 "Update docs" — medium
#
# ### Overdue
# - #6 "Q1 report" — due 2026-02-01 (6 days overdue)

# Write directly to a file
kanban-md context --write-to CLAUDE.md

# Control detail level
kanban-md context --sections in-progress,blocked,ready

# JSON mode (structured data for agent processing)
kanban-md context --json
```

### Context Template

The default output includes these sections (in order):

1. **Summary line** -- Total tasks, in-progress count (with WIP), blocked count, overdue count
2. **In Progress** -- All non-terminal, non-backlog tasks, sorted by priority
3. **Blocked** -- All blocked tasks with reasons
4. **Ready to Start** -- Unblocked tasks in the second status (e.g., "todo"), sorted by priority
5. **Overdue** -- Tasks past their due date

Sections with no items are omitted.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--write-to FILE` | stdout | Write to file instead of stdout |
| `--sections` | all | Comma-separated section list to include |
| `--days N` | 7 | "Recently completed" lookback period |

### Files Affected

- New: `cmd/context.go` -- `context` command
- New: `internal/board/context.go` -- Generate markdown context

---

## 5.5 Config Command (S)

### What

New `config` command to read and modify config values programmatically. Agents should not need to parse YAML to inspect board settings.

### CLI Interface

```bash
# Show full config (table or JSON)
kanban-md config
kanban-md config --json

# Get specific value
kanban-md config get board.name
# "My Project"

kanban-md config get statuses
# ["backlog","todo","in-progress","review","done"]

kanban-md config get defaults.priority
# "medium"

# Set a value
kanban-md config set defaults.priority high
kanban-md config set board.name "New Name"
```

### Supported Keys

| Key | Type | Writable |
|-----|------|----------|
| `board.name` | string | yes |
| `board.description` | string | yes |
| `statuses` | []string | no (use init) |
| `priorities` | []string | no (use init) |
| `defaults.status` | string | yes |
| `defaults.priority` | string | yes |
| `tasks_dir` | string | no |
| `next_id` | int | no |
| `version` | int | no |
| `terminal_statuses` | []string | yes |

Read-only keys are marked as such to prevent accidental corruption (e.g., changing `next_id` could cause ID conflicts).

### Implementation

Use a key-to-accessor map rather than reflection:

```go
var configAccessors = map[string]struct {
    Get func(*config.Config) any
    Set func(*config.Config, string) error
}{
    "board.name": {
        Get: func(c *config.Config) any { return c.Board.Name },
        Set: func(c *config.Config, v string) error { c.Board.Name = v; return nil },
    },
    // ...
}
```

### Files Affected

- New: `cmd/config.go` -- `config` command with `get`/`set` subcommands
- `internal/config/config.go` -- Accessor methods

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestConfigShowAll` | Displays full config |
| `TestConfigGetBoardName` | Returns board name |
| `TestConfigGetStatuses` | Returns status list as JSON array |
| `TestConfigSetDefaultPriority` | Changes and persists default priority |
| `TestConfigSetReadOnlyKey` | Error on read-only keys |
| `TestConfigGetInvalidKey` | Error for unknown key |

---

## Implementation Order

1. **5.1 Structured Error Output** -- Foundation for all agent interaction; other features build on this
2. **5.5 Config Command** -- Small, independent, immediately useful
3. **5.4 Context Generation** -- Small, high-value for agent workflows
4. **5.3 Batch Operations** -- Moderate, builds on existing command structure
5. **5.2 Schema Command** -- Last, because schemas should reflect final state of other features
