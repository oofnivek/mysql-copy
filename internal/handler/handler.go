package handler

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/oofnivek/mysql-copy/internal/config"
	"github.com/oofnivek/mysql-copy/internal/store"
)

type Handler struct {
	cfg         *config.Config
	logger      *slog.Logger
	templates   *template.Template
	staticFS    fs.FS
	connections *store.Connections
	presets     *store.Presets
}

func New(cfg *config.Config, logger *slog.Logger, webFS fs.FS) *Handler {
	tmpl := template.Must(
		template.New("").ParseFS(webFS,
			"templates/layout/*.html",
			"templates/pages/*.html",
			"templates/partials/*.html",
		),
	)

	staticFS, err := fs.Sub(webFS, "static")
	if err != nil {
		panic(err)
	}

	return &Handler{
		cfg:         cfg,
		logger:      logger,
		templates:   tmpl,
		staticFS:    staticFS,
		connections: store.NewConnections(cfg.ConnectionsFile),
		presets:     store.NewPresets(cfg.PresetsFile),
	}
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Error("template render error", "template", name, "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func (h *Handler) isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
