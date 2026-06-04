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
