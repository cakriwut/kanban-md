# Research: Obsidian Plugin Approaches for kanban-md

**Date:** 2026-02-08
**Status:** Complete

## 1. Obsidian Plugin Architecture

### How Community Plugins Work

Obsidian community plugins are TypeScript applications bundled into a single `main.js` file. The minimal plugin structure requires three files:

- **`manifest.json`** -- Plugin metadata (id, name, version, minAppVersion, description, author, `isDesktopOnly` flag)
- **`main.ts`** -- Entry point; exports a default class extending `Plugin` with `onload()` and `onunload()` lifecycle methods
- **`styles.css`** -- Optional CSS for custom styling

The build pipeline typically uses esbuild to bundle TypeScript into a single `main.js`. The official [obsidian-sample-plugin](https://github.com/obsidianmd/obsidian-sample-plugin) template provides the scaffolding.

### File Read/Write via Vault API

Plugins interact with files through the [Vault API](https://docs.obsidian.md/Plugins/Vault):

| Method | Purpose |
|---|---|
| `vault.getMarkdownFiles()` | List all `.md` files in the vault |
| `vault.cachedRead(file)` | Read file (uses cache, fast) |
| `vault.read(file)` | Read file (always from disk) |
| `vault.modify(file, data)` | Overwrite file content |
| `vault.process(file, fn)` | Atomically modify file content (preferred) |
| `vault.create(path, data)` | Create a new file |
| `vault.delete(file)` | Permanently delete |
| `vault.trash(file)` | Move to trash |
| `vault.getAbstractFileByPath(path)` | Get file/folder by vault-relative path |

For frontmatter specifically, `app.fileManager.processFrontMatter(file, fn)` provides YAML-aware modification, though it has a known limitation of reformatting YAML (removing comments, reordering, etc.).

### Shell Command Execution

**Yes, plugins can execute shell commands**, but with constraints:

- Obsidian runs on Electron, which exposes Node.js APIs including `child_process`
- Plugins using Node.js APIs (like `child_process.exec()`) **must** set `isDesktopOnly: true` in `manifest.json`
- This is an accepted pattern -- the [Shell Commands plugin](https://github.com/Taitava/obsidian-shellcommands) (community-approved, 28k+ downloads) uses `child_process` extensively
- Mobile Obsidian does not support Node.js modules

The Shell Commands plugin demonstrates the pattern:

```ts
import { exec } from "child_process";

exec(command, { cwd: vaultPath }, (error, stdout, stderr) => {
  // handle result
});
```

### Community Plugin Publishing Process

1. Develop the plugin following [submission requirements](https://docs.obsidian.md/Plugins/Releasing/Submission+requirements+for+plugins)
2. Create a GitHub release with `main.js`, `manifest.json`, and optionally `styles.css`
3. Submit a PR to [obsidian-releases](https://github.com/obsidianmd/obsidian-releases) adding an entry to `community-plugins.json`
4. Obsidian team reviews the submission; community members may also review
5. After merge, the plugin appears in Obsidian's community plugin browser
6. Subsequent updates are published via GitHub releases only (no re-review)

Key restrictions: descriptions under 250 characters, no emoji, desktop-only flag for Node.js APIs, remove all sample code before submission.

## 2. Kanban Views in Obsidian

### Existing Obsidian Kanban Plugin

The [original Kanban plugin](https://github.com/mgmeyers/obsidian-kanban) by mgmeyers stores boards as **single markdown files** with a specific format:

```markdown
---
kanban-plugin: basic
---

## Backlog
- [ ] Task one
- [ ] Task two

## In Progress
- [ ] Active task

## Done
- [x] Completed task
```

Key characteristics:
- **Single file per board** -- columns are `##` headings, cards are list items with checkboxes
- Built with **Preact** (lightweight React alternative) and **esbuild**
- Codebase: 87% TypeScript, 7% JavaScript, 4% Less/Stylus
- Currently seeking new maintainers (reduced activity)

**This format is fundamentally incompatible with kanban-md's one-file-per-task approach.**

### Folder-Based Kanban Alternatives

Two plugins use a folder-based architecture closer to kanban-md's model:

1. **[obsidian-mkanban](https://github.com/blendonl/obsidian-mkanban)** -- A fork of the Kanban plugin that uses folders as columns and individual `.md` files as tasks. Structure:
   ```
   MyBoard/
   +-- board.md          (board metadata)
   +-- Todo/
   |   +-- task1.md
   |   +-- task2.md
   +-- Done/
       +-- task3.md
   ```
   Task metadata lives in YAML frontmatter. Drag-and-drop moves files between folders.

2. **[Bases Kanban](https://github.com/sil-so/obsidian-bases-kanban)** -- Groups existing vault notes into kanban columns based on a frontmatter property (default: `status`). Drag-and-drop updates the frontmatter property. Does not require specific folder structure.

### Custom View API (ItemView)

Obsidian provides the [ItemView](https://docs.obsidian.md/Reference/TypeScript+API/ItemView) class for building custom views:

```ts
import { ItemView, WorkspaceLeaf } from "obsidian";

const VIEW_TYPE = "kanban-md-board";

class KanbanMdView extends ItemView {
  constructor(leaf: WorkspaceLeaf) { super(leaf); }
  getViewType() { return VIEW_TYPE; }
  getDisplayText() { return "Kanban Board"; }

  async onOpen() {
    const container = this.containerEl.children[1];
    container.empty();
    // Render kanban board UI here
  }

  async onClose() {
    // Cleanup
  }
}
```

Registration in the plugin's `onload()`:

```ts
this.registerView(VIEW_TYPE, (leaf) => new KanbanMdView(leaf));
```

### Drag-and-Drop Feasibility

Drag-and-drop is fully feasible in Obsidian plugins:
- The plugin controls its own DOM within the view container
- Libraries like [SortableJS](https://sortablejs.github.io/Sortable/) or [dnd-kit](https://dndkit.com/) can be used
- Both the original Kanban plugin and the Bases Kanban plugin implement drag-and-drop successfully
- The Datablock plugin also implements drag-and-drop that auto-updates frontmatter on drop

Since the plugin has full control over its `ItemView` DOM, any JavaScript drag-and-drop library works. The existing kanban plugins prove this is a solved problem in the Obsidian ecosystem.

## 3. Integration Approaches

### Approach A: Pure Obsidian Plugin (No CLI Dependency)

**How it works:** The plugin reads `.md` files directly from `kanban/tasks/` using the Vault API, parses YAML frontmatter, renders a kanban board view, and writes changes back to files directly.

**Reading tasks:**
```ts
const taskFiles = this.app.vault.getMarkdownFiles()
  .filter(f => f.path.startsWith("kanban/tasks/"));
for (const file of taskFiles) {
  const content = await this.app.vault.cachedRead(file);
  // Parse frontmatter (id, title, status, priority, tags, etc.)
}
```

**Writing changes (e.g., move task):**
```ts
await this.app.fileManager.processFrontMatter(file, (fm) => {
  fm.status = "done";
  fm.updated = new Date().toISOString();
});
```

| Dimension | Assessment |
|---|---|
| **Development effort** | Large (L) -- must reimplement task parsing, validation, ID generation, config reading, all mutation logic |
| **User experience** | Excellent -- no external dependencies, instant response, works on mobile |
| **Maintenance burden** | High -- every CLI feature change must be mirrored in the plugin; two codebases implementing the same logic will inevitably diverge |
| **Repository** | Separate repository (TypeScript vs Go, different build toolchains) |
| **Mobile support** | Yes |

**Verdict:** Best UX but highest maintenance cost. Risk of behavioral divergence between CLI and plugin (e.g., different ID allocation, different timestamp handling, different validation rules).

### Approach B: CLI-Backed Plugin

**How it works:** The plugin executes kanban-md CLI commands for all operations using `child_process.exec()`. Uses `--json` output for structured data parsing.

**Reading tasks:**
```ts
import { exec } from "child_process";

function execCli(cmd: string): Promise<string> {
  return new Promise((resolve, reject) => {
    exec(`kanban-md ${cmd} --json`, { cwd: vaultPath }, (err, stdout, stderr) => {
      if (err) reject(new Error(stderr));
      else resolve(stdout);
    });
  });
}

// List all tasks
const tasks = JSON.parse(await execCli("list"));

// Move a task
await execCli("move 5 done");

// Create a task
await execCli('create "New task" --priority high --tags bug');
```

| Dimension | Assessment |
|---|---|
| **Development effort** | Medium (M) -- plugin is purely a view/interaction layer; all logic delegated to CLI |
| **User experience** | Good, but with caveats -- requires CLI installation, slight latency on each operation (shell spawn overhead ~50-100ms), error messages must be handled |
| **Maintenance burden** | Low -- plugin only needs to track CLI's `--json` output format, not internal logic |
| **Repository** | Could be same repo (Go + TypeScript) or separate; separate is cleaner |
| **Mobile support** | No (desktop only due to `child_process`) |

**Verdict:** Lowest maintenance, single source of truth for all logic. Requires users to install the CLI binary separately. Desktop-only limitation is significant.

### Approach C: Hybrid -- Read Files Directly, Write via CLI

**How it works:** Plugin reads task files directly from the vault for fast rendering (no shell overhead on reads), but all mutations (create, edit, move, delete) go through the CLI to ensure consistency.

**Reading:**
```ts
// Direct file access -- instant, no shell overhead
const taskFiles = this.app.vault.getMarkdownFiles()
  .filter(f => f.path.startsWith("kanban/tasks/"));
const tasks = await Promise.all(taskFiles.map(async (f) => {
  const content = await this.app.vault.cachedRead(f);
  return parseFrontmatter(content); // lightweight YAML parse
}));
```

**Writing:**
```ts
// Mutations via CLI -- consistent behavior
await execCli(`move ${taskId} ${newStatus}`);
// Then refresh the view by re-reading files
```

| Dimension | Assessment |
|---|---|
| **Development effort** | Medium (M) -- need frontmatter parser for reads, CLI wrapper for writes |
| **User experience** | Good -- fast board rendering, consistent mutations; drag-and-drop feels responsive because the UI can optimistically update before the CLI confirms |
| **Maintenance burden** | Low-Medium -- read-side must understand the frontmatter schema (but not business logic), write-side delegates to CLI |
| **Repository** | Separate recommended |
| **Mobile support** | No (CLI execution requires desktop) |

**Verdict:** Good balance of performance and consistency. Optimistic UI updates can mask CLI latency. Still desktop-only.

### Approach D: Leverage Existing Obsidian Kanban Plugin

**How it works:** Provide a converter/sync between kanban-md's task-per-file format and the Obsidian Kanban plugin's single-file-with-lists format.

The Obsidian Kanban plugin format:
```markdown
---
kanban-plugin: basic
---
## Backlog
- [ ] Task one
- [ ] Task two
## In Progress
- [ ] Active task
## Done
- [x] Completed task
```

kanban-md format (one file per task):
```markdown
---
id: 1
title: "Task title"
status: in-progress
priority: high
created: 2026-02-07T10:30:00Z
tags: [bug, frontend]
---
Task body in markdown.
```

**Implementation options:**
1. A `kanban-md export --format obsidian-kanban` CLI command that generates a `.md` file compatible with the Kanban plugin
2. A `kanban-md import --format obsidian-kanban` command to sync changes back
3. A file watcher or Obsidian plugin that auto-syncs between formats

| Dimension | Assessment |
|---|---|
| **Development effort** | Small (S) for export-only; Large (L) for bidirectional sync |
| **User experience** | Poor to Moderate -- two-way sync is fragile; one-way export loses information (task body, priority, tags, due dates have no representation in the Kanban plugin format) |
| **Maintenance burden** | Medium -- must track Kanban plugin format changes |
| **Repository** | Export command lives in kanban-md repo; sync plugin would be separate |
| **Mobile support** | Yes (if using existing Kanban plugin for viewing) |

**Verdict:** The formats are fundamentally incompatible. The Kanban plugin uses a flat list with no metadata per card (no priority, no tags, no due dates, no ID). A one-way export loses too much data. Bidirectional sync would be extremely fragile. **Not recommended as a primary approach**, though a read-only export could be a minor convenience feature.

## 4. Existing Precedent

### Plugins That Integrate with External CLI Tools

| Plugin | How it integrates | Desktop only? |
|---|---|---|
| [Shell Commands](https://github.com/Taitava/obsidian-shellcommands) | Generic `child_process.exec()` wrapper; user defines commands in settings | Yes |
| [Templater](https://github.com/SilentVoid13/Templater) | `tp.user.shell()` function for template expansion via shell | Yes |
| [Execute Code](https://github.com/twibiral/obsidian-execute-code) | Runs code blocks in various languages via system interpreters | Yes |
| [Obsidian Git](https://github.com/Vinzent03/obsidian-git) | Calls git CLI via `child_process` for vault version control | Yes |
| [Obsidian Terminal](https://github.com/polyipseity/obsidian-terminal) | Embeds full terminal emulator in Obsidian | Yes |

**Key pattern:** CLI integration via `child_process` is an established, community-accepted pattern. All such plugins are desktop-only, which is expected and accepted by the community.

### Relevant Data/View Plugins

| Plugin | Relevance to kanban-md |
|---|---|
| [Dataview](https://github.com/blacksmithgu/obsidian-dataview) | Queries vault files by frontmatter; could render kanban-md tasks as tables. Proves that frontmatter-based querying at scale is feasible |
| [Tasks](https://github.com/obsidian-tasks-group/obsidian-tasks) | Manages tasks via inline markdown checkboxes with metadata; different paradigm but shows custom task management in Obsidian is popular |
| [CardBoard](https://forum.obsidian.md/t/plugin-cardboard-a-kanban-for-your-markdown-tasks/28314) | Kanban view that reads Tasks plugin data; demonstrates rendering a kanban board from distributed task data |
| [Bases Kanban](https://github.com/sil-so/obsidian-bases-kanban) | Groups notes by frontmatter property into kanban columns with drag-and-drop. **Most architecturally similar to what we need** |
| [Datablock](https://github.com/majd3000/datablock) | Renders data as kanban/gallery/list from vault data; drag-and-drop updates frontmatter |
| [mkanban](https://github.com/blendonl/obsidian-mkanban) | Folder-based kanban with individual task files. **Closest existing implementation to kanban-md's model** |

### Especially Relevant: Bases Kanban and mkanban

**Bases Kanban** demonstrates that you can:
- Read frontmatter `status` field from individual note files
- Group notes into kanban columns by status
- Drag-and-drop to update the status property in frontmatter
- No dependency on specific folder structure

**mkanban** demonstrates that you can:
- Use individual `.md` files as task cards
- Organize by folders (our `kanban/tasks/` would be the root)
- Render a full kanban board with drag-and-drop
- Built on Preact (same as original Kanban plugin)

Both prove that our desired UX is achievable.

## 5. Feasibility Assessment Summary

| Dimension | A: Pure Plugin | B: CLI-Backed | C: Hybrid | D: Existing Kanban |
|---|---|---|---|---|
| **Dev effort** | L | M | M | S (export) / L (sync) |
| **UX quality** | Excellent | Good | Good | Poor |
| **Maintenance** | High | Low | Low-Medium | Medium |
| **Mobile** | Yes | No | No | Yes (read-only) |
| **Consistency** | Risk of divergence | Perfect | High | Low |
| **Separate repo** | Yes | Either | Yes | N/A (CLI feature) |
| **Time to MVP** | 4-6 weeks | 2-3 weeks | 2-4 weeks | 1 week (export only) |

## 6. Recommendation

**Start with Approach C (Hybrid), with a path toward Approach A.**

### Rationale

1. **Approach C provides the fastest path to a useful plugin.** Reading files directly gives instant board rendering. Using the CLI for mutations guarantees behavioral consistency from day one. The CLI's `--json` flag already exists for all commands.

2. **The read-only file parsing is low-risk.** Parsing YAML frontmatter in TypeScript is trivial (use the `gray-matter` npm package). The schema is simple and stable. Even if the CLI adds new fields, the plugin gracefully ignores unknown fields.

3. **CLI dependency is acceptable for the initial audience.** Users of a CLI kanban tool already have the binary installed. Desktop-only is a reasonable constraint for v1.

4. **Approach A is the long-term goal.** Over time, as the plugin matures, the mutation logic can be ported to TypeScript to eliminate the CLI dependency and enable mobile support. This can be done incrementally (e.g., move `move` to TypeScript first, then `create`, etc.).

5. **Approach D is not viable as a primary strategy** due to fundamental format incompatibility, but a one-way `export` subcommand could be added to the CLI as a low-effort bonus.

### Suggested Architecture for v1 (Approach C)

```
obsidian-kanban-md/              # Separate repository
+-- src/
|   +-- main.ts                  # Plugin entry, registers view
|   +-- KanbanMdView.ts          # ItemView subclass, renders board
|   +-- TaskParser.ts            # Parses frontmatter from .md files
|   +-- CliExecutor.ts           # Wraps child_process.exec for CLI calls
|   +-- BoardRenderer.ts         # DOM rendering (consider Preact or Svelte)
|   +-- DragDropHandler.ts       # SortableJS integration
|   +-- settings.ts              # Plugin settings (CLI path, task folder)
+-- manifest.json                # isDesktopOnly: true
+-- styles.css
+-- esbuild.config.mjs
+-- package.json
+-- tsconfig.json
```

### Key Design Decisions

- **UI framework:** Preact (lightweight, proven in Obsidian kanban plugins) or Svelte (gaining traction in Obsidian ecosystem). Vanilla DOM is also viable for a simple board.
- **Drag-and-drop:** SortableJS is well-tested and has no framework dependency.
- **Settings:** Configurable CLI binary path (default: `kanban-md` in PATH), configurable task directory (default: `kanban/tasks/`).
- **Reactivity:** Watch `kanban/tasks/` for file changes using `vault.on('modify', ...)` and `vault.on('create', ...)` events to keep the board in sync with external CLI usage.
- **Optimistic updates:** On drag-and-drop, update the UI immediately, then fire the CLI command in the background. If the CLI fails, revert the UI and show an error notice.

## Sources

- [Obsidian Sample Plugin (GitHub)](https://github.com/obsidianmd/obsidian-sample-plugin)
- [Obsidian Plugin API Reference](https://docs.obsidian.md/Reference/TypeScript+API/Plugin)
- [Obsidian Vault API](https://docs.obsidian.md/Plugins/Vault)
- [Obsidian Views Documentation](https://docs.obsidian.md/Plugins/User+interface/Views)
- [Obsidian ItemView API](https://docs.obsidian.md/Reference/TypeScript+API/ItemView)
- [Obsidian Submission Requirements](https://docs.obsidian.md/Plugins/Releasing/Submission+requirements+for+plugins)
- [Obsidian Kanban Plugin (GitHub)](https://github.com/mgmeyers/obsidian-kanban)
- [obsidian-mkanban -- Folder-Based Kanban (GitHub)](https://github.com/blendonl/obsidian-mkanban)
- [Bases Kanban Plugin (GitHub)](https://github.com/sil-so/obsidian-bases-kanban)
- [Datablock Plugin (GitHub)](https://github.com/majd3000/datablock)
- [Shell Commands Plugin (GitHub)](https://github.com/Taitava/obsidian-shellcommands)
- [Dataview Plugin (GitHub)](https://github.com/blacksmithgu/obsidian-dataview)
- [processFrontMatter API](https://docs.obsidian.md/Reference/TypeScript+API/FileManager/processFrontMatter)
- [Obsidian Community Plugins Registry (GitHub)](https://github.com/obsidianmd/obsidian-releases)
