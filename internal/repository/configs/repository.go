// Package configs is the engine-agnostic repository for subscription-config
// ownership: the base config and per-user custom configs (the subscription_configs
// anchor). It knows nothing about any engine's content tables — cloning a base into
// a new custom config is delegated to an engine-specific cloner. One public method
// per file.
package configs

import "database/sql"

// Repository accesses the subscription_configs table. The cloner copies a base
// config's engine content into a freshly created custom config.
type Repository struct {
	db     *sql.DB
	cloner cloner
}

// New builds a configs repository over the given database handle and content cloner.
func New(db *sql.DB, c cloner) *Repository { return &Repository{db: db, cloner: c} }
