# AGENTS.md — mysql-copy Architecture

## Project purpose

A Go single-page web application for copying MySQL tables from one server to another. The destination table is dropped and recreated using the source DDL (with `AUTO_INCREMENT=N` table option stripped). Copy jobs are defined as **presets** — saved source/destination configurations — which can be selected via checkboxes and run in batch. The backend is idiomatic Go using only the standard library plus two small external packages. The frontend uses HTMX — no JavaScript build step, no frontend framework.

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
│   │   ├── mysql.go                # MySQL helpers: openDB, DDL, drop, create, copy
│   │   ├── pages.go                # All HTTP handlers
│   │   └── routes.go               # Route registration (single source of truth)
│   ├── server/server.go            # http.Server construction with timeouts
│   └── store/
│       ├── connections.go          # Connection struct + JSON file store
│       └── presets.go              # Preset struct + JSON file store
├── web/
│   ├── embed.go                    # //go:embed — bakes templates/ and static/ into binary
│   ├── templates/
│   │   ├── layout/base.html        # Shared HTML shell: nav, Connection dropdown, modals
│   │   ├── pages/index.html        # Main page — 4-panel layout
│   │   └── partials/
│   │       ├── connections.html        # Saved connections list fragment
│   │       ├── presets.html            # Preset list fragment (checkboxes + delete)
│   │       ├── source-databases.html   # Source database select (cascades to tables)
│   │       ├── source-tables.html      # Source table select
│   │       └── dest-databases.html     # Destination database select
│   └── static/
│       ├── css/app.css             # All styles: variables, dark mode, panels, modals, forms
│       └── js/app.js               # Dropdown, modal, cascading-select, preset/copy-ready logic
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
  handler.New(cfg, logger, webFS)   ← parses templates, creates stores
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
  GET  /api/presets            → handleListPresets
  POST /api/presets            → handleAddPreset
  DELETE /api/presets/{id}     → handleDeletePreset
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
| `PresetsFile`     | `PRESETS_FILE`     | `~/.mysql-copy/presets.json`     |

Both data files share a helper `defaultDataFile(name)` that resolves under `~/.mysql-copy/`.

### `internal/store`

Two file-backed stores. Both use `sync.Mutex` and persist to JSON under `~/.mysql-copy/`.

**`Connection`** — `connections.json`. JSON key `"user"` (not `"username"`) matches the pre-existing file format; do not change this tag without migrating the file.

```
id, name, host, port, user, password (omitempty), database (omitempty), created_at
```

Public methods: `List`, `GetByName`, `Save`.

**`Preset`** — `presets.json`. JSON keys match the hand-authored format:

```
id (omitempty), src_connection, src_database, src_table,
dst_connection, dst_database, dst_table (omitempty), created_at (omitempty)
```

On first `load()`, any entry with an empty `id` is assigned a random hex ID and the file is re-persisted automatically (silent one-time migration).

Public methods: `List`, `Save`, `Delete(id)`, `GetByID(id)`.

The file and its parent directory are created automatically on first save (`0700` dir, `0600` file).

### `internal/handler`

All HTTP logic. The `Handler` struct holds config, logger, parsed templates, static `fs.FS`, `*store.Connections`, and `*store.Presets`.

- **`handler.go`** — constructor; parses all template globs; `render` executes a named template; `isHTMX` checks `HX-Request`.
- **`routes.go`** — single source of truth for all routes.
- **`middleware.go`** — request logging. Add auth, CORS, or rate-limiting here.
- **`mysql.go`** — package-level MySQL helpers:
  - `openDB(conn)` — opens without a database; 1 max connection, 30s lifetime.
  - `openDBWithDatabase(conn, db)` — opens with a specific database; 5 max connections, 5m lifetime (used for DDL and copy operations).
  - `queryDatabases(conn)` — `SHOW DATABASES`, filters system databases.
  - `queryTables(conn, db)` — `SHOW TABLES FROM \`db\``.
  - `getTableDDL(conn, db, table)` — `SHOW CREATE TABLE`; strips `AUTO_INCREMENT=N` table option via `reAutoIncrement` regexp.
  - `dropTableIfExists(conn, db, table)` — `DROP TABLE IF EXISTS`.
  - `createTable(conn, db, ddl)` — executes the DDL against the target database.
  - `copyTableData(srcConn, srcDB, srcTable, dstConn, dstDB)` — streams `SELECT *` from source; uses a prepared `INSERT` to write each row to destination. Returns row count.
- **`pages.go`** — one function per route:
  - `handleIndex` — loads connections + presets, passes both to `index.html`.
  - `handleListConnections` — renders `connections-list` partial.
  - `handleCreateConnection` — validates, pings MySQL, saves.
  - `handleSourceDatabases` / `handleSourceTables` / `handleDestDatabases` — cascading select handlers.
  - `handleListPresets` — renders `presets-list` partial.
  - `handleAddPreset` — reads five form fields, saves a new preset, returns updated `presets-list`.
  - `handleDeletePreset` — reads `{id}` from path via `r.PathValue`, deletes, returns updated `presets-list`.
  - `handleCopy` — reads `preset_id[]` form values (checked checkboxes); for each preset, resolves connections, gets DDL, drops dest, creates, copies rows; writes HTML log lines to `#progress-log`. Continues to next preset on error.
  - `respondAlert(w, status, ok, msg)` — writes `<div class="alert alert-*">` fragment.

### `internal/server`

Thin constructor. `http.Server` timeouts: read 5s, write 10s, idle 120s.

### `web`

All frontend assets compiled into the binary via `//go:embed`. No files are read from disk at runtime.

Templates use Go's `html/template`. The `base` layout is `{{define "base"}}`. Page templates extend it with `{{template "base" .}}` + `{{define "content"}}`. Partials are standalone `{{define "name"}}` blocks.

---

## UI layout

The main page is a full-viewport 4-panel CSS Grid (3 rows):

```
┌─────────────────────────┬──────────────────────────┐
│  SOURCE                 │  DESTINATION             │
│  Connection ▾           │  Connection ▾            │
│  Database   ▾           │  Database   ▾            │
│  Table      ▾           │                          │
├─────────────────────────┴──────────────────────────┤
│  PRESETS              [Add Preset] [Start Copy]    │
│  ☐  src_conn.src_db.src_table → dst_conn.dst_db  × │
│  ☐  ...                                          × │
├────────────────────────────────────────────────────┤
│  PROGRESS                                          │
│  (monospace log output, auto-scrolls to bottom)    │
└────────────────────────────────────────────────────┘
```

Grid: `grid-template-columns: 1fr 1fr`, `grid-template-rows: 1fr auto 1fr`, areas `source / dest / presets / progress`. Layout fills the viewport via `flex: 1; min-height: 0` on `.layout`.

**Cascading selects (HTMX):**
1. Source Connection → `GET /api/source/databases` → swaps `#source-db-wrap`.
2. Source Database → `GET /api/source/tables` → swaps `#source-table-wrap`.
3. Dest Connection → `GET /api/dest/databases` → swaps `#dest-db-wrap`.

**Add Preset button** (`#btn-add-preset`) — in the Presets panel header. Enabled when all five source/dest selects are non-empty (`checkAddPresetReady()`). POSTs the five field values to `POST /api/presets`; response replaces `#presets-body`.

**Start Copy button** (`#btn-start-copy`) — in the Presets panel header. Enabled when at least one `.preset-check` checkbox is checked (`checkStartCopyReady()`). POSTs all checked `preset_id` values (`hx-include=".preset-check"`) to `POST /api/copy`; response replaces `#progress-log`. Button shows "Copying…" and is disabled for the duration.

**Delete preset** — each row has a `×` button that sends `DELETE /api/presets/{id}`, replacing `#presets-body` with the updated list.

**Source → Destination exclusion** — selecting a source connection disables the same option in the dest dropdown; if dest had the same value, it resets and clears `#dest-db-wrap`.

---

## Nav and modals

The nav has a **Connection ▾** dropdown:

- **Create** — opens `#modal-connection`. On submit, HTMX POSTs to `POST /api/connections`; pings MySQL before saving. Success closes the modal after 1.2s.
- **Saved** — fires `GET /api/connections`, populates `#saved-connections-list`, opens `#modal-saved`.

Modals use the `hidden` attribute. `openModal` / `closeModal` are global JS functions. ESC or backdrop click closes any open modal.

---

## Environment variables

| Variable            | Default                            | Description                        |
|---------------------|------------------------------------|------------------------------------|
| `ADDR`              | `:8080`                            | TCP address the server listens on  |
| `ENV`               | `development`                      | `development` or `production`      |
| `CONNECTIONS_FILE`  | `~/.mysql-copy/connections.json`   | Path to the saved connections file |
| `PRESETS_FILE`      | `~/.mysql-copy/presets.json`       | Path to the saved presets file     |

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
