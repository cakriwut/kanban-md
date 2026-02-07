# Layer 3: Kanban Discipline

**Theme:** Enforce the rules that make Kanban work -- WIP limits, blocked state, and dependency validation. Without WIP limits, a Kanban board is just a task list with columns.

**Target release:** v0.4.0
**Estimated effort:** ~5 days
**Prerequisites:** None (builds on Layer 2)

---

## 3.1 WIP Limits (M)

### What

Cap the number of tasks allowed in a status column. Enforced on `create`, `move`, and `edit --status`. The move command errors when a limit would be exceeded, with `--force` to override.

WIP limits are the defining mechanism of Kanban (David J. Anderson's second practice). They expose bottlenecks: when a column hits its limit, upstream work must stop, forcing the team to address the blockage. Without WIP limits, work accumulates silently in columns, cycle time grows, and flow becomes unpredictable.

### CLI Interface

```bash
# Init with WIP limits (colon syntax)
kanban-md init --name "Project" --statuses "backlog,todo,in-progress:3,review:2,done"

# Move respects limits
kanban-md move 5 in-progress
# Error: WIP limit reached for "in-progress" (3/3). Use --force to override.

kanban-md move 5 in-progress --force
# Moved task #5: todo -> in-progress (WIP limit exceeded: 4/3)

# Create also checks limits
kanban-md create "New task" --status in-progress
# Error: WIP limit reached for "in-progress" (3/3). Use --force to override.

# List shows WIP utilization in header
kanban-md list --status in-progress
# IN-PROGRESS (3/3)
# ID  TITLE              PRIORITY  ASSIGNEE
# 2   Auth flow          high      alice
# 5   Database migration critical  bob
# 7   API integration    medium    --
```

### Config Changes

Statuses change from a flat string list to a mixed list supporting both plain strings and objects. This must be backward-compatible with existing `config.yml` files.

```yaml
# New format (mixed)
statuses:
  - backlog              # plain string: no WIP limit
  - todo                 # plain string: no WIP limit
  - name: in-progress    # object: has WIP limit
    wip_limit: 3
  - name: review         # object: has WIP limit
    wip_limit: 2
  - done                 # plain string: no WIP limit

# Old format (still works)
statuses:
  - backlog
  - todo
  - in-progress
  - review
  - done
```

### Go Implementation Details

New type in `internal/config/config.go`:

```go
// StatusConfig represents a status column with optional WIP limit.
type StatusConfig struct {
    Name     string `yaml:"name" json:"name"`
    WIPLimit int    `yaml:"wip_limit,omitempty" json:"wip_limit,omitempty"` // 0 = unlimited
}

// UnmarshalYAML handles both plain strings and objects.
func (s *StatusConfig) UnmarshalYAML(value *yaml.Node) error {
    if value.Kind == yaml.ScalarNode {
        s.Name = value.Value
        return nil
    }
    // Decode as struct for object form
    type plain StatusConfig
    return value.Decode((*plain)(s))
}
```

The `Config` struct changes `Statuses []string` to `Statuses []StatusConfig`. Helper methods:

```go
func (c *Config) StatusNames() []string           // returns just names for validation
func (c *Config) WIPLimit(status string) int       // returns limit (0 = unlimited)
func (c *Config) IsStatusValid(s string) bool      // replaces current string-in-slice check
```

Board-level helper in `internal/board/board.go`:

```go
func CountByStatus(tasks []*task.Task) map[string]int
```

### WIP Enforcement Logic

Applied in `cmd/move.go`, `cmd/create.go`, `cmd/edit.go`:

```go
func checkWIPLimit(cfg *config.Config, tasks []*task.Task, targetStatus string, force bool) error {
    limit := cfg.WIPLimit(targetStatus)
    if limit == 0 {
        return nil // unlimited
    }
    counts := board.CountByStatus(tasks)
    current := counts[targetStatus]
    if current >= limit {
        if force {
            output.Messagef("Warning: WIP limit exceeded for %q (%d/%d)", targetStatus, current+1, limit)
            return nil
        }
        return fmt.Errorf("WIP limit reached for %q (%d/%d). Use --force to override", targetStatus, current, limit)
    }
    return nil
}
```

### Init Flag Parsing

The `--statuses` flag parses the colon syntax:

```go
// "backlog,todo,in-progress:3,review:2,done"
// Splits on comma, then splits each on colon for optional limit.
func parseStatusFlag(s string) ([]config.StatusConfig, error)
```

### Edge Cases

- **WIP limit of 0 or negative:** Treated as unlimited. Validation rejects negative values.
- **All columns have WIP limits:** Valid. Backlog/done with WIP limits are unusual but allowed.
- **Force flag:** Succeeds but emits a warning. JSON output includes `"wip_exceeded": true`.
- **Edit --status to same status:** No WIP check needed (task already counted in that column).
- **Existing boards:** Old config without WIP limits continues to work (all limits default to 0 = unlimited).

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestInitWithWIPLimits` | `--statuses "backlog,todo:5,done"` creates config with correct limits |
| `TestMoveRespectsWIPLimit` | Move to full column errors |
| `TestMoveForceOverridesWIP` | `--force` allows exceeding limit with warning |
| `TestCreateRespectsWIPLimit` | Create with `--status` into full column errors |
| `TestEditStatusRespectsWIPLimit` | Edit `--status` into full column errors |
| `TestWIPUnlimitedByDefault` | Columns without `:N` have no limit |
| `TestWIPBackwardCompat` | Old config format (plain string list) still works |

---

## 3.2 Wire Dependencies (M)

### What

Activate the existing `Parent *int` and `DependsOn []int` fields in the Task struct. These fields are already declared in YAML/JSON tags but no command reads, writes, validates, or filters on them. This feature adds:

1. `--parent` and `--depends-on` flags on `create` and `edit`
2. Validation that referenced task IDs exist
3. `--unblocked` filter on `list` (tasks whose dependencies are all in terminal status)
4. `--parent` filter on `list` (subtasks of a given parent)
5. Dependency display in `show` output

### CLI Interface

```bash
# Create with dependencies
kanban-md create "Run integration tests" --depends-on 1,2
kanban-md create "Unit test for auth" --parent 3

# Edit dependencies
kanban-md edit 5 --add-dep 4
kanban-md edit 5 --remove-dep 1
kanban-md edit 5 --parent 3
kanban-md edit 5 --clear-parent

# Filter by dependency status
kanban-md list --unblocked          # deps all completed
kanban-md list --parent 3           # subtasks of #3

# Show with dependency info
kanban-md show 5
#   Depends on: #1 (done), #2 (in-progress)
#   Parent:     #3 "Implement auth"
#   Children:   #6, #7
```

### Config Changes

Add `terminal_statuses` to config -- statuses that count as "completed" for dependency resolution. Defaults to the last status in the list.

```yaml
terminal_statuses:
  - done
```

### Dependency Resolution Algorithm

For `--unblocked` filtering, we need to check each task's `DependsOn` list against current task states:

```go
func filterUnblocked(tasks []*task.Task, terminalStatuses map[string]bool) []*task.Task {
    // Build a map of task ID -> status for O(1) lookup
    statusMap := make(map[int]string, len(tasks))
    for _, t := range tasks {
        statusMap[t.ID] = t.Status
    }

    var result []*task.Task
    for _, t := range tasks {
        if isUnblocked(t, statusMap, terminalStatuses) {
            result = append(result, t)
        }
    }
    return result
}

func isUnblocked(t *task.Task, statusMap map[int]string, terminal map[string]bool) bool {
    for _, depID := range t.DependsOn {
        status, exists := statusMap[depID]
        if !exists {
            // Dependency task was deleted -- treat as unblocked
            continue
        }
        if !terminal[status] {
            return false // dependency not yet in terminal status
        }
    }
    return true
}
```

**No transitive dependency resolution.** If A depends on B and B depends on C, we only check A's direct dependencies (B). B's dependency on C is B's problem. This keeps the algorithm O(n) and avoids graph complexity.

### Validation

```go
// ValidateDependencies checks that all referenced IDs exist.
func ValidateDependencies(t *task.Task, allTasks []*task.Task) error {
    idSet := make(map[int]bool, len(allTasks))
    for _, at := range allTasks {
        idSet[at.ID] = true
    }
    for _, depID := range t.DependsOn {
        if !idSet[depID] {
            return fmt.Errorf("dependency task #%d does not exist", depID)
        }
    }
    if t.Parent != nil && !idSet[*t.Parent] {
        return fmt.Errorf("parent task #%d does not exist", *t.Parent)
    }
    // Self-reference check
    if t.Parent != nil && *t.Parent == t.ID {
        return fmt.Errorf("task cannot be its own parent")
    }
    for _, depID := range t.DependsOn {
        if depID == t.ID {
            return fmt.Errorf("task cannot depend on itself")
        }
    }
    return nil
}
```

### Edge Cases

- **Circular dependencies:** Not detected (no transitive resolution). A depends on B, B depends on A -- both show as blocked. This is a valid representation of a deadlock. The `--unblocked` filter simply won't return either.
- **Delete task with dependents:** Warn but allow. "Warning: task #3 is depended on by #5, #7. Proceeding will leave dangling references."
- **Dangling references:** A dependency pointing to a deleted task is treated as satisfied (unblocked). This prevents deleted tasks from permanently blocking the board.
- **Duplicate dependencies:** `--add-dep` is idempotent -- adding an ID that's already in the list is a no-op.
- **Parent hierarchy:** Only one level. A task with a parent cannot itself be a parent (enforced in validation). This keeps things simple.

### Files Affected

- `internal/task/validate.go` -- `ValidateDependencies()`, `ValidateParent()`
- `internal/board/filter.go` -- Add `Unblocked bool`, `ParentID *int` to `FilterOptions`
- `cmd/create.go` -- `--parent`, `--depends-on` flags
- `cmd/edit.go` -- `--add-dep`, `--remove-dep`, `--parent`, `--clear-parent` flags
- `cmd/list.go` -- `--unblocked`, `--parent` flags
- `cmd/delete.go` -- Warning for dependents
- `internal/output/table.go` -- Dependency info in `TaskDetail`
- `internal/config/config.go` -- `TerminalStatuses` field

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestCreateWithDependsOn` | `--depends-on 1,2` sets field correctly |
| `TestCreateWithParent` | `--parent 1` sets field |
| `TestDependsOnInvalidID` | Referencing non-existent ID errors |
| `TestSelfDependency` | `--depends-on` with own ID errors |
| `TestListUnblocked` | Only tasks with all deps completed returned |
| `TestListParent` | Only children of specified parent returned |
| `TestDeleteWithDependents` | Warning message but succeeds |
| `TestEditAddRemoveDep` | `--add-dep` and `--remove-dep` work correctly |
| `TestUnblockedAfterDepCompleted` | Moving dep to terminal status unblocks dependent |

---

## 3.3 Blocked State (S)

### What

Tasks can be marked as blocked with a reason, independent of their status column. A blocked task stays in its current column but is visually distinguished and can be filtered. This is distinct from dependency-based blocking -- a task can be manually blocked for external reasons ("waiting on vendor API keys") even if its formal dependencies are met.

### CLI Interface

```bash
# Block a task with a reason
kanban-md edit 3 --block "Waiting on API keys from vendor"

# Unblock
kanban-md edit 3 --unblock

# Filter
kanban-md list --blocked         # only blocked tasks
kanban-md list --not-blocked     # only non-blocked tasks

# Show displays block info
kanban-md show 3
#   Status:   in-progress
#   BLOCKED:  Waiting on API keys from vendor
#   ...
```

### Data Model Changes

Add to Task struct in `internal/task/task.go`:

```go
Blocked     bool   `yaml:"blocked,omitempty" json:"blocked,omitempty"`
BlockReason string `yaml:"block_reason,omitempty" json:"block_reason,omitempty"`
```

### Behavior

- `--block REASON` sets `Blocked=true` and `BlockReason=REASON`. Also updates `Updated` timestamp.
- `--unblock` sets `Blocked=false` and clears `BlockReason`. Also updates `Updated` timestamp.
- `--block` and `--unblock` cannot be used together (error).
- Blocking does not prevent other operations (move, edit, delete). A blocked task can be moved forward if the block is resolved outside the tool.
- Table output shows a visual indicator (e.g., `[BLOCKED]` prefix or color) for blocked tasks in list view.

### Filter Logic

Add to `FilterOptions` in `internal/board/filter.go`:

```go
Blocked    *bool // nil = no filter, true = only blocked, false = only non-blocked
```

### Edge Cases

- **Block without reason:** Error -- reason is required for blocked state to be useful for analysis (blocker clustering).
- **Unblock a non-blocked task:** No-op (idempotent).
- **Block an already-blocked task:** Updates the reason.
- **Moving a blocked task:** Allowed but emits a warning: "Note: task #3 is blocked."

### Files Affected

- `internal/task/task.go` -- `Blocked`, `BlockReason` fields
- `internal/board/filter.go` -- `Blocked` filter
- `cmd/edit.go` -- `--block REASON`, `--unblock` flags
- `cmd/list.go` -- `--blocked`, `--not-blocked` flags
- `internal/output/table.go` -- Visual indicator in table and detail

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestBlockTask` | `--block` sets blocked and reason |
| `TestUnblockTask` | `--unblock` clears blocked state |
| `TestBlockRequiresReason` | `--block ""` or `--block` without value errors |
| `TestListBlocked` | `--blocked` filters correctly |
| `TestListNotBlocked` | `--not-blocked` filters correctly |
| `TestMoveBlockedTaskWarns` | Moving blocked task succeeds with warning |

---

## 3.4 Idempotent Move (S)

### What

`move ID STATUS` succeeds silently when the task is already at the target status, rather than erroring or performing a redundant write. This is important for scripting and agent usage where retry logic is common.

### CLI Interface

```bash
# Already in-progress -- succeeds, no file write
kanban-md move 1 in-progress
# Task #1 already at status "in-progress"

# JSON output indicates no change
kanban-md move 1 in-progress --json
# {"id":1, "title":"...", "status":"in-progress", "changed":false}
```

### Implementation

In `cmd/move.go`, before applying the status change:

```go
if currentTask.Status == targetStatus {
    // No change needed -- report success without modifying file
    if jsonOutput {
        // Include "changed": false in JSON output
    } else {
        output.Messagef("Task #%d already at status %q", id, targetStatus)
    }
    return nil
}
```

The `changed` field is added to JSON output for move operations:

```go
type moveResult struct {
    *task.Task
    Changed bool `json:"changed"`
}
```

### Edge Cases

- **`--next` when already at last status:** Still an error (boundary error, not idempotency).
- **`--prev` when already at first status:** Still an error.
- **Move to same status with `--force`:** Also no-op (no WIP limit implication).

### Files Affected

- `cmd/move.go` -- Early return for same-status, `changed` field in JSON

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestMoveIdempotent` | Move to current status succeeds with `changed: false` |
| `TestMoveIdempotentNoFileChange` | File `updated` timestamp is NOT modified |
| `TestMoveNextAtEndStillErrors` | Boundary errors are not idempotent |

---

## Implementation Order

Within Layer 3, implement in this order:

1. **3.4 Idempotent Move** -- Smallest, no dependencies, immediately useful
2. **3.3 Blocked State** -- Small, no dependencies on other 3.x features
3. **3.1 WIP Limits** -- Medium, config schema change is the hardest part
4. **3.2 Wire Dependencies** -- Medium, benefits from WIP limits being in place first (dependency resolution interacts with status validation)
