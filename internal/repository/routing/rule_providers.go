package routing

import (
	"context"

	"github.com/postlog/subgen/internal/mihomo"
)

// RuleProviders returns the config's rule-providers ordered by name.
func (r *Repository) RuleProviders(ctx context.Context, configID int64) ([]mihomo.RuleProvider, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name,behavior,format,mirror,url,interval,mirror_interval FROM mihomo_rule_providers WHERE config_id=? ORDER BY name`, configID)
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
