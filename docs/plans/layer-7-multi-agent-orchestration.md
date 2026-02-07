# Layer 7: Multi-Agent Orchestration

**Theme:** Enable multiple agents (or humans) to work concurrently on the same board with claim semantics, coordination primitives, and workflow classification. This is the frontier use case for AI-powered project management.

**Target release:** v1.1.0
**Estimated effort:** ~6 days
**Prerequisites:** Layer 3 (dependencies, WIP limits, blocked state), Layer 6 (MCP server)

---

## Background: Multi-Agent Coordination

Tools like Vibe Kanban (BloopAI), Claude Code's swarm pattern, and Cursor 2.0's multi-agent suite demonstrate that multiple AI agents working in parallel on different tasks is becoming practical. The key challenge is **coordination** -- preventing agents from:

1. Working on the same task simultaneously
2. Making conflicting file changes
3. Starting work on tasks whose dependencies aren't met
4. Overloading a workflow stage (exceeding WIP limits)

kanban-md's file-based architecture is well-suited for multi-agent work because:
- Each task is an independent file (no database locks)
- Git provides conflict detection and merge resolution
- JSON output enables machine-readable status checking
- WIP limits (Layer 3) naturally throttle work

What's missing is **claim semantics** -- a way for an agent to signal "I'm working on this, back off" -- and a **pick** command for atomic task assignment.

---

## 7.1 Claim/Release Semantics (M)

### What

Tasks can be claimed by an agent (or person), preventing others from modifying them. Claims are stored in the task's YAML frontmatter. Claims expire after a configurable timeout to prevent deadlocks from crashed agents.

### CLI Interface

```bash
# Claim a task
kanban-md edit 3 --claim agent-1

# Attempt to claim already-claimed task
kanban-md edit 3 --claim agent-2
# Error: task #3 is claimed by "agent-1" (expires in 45m). Use --force to override.

# Force-claim (override stale or abandoned claim)
kanban-md edit 3 --claim agent-2 --force

# Release a claim (explicit)
kanban-md edit 3 --release

# Claims auto-expire (default: 1 hour)
# After expiry, the task becomes claimable again

# Filter unclaimed tasks
kanban-md list --unclaimed --status todo

# Show claim info
kanban-md show 3
#   Claimed by: agent-1
#   Claimed at: 2026-02-07 14:30 (expires in 45m)
```

### Data Model Changes

Add to Task struct in `internal/task/task.go`:

```go
ClaimedBy  string     `yaml:"claimed_by,omitempty" json:"claimed_by,omitempty"`
ClaimedAt  *time.Time `yaml:"claimed_at,omitempty" json:"claimed_at,omitempty"`
```

### Config Changes

```yaml
# New field in config.yml
claim_timeout: 1h   # duration string (Go time.ParseDuration format)
```

Default: `1h`. Set to `0` to disable claim expiration (claims are permanent until released).

### Claim Enforcement

Claims are checked on all mutating operations (`edit`, `move`, `delete`). The check logic:

```go
func checkClaim(t *task.Task, claimant string, force bool, timeout time.Duration) error {
    if t.ClaimedBy == "" {
        return nil // unclaimed, proceed
    }
    if t.ClaimedBy == claimant {
        return nil // claimed by the same agent, proceed
    }

    // Check if claim has expired
    if timeout > 0 && t.ClaimedAt != nil {
        if time.Since(*t.ClaimedAt) > timeout {
            // Claim expired -- allow operation and clear stale claim
            t.ClaimedBy = ""
            t.ClaimedAt = nil
            return nil
        }
    }

    if force {
        // Force override -- clear existing claim
        t.ClaimedBy = ""
        t.ClaimedAt = nil
        return nil
    }

    remaining := timeout - time.Since(*t.ClaimedAt)
    return fmt.Errorf("task #%d is claimed by %q (expires in %s). Use --force to override",
        t.ID, t.ClaimedBy, remaining.Round(time.Minute))
}
```

### Claim Rules

1. **Claiming:** Sets `ClaimedBy` and `ClaimedAt`. The claim name is a free-form string (agent name, session ID, username).
2. **Editing a claimed task:** Only the claim holder can edit. Others get an error unless they use `--force`.
3. **Moving a claimed task:** Same rules as editing.
4. **Deleting a claimed task:** Same rules as editing.
5. **Releasing:** Clears `ClaimedBy` and `ClaimedAt`. Anyone can release (not just the holder) since this is a coordination hint, not a security mechanism.
6. **Expiration:** After `claim_timeout`, the claim is treated as if it doesn't exist. Stale claims are cleaned up lazily (when the task is next read/modified).
7. **Claim + other edits:** `--claim` can be combined with other flags: `kanban-md edit 3 --claim agent-1 --status in-progress`

### Race Conditions

kanban-md is file-based with no locking daemon. Two agents could theoretically claim the same task simultaneously:

1. Agent A reads task file (unclaimed)
2. Agent B reads task file (unclaimed)
3. Agent A writes claim to file
4. Agent B writes claim to file (overwrites A's claim)

**Mitigation strategy:**
- This is a **cooperative** system, not an adversarial one. Agents are assumed to be well-behaved.
- The `pick` command (7.2) reduces the window by combining find + claim into one operation.
- In git-based workflows, conflicting claims would show up as merge conflicts, providing detection.
- For high-contention scenarios, external locking (file locks, advisory locks) can be added later. This is deliberately deferred to avoid complexity.

### Filter Logic

Add to `FilterOptions` in `internal/board/filter.go`:

```go
Unclaimed  bool   // only unclaimed or expired-claim tasks
ClaimedBy  string // filter to specific claimant
```

### Files Affected

- `internal/task/task.go` -- `ClaimedBy`, `ClaimedAt` fields
- `internal/config/config.go` -- `ClaimTimeout` field (parsed as `time.Duration`)
- `cmd/edit.go` -- `--claim NAME`, `--release` flags; claim check on all mutations
- `cmd/move.go` -- Claim check before move; `--claim` flag for claim-and-move
- `cmd/delete.go` -- Claim check before delete
- `internal/board/filter.go` -- `Unclaimed`, `ClaimedBy` filters
- `cmd/list.go` -- `--unclaimed`, `--claimed-by` flags
- `internal/output/table.go` -- Claim info in detail view and list table

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestClaimTask` | `--claim` sets claimed_by and claimed_at |
| `TestClaimBlocksOtherAgent` | Different claimant can't edit |
| `TestClaimSameAgentAllowed` | Same claimant can edit |
| `TestClaimForceOverride` | `--force` overrides existing claim |
| `TestClaimRelease` | `--release` clears claim |
| `TestClaimExpiration` | Expired claims allow new claims |
| `TestListUnclaimed` | `--unclaimed` filters correctly |
| `TestClaimAndEdit` | `--claim agent --status todo` works in single command |

---

## 7.2 Pick Command (S)

### What

New `pick` command that atomically finds the highest-priority unclaimed, unblocked task in a given status and claims it. This is the primary entrypoint for agents starting work.

The `pick` command replaces a common multi-step agent pattern:
```bash
# Without pick (3 commands, race condition window):
TASK=$(kanban-md list --status todo --unblocked --unclaimed --sort priority --reverse --limit 1 --json | jq '.[0].id')
kanban-md edit $TASK --claim agent-1
kanban-md move $TASK in-progress

# With pick (1 command, atomic):
kanban-md pick --status todo --move in-progress --claim agent-1
```

### CLI Interface

```bash
# Pick highest-priority unclaimed, unblocked task in "todo"
kanban-md pick --status todo --claim agent-1
# Claimed task #7: "Implement auth" (critical)

# Pick, claim, AND move in one step
kanban-md pick --status todo --move in-progress --claim agent-1
# Claimed and moved task #7 to "in-progress"

# Pick from any non-terminal status
kanban-md pick --claim agent-1
# Searches all non-terminal statuses in order

# Nothing available
kanban-md pick --status todo --claim agent-1
# No unblocked, unclaimed tasks found in "todo"
# Exit code: 1

# JSON output (for agent parsing)
kanban-md pick --status todo --claim agent-1 --json
# {"id":7,"title":"Implement auth","status":"todo","priority":"critical","claimed_by":"agent-1",...}
```

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--claim NAME` | yes | Agent name to claim as |
| `--status STATUS` | no | Status column to pick from (default: all non-terminal) |
| `--move STATUS` | no | Also move the task to this status |
| `--class CLASS` | no | Prefer tasks of this class of service |

### Pick Algorithm

```go
func Pick(cfg *config.Config, tasks []*task.Task, opts PickOptions) *task.Task {
    // 1. Filter to target status(es)
    candidates := filterByStatus(tasks, opts.Statuses)

    // 2. Filter out claimed (non-expired) tasks
    candidates = filterUnclaimed(candidates, cfg.ClaimTimeout)

    // 3. Filter out blocked tasks (manual blocks AND unresolved dependencies)
    candidates = filterUnblocked(candidates, tasks, cfg.TerminalStatusSet())
    candidates = filterNotBlocked(candidates) // manual block_reason

    // 4. Sort by priority (highest first, using config priority order)
    sort.SliceStable(candidates, func(i, j int) bool {
        return priorityIndex(candidates[i].Priority, cfg) > priorityIndex(candidates[j].Priority, cfg)
    })

    // 5. If class filter specified, prefer that class
    if opts.Class != "" {
        // Move matching class to front while preserving priority order within each group
        candidates = preferClass(candidates, opts.Class)
    }

    // 6. Return first candidate (or nil if none)
    if len(candidates) == 0 {
        return nil
    }
    return candidates[0]
}
```

### Atomicity

The `pick` command performs find + claim + optional move as a single logical operation. Since the filesystem has no transactions, the operations happen sequentially, but the window for race conditions is minimal (microseconds between operations on local disk).

For stronger guarantees in high-contention scenarios, a file-level advisory lock could be added:
```go
lockFile := filepath.Join(kanbanDir, ".pick.lock")
lock, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL, 0o600)
// ... perform pick ...
os.Remove(lockFile)
```

This is deferred to avoid complexity unless users report actual contention issues.

### Files Affected

- New: `cmd/pick.go` -- `pick` command
- `internal/board/board.go` -- `Pick()` function
- `internal/mcp/tools.go` -- `kanban_pick` MCP tool definition
- `internal/mcp/handlers.go` -- Pick handler

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestPickHighestPriority` | Returns highest-priority unclaimed task |
| `TestPickSkipsClaimed` | Skips claimed tasks |
| `TestPickSkipsBlocked` | Skips blocked tasks |
| `TestPickSkipsDeps` | Skips tasks with unmet dependencies |
| `TestPickAndMove` | `--move` moves task in same operation |
| `TestPickNoneAvailable` | Returns error when no tasks match |
| `TestPickFromSpecificStatus` | `--status` filters correctly |

---

## 7.3 Classes of Service (M)

### What

Tasks can be tagged with a class of service that affects their handling:

- **Expedite:** Urgent work. Bypasses WIP limits. Picked first by `pick`. Visually highlighted. Limited to N items board-wide (default: 1).
- **Fixed-date:** Has a hard deadline. Picked after expedite, before standard. No WIP bypass.
- **Standard:** Normal work. Default class. Processed FIFO within priority.
- **Intangible:** Improvement work (tech debt, documentation, tooling). Picked last. Teams should allocate a percentage of capacity to intangible work.

Classes of service are a Kanban concept from David J. Anderson's framework. They provide risk-based prioritization that goes beyond simple priority levels.

### CLI Interface

```bash
# Create with class
kanban-md create "Security hotfix" --class expedite --priority critical
kanban-md create "Q1 report" --class fixed-date --due 2026-03-31
kanban-md create "Refactor auth module" --class intangible
kanban-md create "Add user search" --class standard  # or omit (default)

# Edit class
kanban-md edit 5 --class expedite

# Filter by class
kanban-md list --class expedite
kanban-md list --class intangible

# Pick respects class ordering
kanban-md pick --status todo --claim agent-1
# Picks expedite first, then fixed-date (by due date), then standard, then intangible.

# Board summary shows class distribution
kanban-md board
# Classes: 1 expedite | 2 fixed-date | 18 standard | 3 intangible
```

### Data Model Changes

Add to Task struct in `internal/task/task.go`:

```go
Class string `yaml:"class,omitempty" json:"class,omitempty"` // expedite, fixed-date, standard, intangible
```

### Config Changes

```yaml
classes:
  - name: expedite
    wip_limit: 1        # board-wide limit for expedite items
    bypass_column_wip: true
  - name: fixed-date
  - name: standard      # default class
  - name: intangible

defaults:
  class: standard
```

### Expedite WIP Bypass

When an expedite task is created or moved, it bypasses column WIP limits. However, the expedite class itself has a board-wide WIP limit (default: 1). This prevents abuse -- you can't mark everything as expedite.

```go
func checkWIPForClass(cfg *config.Config, tasks []*task.Task, t *task.Task, targetStatus string) error {
    classConfig := cfg.ClassConfig(t.Class)

    // Check class-level WIP limit (board-wide)
    if classConfig.WIPLimit > 0 {
        count := countByClass(tasks, t.Class)
        if count >= classConfig.WIPLimit {
            return fmt.Errorf("expedite WIP limit reached (%d/%d board-wide)", count, classConfig.WIPLimit)
        }
    }

    // If class bypasses column WIP, skip column check
    if classConfig.BypassColumnWIP {
        return nil
    }

    // Normal column WIP check
    return checkColumnWIPLimit(cfg, tasks, targetStatus)
}
```

### Pick Ordering with Classes

The `pick` algorithm orders candidates by:
1. Class priority (expedite > fixed-date > standard > intangible)
2. Within fixed-date: by due date (soonest first)
3. Within same class: by priority (highest first)
4. Within same priority: by ID (oldest first)

### Files Affected

- `internal/task/task.go` -- `Class` field
- `internal/task/validate.go` -- `ValidateClass()`
- `internal/config/config.go` -- `ClassConfig` type, `Classes` list, `Defaults.Class`
- `cmd/create.go` -- `--class` flag
- `cmd/edit.go` -- `--class` flag
- `cmd/list.go` -- `--class` flag
- `internal/board/filter.go` -- Class filter
- `cmd/move.go` -- Expedite WIP bypass logic
- `cmd/pick.go` -- Class-aware ordering
- `internal/output/table.go` -- Class display in table and detail

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestCreateWithClass` | `--class expedite` sets field |
| `TestExpediteBypassesWIP` | Expedite task moves to full column |
| `TestExpediteOwnWIPLimit` | Can't exceed expedite board-wide limit |
| `TestPickOrdersByClass` | Expedite picked before standard |
| `TestFixedDateOrderedByDue` | Fixed-date tasks sorted by due date |
| `TestDefaultClassIsStandard` | Omitting --class defaults to standard |
| `TestListFilterByClass` | `--class expedite` filters correctly |

---

## 7.4 Swimlanes (M)

### What

The `board` command can group tasks into horizontal lanes by any field (assignee, tag, class, priority). Swimlanes provide visual separation of work streams and help identify imbalances.

### CLI Interface

```bash
# Group by assignee
kanban-md board --group-by assignee
# === alice ===
# backlog: 2  todo: 1  in-progress: 2/3  done: 5
# ─────────────────────────────────────────────
# === bob ===
# backlog: 1  todo: 3  in-progress: 1/3  done: 3
# ─────────────────────────────────────────────
# === (unassigned) ===
# backlog: 4  todo: 0  in-progress: 0/3  done: 0

# Group by tag
kanban-md board --group-by tag
# Groups by each unique tag. Tasks with multiple tags appear in multiple groups.

# Group by class
kanban-md board --group-by class

# Group by priority
kanban-md board --group-by priority

# JSON output
kanban-md board --group-by assignee --json
# {
#   "groups": [
#     {"key": "alice", "statuses": [{"name": "backlog", "count": 2}, ...]},
#     {"key": "bob", "statuses": [...]},
#     {"key": "(unassigned)", "statuses": [...]}
#   ]
# }

# List can also group
kanban-md list --group-by status
# Groups tasks under status headers instead of a flat list
```

### Group-By Logic

```go
type GroupedSummary struct {
    Groups []GroupSummary `json:"groups"`
}

type GroupSummary struct {
    Key      string          `json:"key"`
    Statuses []StatusSummary `json:"statuses"`
    Total    int             `json:"total"`
}

func GroupBy(tasks []*task.Task, field string, cfg *config.Config) GroupedSummary {
    groups := make(map[string][]*task.Task)

    for _, t := range tasks {
        keys := extractGroupKeys(t, field)
        for _, key := range keys {
            groups[key] = append(groups[key], t)
        }
    }

    // Sort groups alphabetically (or by config order for status/priority)
    // Build summary per group
}

func extractGroupKeys(t *task.Task, field string) []string {
    switch field {
    case "assignee":
        if t.Assignee == "" { return []string{"(unassigned)"} }
        return []string{t.Assignee}
    case "tag":
        if len(t.Tags) == 0 { return []string{"(untagged)"} }
        return t.Tags // task appears in multiple groups
    case "class":
        if t.Class == "" { return []string{"standard"} }
        return []string{t.Class}
    case "priority":
        return []string{t.Priority}
    case "status":
        return []string{t.Status}
    default:
        return []string{"(all)"}
    }
}
```

### Files Affected

- `internal/board/board.go` -- `GroupBy()` function, `GroupedSummary` type
- `cmd/board.go` -- `--group-by` flag
- `cmd/list.go` -- `--group-by` flag (optional, groups list output)
- `internal/output/table.go` -- Grouped board renderer

### E2E Tests

| Test | What it verifies |
|------|-----------------|
| `TestBoardGroupByAssignee` | Groups tasks by assignee with counts |
| `TestBoardGroupByTag` | Multi-tag tasks appear in multiple groups |
| `TestBoardGroupByClass` | Groups by class of service |
| `TestBoardGroupByUnassigned` | Unassigned tasks in "(unassigned)" group |
| `TestBoardGroupByJSON` | JSON output matches expected schema |

---

## Example Multi-Agent Workflow

Here's how three Claude Code agents could work on the same board concurrently:

```bash
# Human sets up the board
kanban-md init --name "API Rewrite" --statuses "backlog,todo:5,in-progress:3,review:2,done"
kanban-md create "Design new API schema" --priority critical
kanban-md create "Implement auth endpoints" --priority high --depends-on 1
kanban-md create "Implement CRUD endpoints" --priority high --depends-on 1
kanban-md create "Write integration tests" --priority medium --depends-on 2,3
kanban-md create "Update documentation" --priority low --depends-on 2,3
kanban-md create "Migrate old clients" --priority high --depends-on 2,3
kanban-md move 1,2,3,4,5,6 todo

# Agent 1 picks up the first available task
kanban-md pick --status todo --move in-progress --claim agent-1
# Picks #1 "Design new API schema" (critical, no deps)
# Agent 1 works on it...

# Agent 2 tries to pick
kanban-md pick --status todo --move in-progress --claim agent-2
# Nothing available! #2,#3 depend on #1 which is in-progress, not done.

# Agent 1 finishes
kanban-md move 1 done
# #1 completed. #2 and #3 are now unblocked.

# Agent 2 picks again
kanban-md pick --status todo --move in-progress --claim agent-2
# Picks #2 "Implement auth endpoints" (high priority)

# Agent 3 picks
kanban-md pick --status todo --move in-progress --claim agent-3
# Picks #3 "Implement CRUD endpoints" (high priority, parallel with #2)

# Agent 1 picks again
kanban-md pick --status todo --move in-progress --claim agent-1
# Nothing available -- #4,#5,#6 all depend on #2 and #3.
# WIP limit (3) is also reached with agents 1's old claim expired and 2,3 active.

# Agents 2 and 3 finish
kanban-md move 2 done
kanban-md move 3 done
# #4, #5, #6 are now unblocked.

# All three agents pick tasks
kanban-md pick --status todo --move in-progress --claim agent-1
# Picks #6 "Migrate old clients" (high, after dependencies)
kanban-md pick --status todo --move in-progress --claim agent-2
# Picks #4 "Write integration tests" (medium)
kanban-md pick --status todo --move in-progress --claim agent-3
# Picks #5 "Update documentation" (low)
# WIP limit (3) is exactly met.
```

---

## Implementation Order

1. **7.1 Claim/Release** -- Foundation for all multi-agent features
2. **7.2 Pick Command** -- Primary agent interface, depends on claims
3. **7.3 Classes of Service** -- Enhances pick ordering, WIP bypass logic
4. **7.4 Swimlanes** -- Independent display feature, can be done in parallel with 7.3
