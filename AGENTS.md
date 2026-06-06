# AGENTS.md — mysql-copy Architecture

## Project purpose

A Go single-page web application for copying MySQL databases. The backend is written in idiomatic Go using only the standard library (plus `godotenv`). The frontend uses HTMX for SPA-like interactions — no JavaScript build step, no frontend framework.

---

## Repository layout

```
mysql-copy/
├── cmd/server/main.go          # Binary entry point
├── internal/
│   ├── config/config.go        # Environment-based configuration
│   ├── handler/
│   │   ├── handler.go          # Handler type + template rendering
│   │   ├── middleware.go       # Request logging middleware
│   │   ├── pages.go            # HTTP handlers (pages + API)
│   │   └── routes.go           # Route registration
│   └── server/server.go        # http.Server construction
├── web/
│   ├── embed.go                # Embeds templates/ and static/ into the binary
│   ├── templates/
│   │   ├── layout/base.html    # Shared HTML shell (nav, modal, footer)
│   │   └── pages/index.html    # Home page — extends base
│   └── static/
│       ├── css/app.css         # All styles (CSS variables, dark mode, modal, forms)
│       └── js/app.js           # Modal open/close, HTMX event hooks
├── .env.example                # Documented environment variables
├── Makefile                    # run / build / test / tidy / lint / clean
└── go.mod                      # Module: github.com/oofnivek/mysql-copy
```

---

## Request lifecycle

```
SIGINT/SIGTERM
      │
      ▼
cmd/server/main.go
  godotenv.Load()          ← reads .env if present
  config.Load()            ← maps env vars to Config struct
  slog setup               ← structured logger (text handler)
  server.New(cfg, logger, web.FS)
      │
      ▼
internal/server/server.go
  handler.New(cfg, logger, webFS)
  http.Server{timeouts}
      │
      ▼
internal/handler/routes.go  ← Go 1.22 method+path mux
  GET  /static/*            → embedded static files
  GET  /                    → handleIndex
  GET  /api/health          → handleHealth
  POST /api/connections     → handleCreateConnection
      │
      ▼
internal/handler/middleware.go
  responseWriter wrapper    ← captures status code
  slog.Debug per request    ← method, path, status, duration
```

---

## Key packages

### `internal/config`

Reads `ADDR` (default `:8080`) and `ENV` (default `development`) from the environment. `ENV=production` raises the log level from Debug to Info. Loaded once at startup; passed through as a pointer.

### `internal/handler`

All HTTP logic lives here. The `Handler` struct holds config, logger, parsed templates, and the static `fs.FS`. It is constructed once in `server.New`.

- **`handler.go`** — constructor, `render` (executes a named template), `isHTMX` (checks `HX-Request` header).
- **`routes.go`** — single source of truth for all routes. Add new routes here.
- **`middleware.go`** — wraps the mux with request logging. Add new middleware here (auth, CORS, rate limiting).
- **`pages.go`** — one function per page or API action. `respondAlert` is a shared helper that writes an HTMX-targeted HTML alert fragment.

### `internal/server`

Thin constructor. Creates the `http.Server` with hardened timeouts: read 5s, write 10s, idle 120s.

### `web`

All frontend assets. `embed.go` bakes `templates/` and `static/` into the binary at compile time via `//go:embed`. The resulting `embed.FS` is passed into the handler at startup — no files are read from disk at runtime.

Template rendering uses Go's `html/template`. Layout templates are defined with `{{define "base"}}` and page templates extend them with `{{template "base" .}}` plus `{{define "content"}}`.

---

## Frontend conventions

- **HTMX** is loaded from CDN (`unpkg.com/htmx.org@2.0.3`). No npm, no bundler.
- **SPA navigation** uses `hx-boost="true"` on anchor tags — full-page navigations become fetch requests, replacing only `<body>`.
- **Partial responses** (modal form submission, dynamic content) target a specific DOM id via `hx-target` and swap the response HTML with `hx-swap`.
- **Modals** are controlled by `openModal(id)` / `closeModal(id)` in `app.js`. They use the `hidden` attribute as the visibility toggle. ESC key closes any open modal.
- **CSS variables** in `:root` drive all colours and spacing. Dark mode is handled automatically via `@media (prefers-color-scheme: dark)`.

---

## Environment variables

| Variable | Default       | Description                        |
|----------|---------------|------------------------------------|
| `ADDR`   | `:8080`       | TCP address the server listens on  |
| `ENV`    | `development` | `development` or `production`      |

Copy `.env.example` to `.env` to override locally. `.env` is gitignored.

---

## Adding a new page

1. Create `web/templates/pages/yourpage.html` — use `{{template "base" .}}` and `{{define "content"}}`.
2. Add a handler function in `internal/handler/pages.go`.
3. Register the route in `internal/handler/routes.go`.
4. Add a nav link in `web/templates/layout/base.html` if needed.

## Adding a new API endpoint

1. Add a handler function in `internal/handler/pages.go` (or a new file in `internal/handler/`).
2. Register the route in `internal/handler/routes.go` with the method prefix (`POST /api/...`).
3. Use `h.respondAlert` for HTMX fragment responses or `json.NewEncoder(w).Encode` for JSON.

## Adding middleware

Wrap the existing chain in `internal/handler/middleware.go`. The current middleware signature is `func (h *Handler) middleware(next http.Handler) http.Handler`.

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

## Dependency policy

Keep external dependencies minimal. The only current dependency is `github.com/joho/godotenv` for `.env` loading. Before adding a new dependency, check whether the standard library already provides the capability.
