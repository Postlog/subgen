// Package migrations holds the SQLite schema, embedded so it ships in the
// binary and is applied on store open.
package migrations

import _ "embed"

// Schema is the full DDL applied on startup (idempotent: CREATE TABLE/INDEX IF
// NOT EXISTS + connection PRAGMAs).
//
//go:embed init.sql
var Schema string
