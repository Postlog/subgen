package routing

import (
	"context"

	"github.com/postlog/subgen/internal/mihomo"
)

// RuleProviders returns the config's rule-providers ordered by name. Authored providers
// also carry their matcher tree (Matchers); external ones leave it empty.
func (r *Repository) RuleProviders(ctx context.Context, configID int64) ([]mihomo.RuleProvider, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,source,behavior,format,mirror,url,interval,mirror_interval FROM mihomo_rule_providers WHERE config_id=? ORDER BY name`, configID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []mihomo.RuleProvider

	for rows.Next() {
		var rp mihomo.RuleProvider

		var (
			mirror int
			source string
		)

		if err := rows.Scan(&rp.ID, &rp.Name, &source, &rp.Behavior, &rp.Format, &mirror, &rp.URL, &rp.Interval, &rp.MirrorInterval); err != nil {
			return nil, err
		}

		rp.Source = mihomo.RuleProviderSource(source)
		rp.Mirror = mirror != 0
		out = append(out, rp)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := r.attachAuthoredMatchers(ctx, out); err != nil {
		return nil, err
	}

	return out, nil
}
