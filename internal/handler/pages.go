package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	h.render(w, r, "index.html", map[string]any{
		"Title": "mysql-copy",
	})
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.respondAlert(w, http.StatusBadRequest, false, "Invalid form data.")
		return
	}

	name := r.FormValue("name")
	host := r.FormValue("host")
	port := r.FormValue("port")
	username := r.FormValue("username")
	database := r.FormValue("database")

	if name == "" || host == "" || port == "" || username == "" {
		h.respondAlert(w, http.StatusUnprocessableEntity, false, "Name, host, port and username are required.")
		return
	}

	h.logger.Info("connection created",
		"name", name,
		"host", host,
		"port", port,
		"user", username,
		"database", database,
	)

	h.respondAlert(w, http.StatusOK, true, fmt.Sprintf("Connected to %s@%s:%s successfully.", username, host, port))
}

func (h *Handler) respondAlert(w http.ResponseWriter, status int, ok bool, msg string) {
	cls := "alert-success"
	if !ok {
		cls = "alert-error"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<div class="alert %s">%s</div>`, cls, msg)
}
