#!/bin/bash
# TUI demo recording script for asciinema + agg.
# Produces a .cast file that agg converts to a GIF.
#
# Usage (from repo root):
#   bash assets/tui-demo-gen.sh
#
# Requirements: asciinema, agg, python3, kanban-md binary at /tmp/kanban-md

set -e
export PATH=/tmp:$PATH
# Force 256-color output even without a real TTY.
export CLICOLOR_FORCE=1
export COLORFGBG="15;0"

# Resolve script directory before cd'ing away.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

DEMO_DIR="$(mktemp -d)"
cd "$DEMO_DIR"

K="kanban-md"

# --- Silent setup: create a realistic board ---
$K init --name "My Project" >/dev/null 2>&1
$K config set tui.title_lines 2 >/dev/null 2>&1

# Backlog tasks (stay put during demo)
$K create "Performance testing" --priority low --tags testing >/dev/null 2>&1
$K create "Mobile responsive layout" --priority medium --tags frontend >/dev/null 2>&1
$K create "Localization support" --priority low --tags i18n >/dev/null 2>&1

# Todo tasks (all unclaimed — will be claimed and animated during demo)
$K create "Add rate limiting" --priority medium --tags backend >/dev/null 2>&1
$K create "Set up monitoring" --priority high --tags devops >/dev/null 2>&1
$K create "Fix auth token refresh" --priority critical --tags security >/dev/null 2>&1
$K create "Write integration tests" --priority high --tags testing >/dev/null 2>&1
$K create "Build dashboard UI" --priority high --tags frontend >/dev/null 2>&1

$K move 4 todo >/dev/null 2>&1
$K move 5 todo >/dev/null 2>&1
$K move 6 todo >/dev/null 2>&1
$K move 7 todo >/dev/null 2>&1
$K move 8 todo >/dev/null 2>&1

# Done tasks (project history)
$K create "Create project scaffold" --priority high --tags setup >/dev/null 2>&1
$K create "Set up CI pipeline" --priority high --tags devops >/dev/null 2>&1
$K move 9 done >/dev/null 2>&1
$K move 10 done >/dev/null 2>&1

# --- Show the command being typed (visible on first frame) ---
printf '\033[38;5;75m$\033[0m kanban-md tui\n'
sleep 0.5

# --- Start background commands that modify the board while TUI runs ---
export DEMO_KANBAN_DIR="$DEMO_DIR/kanban"
bash "$SCRIPT_DIR/tui-demo-background.sh" &
BG_PID=$!

# --- Launch the TUI (takes over screen, watcher auto-refreshes) ---
$K tui --dir "$DEMO_DIR/kanban" 2>/dev/null &
TUI_PID=$!

# Wait for background script to finish, then hold final state.
wait $BG_PID 2>/dev/null || true
sleep 2

# SIGKILL the TUI so bubbletea can't restore the alternate screen.
# This keeps the TUI board as the last visible frame.
kill -9 $TUI_PID 2>/dev/null || true
wait $TUI_PID 2>/dev/null || true

# Clean up temp directory.
rm -rf "$DEMO_DIR"
