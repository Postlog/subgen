// Package dberr classifies SQLite driver errors by their typed result code. It is a
// leaf package (only the sqlite driver) so the per-entity repositories can import it
// without pulling in the repository root (which imports them in its tests).
package dberr

import (
	"errors"

	sqlitedrv "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// IsUniqueViolation reports whether err is a SQLite UNIQUE / PRIMARY KEY constraint
// violation, decided from the driver's typed result code — never by matching the
// message string. modernc enables extended result codes on every connection, so
// *sqlite.Error.Code() carries the extended code: SQLITE_CONSTRAINT_UNIQUE (2067)
// for UNIQUE columns, SQLITE_CONSTRAINT_PRIMARYKEY (1555) for a PRIMARY KEY (e.g.
// mihomo_rule_providers.name). Per-entity repositories use this to translate a
// duplicate into a domain sentinel (entity.ErrNameTaken, …) without a pre-check
// SELECT — the constraint itself is the source of truth.
func IsUniqueViolation(err error) bool {
	var e *sqlitedrv.Error

	return errors.As(err, &e) &&
		(e.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE ||
			e.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY)
}

// IsForeignKeyViolation reports whether err is a SQLite FOREIGN KEY constraint violation
// (SQLITE_CONSTRAINT_FOREIGNKEY, 787), decided from the driver's typed result code — never
// by matching the message string. A node-inbound delete (dropping an inbound, or cascading
// from a node delete) hits this when the inbound is still referenced by a user connection
// or a mihomo rule / proxy-group member (those FKs RESTRICT). The nodes repository turns it
// into entity.ErrInboundReferenced — the FK is the source of truth, no pre-check SELECT.
func IsForeignKeyViolation(err error) bool {
	var e *sqlitedrv.Error

	return errors.As(err, &e) && e.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
}
