// Command dbschema derives the initial PostgreSQL schema from a fresh Arc SQLite catalog.
package main

import (
	"ams/internal/store"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	schema, err := generate(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	if _, err := fmt.Print(schema); err != nil {
		log.Fatal(err)
	}
}

func generate(ctx context.Context) (string, error) {
	dir, err := os.MkdirTemp("", "arc-schema-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "schema.db")
	st, err := store.Open(ctx, path, store.Options{})
	if err != nil {
		return "", err
	}
	defer st.Close()
	db, err := sql.Open("sqlite", path+"?_pragma=query_only(1)")
	if err != nil {
		return "", err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return "", err
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	ddl, err := postgresBaseline(ctx, db)
	if err != nil {
		return "", err
	}
	return "-- Arc migrations: " + strings.Join(versions, ",") + "\n" + ddl, nil
}
