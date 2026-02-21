# TUI create/edit typing experience fix (Task #188)

## Problem observed

The create wizard text inputs (title/body/tags) behaved like append-only fields:

- typing always appended at the end,
- backspace always deleted from the end,
- cursor keys were ignored.

This made in-place edits impossible and caused a poor typing experience.

## Reproduction and TDD

A failing test was added first:

- `TestCreate_TitleCursorMovementInsertsAtCursor`

Flow:
1. open create dialog,
2. type `Task`,
3. move cursor left twice,
4. type `X`,
5. create task,
6. expect slug `taxsk` (mid-string insertion) rather than `taskx` (append-only).

The test failed before implementation and passed after.

## Implementation choices

1. Added cursor columns for create wizard text fields:
   - title cursor,
   - body cursor,
   - tags cursor.
2. Refactored text handling into a shared editor routine used by title/body/tags.
3. Added support for:
   - left/right cursor movement,
   - home/end and ctrl+a/ctrl+e,
   - backspace/delete at cursor,
   - rune insertion and space insertion at cursor.
4. Updated text rendering to display cursor at the current position, not always at end.
5. Applied to both create and edit wizard flows (edit reuses create flow).

## Validation

- `go test ./internal/tui -run TestCreate_TitleCursorMovementInsertsAtCursor`
- `go test ./internal/tui`
- `go test ./...`
- `golangci-lint run ./...` was not available as a direct shell command in this environment; pre-commit hook enforces lint during commit.
