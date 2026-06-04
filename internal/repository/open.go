// Package repository opens the SQLite database: it creates the handle, applies the
// embedded schema, and runs the one-off legacy fixups. Each per-entity repository
// (users, nodes, routing) is its own package and is constructed over the returned
// *sql.DB by the composition root — there is no bundle/oracle type here.
package repository

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go driver (builds with CGO_ENABLED=0)

	"github.com/postlog/subgen/migrations"
)

// Open opens (creating if needed) the SQLite database at path, applies the schema,
// and returns the handle. The caller constructs the per-entity repositories over
// it (users.New(db), …) and owns its lifecycle (db.Close).
func Open(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite: serialize writes, avoid "database is locked"

	if _, err := db.ExecContext(ctx, migrations.Schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return db, nil
}
