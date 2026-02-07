# Integration Test System for kanban-md

## Context

Layers 0 and 1 are complete — all foundation packages and 7 CLI commands are implemented with passing unit tests and zero lint issues. However, the **command layer has zero integration tests**. The internal packages (date, config, task, board, output) have good unit coverage, but nothing tests the full CLI flow: binary → flags → config → task operations → file system → output.

This plan adds a robust end-to-end test suite that exercises the binary as a real user would.

---

## Approach: Binary Execution via `exec.Command`

**Why not Cobra's `cmd.SetOut()`?** All output functions (`output.Messagef`, `output.JSON`, `output.TaskTable`) write directly to `os.Stdout` — they bypass `cmd.OutOrStdout()`. There's no `io.Writer` abstraction to intercept without refactoring production code.

**The approach:** Build the binary once in `TestMain`, then run it via `exec.Command` for each test. This:

- Captures stdout/stderr naturally
- Tests the full stack (main → cmd.Execute → RunE → config → task → output)
- Exercises flag parsing, config discovery, and TTY auto-detection
- Requires no changes to production code
- Tests exit codes

**Output strategy:** Use `--json` for all assertion-heavy tests (deterministic, parseable). A few dedicated tests verify table/text output works (substring checks).

---

## Files to Create/Modify

### New: `e2e/cli_test.go`

Single file containing TestMain, helpers, and all test functions. Package `e2e_test` — external test package that imports nothing from internal packages (tests the public binary contract only).

No build tags — these tests run with `go test ./...`. The binary build takes <1s.

### Modify: `Makefile`

Add `test-e2e` target for running integration tests separately:

```makefile
.PHONY: test-e2e
test-e2e: ## e2e tests
 go test $(RACE_OPT) -v ./e2e/
```

---

## Structure of `e2e/cli_test.go`

### TestMain

- Build binary to temp dir via `go build -o <tempdir>/kanban-md ..`
- Store path in package-level `binPath`
- Clean up with `defer os.RemoveAll`

### Helper Types & Functions

**`result`** — captures stdout, stderr, exitCode from a command run.

**`taskJSON`** — mirrors the task JSON output schema (id, title, status, priority, assignee, tags, due, estimate, body, file, created, updated). Defined locally in the test file — deliberately NOT importing `internal/task` to test the external contract.

**`runKanban(t, dir, args...)`** — runs `kanban-md --dir <dir> <args...>`, returns `result`. Auto-prepends `--dir` to every invocation for test isolation.

**`runKanbanJSON(t, dir, dest, args...)`** — runs with `--json` prepended, unmarshals stdout into `dest`.

**`initBoard(t, args...)`** — creates a fresh board in `t.TempDir()`, returns the kanban directory path.

**`mustCreateTask(t, dir, title, extraArgs...)`** — creates a task, parses JSON output, returns `taskJSON`. Fails test on error.

### Test Functions (27 tests)

#### Init (4 tests)

| Test | What it verifies |
|------|-----------------|
| `TestInitDefault` | Creates config.yml + tasks/, JSON output has status/dir/name |
| `TestInitWithName` | `--name` flag sets board name |
| `TestInitCustomStatuses` | `--statuses open,closed` configures custom columns |
| `TestInitAlreadyInitialized` | Second init fails with "already initialized" |

#### Create (5 tests)

| Test | What it verifies |
|------|-----------------|
| `TestCreateBasic` | Default status/priority, ID=1, title matches |
| `TestCreateWithAllFlags` | All flags (status, priority, assignee, tags, due, estimate, body) |
| `TestCreateIncrementID` | Three creates produce IDs 1, 2, 3 |
| `TestCreateInvalidStatus` | Rejects unknown status with error |
| `TestCreateBadDateFormat` | Rejects `02-15-2026` (wrong format) |

#### List (3 tests)

| Test | What it verifies |
|------|-----------------|
| `TestListEmpty` | JSON returns `[]`, table output says "No tasks found." on stderr |
| `TestListFilters` | Table-driven: 8 filter combos (status, priority, assignee, tag, combined, no match) |
| `TestListSortAndLimit` | Table-driven: sort by id/priority, reverse, limit, combined |

#### Show (3 tests)

| Test | What it verifies |
|------|-----------------|
| `TestShow` | Returns full task detail including body, assignee, tags |
| `TestShowNotFound` | Non-existent ID fails with "not found" |
| `TestShowInvalidID` | Non-numeric ID fails with "invalid task ID" |

#### Edit (4 tests)

| Test | What it verifies |
|------|-----------------|
| `TestEditFields` | Table-driven: status, priority, add-tag, due, body changes |
| `TestEditTitleRename` | Title change renames file, old file removed, new file has correct slug |
| `TestEditNoChanges` | No flags fails with "no changes specified" |
| `TestEditClearDue` | `--clear-due` removes due date |

#### Move (4 tests)

| Test | What it verifies |
|------|-----------------|
| `TestMoveDirectStatus` | `move 1 in-progress` changes status |
| `TestMoveNextPrev` | `--next` advances, `--prev` goes back |
| `TestMoveBoundaryErrors` | `--prev` at first / `--next` at last status fails |
| `TestMoveNoStatusSpecified` | No status or direction flag fails |

#### Delete (2 tests)

| Test | What it verifies |
|------|-----------------|
| `TestDeleteWithForce` | `--force` deletes file, JSON reports "deleted" |
| `TestDeleteWithoutForceNonTTY` | Without `--force` in non-TTY (test env) fails |

#### Cross-cutting (2 tests)

| Test | What it verifies |
|------|-----------------|
| `TestNoInitErrors` | Table-driven: all 6 commands fail with "no kanban board found" when no board exists |
| `TestCommandAliases` | `add`, `ls`, `rm` aliases work |

#### Workflow & Edge Cases (5 tests)

| Test | What it verifies |
|------|-----------------|
| `TestFullLifecycle` | init → create → list → show → edit → move → delete → list(empty) |
| `TestCustomStatusesWorkflow` | Custom statuses work in create/move, old defaults rejected |
| `TestTagOperations` | Add, add duplicate (no-op), remove, remove non-existent (no-op) |
| `TestDueDateLifecycle` | Set → clear → re-set due date |
| `TestSortByDueWithNilValues` | Tasks without due dates sort last |

#### Output Format (2 tests)

| Test | What it verifies |
|------|-----------------|
| `TestTextOutput` | `--table` flag produces table output for list/show/create |
| `TestLongTitleSlugTruncation` | Slug in filename is ≤50 chars |

---

## Lint Compliance Notes

- **gosec G204**: `exec.Command` with variable args → `//nolint:gosec // e2e test binary execution`
- **gosec G306**: `os.WriteFile` in TestMain setup → use 0o600
- **usetesting**: Use `t.TempDir()` in test functions, `os.MkdirTemp` only in TestMain (no `t` available)
- **perfsprint**: Use `strconv.Itoa` not `fmt.Sprintf`
- **goimports**: Stdlib-only imports (no third-party or local imports in this file)
- **funlen**: Keep test functions under 100 lines; split if needed

---

## Verification

After implementation:

1. `go test -race ./e2e/` — all 27 tests pass
2. `go test -race ./...` — full suite still passes (unit + e2e)
3. `golangci-lint run ./...` — zero issues
4. Copy plan to `docs/plans/integration-tests.md`
