// Package routing is the repository for the mihomo config: ordered routing
// rules, proxy-groups (+ members), rule-providers and free-form settings — the
// mihomo_* tables. One public method per file.
package routing

import "database/sql"

// Repository accesses the mihomo_routing_rules, mihomo_proxy_groups,
// mihomo_proxy_group_members, mihomo_rule_providers and mihomo_settings tables.
type Repository struct {
	db *sql.DB
}

// New builds a routing repository over the given database handle.
func New(db *sql.DB) *Repository { return &Repository{db: db} }
