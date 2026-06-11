package dberr

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite" // pure-Go driver registered under the "sqlite" name
)

func TestIsUniqueViolation(t *testing.T) {
	generic := errors.New("boom")

	tt := []struct {
		name string
		// err is the candidate error; when nil, errFn builds a real driver error
		// from a live temp SQLite (UNIQUE / PRIMARY KEY violations can't be
		// hand-constructed — *sqlite.Error has unexported fields).
		err   error
		errFn func(t *testing.T) error
		want  bool
	}{
		{name: "false.nil", err: nil, want: false},
		{name: "false.generic", err: generic, want: false},
		{name: "false.wrapped_non_sqlite", err: fmt.Errorf("repo.Save: %w", generic), want: false},
		{name: "true.unique", errFn: uniqueViolation, want: true},
		{name: "true.primary_key", errFn: primaryKeyViolation, want: true},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.err
			if tc.errFn != nil {
				err = tc.errFn(t)
			}

			assert.Equal(t, tc.want, IsUniqueViolation(err))
		})
	}
}

func TestIsForeignKeyViolation(t *testing.T) {
	generic := errors.New("boom")

	tt := []struct {
		name  string
		err   error
		errFn func(t *testing.T) error
		want  bool
	}{
		{name: "false.nil", err: nil, want: false},
		{name: "false.generic", err: generic, want: false},
		{name: "false.wrapped_non_sqlite", err: fmt.Errorf("repo.Delete: %w", generic), want: false},
		{name: "false.unique_is_not_fk", errFn: uniqueViolation, want: false},
		{name: "true.foreign_key", errFn: foreignKeyViolation, want: true},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.err
			if tc.errFn != nil {
				err = tc.errFn(t)
			}

			assert.Equal(t, tc.want, IsForeignKeyViolation(err))
		})
	}
}

// openTmpDB opens a fresh temp SQLite. dberr is a leaf package (nodes/users/routing
// import it), so its test stays self-contained — it must not import a higher-level
// db-open helper (that would form an import cycle and pull in a build tag).
func openTmpDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	return db
}

// uniqueViolation creates a table with a UNIQUE column, inserts a duplicate value,
// and returns the resulting driver error.
func uniqueViolation(t *testing.T) error {
	db := openTmpDB(t)

	_, err := db.Exec(`CREATE TABLE u (id INTEGER PRIMARY KEY, name TEXT UNIQUE)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO u(name) VALUES('dup')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO u(name) VALUES('dup')`)
	require.Error(t, err)

	return err
}

// primaryKeyViolation creates a table with a TEXT PRIMARY KEY, inserts a duplicate
// key, and returns the resulting driver error.
func primaryKeyViolation(t *testing.T) error {
	db := openTmpDB(t)

	_, err := db.Exec(`CREATE TABLE p (name TEXT PRIMARY KEY)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO p(name) VALUES('k')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO p(name) VALUES('k')`)
	require.Error(t, err)

	return err
}

// foreignKeyViolation opens a db with foreign_keys ON (via DSN pragma — it must hold on
// the connection that runs the DELETE), wires a RESTRICT child→parent FK like
// user_connections→node_inbounds, and deletes a referenced parent row, returning the
// resulting driver error.
func foreignKeyViolation(t *testing.T) error {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "fk.db")+"?_pragma=foreign_keys(1)")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`CREATE TABLE parent (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL REFERENCES parent(id))`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO parent(id) VALUES(1)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO child(parent_id) VALUES(1)`)
	require.NoError(t, err)

	_, err = db.Exec(`DELETE FROM parent WHERE id=1`)
	require.Error(t, err)

	return err
}
