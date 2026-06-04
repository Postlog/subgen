// Package users is the repository for service-owned subscribers and their
// per-inbound connections (the users + user_connections tables). One public
// method per file.
package users

import "database/sql"

// Repository accesses the users + user_connections tables.
type Repository struct {
	db *sql.DB
}

// New builds a users repository over the given database handle.
func New(db *sql.DB) *Repository { return &Repository{db: db} }
