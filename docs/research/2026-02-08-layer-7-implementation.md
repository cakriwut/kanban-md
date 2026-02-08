# Layer 7: Multi-Agent Orchestration — Implementation Report

**Date:** 2026-02-08
**Branch:** `feature/layer-7-multi-agent`

## Overview

Layer 7 adds multi-agent coordination to kanban-md, enabling multiple AI agents (or humans) to work concurrently on the same board without conflicts. The implementation comprises four sub-features: claim/release semantics, an atomic pick command, classes of service, and swimlanes.

## Implementation Summary

### Config Schema Migration (v2→v3)

Introduced config version 3 with three new top-level fields:

- `claim_timeout` (string, default: `"1h"`): Duration after which claims expire. Stored as a string and parsed via `time.ParseDuration` to avoid YAML marshaling issues with `time.Duration`.
- `classes` (list of ClassConfig): Defines available classes of service with optional `wip_limit` and `bypass_column_wip` fields.
- `defaults.class` (string, default: `"standard"`): Default class for new tasks.

The migration `migrateV2ToV3()` applies defaults non-destructively — only setting fields that are zero-valued.

**Key decision:** Used a string field for `claim_timeout` with a `ClaimTimeoutDuration()` accessor method rather than a custom YAML unmarshaler. This avoids complexity while keeping the config file human-readable.

### Task Data Model

Added three fields to the Task struct (all optional with `omitempty`):

- `claimed_by` (string): Agent identifier
- `claimed_at` (*time.Time): Claim timestamp
- `class` (string): Class of service

Added five error codes to `clierr`: `TaskClaimed`, `InvalidClass`, `ClassWIPExceeded`, `NothingToPick`, `InvalidGroupBy`.

### Claim/Release Semantics (7.1)

Central `checkClaim()` function in `cmd/root.go` enforces claim discipline:

1. Unclaimed task → allow
2. Same agent as claimant → allow
3. Expired claim → clear and allow
4. Force flag set → clear and allow
5. Otherwise → return `TaskClaimed` error

Claims are enforced on `edit`, `move`, and `delete`. The `--force` flag (pre-existing for WIP override) doubles as claim override to avoid flag proliferation.

**Activity logging:** Claims and releases are logged with `claimed` and `released` actions.

**Filter additions:** `--unclaimed` and `--claimed-by` on the `list` command. `IsUnclaimed()` exported for reuse by the pick algorithm.

### Pick Command (7.2)

The `pick` command atomically finds and claims the next available task:

1. Filter by target statuses (non-terminal if not specified)
2. Exclude claimed (non-expired), blocked, and tag-mismatched tasks
3. Remove tasks with unmet dependencies (checks full task set, not just candidates)
4. Sort by class priority, then task priority
5. Claim and optionally move the winner

**Dependency lookup fix:** The initial implementation only checked candidate tasks for dependency status, missing the fact that dependencies might be tasks outside the candidate set. Fixed by building `statusByID` from all tasks.

### Classes of Service (7.3)

Four default classes with distinct behaviors:

| Class | WIP Behavior | Pick Order |
|-------|-------------|------------|
| expedite | Bypasses column WIP, own board-wide limit (default: 1) | First |
| fixed-date | Normal column WIP | Second (sorted by due date) |
| standard | Normal column WIP | Third |
| intangible | Normal column WIP | Last |

The `enforceWIPLimitForClass()` function checks class-specific WIP rules before column WIP, allowing expedite tasks to bypass column limits while respecting their own cap.

### Swimlanes (7.4)

`GroupBy()` groups tasks by any of: assignee, tag, class, priority, status.

Key behaviors:
- Tags create multi-group membership (a task with tags `[a, b]` appears in both groups)
- Empty values get sentinel keys: `(unassigned)`, `(untagged)`
- Groups sorted by config order for status/priority/class, alphabetically otherwise
- Each group includes per-status task counts

The `--group-by` flag is available on both `board` and `list` commands.

## Cyclomatic Complexity Management

Several functions exceeded the project's limit of 15. Solutions applied:

- `matchesFilter` → split into `matchesCoreFilter` + `matchesExtendedFilter`
- `runPick` → extracted `validatePickFlags`, `executePick`, `outputPickResult`
- `Pick` → extracted `pickCandidates`, `filterPickDeps`, `sortPickCandidates`

## Testing

- 12 unit tests for pick algorithm (priority, claims, blocks, deps, tags, classes, due dates)
- 5 unit tests for group-by (assignee, tag, class, status, status-summary-per-group)
- Config compat tests for v1→v3 and v2→v3 migration paths
- Task compat tests for new fields
- All existing E2E tests continue to pass

## Files Changed

| Category | Files |
|----------|-------|
| Config | config.go, defaults.go, migrate.go, migrate_test.go, compat_test.go |
| Task | task.go, validate.go, compat_test.go |
| Board | board.go, filter.go, sort.go, pick.go (new), pick_test.go (new), group.go (new), group_test.go (new) |
| CLI | root.go, create.go, edit.go, move.go, delete.go, list.go, pick.go (new), board.go |
| Output | table.go |
| Errors | clierr.go |
| Fixtures | 2 new fixture files |

## What's Not Included

- **MCP server tool definitions** (Layer 6 not yet implemented — deferred)
- **E2E tests for new commands** (pick, claim, class, group-by) — could be added as follow-up
- **TUI integration** — claim display in TUI tracked as separate backlog item (#57)
