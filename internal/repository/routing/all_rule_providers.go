package routing

import (
	"context"

	"github.com/postlog/subgen/internal/mihomo"
)

// AllRuleProviders returns the rule-providers across every config (base + all custom
// configs), ordered by name. The mirror service needs the union: a custom config may
// reference a mirrored provider the base does not. Names can repeat across configs;
// the mirror keys by name+ext, so a later duplicate just overwrites the source.
func (r *Repository) AllRuleProviders(ctx context.Context) ([]mihomo.RuleProvider, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name,behavior,format,mirror,url,interval,mirror_interval FROM mihomo_rule_providers ORDER BY name`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []mihomo.RuleProvider

	for rows.Next() {
		var rp mihomo.RuleProvider

		var mirror int
		if err := rows.Scan(&rp.Name, &rp.Behavior, &rp.Format, &mirror, &rp.URL, &rp.Interval, &rp.MirrorInterval); err != nil {
			return nil, err
		}

		rp.Mirror = mirror != 0
		out = append(out, rp)
	}

	return out, rows.Err()
}
