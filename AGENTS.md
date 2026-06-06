# AGENTS.md — mysql-copy Architecture

## Project purpose

A Go single-page web application for copying a MySQL table from one server to another. The destination table is dropped and recreated using the source DDL with `AUTO_INCREMENT` removed. The backend is idiomatic Go using only the standard library plus two small external packages. The frontend uses HTMX — no JavaScript build step, no frontend framework.

---

## Repository layout

```
mysql-copy/
├── cmd/server/main.go              # Binary entry point
├── internal/
│   ├── config/config.go            # Environment-based configuration
│   ├── handler/
│   │   ├── handler.go              # Handler type, template rendering, isHTMX helper
│   │   ├── middleware.go           # Request logging middleware
│   │   ├── mysql.go                # MySQL helpers: openDB, queryDatabases, queryTables
│   │   ├── pages.go                # All HTTP handlers
│   │   └── routes.go               # Route registration (single source of truth)
│   ├── server/server.go            # http.Server construction with timeouts
│   └── store/
│       └── connections.go          # Connection struct + JSON file store
├── web/
│   ├── embed.go                    # //go:embed — bakes templates/ and static/ into binary
│   ├── templates/
│   │   ├── layout/base.html        # Shared HTML shell: nav, Connection dropdown, modals
│   │   ├── pages/index.html        # Main page — 3-panel layout (source, dest, progress)
│   │   └── partials/
│   │       ├── connections.html        # Saved connections list fragment
│   │       ├── source-databases.html   # Source database select (cascades to tables)
│   │       ├── source-tables.html      # Source table select
│   │       └── dest-databases.html     # Destination database select
│   └── static/
│       ├── css/app.css             # All styles: variables, dark mode, panels, modals, forms
│       └── js/app.js               # Dropdown, modal, cascading-select, copy-ready logic
├── .env.example                    # Documented environment variables
├── Makefile                        # run / build / test / tidy / lint / clean
└── go.mod                          # Module: github.com/oofnivek/mysql-copy
```

---

## Request lifecycle

```
SIGINT/SIGTERM
      │
      ▼
cmd/server/main.go
  godotenv.Load()              ← reads .env if present
  config.Load()                ← maps env vars to Config struct
  slog setup                   ← structured text logger
  server.New(cfg, logger, web.FS)
      │
      ▼
internal/server/server.go
  handler.New(cfg, logger, webFS)   ← parses templates, creates store
  http.Server{timeouts}
      │
      ▼
internal/handler/routes.go    ← Go 1.22 method+path mux
  GET  /static/*               → embedded static files
  GET  /                       → handleIndex
  GET  /api/health             → handleHealth
  GET  /api/connections        → handleListConnections
  POST /api/connections        → handleCreateConnection
  GET  /api/source/databases   → handleSourceDatabases
  GET  /api/source/tables      → handleSourceTables
  GET  /api/dest/databases     → handleDestDatabases
  POST /api/copy               → handleCopy
      │
      ▼
internal/handler/middleware.go
  responseWriter wrapper       ← captures status code
  slog.Debug per request       ← method, path, status, duration
```

---

## Key packages

### `internal/config`

Reads env vars into a `Config` struct. Loaded once at startup; passed as a pointer everywhere.

| Field             | Env var            | Default                          |
|-------------------|--------------------|----------------------------------|
| `Addr`            | `ADDR`             | `:8080`                          |
| `Env`             | `ENV`              | `development`                    |
| `LogLevel`        | (derived)          | Debug in dev, Info in production |
| `ConnectionsFile` | `CONNECTIONS_FILE` | `~/.mysql-copy/connections.json` |

### `internal/store`

`Connection` struct and `Connections` file store. Persists to a JSON file protected by a `sync.Mutex`.

**`Connection` struct JSON keys** — the file uses `"user"` (not `"username"`) to match the pre-existing connections file format. Do not change this tag without migrating the file.

```
id, name, host, port, user, password (omitempty), database (omitempty), created_at
```

**Public methods:**
- `List() ([]Connection, error)` — read all connections
- `GetByName(name string) (*Connection, error)` — look up by name; used by all MySQL query handlers
- `Save(c Connection) error` — assign ID + timestamp, append, persist

The file and its parent directory are created automatically on first save (`0700` dir, `0600` file).

### `internal/handler`

All HTTP logic. The `Handler` struct holds config, logger, parsed templates, static `fs.FS`, and `*store.Connections`.

- **`handler.go`** — constructor; parses all template globs (`layout/*.html`, `pages/*.html`, `partials/*.html`); `render` executes a named template; `isHTMX` checks the `HX-Request` header.
- **`routes.go`** — single source of truth for all routes. Add new routes here only.
- **`middleware.go`** — request logging. Add auth, CORS, or rate-limiting here.
- **`mysql.go`** — package-level MySQL helpers used by handlers:
  - `openDB(conn)` — opens a `*sql.DB` without selecting a database; 1 max connection, 30s lifetime.
  - `queryDatabases(conn)` — runs `SHOW DATABASES`, filters out `information_schema`, `performance_schema`, `mysql`, `sys`.
  - `queryTables(conn, db)` — runs `SHOW TABLES FROM \`db\``.
- **`pages.go`** — one function per route:
  - `handleIndex` — loads saved connections, passes them to `index.html` for pre-populating dropdowns.
  - `handleListConnections` — renders `connections-list` partial (used by Saved modal).
  - `handleCreateConnection` — validates form, calls `pingMySQL`, saves on success.
  - `handleSourceDatabases` — looks up `source_conn`, calls `queryDatabases`, renders `source-databases`.
  - `handleSourceTables` — looks up `source_conn` + `source_db`, calls `queryTables`, renders `source-tables`.
  - `handleDestDatabases` — looks up `dest_conn`, calls `queryDatabases`, renders `dest-databases`.
  - `handleCopy` — validates all 5 fields, logs the job, writes first progress line to `#progress-log`.
  - `respondAlert(w, status, ok, msg)` — shared helper; writes an `<div class="alert alert-*">` fragment.

### `internal/server`

Thin constructor. `http.Server` timeouts: read 5s, write 10s, idle 120s.

### `web`

All frontend assets compiled into the binary via `//go:embed`. No files are read from disk at runtime.

Templates use Go's `html/template`. The `base` layout is defined with `{{define "base"}}`. Page templates extend it with `{{template "base" .}}` + `{{define "content"}}`. Partials are standalone `{{define "name"}}` blocks rendered directly by handlers via `h.render`.

---

## UI layout

The main page has no footer. It is a full-viewport 3-panel CSS Grid:

```
┌─────────────────────────┬──────────────────────────┐
│  SOURCE                 │  DESTINATION             │
│  Connection ▾           │  Connection ▾            │
│  Database   ▾           │  Database   ▾            │
│  Table      ▾           │                          │
├─────────────────────────┴──────────────────────────┤
│  PROGRESS                          [Start Copy]    │
│  (monospace log output)                            │
└────────────────────────────────────────────────────┘
```

Grid definition: `grid-template-columns: 1fr 1fr`, `grid-template-rows: 1fr 220px`, named areas `source / dest / progress`. The `1px` gap between panels uses `background-color: var(--color-border)` on the grid container.

**Cascading selects (HTMX):**
1. Source Connection changes → `GET /api/source/databases?source_conn=NAME` → swaps `#source-db-wrap` with database select partial (which contains `#source-table-wrap`).
2. Source Database changes → `GET /api/source/tables?source_conn=NAME&source_db=DB` → swaps `#source-table-wrap` with table select partial.
3. Dest Connection changes → `GET /api/dest/databases?dest_conn=NAME` → swaps `#dest-db-wrap` with database select partial.

**Start Copy button** — lives in the Progress panel header. Disabled by default. `checkCopyReady()` in `app.js` runs on every `change` event and every `htmx:afterSwap`; it queries all five selects by `name` attribute and enables the button only when all have a non-empty value.

**Source → Destination exclusion** — when `source_conn` changes, the matching `<option>` in `dest_conn` is disabled. If dest already had the same value selected, it is reset to the placeholder and `#dest-db-wrap` is cleared.

---

## Nav and modals

The nav has a **Connection ▾** dropdown with two items:

- **Create** — opens `#modal-connection` (form: name, host, port, username, password, database). On submit, HTMX POSTs to `POST /api/connections`; the handler pings MySQL with a 5s timeout before saving. Success auto-closes the modal after 1.2s.
- **Saved** — fires `GET /api/connections`, populates `#saved-connections-list`, then opens `#modal-saved`.

Modals use the `hidden` attribute as the visibility toggle. `openModal` / `closeModal` are global JS functions. ESC closes any open modal. Clicking the backdrop also closes.

---

## Environment variables

| Variable            | Default                            | Description                        |
|---------------------|------------------------------------|------------------------------------|
| `ADDR`              | `:8080`                            | TCP address the server listens on  |
| `ENV`               | `development`                      | `development` or `production`      |
| `CONNECTIONS_FILE`  | `~/.mysql-copy/connections.json`   | Path to the saved connections file |

Copy `.env.example` to `.env` to override locally. `.env` is gitignored.

---

## Adding a new page

1. Create `web/templates/pages/yourpage.html` — use `{{template "base" .}}` and `{{define "content"}}`.
2. Add a handler in `internal/handler/pages.go`.
3. Register the route in `internal/handler/routes.go`.
4. Add a nav link in `web/templates/layout/base.html` if needed.

## Adding a new API endpoint

1. Add a handler in `internal/handler/pages.go` (or a new file under `internal/handler/`).
2. Register the route in `internal/handler/routes.go` with the method prefix (`POST /api/...`).
3. Use `h.respondAlert` for HTMX fragment responses or `json.NewEncoder(w).Encode` for JSON.

## Adding a new HTMX partial

1. Create `web/templates/partials/yourpartial.html` with `{{define "yourpartial"}}`.
2. Call `h.render(w, r, "yourpartial", data)` from the handler — no glob change needed, the constructor already parses `partials/*.html`.

## Adding middleware

Wrap the existing chain in `internal/handler/middleware.go`. Current signature: `func (h *Handler) middleware(next http.Handler) http.Handler`.

---

## Development commands

```sh
make run          # kill :8080 (default), then go run ./cmd/server/...
make build        # compile to ./bin/server
make test         # go test ./... -race -count=1
make tidy         # go mod tidy
make lint         # golangci-lint run ./...
make clean        # remove ./bin

make run PORT=9090  # override port (must match ADDR in .env)
```

---

## Dependencies

| Package                           | Purpose                              |
|-----------------------------------|--------------------------------------|
| `github.com/joho/godotenv`        | Load `.env` file at startup          |
| `github.com/go-sql-driver/mysql`  | MySQL driver (blank-imported in `pages.go`) |

Keep dependencies minimal. Check the standard library before adding anything new.
