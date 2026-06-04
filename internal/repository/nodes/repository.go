// Package nodes is the repository for the fleet node registry (the nodes +
// node_inbounds tables). One public method per file.
package nodes

import "database/sql"

// Repository accesses the nodes + node_inbounds tables.
type Repository struct {
	db *sql.DB
}

// New builds a nodes repository over the given database handle.
func New(db *sql.DB) *Repository { return &Repository{db: db} }
