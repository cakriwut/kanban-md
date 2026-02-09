#!/bin/bash
set -e
export PATH=/tmp:$PATH
export NO_COLOR=
cd "$(mktemp -d)"

kanban-md init --name "My Project" >/dev/null 2>&1

kanban-md create "Set up CI pipeline" --priority high --tags devops >/dev/null 2>&1
kanban-md create "Write API docs" --tags docs >/dev/null 2>&1
kanban-md create "Fix login bug" --priority critical --tags backend >/dev/null 2>&1

echo '$ kanban-md list --compact'
kanban-md list --compact 2>/dev/null
echo ""
echo '$ kanban-md pick --claim agent-1 --move in-progress'
kanban-md pick --claim agent-1 --move in-progress 2>/dev/null
echo ""
echo '$ kanban-md move 3 done --force'
kanban-md move 3 done --force 2>/dev/null
echo ""
echo '$ kanban-md list --compact'
kanban-md list --compact 2>/dev/null
