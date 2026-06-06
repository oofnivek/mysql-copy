package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/oofnivek/mysql-copy/internal/store"
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

func (h *Handler) handleListConnections(w http.ResponseWriter, r *http.Request) {
	list, err := h.connections.List()
	if err != nil {
		h.logger.Error("failed to list connections", "err", err)
		http.Error(w, "could not load connections", http.StatusInternalServerError)
		return
	}

	h.render(w, r, "connections-list", list)
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
	password := r.FormValue("password")
	database := r.FormValue("database")

	if name == "" || host == "" || port == "" || username == "" {
		h.respondAlert(w, http.StatusUnprocessableEntity, false, "Name, host, port and username are required.")
		return
	}

	if err := pingMySQL(host, port, username, password, database); err != nil {
		h.logger.Warn("connection test failed", "host", host, "port", port, "user", username, "err", err)
		h.respondAlert(w, http.StatusBadGateway, false, fmt.Sprintf("Connection failed: %s", err.Error()))
		return
	}

	conn := store.Connection{
		Name:     name,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Database: database,
	}
	if err := h.connections.Save(conn); err != nil {
		h.logger.Error("failed to save connection", "err", err)
		h.respondAlert(w, http.StatusInternalServerError, false, "Connection succeeded but could not be saved.")
		return
	}

	h.logger.Info("connection saved", "name", name, "host", host, "port", port, "user", username)
	h.respondAlert(w, http.StatusOK, true, fmt.Sprintf(`Connection "%s" saved successfully.`, name))
}

func pingMySQL(host, port, username, password, database string) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?timeout=5s&readTimeout=5s&writeTimeout=5s",
		username, password, host, port, database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return db.PingContext(ctx)
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
