// Package db provides the SQLite-backed app state store for gitura.
// State stored here is managed by the application and is not intended
// for direct user editing (use internal/settings for user-editable config).
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// schema is the DDL run on every Open to ensure tables exist.
// It is idempotent — all statements use IF NOT EXISTS.
const schema = `
CREATE TABLE IF NOT EXISTS pr_state (
    owner      TEXT    NOT NULL,
    repo       TEXT    NOT NULL,
    number     INTEGER NOT NULL,
    local_path TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (owner, repo, number)
);
`

// Open opens (or creates) the gitura state database at stateDir/gitura/state.db.
// The directory is created with mode 0700 if it does not exist.
// The schema is applied on every open so new tables are created automatically.
func Open(stateDir string) (*sql.DB, error) {
	dir := filepath.Join(stateDir, "gitura")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("db: create state dir: %w", err)
	}

	dbPath := filepath.Join(dir, "state.db")
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_fk=on")
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}

	if _, err := sqlDB.Exec(schema); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db: migrate: %w", err)
	}

	return sqlDB, nil
}
