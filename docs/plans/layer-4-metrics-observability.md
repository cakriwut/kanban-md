# Layer 4: Metrics & Observability

**Theme:** Make work visible with computed metrics, activity tracking, and board-level summaries. Kanban is a feedback-driven system -- without metrics, teams cannot identify bottlenecks, predict delivery, or measure improvement.

**Target release:** v0.5.0
**Estimated effort:** ~5 days
**Prerequisites:** Layer 3 (WIP limits used in board summary display)

---

## 4.1 Board Summary Command (S)

### What

New `board` command showing a high-level overview: task counts per status, WIP utilization, priority distribution, blocked count, and overdue count. This is the "dashboard" view of the board without listing individual tasks.

### CLI Interface

```bash
kanban-md board

# Table output:
# Board: My Project
# ──────────────────────────────────────────
# STATUS          COUNT   WIP     BLOCKED
# backlog            5     -        -
# todo               3     -        -
# in-progress        3   3/3 !!    1
# review             1   1/2       -
# done              12     -        -
# ──────────────────────────────────────────
# Total: 24 tasks | 2 blocked | 1 overdue
#
# Priority distribution:
#   critical: 2 | high: 5 | medium: 12 | low: 5

kanban-md board --json
# {
#   "name": "My Project",
#   "total_tasks": 24,
#   "blocked_count": 2,
#   "overdue_count": 1,
#   "statuses": [
#     {"name": "backlog", "count": 5, "wip_limit": 0, "blocked": 0},
#     {"name": "in-progress", "count": 3, "wip_limit": 3, "blocked": 1},
#     ...
#   ],
#   "priorities": [
#     {"name": "critical", "count": 2},
#     {"name": "high", "count": 5},
#     ...
#   ]
# }
```

### Data Model

New summary struct in `internal/board/board.go`:

```go
type BoardSummary struct {
    Name          string           `json:"name"`
    TotalTasks    int              `json:"total_tasks"`
    BlockedCount  int              `json:"blocked_count"`
    OverdueCount  int              `json:"overdue_count"`
    Statuses      []StatusSummary  `json:"statuses"`
    Priorities    []PrioritySummary `json:"priorities"`
}

type StatusSummary struct {
    Name     string `json:"name"`
    Count    int    `json:"count"`
    WIPLimit int    `json:"wip_limit"`
    Blocked  int    `json:"blocked"`
}

type PrioritySummary struct {
    Name  string `json:"name"`
    Count int    `json:"count"`
}
```

### Files Affected

- New: `cmd/board.go` -- `board` command
- `internal/board/board.go` -- `Summary(cfg, tasks)` function
- `internal/output/table.go` -- `BoardSummary()` renderer

---

## 4.2 Start/Complete Timestamps (M)

### What

Add `started` and `completed` timestamps to tasks, automatically set when a task transitions through status columns:

- **`started`**: Set the first time a task moves out of the initial status (first status in config). Once set, never overwritten (even if task moves back to backlog and forward again).
- **`completed`**: Set when a task enters a terminal status. Cleared if the task moves back out of a terminal status (reopening).

These timestamps enable cycle time and lead time computation.

### CLI Interface

```bash
# Automatic -- no explicit flags needed
kanban-md move 1 in-progress
# started field auto-set to now (if not already set)

kanban-md move 1 done
# completed field auto-set to now

kanban-md show 1 --json
# {
#   "id": 1,
#   "started": "2026-02-03T14:30:00Z",
#   "completed": "2026-02-05T09:15:00Z",
#   ...
# }

# Show computes lead/cycle time from timestamps
kanban-md show 1
#   Created:    2026-02-01 10:00
#   Started:    2026-02-03 14:30
#   Completed:  2026-02-05 09:15
#   Lead time:  4d 23h
#   Cycle time: 1d 18h 45m
```

### Data Model Changes

Add to Task struct in `internal/task/task.go`:

```go
Started   *time.Time `yaml:"started,omitempty" json:"started,omitempty"`
Completed *time.Time `yaml:"completed,omitempty" json:"completed,omitempty"`
```

### Transition Logic

In `cmd/move.go`, after determining the target status:

```go
now := time.Now()
initialStatus := cfg.Statuses[0].Name
terminalStatuses := cfg.TerminalStatusSet()

// Set started on first move out of initial status
if t.Started == nil && t.Status == initialStatus && targetStatus != initialStatus {
    t.Started = &now
}

// Set/clear completed based on terminal status
if terminalStatuses[targetStatus] {
    t.Completed = &now
} else if t.Completed != nil {
    // Reopening: clear completed timestamp
    t.Completed = nil
}
```

### Metric Computation

Display in `show` output:

```go
func formatDuration(d time.Duration) string {
    days := int(d.Hours()) / 24
    hours := int(d.Hours()) % 24
    if days > 0 {
        return fmt.Sprintf("%dd %dh", days, hours)
    }
    return fmt.Sprintf("%dh %dm", hours, int(d.Minutes())%60)
}

// Lead time: created -> completed (or now if not completed)
// Cycle time: started -> completed (or now if not completed)
```

### Graceful Degradation

Pre-existing tasks (created before this feature) will have `nil` for `started` and `completed`. The metrics system handles this:

- **No `started`:** Cycle time shows as "--" or "N/A"
- **No `completed`:** Task is in-progress; work item age is computed as `now - started` (or `now - created` if no started)
- **Backfill option:** Users can manually set timestamps via `kanban-md edit 1 --started 2026-02-03` if they want to backfill historical data

### Edge Cases

- **Direct move to terminal status:** Both `started` and `completed` set to now (cycle time = 0).
- **Move back from terminal status (reopen):** `completed` is cleared; `started` is preserved.
- **Move back to initial status:** `started` is preserved (it was set when first started).
- **Task created directly in terminal status:** `completed` set on creation; `started` not set (never started work).

### Files Affected

- `internal/task/task.go` -- `Started`, `Completed` fields
- `cmd/move.go` -- Auto-set timestamps on transitions
- `cmd/edit.go` -- Optional `--started`, `--completed` flags for manual override/backfill
- `internal/output/table.go` -- Display lead/cycle time in detail view

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestMoveAutoSetsStarted` | First move from initial status sets started |
| `TestMoveAutoSetsCompleted` | Move to terminal status sets completed |
| `TestReopenClearsCompleted` | Moving out of terminal clears completed |
| `TestStartedNotOverwritten` | Second move doesn't change started |
| `TestShowDisplaysLeadCycleTime` | Show output includes computed times |
| `TestPreExistingTasksGraceful` | Tasks without timestamps show "--" for metrics |

---

## 4.3 Metrics Command (M)

### What

New `metrics` command computing Kanban flow metrics across the board. All metrics are derived from task timestamps (created, started, completed).

### CLI Interface

```bash
# Default: last 30 days
kanban-md metrics
# Throughput (7d):      5 tasks completed
# Throughput (30d):    18 tasks completed
# Avg lead time:      3.2 days
# Avg cycle time:     1.8 days
# Flow efficiency:   56.3%
# ──────────────────────────────────────
# Aging work items (in-progress):
#   #7  "API integration"   4 days (!!!)
#   #5  "Database migration" 2 days
#   #4  "Auth flow"          1 day

# Custom time range
kanban-md metrics --since 2026-01-01

# Per-status breakdown
kanban-md metrics --by-status
# STATUS          AVG TIME    TASKS
# todo            1.2 days    15
# in-progress     2.1 days    12
# review          0.5 days    10

# JSON for dashboards
kanban-md metrics --json
```

### Metric Definitions

**Lead time:** Total time from creation to completion.
```
lead_time = completed - created
```
Only computed for tasks with both `created` and `completed` timestamps.

**Cycle time:** Active work time from start to completion.
```
cycle_time = completed - started
```
Only computed for tasks with both `started` and `completed` timestamps.

**Throughput:** Number of tasks completed in a time period.
```
throughput_7d  = count(tasks where completed >= now - 7 days)
throughput_30d = count(tasks where completed >= now - 30 days)
```

**Work item age:** How long an in-progress task has been active.
```
age = now - started     (if started is set)
age = now - created     (fallback if started is nil)
```
Only applies to non-terminal tasks. Tasks exceeding the average cycle time are flagged.

**Flow efficiency:** Ratio of active work time to total lead time. Approximated as:
```
flow_efficiency = avg_cycle_time / avg_lead_time * 100
```

### JSON Output Schema

```json
{
  "period_start": "2026-01-08T00:00:00Z",
  "period_end": "2026-02-07T00:00:00Z",
  "throughput_7d": 5,
  "throughput_30d": 18,
  "avg_lead_time_hours": 76.8,
  "avg_cycle_time_hours": 43.2,
  "flow_efficiency_pct": 56.3,
  "completed_tasks": 18,
  "aging_items": [
    {
      "id": 7,
      "title": "API integration",
      "status": "in-progress",
      "age_hours": 96.0,
      "exceeds_avg": true
    }
  ],
  "by_status": [
    {
      "status": "todo",
      "avg_time_hours": 28.8,
      "task_count": 15
    }
  ]
}
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | 30 days ago | Start of period (YYYY-MM-DD) |
| `--by-status` | false | Show per-status time breakdown |
| `--json` | auto | Force JSON output |

### Files Affected

- New: `cmd/metrics.go` -- `metrics` command
- New: `internal/board/metrics.go` -- Computation functions
- `internal/output/table.go` -- Metrics table renderer

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestMetricsEmpty` | Empty board shows zeroes |
| `TestMetricsThroughput` | Correct count of completed tasks |
| `TestMetricsCycleTime` | Correct average computation |
| `TestMetricsSinceFilter` | `--since` filters correctly |
| `TestMetricsAgingItems` | In-progress items listed with age |
| `TestMetricsJSON` | JSON output matches schema |

---

## 4.4 Activity Log (M)

### What

Append-only log file (`kanban/activity.log`) recording every state change. Each line is a JSON object (JSON Lines format) with timestamp, action, task ID, and details. The log provides an audit trail and enables retrospectives.

### CLI Interface

```bash
# View recent activity (table format)
kanban-md log
# TIMESTAMP              ACTION    ID  DETAILS
# 2026-02-07 14:30:00    created    5  "New feature" (backlog, medium)
# 2026-02-07 14:35:22    moved      5  backlog -> todo
# 2026-02-07 15:00:10    edited     3  priority: medium -> high
# 2026-02-07 15:10:05    blocked    3  "Waiting on API keys"
# 2026-02-07 16:00:00    deleted    2  "Old task" (force)

# Filter by action
kanban-md log --action moved,created

# Filter by time
kanban-md log --since 2026-02-07

# Limit entries
kanban-md log --limit 10

# JSON output
kanban-md log --json --limit 5

# Direct file access (JSON Lines)
cat kanban/activity.log | jq 'select(.action=="moved")'
```

### Log Entry Format

Each line in `activity.log` is a self-contained JSON object:

```json
{"timestamp":"2026-02-07T14:30:00Z","action":"created","task_id":5,"title":"New feature","details":"status=backlog priority=medium"}
{"timestamp":"2026-02-07T14:35:22Z","action":"moved","task_id":5,"title":"New feature","details":"backlog -> todo"}
{"timestamp":"2026-02-07T15:00:10Z","action":"edited","task_id":3,"title":"Auth flow","details":"priority: medium -> high"}
{"timestamp":"2026-02-07T15:10:05Z","action":"blocked","task_id":3,"title":"Auth flow","details":"Waiting on API keys"}
{"timestamp":"2026-02-07T16:00:00Z","action":"deleted","task_id":2,"title":"Old task","details":"force=true"}
```

### Go Types

```go
// LogEntry represents a single activity log line.
type LogEntry struct {
    Timestamp time.Time `json:"timestamp"`
    Action    string    `json:"action"`    // created, moved, edited, deleted, blocked, unblocked
    TaskID    int       `json:"task_id"`
    Title     string    `json:"title"`
    Details   string    `json:"details"`
    Actor     string    `json:"actor,omitempty"` // future: for multi-agent (Layer 7)
}
```

### Logging Infrastructure

In `internal/board/log.go`:

```go
// AppendLog writes a single log entry to the activity log file.
func AppendLog(kanbanDir string, entry LogEntry) error {
    path := filepath.Join(kanbanDir, "activity.log")
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
    if err != nil {
        return err
    }
    defer f.Close()
    return json.NewEncoder(f).Encode(entry)
}

// ReadLog reads and filters log entries.
func ReadLog(kanbanDir string, opts LogOptions) ([]LogEntry, error)
```

### Integration with Commands

Each write command appends a log entry:

- `create` -> `action: "created"`, details: initial status + priority
- `move` -> `action: "moved"`, details: "oldStatus -> newStatus"
- `edit` -> `action: "edited"`, details: comma-separated field changes
- `delete` -> `action: "deleted"`, details: "force=true/false"
- `edit --block` -> `action: "blocked"`, details: block reason
- `edit --unblock` -> `action: "unblocked"`

### Flags for `log` Command

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | none | Only entries after this date |
| `--limit` | 0 | Max entries (0 = all, newest first) |
| `--action` | none | Filter by action type(s), comma-separated |
| `--task` | none | Filter by task ID |

### Edge Cases

- **Log file doesn't exist:** Created on first write. `log` command returns empty list.
- **Corrupted log line:** Skip and continue reading (warn to stderr).
- **Log rotation:** Not built-in. Users can rotate with standard tools (`logrotate`) since it's an append-only file.
- **`init` command:** Does not log (no tasks exist yet).
- **Log file permissions:** 0o600 (same as task files).

### Files Affected

- New: `internal/board/log.go` -- `AppendLog()`, `ReadLog()`
- New: `cmd/log.go` -- `log` command with filter flags
- `cmd/create.go` -- Append log entry
- `cmd/move.go` -- Append log entry
- `cmd/edit.go` -- Append log entry
- `cmd/delete.go` -- Append log entry
- `internal/output/table.go` -- Log table renderer

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestLogAfterCreate` | Create generates log entry |
| `TestLogAfterMove` | Move generates log entry with old->new |
| `TestLogAfterEdit` | Edit generates log entry with changed fields |
| `TestLogAfterDelete` | Delete generates log entry |
| `TestLogSinceFilter` | `--since` filters correctly |
| `TestLogActionFilter` | `--action moved` shows only moves |
| `TestLogLimit` | `--limit 5` returns at most 5 entries |
| `TestLogEmptyBoard` | Empty log returns nothing |
| `TestLogJSON` | JSON output is valid JSON Lines |

---

## Implementation Order

1. **4.2 Start/Complete Timestamps** -- Foundation for all metrics; must be in place first
2. **4.1 Board Summary** -- Uses timestamps and WIP limits for a useful dashboard
3. **4.3 Metrics Command** -- Depends on timestamps existing
4. **4.4 Activity Log** -- Independent of metrics, but logically last (cross-cutting concern)
