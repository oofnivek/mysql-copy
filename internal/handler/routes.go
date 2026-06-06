package handler

import (
	"net/http"
)

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// Static assets
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(h.staticFS)))

	// Pages
	mux.HandleFunc("GET /", h.handleIndex)

	// API — connections
	mux.HandleFunc("GET /api/health", h.handleHealth)
	mux.HandleFunc("GET /api/connections", h.handleListConnections)
	mux.HandleFunc("POST /api/connections", h.handleCreateConnection)

	// API — cascading selects
	mux.HandleFunc("GET /api/source/databases", h.handleSourceDatabases)
	mux.HandleFunc("GET /api/source/tables", h.handleSourceTables)
	mux.HandleFunc("GET /api/dest/databases", h.handleDestDatabases)

	// API — copy
	mux.HandleFunc("POST /api/copy", h.handleCopy)

	return h.middleware(mux)
}
