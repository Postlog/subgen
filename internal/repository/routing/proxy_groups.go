package routing

import (
	"context"
	"database/sql"

	"github.com/postlog/subgen/internal/mihomo"
)

// ProxyGroups returns the config's proxy-groups in order, each with its ordered
// members (typed PolicyRefs). Two queries (groups, then their members) are assembled
// in memory — the set is small.
func (r *Repository) ProxyGroups(ctx context.Context, configID int64) ([]mihomo.ProxyGroup, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,position,name,type,url,interval,tolerance,lazy
		   FROM mihomo_proxy_groups WHERE config_id=? ORDER BY position`, configID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var groups []mihomo.ProxyGroup

	byID := map[int64]int{} // group id -> index in groups

	for rows.Next() {
		var (
			g         mihomo.ProxyGroup
			interval  sql.Null[int64]
			tolerance sql.Null[int64]
			lazy      sql.Null[int64]
		)

		if err := rows.Scan(&g.ID, &g.Position, &g.Name, &g.Type, &g.URL, &interval, &tolerance, &lazy); err != nil {
			return nil, err
		}

		if interval.Valid {
			v := int(interval.V)
			g.Interval = &v
		}

		if tolerance.Valid {
			v := int(tolerance.V)
			g.Tolerance = &v
		}

		if lazy.Valid {
			b := lazy.V != 0
			g.Lazy = &b
		}

		byID[g.ID] = len(groups)
		groups = append(groups, g)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := r.loadMembers(ctx, groups, byID); err != nil {
		return nil, err
	}

	return groups, nil
}

// loadMembers attaches members to their groups (in position order).
func (r *Repository) loadMembers(ctx context.Context, groups []mihomo.ProxyGroup, byID map[int64]int) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT group_id,kind,inbound_id,ref_group_id
		   FROM mihomo_proxy_group_members ORDER BY group_id,position`)
	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var (
			groupID   int64
			kind      string
			inboundID sql.Null[int64]
			refGroup  sql.Null[int64]
		)

		if err := rows.Scan(&groupID, &kind, &inboundID, &refGroup); err != nil {
			return err
		}

		idx, ok := byID[groupID]
		if !ok {
			continue // orphan member (shouldn't happen with the FK cascade)
		}

		groups[idx].Members = append(groups[idx].Members, policyRef(kind, inboundID, refGroup))
	}

	return rows.Err()
}
