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

	h.render(w, r, "index.html", map[string]any{
		"Title":       "mysql-copy",
		"Connections": connections,
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

func (h *Handler) handleCopy(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.respondAlert(w, http.StatusBadRequest, false, "Invalid request.")
		return
	}

	srcConnName := r.FormValue("source_conn")
	srcDB := r.FormValue("source_db")
	srcTable := r.FormValue("source_table")
	dstConnName := r.FormValue("dest_conn")
	dstDB := r.FormValue("dest_db")

	if srcConnName == "" || srcDB == "" || srcTable == "" || dstConnName == "" || dstDB == "" {
		h.respondAlert(w, http.StatusUnprocessableEntity, false, "All five fields are required.")
		return
	}

	srcConn, err := h.connections.GetByName(srcConnName)
	if err != nil {
		h.respondAlert(w, http.StatusNotFound, false, "Source connection not found.")
		return
	}
	dstConn, err := h.connections.GetByName(dstConnName)
	if err != nil {
		h.respondAlert(w, http.StatusNotFound, false, "Destination connection not found.")
		return
	}

	h.logger.Info("copy requested",
		"src_conn", srcConnName, "src_db", srcDB, "src_table", srcTable,
		"dst_conn", dstConnName, "dst_db", dstDB,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	logInfo := func(msg string) { fmt.Fprintf(&buf, `<p class="log-line log-info">%s</p>`, msg) }
	logOK := func(msg string) { fmt.Fprintf(&buf, `<p class="log-line log-ok">%s</p>`, msg) }
	logErr := func(msg string) { fmt.Fprintf(&buf, `<p class="log-line log-error">%s</p>`, msg) }
	flush := func() { fmt.Fprint(w, buf.String()) }

	logInfo(fmt.Sprintf("Fetching DDL for <strong>%s.%s</strong>…", srcDB, srcTable))
	ddl, err := getTableDDL(srcConn, srcDB, srcTable)
	if err != nil {
		logErr(fmt.Sprintf("Failed to get DDL: %s", err))
		flush()
		return
	}
	logOK("DDL fetched.")

	logInfo(fmt.Sprintf("Dropping <strong>%s.%s</strong> if it exists…", dstDB, srcTable))
	if err := dropTableIfExists(dstConn, dstDB, srcTable); err != nil {
		logErr(fmt.Sprintf("Failed to drop table: %s", err))
		flush()
		return
	}
	logOK("Dropped (or did not exist).")

	logInfo(fmt.Sprintf("Creating table <strong>%s.%s</strong>…", dstDB, srcTable))
	if err := createTable(dstConn, dstDB, ddl); err != nil {
		logErr(fmt.Sprintf("Failed to create table: %s", err))
		flush()
		return
	}
	logOK("Table created.")

	logInfo("Copying rows…")
	n, err := copyTableData(srcConn, srcDB, srcTable, dstConn, dstDB)
	if err != nil {
		logErr(fmt.Sprintf("Copy failed after %d rows: %s", n, err))
		flush()
		return
	}
	logOK(fmt.Sprintf("Done. Copied <strong>%d</strong> rows.", n))

	flush()
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
