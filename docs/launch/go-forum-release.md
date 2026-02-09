# Go Forum Releases Post Draft

## Title

[Release] kanban-md: file-based Kanban CLI/TUI for multi-agent Go workflows

## Category

Releases

## Post body

I built **kanban-md** to solve a practical problem: AI coding agents and humans need to coordinate work, but most trackers are API-first and context-heavy for terminal agents.

`kanban-md` is a **file-based Kanban board** where each task is a Markdown file with YAML frontmatter. There is no server and no database. You can commit the board with your code, review task changes in PRs, and work offline.

### What it does

- Manage tasks via CLI (`create`, `list`, `show`, `edit`, `move`, `delete`, `archive`, `pick`, `metrics`, `context`)
- Interactive TUI (`kanban-md-tui`) for human browsing and editing
- Atomic `pick --claim` workflow to coordinate parallel agents safely
- Classes of service + WIP limits + dependency tracking
- Compact output mode designed for agent context efficiency

### Why I made it

In multi-agent coding, race conditions are common: two agents pick the same task, stale claims block progress, and verbose JSON burns tokens. I wanted a Go-native tool that keeps workflows deterministic while staying simple.

### Why it is useful vs alternatives

Compared to hosted issue trackers and API-first boards:

- **No auth/API dependency:** works in local/CI environments without setup friction
- **Git-native workflow:** tasks are plain files, diff-able and merge-able
- **Agent-safe primitives:** claim/expiry + atomic pick reduce duplicate work
- **Token-efficient output:** `--compact` is much lighter than full JSON in agent loops
- **Cross-platform binary distribution:** straightforward install for macOS/Linux/Windows

### Install

```bash
# Homebrew
brew install antopolskiy/tap/kanban-md

# Go install
go install github.com/antopolskiy/kanban-md/cmd/kanban-md@latest
go install github.com/antopolskiy/kanban-md/cmd/kanban-md-tui@latest
```

### Quickstart

```bash
# initialize board
kanban-md init

# create tasks
kanban-md create "Add retry logic" --priority high --tags backend
kanban-md create "Write API tests" --priority medium --tags testing

# atomic claim + move to in-progress
kanban-md pick --claim agent-1 --move in-progress

# token-efficient list for agents
kanban-md list --compact

# optional: visual board for humans
kanban-md-tui
```

### Links

- Repo: https://github.com/antopolskiy/kanban-md
- Demo page: https://antopolskiy.github.io/kanban-md/
- Latest release: https://github.com/antopolskiy/kanban-md/releases/latest
