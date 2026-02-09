#!/bin/bash
# Demo recording script for asciinema + agg.
# Produces a .cast file that agg converts to a GIF.
#
# Usage:
#   # 1. Build kanban-md to /tmp
#   go build -o /tmp/kanban-md ./cmd/kanban-md
#
#   # 2. Record the demo
#   asciinema rec assets/demo.cast --cols 80 --rows 20 \
#     --command "bash assets/demo-record.sh" --overwrite
#
#   # 3. Convert to GIF
#   agg assets/demo.cast assets/demo.gif \
#     --font-size 16 --theme dracula --idle-time-limit 3 --last-frame-duration 3
#
# Requirements: asciinema, agg, kanban-md binary at /tmp/kanban-md

set -e
export PATH=/tmp:$PATH
export NO_COLOR=1
export TERM=dumb
cd "$(mktemp -d)"

# Silent setup — create board and tasks before recording starts
kanban-md init --name "My Project" >/dev/null 2>&1
kanban-md create "Set up CI pipeline" --priority high --tags devops >/dev/null 2>&1
kanban-md create "Write API docs" --tags docs >/dev/null 2>&1
kanban-md create "Fix login bug" --priority critical --tags backend >/dev/null 2>&1

# Show compact list
printf '\033[38;5;75m$\033[0m kanban-md list --compact\n'
sleep 0.3
kanban-md list --compact 2>/dev/null
sleep 1.2
echo

# Agent picks highest priority task atomically
printf '\033[38;5;75m$\033[0m kanban-md pick --claim agent-1 --move in-progress\n'
sleep 0.3
kanban-md pick --claim agent-1 --move in-progress 2>/dev/null
sleep 1.2
echo

# Move task to done
printf '\033[38;5;75m$\033[0m kanban-md move 2 done --force\n'
sleep 0.3
kanban-md move 2 done --force 2>/dev/null
sleep 1.2
echo

# Final state — show updated compact list
printf '\033[38;5;75m$\033[0m kanban-md list --compact\n'
sleep 0.3
kanban-md list --compact 2>/dev/null
sleep 2
