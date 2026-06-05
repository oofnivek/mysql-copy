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

	// API
	mux.HandleFunc("GET /api/health", h.handleHealth)
	mux.HandleFunc("POST /api/connections", h.handleCreateConnection)

	return h.middleware(mux)
}
