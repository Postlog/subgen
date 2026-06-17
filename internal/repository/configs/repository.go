// Package configs is the engine-agnostic repository for subscription-config
// ownership: the base config and per-user custom configs (the subscription_configs
// anchor). It knows nothing about any engine's content tables — cloning a base into
// a new custom config is delegated to an engine-specific content repo. One public method
// per file.
package configs

import "database/sql"

// Repository accesses the subscription_configs table. routing copies a base config's
// engine content into a freshly created custom config.
type Repository struct {
	db      *sql.DB
	routing routingRepo
}

// New builds a configs repository over the given database handle and content repo.
func New(db *sql.DB, routing routingRepo) *Repository { return &Repository{db: db, routing: routing} }
