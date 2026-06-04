package routing

import (
	"context"

	"github.com/postlog/subgen/internal/mihomo"
)

// SetRuleProvider upserts a rule-provider.
func (r *Repository) SetRuleProvider(ctx context.Context, rp mihomo.RuleProvider) error {
	m := 0
	if rp.Mirror {
		m = 1
	}

	_, err := r.db.ExecContext(ctx, `INSERT INTO mihomo_rule_providers(name,behavior,format,mirror,url,interval,mirror_interval) VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(name) DO UPDATE SET behavior=excluded.behavior,format=excluded.format,mirror=excluded.mirror,url=excluded.url,interval=excluded.interval,mirror_interval=excluded.mirror_interval`,
		rp.Name, rp.Behavior, rp.Format, m, rp.URL, rp.Interval, rp.MirrorInterval)

	return err
}
