# Research: Web UI Approaches for kanban-md

**Date:** 2026-02-08
**Goal:** Evaluate approaches for adding a web-based board view to kanban-md, where the CLI remains the source of truth for all mutations and the web UI is primarily a view layer.

---

## Context

kanban-md stores tasks as individual markdown files with YAML frontmatter in a local directory. The CLI already supports `--json` output on all read commands (`list`, `show`, `board`, `metrics`, `log`) and has structured commands for mutations (`create`, `edit`, `move`, `delete`). The task data model includes id, title, status, priority, tags, assignee, due date, dependencies, blocked state, timestamps, and a markdown body.

The key design constraint is that **all writes must go through CLI commands**, not a custom backend. The web UI should be a thin view layer that either delegates mutations to the CLI or is entirely read-only.

---

## Approach A: CLI Serves Its Own Web UI (`kanban-md serve`)

### Description

Add a `serve` command to the CLI that starts a local HTTP server on a configurable port. The server:
- Serves a static HTML/CSS/JS frontend embedded in the binary via `go:embed`
- Exposes a thin JSON API where each endpoint shells out to the CLI itself (or more practically, calls the existing Go functions directly from the `board`, `task`, and `config` packages)
- The frontend fetches data from these API endpoints and renders a kanban board

### How Mutations Would Work

The frontend sends API requests (e.g., `POST /api/move` with `{id: 5, status: "done"}`). The server handler calls the same internal Go functions that the CLI commands use -- `task.Read()`, `task.Save()`, `board.List()`, etc. -- rather than literally shelling out to the CLI binary via `os/exec`. This avoids the overhead and fragility of spawning subprocesses while keeping the same code paths.

Alternatively, for strict separation, the server could literally execute `kanban-md move 5 done --json` via `os/exec.Command()`. This guarantees identical behavior to the CLI but adds process overhead and makes error handling more complex.

For drag-and-drop, the frontend would capture the drop event, determine the source task ID and target column (status), then call `POST /api/move`.

### Implementation Complexity

**Medium.** The major pieces:

1. **Embedded frontend** (~200-400 lines of HTML/CSS/JS for a basic kanban board). Use `//go:embed web/*` to include the frontend directory. A single `index.html` with inline CSS and JS keeps things simple.

2. **HTTP server** (~100-150 lines of Go). Use `net/http` from the standard library. Define routes for:
   - `GET /` -- serve the embedded `index.html`
   - `GET /api/board` -- return board summary (calls `board.Summary()`)
   - `GET /api/tasks` -- return task list (calls `board.List()`)
   - `GET /api/tasks/{id}` -- return single task (calls `task.FindByID()`)
   - `POST /api/tasks/{id}/move` -- move task (calls `task.Read()`, update status, `task.Save()`)
   - `POST /api/tasks` -- create task
   - `DELETE /api/tasks/{id}` -- delete task
   - `GET /api/config` -- return board config (statuses, priorities, etc.)

3. **Cobra command** (~30 lines). A `serveCmd` with `--port` and `--open` (auto-open browser) flags.

### Key Go Libraries

| Library | Purpose | Notes |
|---------|---------|-------|
| `embed` (stdlib) | Embed static files in binary | Available since Go 1.16. Use `//go:embed web/*` |
| `net/http` (stdlib) | HTTP server | No external dependency needed for simple routing |
| `encoding/json` (stdlib) | JSON serialization | Already used throughout the project |
| `html/template` (stdlib) | Optional: server-side rendering | Could be used instead of a JS frontend |

No external dependencies are required for this approach.

### Real-World Examples

- **Prometheus** -- embeds its entire web UI (React app) into the binary using `go:embed`. The web UI consumes the same `/api/v1/query` endpoints available externally. See [prometheus/web/ui](https://pkg.go.dev/github.com/prometheus/prometheus/web/ui).
- **Grafana Alloy** -- embeds a monitoring UI with optional build tag `builtinassets`. See [grafana/alloy/internal/web/ui](https://pkg.go.dev/github.com/grafana/alloy/internal/web/ui).
- **Dagu** -- a self-contained workflow engine with a built-in web UI. Single binary, no external database. Workflows defined in YAML, UI embedded in the binary. Very similar architectural pattern to what kanban-md would do. See [dagu-org/dagu](https://github.com/dagu-org/dagu).

### Pros

- **Single binary distribution** -- no separate frontend build, no separate server process
- **Full mutation support** -- can handle create, move, edit, delete through API endpoints
- **Consistent with CLI** -- uses the same internal functions, guaranteed same behavior
- **No external dependencies** -- stdlib only for the server component
- **Browser auto-open** -- can open the user's default browser on `serve`
- **Extensible** -- easy to add more API endpoints as the CLI grows

### Cons

- **Most code to write** -- needs both frontend JS and backend HTTP handlers
- **Frontend maintenance** -- HTML/CSS/JS needs to be maintained alongside Go code
- **Security surface** -- local HTTP server is accessible to any local process (mitigated by binding to localhost only)
- **Not auto-refreshing** -- requires manual refresh or polling unless combined with Approach D

---

## Approach B: File-Watcher with Exported JSON

### Description

The CLI has an `export` or `watch` command that:
1. Runs `kanban-md list --json` and writes the output to a known JSON file (e.g., `kanban/board.json`)
2. Optionally watches the tasks directory for changes and re-exports automatically
3. A standalone HTML file reads this JSON and renders the board

The HTML file could be opened via `file://` protocol or served by a minimal HTTP server.

### How Mutations Would Work

This approach is primarily **read-only**. The HTML file displays the board but has no way to send mutations back. Users would use the CLI for all changes. The watch mode would detect the file changes and update the JSON export, and the browser would either:
- Poll the JSON file periodically (works with a minimal server, not with `file://`)
- Require manual refresh

For mutations to work, the HTML page would need a companion server, which essentially becomes Approach A.

### Implementation Complexity

**Low for read-only, medium if adding a server.**

1. **Export command** (~40 lines of Go). Read all tasks, marshal to JSON, write to file.
2. **File watcher** (~60 lines of Go). Use `fsnotify` to watch the tasks directory, re-export on changes.
3. **HTML file** (~150-250 lines). Standalone HTML with embedded CSS/JS that fetches and renders `board.json`.

### Key Go Libraries

| Library | Purpose | Notes |
|---------|---------|-------|
| `fsnotify/fsnotify` | Watch filesystem changes | Well-maintained, cross-platform. v1.8.0+ |
| `encoding/json` (stdlib) | JSON export | Already used |

### Real-World Examples

- **Hugo** -- has a `hugo server` command that watches for file changes and live-reloads, though it serves generated HTML rather than JSON
- **Jupyter notebooks** -- export to HTML for static viewing

### Pros

- **Very simple** -- minimal code for the export command
- **No server needed for basic view** -- open HTML file directly (if JSON is inlined or embedded)
- **Separation of concerns** -- CLI handles data, HTML handles display

### Cons

- **Read-only** -- no mutation support without adding a server (which makes this Approach A)
- **`file://` CORS issues** -- browsers block `fetch()` to local files from `file://` pages. The JSON would need to be inlined into the HTML, or a server is needed
- **Stale data** -- without a watcher, the export is a point-in-time snapshot
- **Extra file** -- generates a `board.json` that can go stale and clutter the directory
- **Two-step workflow** -- run export, then open HTML

---

## Approach C: Generated Static HTML (`kanban-md board --html`)

### Description

Add an `--html` flag to the `board` command (or a new `export` command) that generates a self-contained HTML file. The HTML file includes all task data embedded as inline JSON, plus CSS and JS for rendering a kanban board. No server needed -- just open the file in a browser.

### How Mutations Would Work

**No mutations.** This is a pure read-only snapshot. To see updated data, regenerate the HTML and refresh the browser. Could be combined with a file watcher that auto-regenerates on task changes.

### Implementation Complexity

**Low.** This is the simplest approach.

1. **HTML template** (~200-300 lines). Use `html/template` to render a kanban board with task data injected as a `<script>` tag containing JSON. Include inline CSS for the board layout and inline JS for rendering.
2. **Command flag** (~20 lines of Go). Add `--html` flag that renders the template to stdout or a file.
3. **Embedded template** (~10 lines). Use `//go:embed` to include the HTML template.

```go
//go:embed templates/board.html
var boardTemplate string

// In the command handler:
tmpl := template.Must(template.New("board").Parse(boardTemplate))
data := struct {
    Config *config.Config
    Tasks  []*task.Task
}{cfg, tasks}
tmpl.Execute(os.Stdout, data)
```

### Key Go Libraries

| Library | Purpose | Notes |
|---------|---------|-------|
| `html/template` (stdlib) | Generate HTML | Safe HTML output, supports embedded data |
| `embed` (stdlib) | Embed the template | Keep template in a separate file |

### Real-World Examples

- **k6** -- has a `--out web-dashboard` option that generates a single-file HTML report from load test results. See [xk6-dashboard](https://pkg.go.dev/github.com/grafana/xk6-dashboard/cmd/k6-web-dashboard).
- **go test -coverprofile + go tool cover -html** -- generates a self-contained HTML coverage report
- **sfkboard** -- a [single-file HTML/CSS/jQuery kanban board](https://github.com/dgski/sfkboard) that demonstrates the pattern of a completely self-contained board in one HTML file

### Pros

- **Simplest to implement** -- least code, least complexity
- **No server needed** -- just open the file in a browser
- **No external dependencies** -- stdlib only
- **Portable** -- the HTML file can be shared, attached to emails, stored in version control
- **Works offline** -- no network access needed
- **Pipeable** -- `kanban-md board --html > board.html && open board.html`
- **Good for CI/CD** -- generate board snapshots as build artifacts

### Cons

- **Read-only** -- no mutation support at all
- **Stale instantly** -- a snapshot that requires manual regeneration
- **No live updates** -- must re-run the command and refresh the browser
- **Duplicate data** -- task data is duplicated into the HTML file

---

## Approach D: WebSocket/SSE Live View

### Description

Extend Approach A with real-time updates. The `serve` command:
1. Watches the kanban directory using `fsnotify`
2. When task files change, pushes updates to connected browsers via WebSocket or Server-Sent Events (SSE)
3. The frontend updates the board in-place without requiring a page refresh

### How Mutations Would Work

Same as Approach A -- the frontend sends API requests to the server, which calls internal functions. The difference is that after a mutation, the file watcher detects the change and broadcasts an update to all connected clients. This also means changes made via the CLI (in another terminal) are reflected in the browser automatically.

**Drag-and-drop flow:**
1. User drags card from "todo" to "in-progress"
2. Frontend calls `POST /api/tasks/5/move` with `{status: "in-progress"}`
3. Server updates the task file
4. fsnotify detects the file change
5. Server pushes updated board state via SSE/WebSocket
6. Frontend re-renders the board

### Implementation Complexity

**Medium-high.** Builds on Approach A and adds:

1. **File watcher** (~50 lines). Use `fsnotify` to watch the tasks directory, debounce rapid changes (multiple files might change in quick succession during batch operations).
2. **SSE or WebSocket endpoint** (~40-60 lines).
   - **SSE** (recommended): Simpler to implement, works over standard HTTP, no special libraries needed. Add a `GET /api/events` endpoint that keeps the connection open and sends events.
   - **WebSocket**: More flexible (bidirectional) but requires a library. Use `github.com/coder/websocket` (the maintained successor of `nhooyr.io/websocket`) for a minimal, idiomatic implementation.
3. **Frontend event handling** (~30 lines of JS). Listen for SSE events and re-fetch board data, or apply incremental updates.

**SSE is strongly recommended over WebSocket** for this use case because:
- It only needs server-to-client communication (file change notifications)
- It works with the Go stdlib (`http.Flusher` interface) -- no external library needed
- It automatically reconnects on connection loss
- htmx has native SSE support via the `sse` extension, if htmx is used for the frontend

### Key Go Libraries

| Library | Purpose | Notes |
|---------|---------|-------|
| `fsnotify/fsnotify` | Watch filesystem | Cross-platform, well-maintained |
| `net/http` (stdlib) | SSE endpoint | Use `http.Flusher` for streaming |
| `github.com/coder/websocket` | WebSocket (if needed) | Minimal, idiomatic. Only if bidirectional comms needed |

### Real-World Examples

- **Brandur's live reload** -- a [detailed writeup](https://brandur.org/live-reload) on building a live reloader with WebSockets and Go, including debouncing fsnotify events
- **Three Dots Labs** -- [live website updates with Go, SSE, and htmx](https://threedots.tech/post/live-website-updates-go-sse-htmx/), demonstrating the Go + SSE + htmx stack
- **Dagu** -- uses WebSocket in its embedded web UI for live log tailing and DAG status updates

### Pros

- **Live updates** -- board reflects changes instantly, including changes made via CLI
- **Multi-user awareness** -- multiple browser tabs or users see the same live state
- **Best UX** -- feels like a real web app
- **SSE is simple** -- with stdlib-only SSE, adds minimal complexity over Approach A

### Cons

- **Most complex** -- file watching, debouncing, event broadcasting, frontend event handling
- **Resource overhead** -- file watcher + SSE connections consume resources (minimal, but non-zero)
- **Edge cases** -- need to handle watcher errors, connection drops, rapid changes, race conditions between file write and read
- **Testing complexity** -- harder to test the real-time pipeline

---

## Comparison Matrix

| Criterion | A: Serve | B: File-Watcher | C: Static HTML | D: Live View |
|-----------|----------|-----------------|----------------|--------------|
| **Mutations** | Yes (API) | No | No | Yes (API) |
| **Live updates** | No (polling) | Partial | No | Yes (SSE/WS) |
| **Server required** | Yes (embedded) | Optional | No | Yes (embedded) |
| **External deps** | None | fsnotify | None | fsnotify |
| **Implementation effort** | Medium | Low | Low | Medium-High |
| **Lines of Go code** | ~200 | ~100 | ~50 | ~300 |
| **Lines of frontend code** | ~300 | ~200 | ~250 | ~350 |
| **Single binary** | Yes | Yes | Yes | Yes |
| **Works offline** | Yes (localhost) | Yes | Yes | Yes (localhost) |
| **Auto-refresh** | No | No | No | Yes |
| **Sharable output** | No | No | Yes (HTML file) | No |
| **CI/CD friendly** | No | No | Yes | No |

---

## The "View Layer Over CLI" Pattern

Several tools follow the pattern of a web UI that delegates all writes to an underlying CLI or API:

### Taskwarrior Web UIs

[taskwarrior-webui](https://github.com/DCsunset/taskwarrior-webui) is a Vue.js + Koa.js web interface that shells out to `task` CLI commands. It periodically runs `task sync` and executes `task add`, `task modify`, etc. for mutations. This is a separate Node.js process, not embedded in the CLI. [taskwarrior-web](https://github.com/theunraveler/taskwarrior-web) (Ruby/Sinatra) follows the same pattern.

**Lesson:** Separate web server processes work but add deployment complexity. The embedded approach (Approach A) avoids this.

### gh-dash

[gh-dash](https://github.com/dlvhdr/gh-dash) is a TUI (terminal UI), not a web UI. It uses the GitHub API directly (via `gh` CLI's auth) for both reads and writes. It demonstrates that a view layer over an API can provide a rich interactive experience. However, it chose TUI over web precisely to avoid the complexity of a web server.

### Dagu

[Dagu](https://github.com/dagu-org/dagu) is the closest architectural match. It is a single Go binary with an embedded web UI that manages YAML-defined workflows stored on the local filesystem. The web UI communicates with the same internal functions that the CLI uses. It demonstrates that the pattern works well in practice.

### Portainer vs. Lazydocker

[Portainer](https://github.com/portainer/portainer) runs a web server that wraps the Docker API. [Lazydocker](https://github.com/jesseduffield/lazydocker) chose a TUI instead, connecting directly to the Docker socket. Portainer's approach maps to Approach A; Lazydocker's maps to the existing CLI-only approach.

**Key insight from this comparison:** Web UIs add resource overhead and maintenance burden. A TUI is lighter but less accessible. For kanban-md, the embedded web UI (Approach A) is a good middle ground because the server only runs when explicitly started with `serve`.

---

## Frontend Technology Choices

For the frontend (relevant to Approaches A and D), there are several options:

### Option 1: Vanilla HTML/CSS/JS (Recommended for v1)

A single `index.html` file with:
- CSS Grid or Flexbox for the kanban column layout
- HTML5 Drag and Drop API for card movement
- `fetch()` for API calls
- ~250-350 lines total

This approach has zero dependencies, embeds trivially, and is easy to maintain. [MDN's Kanban board tutorial](https://developer.mozilla.org/en-US/docs/Web/API/HTML_Drag_and_Drop_API/Kanban_board) provides a complete reference implementation.

### Option 2: htmx

[htmx](https://htmx.org/) (14kb gzipped) adds AJAX, SSE, and WebSocket support via HTML attributes. This is attractive for Approach D because htmx has a built-in [SSE extension](https://htmx.org/extensions/sse/). The server would render HTML fragments (using `html/template`) and htmx would swap them in. This eliminates the need for a JSON API + client-side rendering.

However, htmx adds a dependency (albeit small) and a different mental model from pure client-side rendering.

### Option 3: Framework (React, Vue, Svelte)

Overkill for this use case. Adds a build step (npm, bundler), increases maintenance burden, and the embed size grows significantly. Only consider if the UI becomes much more complex in the future.

**Recommendation:** Start with vanilla HTML/CSS/JS. If SSE/live updates are added later, consider migrating to htmx at that point.

---

## Recommended Approach: Phased Implementation

### Phase 1: Static HTML Export (Approach C)

Start with the simplest option. Add `kanban-md board --html` that generates a self-contained HTML file. This provides immediate value with minimal effort (~50 lines of Go, ~250 lines of HTML template).

```
kanban-md board --html > board.html
open board.html
```

This is useful for:
- Quick visual overview of the board
- Sharing board state (attach to a PR, email, etc.)
- CI/CD artifacts (generate board snapshot on each commit)

### Phase 2: Embedded Server (Approach A)

Add `kanban-md serve` with an embedded web UI and API. Reuse the HTML template from Phase 1 as the starting point, but make it dynamic by fetching data from the API.

```
kanban-md serve --port 8080
```

Add drag-and-drop for moving tasks between columns. This provides the core interactive experience.

### Phase 3: Live Updates (Approach D)

Add `fsnotify` file watching and SSE to the `serve` command. When task files change (from CLI usage in another terminal, or from direct file editing), the browser updates automatically.

This phased approach means each phase delivers value independently, and later phases build on earlier work without throwing anything away.

---

## Implementation Notes

### API Design (for Phases 2-3)

```
GET  /                          -- serve embedded frontend
GET  /api/config                -- board config (statuses, priorities, WIP limits)
GET  /api/tasks                 -- all tasks (supports ?status=X&priority=Y filters)
GET  /api/tasks/{id}            -- single task with body
POST /api/tasks                 -- create task (body: {title, priority, tags, ...})
PUT  /api/tasks/{id}            -- edit task (body: {title?, priority?, status?, ...})
POST /api/tasks/{id}/move       -- move task (body: {status})
DELETE /api/tasks/{id}          -- delete task
GET  /api/board                 -- board summary/overview
GET  /api/events                -- SSE stream (Phase 3)
```

### Embedding Frontend Files

```go
package web

import "embed"

//go:embed static/*
var StaticFiles embed.FS
```

Serve with:
```go
sub, _ := fs.Sub(web.StaticFiles, "static")
http.Handle("/", http.FileServer(http.FS(sub)))
```

### SSE Implementation (Phase 3)

SSE can be implemented with stdlib only, using `http.Flusher`:

```go
func sseHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    // Subscribe to file change events
    ch := broker.Subscribe()
    defer broker.Unsubscribe(ch)

    for {
        select {
        case <-r.Context().Done():
            return
        case event := <-ch:
            fmt.Fprintf(w, "event: update\ndata: %s\n\n", event)
            flusher.Flush()
        }
    }
}
```

### Debouncing File Events

Multiple file changes can happen in quick succession (e.g., `kanban-md move 1,2,3 done` modifies three files). Debounce with a timer:

```go
var debounceTimer *time.Timer
const debounceDelay = 100 * time.Millisecond

for event := range watcher.Events {
    if debounceTimer != nil {
        debounceTimer.Stop()
    }
    debounceTimer = time.AfterFunc(debounceDelay, func() {
        broker.Broadcast("board-updated")
    })
}
```

---

## Summary

| Phase | What | Effort | Value |
|-------|------|--------|-------|
| Phase 1 | `board --html` static export | ~1 day | Visual board snapshots, CI integration |
| Phase 2 | `serve` with embedded UI + API | ~2-3 days | Interactive board with drag-and-drop |
| Phase 3 | SSE live updates + fsnotify | ~1 day | Real-time sync between CLI and browser |

The phased approach starts delivering value immediately (Phase 1) while building toward a fully interactive live board (Phase 3). Each phase is independently useful and builds on the previous one.
