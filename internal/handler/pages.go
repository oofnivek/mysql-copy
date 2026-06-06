package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/oofnivek/mysql-copy/internal/store"
)

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	connections, err := h.connections.List()
	if err != nil {
		h.logger.Error("failed to load connections", "err", err)
		connections = nil
	}

	presets, err := h.presets.List()
	if err != nil {
		h.logger.Error("failed to load presets", "err", err)
		presets = nil
	}

	h.render(w, r, "index.html", map[string]any{
		"Title":       "mysql-copy",
		"Connections": connections,
		"Presets":     presets,
	})
}

func (h *Handler) handleSourceDatabases(w http.ResponseWriter, r *http.Request) {
	connName := r.FormValue("source_conn")
	if connName == "" {
		return
	}

	conn, err := h.connections.GetByName(connName)
	if err != nil {
		h.respondAlert(w, http.StatusNotFound, false, "Connection not found.")
		return
	}

	dbs, err := queryDatabases(conn)
	if err != nil {
		h.logger.Warn("SHOW DATABASES failed", "conn", connName, "err", err)
		h.respondAlert(w, http.StatusBadGateway, false, fmt.Sprintf("Could not list databases: %s", err))
		return
	}

	h.render(w, r, "source-databases", dbs)
}

func (h *Handler) handleSourceTables(w http.ResponseWriter, r *http.Request) {
	connName := r.FormValue("source_conn")
	database := r.FormValue("source_db")
	if connName == "" || database == "" {
		return
	}

	conn, err := h.connections.GetByName(connName)
	if err != nil {
		h.respondAlert(w, http.StatusNotFound, false, "Connection not found.")
		return
	}

	tables, err := queryTables(conn, database)
	if err != nil {
		h.logger.Warn("SHOW TABLES failed", "conn", connName, "db", database, "err", err)
		h.respondAlert(w, http.StatusBadGateway, false, fmt.Sprintf("Could not list tables: %s", err))
		return
	}

	h.render(w, r, "source-tables", tables)
}

func (h *Handler) handleDestDatabases(w http.ResponseWriter, r *http.Request) {
	connName := r.FormValue("dest_conn")
	if connName == "" {
		return
	}

	conn, err := h.connections.GetByName(connName)
	if err != nil {
		h.respondAlert(w, http.StatusNotFound, false, "Connection not found.")
		return
	}

	dbs, err := queryDatabases(conn)
	if err != nil {
		h.logger.Warn("SHOW DATABASES failed", "conn", connName, "err", err)
		h.respondAlert(w, http.StatusBadGateway, false, fmt.Sprintf("Could not list databases: %s", err))
		return
	}

	h.render(w, r, "dest-databases", dbs)
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

func (h *Handler) handleListPresets(w http.ResponseWriter, r *http.Request) {
	list, err := h.presets.List()
	if err != nil {
		h.logger.Error("failed to list presets", "err", err)
		http.Error(w, "could not load presets", http.StatusInternalServerError)
		return
	}
	h.render(w, r, "presets-list", list)
}

func (h *Handler) handleAddPreset(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.respondAlert(w, http.StatusBadRequest, false, "Invalid form data.")
		return
	}

	srcConn := r.FormValue("source_conn")
	srcDB := r.FormValue("source_db")
	srcTable := r.FormValue("source_table")
	dstConn := r.FormValue("dest_conn")
	dstDB := r.FormValue("dest_db")

	if srcConn == "" || srcDB == "" || srcTable == "" || dstConn == "" || dstDB == "" {
		h.respondAlert(w, http.StatusUnprocessableEntity, false, "All five fields are required.")
		return
	}

	if err := h.presets.Save(store.Preset{
		SrcConn:  srcConn,
		SrcDB:    srcDB,
		SrcTable: srcTable,
		DstConn:  dstConn,
		DstDB:    dstDB,
		DstTable: srcTable,
	}); err != nil {
		h.logger.Error("failed to save preset", "err", err)
		h.respondAlert(w, http.StatusInternalServerError, false, "Could not save preset.")
		return
	}

	list, err := h.presets.List()
	if err != nil {
		h.logger.Error("failed to list presets after save", "err", err)
		http.Error(w, "could not load presets", http.StatusInternalServerError)
		return
	}
	h.render(w, r, "presets-list", list)
}

func (h *Handler) handleDeletePreset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	if err := h.presets.Delete(id); err != nil {
		h.logger.Error("failed to delete preset", "id", id, "err", err)
		http.Error(w, "could not delete preset", http.StatusInternalServerError)
		return
	}

	list, err := h.presets.List()
	if err != nil {
		h.logger.Error("failed to list presets after delete", "err", err)
		http.Error(w, "could not load presets", http.StatusInternalServerError)
		return
	}
	h.render(w, r, "presets-list", list)
}

func (h *Handler) handleCopy(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.respondAlert(w, http.StatusBadRequest, false, "Invalid request.")
		return
	}

	presetIDs := r.Form["preset_id"]
	if len(presetIDs) == 0 {
		h.respondAlert(w, http.StatusUnprocessableEntity, false, "No presets selected.")
		return
	}

	allPresets, err := h.presets.List()
	if err != nil {
		h.respondAlert(w, http.StatusInternalServerError, false, "Could not load presets.")
		return
	}
	presetMap := make(map[string]store.Preset, len(allPresets))
	for _, p := range allPresets {
		presetMap[p.ID] = p
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	logInfo := func(msg string) { fmt.Fprintf(&buf, `<p class="log-line log-info">%s</p>`, msg) }
	logOK := func(msg string) { fmt.Fprintf(&buf, `<p class="log-line log-ok">%s</p>`, msg) }
	logErr := func(msg string) { fmt.Fprintf(&buf, `<p class="log-line log-error">%s</p>`, msg) }

	for i, id := range presetIDs {
		p, ok := presetMap[id]
		if !ok {
			logErr(fmt.Sprintf("Preset not found: %s", id))
			continue
		}

		if i > 0 {
			buf.WriteString(`<p class="log-line log-info">&nbsp;</p>`)
		}
		logInfo(fmt.Sprintf("<strong>%s.%s.%s → %s.%s</strong>",
			p.SrcConn, p.SrcDB, p.SrcTable, p.DstConn, p.DstDB))

		srcConn, err := h.connections.GetByName(p.SrcConn)
		if err != nil {
			logErr(fmt.Sprintf("Source connection %q not found.", p.SrcConn))
			continue
		}
		dstConn, err := h.connections.GetByName(p.DstConn)
		if err != nil {
			logErr(fmt.Sprintf("Destination connection %q not found.", p.DstConn))
			continue
		}

		dstTable := p.DstTable
		if dstTable == "" {
			dstTable = p.SrcTable
		}

		h.logger.Info("copy requested",
			"src_conn", p.SrcConn, "src_db", p.SrcDB, "src_table", p.SrcTable,
			"dst_conn", p.DstConn, "dst_db", p.DstDB, "dst_table", dstTable,
		)

		ddl, err := getTableDDL(srcConn, p.SrcDB, p.SrcTable)
		if err != nil {
			logErr(fmt.Sprintf("Failed to get DDL: %s", err))
			continue
		}
		logInfo(fmt.Sprintf("Dropping <strong>%s.%s</strong> if it exists…", p.DstDB, dstTable))
		if err := dropTableIfExists(dstConn, p.DstDB, dstTable); err != nil {
			logErr(fmt.Sprintf("Drop failed: %s", err))
			continue
		}
		logInfo(fmt.Sprintf("Creating <strong>%s.%s</strong>…", p.DstDB, dstTable))
		if err := createTable(dstConn, p.DstDB, ddl); err != nil {
			logErr(fmt.Sprintf("Create failed: %s", err))
			continue
		}
		logInfo("Copying rows…")
		n, err := copyTableData(srcConn, p.SrcDB, p.SrcTable, dstConn, p.DstDB)
		if err != nil {
			logErr(fmt.Sprintf("Copy failed after %d rows: %s", n, err))
			continue
		}
		logOK(fmt.Sprintf("Done — <strong>%d</strong> rows copied.", n))
	}

	fmt.Fprint(w, buf.String())
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
