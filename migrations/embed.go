// Package migrations holds subgen's SQL schema as ordered migration files plus the
// runner that applies them. Every file is NNNN-prefixed (0001-init.sql is the baseline
// full schema; later changes are 0002-*.sql, …) and applied in plain filename order.
// Applied files are recorded in a schema_migrations table so each runs exactly once — a
// fresh database and a restart of an up-to-date one both converge. Migrations are pure
// DDL (connection PRAGMAs live in the DSN, see internal/repository/open.go), so each can
// run inside a transaction.
package migrations

import "embed"

// files holds every migration (NNNN-*.sql), embedded so they ship in the binary and are
// applied on store open by Apply.
//
//go:embed *.sql
var files embed.FS
