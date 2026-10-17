// Package db is belfast's SQLite storage. The schema lives in numbered
// migrations/*.sql files, the queries in queries/*.sql, compiled to Go by sqlc
// (`make sqlc`); everything else in this package but open.go is generated.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:generate sqlc generate

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens (creating it if needed) the database at path and applies the
// migrations it hasn't seen yet.
func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// migrate applies migrations/NNNN_*.sql in order, each in its own
// transaction, tracking the last applied number in PRAGMA user_version.
func migrate(conn *sql.DB) error {
	var version int
	if err := conn.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		number, err := strconv.Atoi(strings.SplitN(strings.TrimPrefix(name, "migrations/"), "_", 2)[0])
		if err != nil {
			return fmt.Errorf("migration %s: name must start with its number", name)
		}
		if number <= version {
			continue
		}
		script, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := conn.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(script)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		// PRAGMA doesn't take parameters
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", number)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
