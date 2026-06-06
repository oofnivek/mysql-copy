package handler

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/oofnivek/mysql-copy/internal/store"
)

var systemDatabases = map[string]bool{
	"information_schema": true,
	"performance_schema": true,
	"mysql":              true,
	"sys":                true,
}

func openDB(conn *store.Connection) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?timeout=5s&readTimeout=10s&writeTimeout=10s",
		conn.Username, conn.Password, conn.Host, conn.Port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(30 * time.Second)
	return db, nil
}

func queryDatabases(conn *store.Connection) ([]string, error) {
	db, err := openDB(conn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dbs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if !systemDatabases[name] {
			dbs = append(dbs, name)
		}
	}
	return dbs, rows.Err()
}

func openDBWithDatabase(conn *store.Connection, database string) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?timeout=5s&readTimeout=300s&writeTimeout=300s",
		conn.Username, conn.Password, conn.Host, conn.Port, database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}

var reAutoIncrement = regexp.MustCompile(`\s+AUTO_INCREMENT=\d+`)

func getTableDDL(conn *store.Connection, database, table string) (string, error) {
	db, err := openDB(conn)
	if err != nil {
		return "", err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var tableName, createStmt string
	row := db.QueryRowContext(ctx, fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", database, table))
	if err := row.Scan(&tableName, &createStmt); err != nil {
		return "", err
	}
	return reAutoIncrement.ReplaceAllString(createStmt, ""), nil
}

func dropTableIfExists(conn *store.Connection, database, table string) error {
	db, err := openDBWithDatabase(conn, database)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS `%s`", table))
	return err
}

func createTable(conn *store.Connection, database, ddl string) error {
	db, err := openDBWithDatabase(conn, database)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = db.ExecContext(ctx, ddl)
	return err
}

func copyTableData(srcConn *store.Connection, srcDB, srcTable string, dstConn *store.Connection, dstDB string) (int64, error) {
	src, err := openDBWithDatabase(srcConn, srcDB)
	if err != nil {
		return 0, fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	dst, err := openDBWithDatabase(dstConn, dstDB)
	if err != nil {
		return 0, fmt.Errorf("open destination: %w", err)
	}
	defer dst.Close()

	ctx := context.Background()

	rows, err := src.QueryContext(ctx, fmt.Sprintf("SELECT * FROM `%s`", srcTable))
	if err != nil {
		return 0, fmt.Errorf("query source: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return 0, fmt.Errorf("get columns: %w", err)
	}

	quotedCols := make([]string, len(cols))
	placeholders := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = fmt.Sprintf("`%s`", c)
		placeholders[i] = "?"
	}
	insertSQL := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)",
		srcTable, strings.Join(quotedCols, ", "), strings.Join(placeholders, ", "))

	stmt, err := dst.PrepareContext(ctx, insertSQL)
	if err != nil {
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	vals := make([]any, len(cols))
	valPtrs := make([]any, len(cols))
	for i := range vals {
		valPtrs[i] = &vals[i]
	}

	var copied int64
	for rows.Next() {
		if err := rows.Scan(valPtrs...); err != nil {
			return copied, fmt.Errorf("scan row: %w", err)
		}
		if _, err := stmt.ExecContext(ctx, vals...); err != nil {
			return copied, fmt.Errorf("insert row: %w", err)
		}
		copied++
	}
	return copied, rows.Err()
}

func queryTables(conn *store.Connection, database string) ([]string, error) {
	db, err := openDB(conn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, fmt.Sprintf("SHOW TABLES FROM `%s`", database))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}
