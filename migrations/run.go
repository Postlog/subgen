package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"
)

// Apply runs every not-yet-applied migration in filename order (0001-init.sql,
// 0002-*.sql, …), each wrapped in its own transaction and recorded in schema_migrations.
// The NNNN- prefix makes lexical order the apply order, so no special-casing is needed.
// It is idempotent: already-applied files are skipped, so a fresh database and a restart
// of an up-to-date one both converge. Any failure aborts and is returned (the caller
// crashes) — a partially-applied migration is never recorded, so the next start retries it.
//
// Adopting an existing pre-runner database is safe: schema_migrations is created empty,
// the baseline (0001-init.sql) re-runs (it is CREATE … IF NOT EXISTS, a no-op on existing
// tables) and gets recorded, then later files apply on top.
func Apply(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
  name       TEXT PRIMARY KEY,
  applied_at INTEGER NOT NULL
)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	names, err := ordered()
	if err != nil {
		return err
	}

	for _, name := range names {
		applied, err := isApplied(ctx, db, name)
		if err != nil {
			return fmt.Errorf("check %s: %w", name, err)
		}

		if applied {
			continue
		}

		if err := applyOne(ctx, db, name); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}

		slog.Info("migrations: applied", "name", name)
	}

	return nil
}

// ordered lists the embedded migration filenames in apply order — plain lexical sort,
// which is the right order because every file is NNNN-prefixed (0001-init.sql first).
func ordered() ([]string, error) {
	all, err := fs.Glob(files, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("glob migrations: %w", err)
	}

	sort.Strings(all)

	return all, nil
}

// isApplied reports whether a migration is already recorded in schema_migrations.
func isApplied(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE name=?`, name).Scan(&n); err != nil {
		return false, err
	}

	return n > 0, nil
}

// applyOne runs one migration file and records it, atomically: the file's DDL and the
// schema_migrations insert share a transaction, so a crash mid-file leaves nothing
// recorded and the migration retries cleanly on the next start.
func applyOne(ctx context.Context, db *sql.DB, name string) error {
	body, err := files.ReadFile(name)
	if err != nil {
		return err
	}

	// A `.notx.sql` migration runs OUTSIDE a transaction — the escape hatch for what
	// SQLite can't do inside one, notably toggling `PRAGMA foreign_keys` (a no-op
	// mid-transaction) for a table rebuild that drops a table other tables reference.
	// Such a file owns its own atomicity AND its schema_migrations record (its DDL and
	// the INSERT share the file's own BEGIN/COMMIT), so "applied" commits with the change.
	if strings.HasSuffix(name, ".notx.sql") {
		return applyNoTx(ctx, db, string(body))
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(name, applied_at) VALUES(?, ?)`, name, time.Now().Unix()); err != nil {
		return err
	}

	return tx.Commit()
}

// applyNoTx runs a `.notx.sql` migration on a dedicated connection without wrapping it in
// a transaction, then discards the connection so any PRAGMA the file left (e.g.
// foreign_keys) can't leak into the pool. The file records itself in schema_migrations
// inside its own transaction (see applyOne), so there is no separate, non-atomic mark.
func applyNoTx(ctx context.Context, db *sql.DB, body string) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}

	defer conn.Close()

	_, err = conn.ExecContext(ctx, body)

	return err
}
