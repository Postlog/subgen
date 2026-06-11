// Package repository opens the SQLite database: it creates the handle and runs the
// embedded migrations (migrations.Apply — ordered files tracked in schema_migrations).
// Each per-entity repository (users, nodes, routing) is its own package and is
// constructed over the returned *sql.DB by the composition root — there is no
// bundle/oracle type here.
package repository

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go driver (builds with CGO_ENABLED=0)

	"github.com/postlog/subgen/migrations"
)

// Open opens (creating if needed) the SQLite database at path, runs the pending
// migrations, and returns the handle. The caller constructs the per-entity repositories
// over it (users.New(db), …) and owns its lifecycle (db.Close).
func Open(ctx context.Context, path string) (*sql.DB, error) {
	// Connection PRAGMAs live in the DSN (applied per connection): WAL for concurrent
	// reads, foreign_keys ON for FK enforcement, busy_timeout to ride out brief locks.
	// They are here (not in a migration) because PRAGMA journal_mode can't run inside the
	// transactions the migration runner wraps each file in.
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite: serialize writes, avoid "database is locked"

	if err := migrations.Apply(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return db, nil
}
