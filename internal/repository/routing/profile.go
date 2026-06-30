package routing

import (
	"context"
	"database/sql"
	"errors"

	"github.com/postlog/subgen/internal/mihomo"
)

// Profile returns a config's subscription-profile knobs (title, filename, update
// interval, proxies interval). A config with no row yet yields a zero Profile — the caller
// substitutes defaults. Values are returned as stored; no default substitution happens here.
func (r *Repository) Profile(ctx context.Context, configID int64) (mihomo.Profile, error) {
	var p mihomo.Profile

	err := r.db.QueryRowContext(ctx,
		`SELECT title,filename,update_interval,proxies_interval FROM mihomo_profile WHERE config_id=?`, configID).
		Scan(&p.Title, &p.Filename, &p.UpdateInterval, &p.ProxiesInterval)
	if errors.Is(err, sql.ErrNoRows) {
		return mihomo.Profile{}, nil
	}

	return p, err
}
