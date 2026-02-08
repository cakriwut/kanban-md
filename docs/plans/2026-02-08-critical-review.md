# Critical Review: kanban-md

**Date**: 2026-02-08
**Scope**: Full codebase audit — architecture, consistency, bugs, dead code, test coverage, CI/CD, documentation

---

## Executive Summary

The kanban-md codebase is well-structured with clean dependency graphs, consistent error handling, and strong linting discipline. However, there are several concrete issues ranging from rendering bugs and dead code to significant README drift and CI misconfigurations. This report catalogs **47 findings** across 5 categories.

---

## 1. Bugs & Correctness Issues

### 1.1 Table column misalignment with ANSI-styled placeholders
**Severity**: High
**File**: `internal/output/table.go:60-77`

When optional fields (assignee, tags, due) are empty, they are replaced with `dimStyle.Render("--")`, which includes ANSI escape codes. These styled strings are then printed with `%-*s` formatting, which counts *bytes*, not visible characters. The ANSI sequences add ~13 invisible bytes per placeholder, causing downstream columns to shift right.

```go
// line 74 — the format specifier counts ANSI bytes as width
fmt.Fprintf(os.Stdout, "%-*d %-*s %-*s %-*s %-*s %-*s %-*s\n",
    idW, t.ID, statusW, t.Status, prioW, t.Priority,
    titleW, title, assignW, assignee, tagsW, tags, dueW, due)
```

**Fix**: Strip ANSI codes before computing column widths, or use a library-aware width function (lipgloss provides `lipgloss.Width()`).

### 1.2 Hardcoded `cfg.Statuses[1]` as "ready" status
**Severity**: Medium
**File**: `internal/board/context.go:187-192`

`buildReadySection` assumes `cfg.Statuses[1]` is the "ready/todo" status. Users with custom statuses (e.g., `["new", "accepted", "active", "done"]`) get unexpected behavior — "accepted" would be treated as "ready" regardless of its semantic meaning.

### 1.3 Missing `endIdx > beginIdx` validation in `WriteContextToFile`
**Severity**: Low
**File**: `internal/board/context.go:327-334`

If sentinel markers `<!-- BEGIN/END kanban-md context -->` appear in reverse order in the target file, the replacement logic produces malformed output. No guard verifies that `endIdx > beginIdx`.

### 1.4 `ReadAll` fails entirely on one malformed file
**Severity**: Medium
**File**: `internal/task/find.go:58-62`

If any single `.md` file in the tasks directory has malformed frontmatter, `ReadAll` returns an error and zero tasks. One corrupted file prevents the entire board from loading. There is no lenient/partial-load option.

### 1.5 `FindDependents` silently swallows errors
**Severity**: Low
**File**: `internal/board/board.go:51-53`

If `task.ReadAll` fails, `FindDependents` returns `nil` instead of propagating the error. The caller cannot distinguish "no dependents" from "I/O error while checking."

### 1.6 Batch delete skips dependent-task warning
**Severity**: Medium
**File**: `cmd/delete.go:115`

Single delete calls `warnDependents()` (line 77), but batch mode (`deleteSingleCore`) does not. Batch deletes silently skip the "other tasks depend on this" warning.

### 1.7 `edit --status` has no WIP limit override
**Severity**: Medium
**Files**: `cmd/edit.go:103,155`

`enforceWIPLimit` is called with `force: false` hardcoded. Unlike `move --force`, there is no `--force` flag on `edit` to override WIP limits when changing status. Users hit a wall with no escape hatch.

---

## 2. Dead Code

### 2.1 Unused sentinel errors in `task` package
**Files**: `internal/task/validate.go:12-13`, `internal/task/find.go:14-15`

| Symbol | Location | Status |
|--------|----------|--------|
| `ErrInvalidStatus` | validate.go:12 | Never referenced |
| `ErrInvalidPriority` | validate.go:13 | Never referenced |
| `ErrNotFound` | find.go:15 | Never referenced (`FindByID` returns `clierr.Error` instead) |

The comment "kept for backward compatibility with errors.Is" is misleading — nothing in the codebase uses `errors.Is` with these sentinels.

### 2.2 Unused `FormatParentError` function
**File**: `internal/task/validate.go:98-101`

Defined but never called. Both `validateCreateDeps` and `validateEditDeps` use inline `fmt.Errorf` instead.

### 2.3 Unused `FormatAuto` constant
**File**: `internal/output/output.go:12-13`

`FormatAuto` is defined as `iota` (value 0) but `Detect` never returns it. The default case always returns `FormatTable`.

---

## 3. Inconsistencies & Design Issues

### 3.1 `--force` flag semantic confusion
**Files**: `cmd/move.go:30`, `cmd/delete.go:30`

| Command | `--force` meaning |
|---------|------------------|
| `move` | Override WIP limits |
| `delete` | Skip confirmation prompt |

Same flag name and shorthand (`-f`) with completely different semantics.

### 3.2 `--since` date errors not structured
**Files**: `cmd/metrics.go:43`, `cmd/log.go:35`

Date parsing errors from `--since` are returned raw, not wrapped in `clierr.Error`. Unlike `--due` errors (which go through `task.ValidateDate`), these won't produce structured JSON error output.

### 3.3 `--limit` flag shorthand inconsistency
**Files**: `cmd/list.go:26`, `cmd/log.go:20`

`list` has `-n`/`--limit`; `log` has `--limit` without a shorthand.

### 3.4 TTY auto-detection not implemented
**File**: `internal/output/output.go:22-39`

Despite `FormatAuto` being defined and documentation claiming "Output auto-detect: TTY→table, piped→JSON," the `Detect` function never checks if stdout is a TTY. The default is unconditionally `FormatTable`.

### 3.5 `time.Now()` injection inconsistency
**Files**: Multiple

| Function | `time.Now()` handling |
|----------|----------------------|
| `ComputeMetrics` | Accepts `now time.Time` parameter ✓ |
| `Summary` | Calls `time.Now()` directly ✗ |
| `GenerateContext` | Calls `time.Now()` directly ✗ |
| `Today` | Calls `time.Now()` directly (acceptable for this use) |

Functions with time-dependent logic that call `time.Now()` directly are harder to test deterministically.

### 3.6 `Date` embeds `time.Time`, leaking full API
**File**: `internal/date/date.go:16`

Because `Date` embeds `time.Time`, all of `time.Time`'s methods (`Hour`, `Minute`, `Add`, `Sub`, etc.) are promoted. These are semantically meaningless for a date-only type. A consumer calling `d.Add(time.Hour)` gets a `time.Time` back, not a `Date`, silently breaking the type contract.

### 3.7 `Error` vs `SilentError` code field naming collision
**File**: `internal/clierr/clierr.go`

`Error.Code` is a string (e.g., `"TASK_NOT_FOUND"`). `SilentError.Code` is an int exit code (e.g., `1`). Same field name, different types, same package.

### 3.8 Output functions not testable
**Files**: `internal/output/table.go`, `internal/output/json.go`

All output functions write directly to `os.Stdout`/`os.Stderr` with no `io.Writer` injection. This makes them impossible to unit test without stdout capture, which is why only `FormatDuration` and `Detect` have tests.

### 3.9 `JSONError` writes to `os.Stdout`, not `os.Stderr`
**File**: `internal/output/json.go:28-31`

Error output goes to stdout, mixing errors with normal output on the same stream.

### 3.10 Activity log grows unbounded
**File**: `internal/board/log.go:34-53`

`AppendLog` appends to `activity.jsonl` forever with no size limit, rotation, or archival mechanism. `ReadLog` loads the entire file into memory before applying the limit.

### 3.11 Default config values use `var` instead of `const`
**File**: `internal/config/defaults.go:6-26`

`DefaultDir`, `DefaultTasksDir`, `DefaultStatus`, `DefaultPriority` are `var` instead of `const`. They could be mutated at runtime.

### 3.12 Migration result not persisted to disk
**File**: `internal/config/config.go:176-184`

When `migrate` upgrades a config (e.g., v1→v2), the migrated config is returned but not saved. Every subsequent `Load` re-runs the migration.

### 3.13 Regex recompilation on every call
**Files**: `internal/task/find.go:20-24,70`

Both `FindByID` and `ExtractIDFromFilename` compile regexes inside the function body instead of at package level, causing unnecessary recompilation on every invocation.

---

## 4. Code Duplication

### 4.1 `validateCreateDeps` / `validateEditDeps` near-identical
**Files**: `cmd/create.go:148-160`, `cmd/edit.go:449-461`

Both functions have identical logic: check parent via `validateDepIDs`, check `DependsOn` via `validateDepIDs`. Could be a single `validateDeps(cfg, t)` function.

### 4.2 Single/Core command function duplication
**Files**: `cmd/edit.go`, `cmd/move.go`, `cmd/delete.go`

The batch operation pattern causes each command to have two functions (e.g., `editSingleTask` / `editSingleCore`) that share ~90% of their logic. The only difference is output handling. A refactoring could extract shared logic into a function returning a result struct, with callers handling output.

### 4.3 `containsStr` duplicates `contains`
**Files**: `internal/board/filter.go:82-88`, `internal/config/config.go:235-237`

Identical `containsStr` / `contains` implementations in different packages.

### 4.4 `fileMode = 0o600` defined in 4 places
**Files**: `internal/config/config.go:14`, `internal/task/file.go:13`, `internal/board/log.go:14`, `internal/board/context.go:313`

### 4.5 `hoursPerDay = 24` defined in 2 places
**Files**: `internal/board/metrics.go:11`, `internal/output/table.go:215`

### 4.6 Test helpers `testConfig()` duplicated across packages
**Files**: `cmd/move_test.go:11`, `internal/board/sort_test.go:12`, `internal/board/context_test.go:15`

---

## 5. Documentation Drift

### 5.1 Four commands missing from README
**File**: `README.md` (Commands section)

| Command | Alias | Status |
|---------|-------|--------|
| `board` | `summary` | Implemented, undocumented |
| `metrics` | — | Implemented, undocumented |
| `log` | — | Implemented, undocumented |
| `completion` | — | Implemented, undocumented |

### 5.2 README config example shows version 1, current is 2
**File**: `README.md:107` vs `internal/config/defaults.go:33`

The example `config.yml` in the README is outdated and doesn't show the `wip_limits` field added in v2.

### 5.3 14+ flags undocumented in README

| Command | Missing flags |
|---------|--------------|
| `init` | `--wip-limit` |
| `create` | `--parent`, `--depends-on` |
| `edit` | `--started`, `--clear-started`, `--completed`, `--clear-completed`, `--parent`, `--clear-parent`, `--add-dep`, `--remove-dep`, `--block`, `--unblock` |
| `list` | `--blocked`, `--not-blocked`, `--parent`, `--unblocked` |

---

## 6. CI/CD Issues

### 6.1 Released binaries report `version = "dev"`
**Severity**: High
**Files**: `.goreleaser.yml` (builds section), `cmd/root.go:22`

No `-ldflags` to inject version at build time. Released binaries will show `version = "dev"` instead of the release tag. Fix:

```yaml
builds:
  - ldflags:
      - -s -w -X github.com/antopolskiy/kanban-md/cmd.version={{.Version}}
```

### 6.2 `--fix` flag in CI lint step
**Severity**: Medium
**Files**: `.github/workflows/build.yml:34`, `.github/workflows/release.yml:34`

The golangci-lint action uses `args: --fix`, which silently auto-corrects issues. The lint step always passes, and the actual failure appears at `make diff` — confusing for developers reading CI logs. Standard practice: run without `--fix` in CI.

### 6.3 `workflow_dispatch` on release without guard
**Severity**: Medium
**File**: `.github/workflows/release.yml:8`

Manual dispatch is allowed but `AGENTS.md` warns against it ("causes a duplicate build"). Either remove `workflow_dispatch` or add a tag check.

### 6.4 No coverage reporting
**Severity**: Low
**File**: `.github/workflows/build.yml:43-47`

Coverage artifacts are uploaded but never analyzed — no codecov, coveralls, or badge integration.

### 6.5 AGENTS.md and CLAUDE.md are exact duplicates
**Severity**: Low

Every line is identical. Updates must be made in two places.

---

## 7. Test Coverage Gaps

### 7.1 Packages with no/minimal unit tests

| File | Gap |
|------|-----|
| `internal/task/validate.go` | 7 validation functions, zero unit tests |
| `internal/output/json.go` | `JSON()`, `JSONError()` — zero tests |
| `internal/output/table.go` | Only `FormatDuration` tested; `TaskTable`, `TaskDetail`, `OverviewTable`, `MetricsTable`, `ActivityLogTable`, `Messagef` all untested |
| `cmd/*.go` (12 files) | Only `root_test.go` (trivial) and `move_test.go` (`updateTimestamps` only) have unit tests |

### 7.2 Specific untested paths

| What | Where |
|------|-------|
| `parseIDs`, `runBatch`, `checkWIPLimit`, `loadConfig` | `cmd/root.go` |
| `appendUnique`, `removeAll`, `appendUniqueInts`, `removeInts` | `cmd/edit.go` |
| Sort by `created`/`updated` fields | `internal/board/sort.go` |
| Default/unknown sort field fallback | `internal/board/sort.go:37` |
| Windows line endings in task files | `internal/task/file.go` |

The comprehensive e2e test suite (2714 lines, ~120 test functions) provides good behavioral coverage, but unit tests would give faster feedback and catch edge cases like the rendering bug (§1.1).

---

## 8. Recommended Priorities

### Quick Wins (low effort, high value)
1. Remove dead code (§2.1–2.3)
2. Add ldflags to `.goreleaser.yml` (§6.1)
3. Remove `--fix` from CI lint steps (§6.2)
4. Compile regexes at package level (§3.13)
5. Change `var` to `const` for default strings (§3.11)

### Medium Effort
6. Fix table column misalignment (§1.1) — switch to lipgloss-aware width
7. Update README with missing commands and flags (§5.1–5.3)
8. Add `--force` to `edit` for WIP limit override (§1.7)
9. Add `io.Writer` parameter to output functions for testability (§3.8)
10. Consolidate `Single`/`Core` command duplication (§4.2)

### Larger Refactors
11. Add lenient mode to `ReadAll` (§1.4)
12. Implement actual TTY auto-detection (§3.4)
13. Add activity log rotation (§3.10)
14. Inject `time.Now` via parameter or clock interface (§3.5)
15. Unit test `output` and `cmd` packages (§7.1)
