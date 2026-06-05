package routing

import (
	"context"
	"database/sql"
)

// CloneConfig copies all mihomo content (proxy-groups + members, routing rules,
// rule-providers, settings) from srcConfigID into dstConfigID, within the caller's
// transaction. It is the engine-specific half of configs.CreateUserConfig (the
// generic anchor repo owns the tx and calls this through a narrow cloner contract),
// so a custom config starts as a full snapshot of the base.
//
// Group ids change on copy, so member ref_group_id and rule target_group_id are
// remapped to the cloned groups. Rows are fully read before any insert: SQLite runs
// on a single connection, so an open query can't overlap writes on the same tx.
func (r *Repository) CloneConfig(ctx context.Context, tx *sql.Tx, srcConfigID, dstConfigID int64) error {
	idMap, err := cloneGroups(ctx, tx, srcConfigID, dstConfigID)
	if err != nil {
		return err
	}

	if err := cloneMembers(ctx, tx, srcConfigID, idMap); err != nil {
		return err
	}

	if err := cloneRules(ctx, tx, srcConfigID, dstConfigID, idMap); err != nil {
		return err
	}

	if err := cloneProviders(ctx, tx, srcConfigID, dstConfigID); err != nil {
		return err
	}

	return cloneSettings(ctx, tx, srcConfigID, dstConfigID)
}

// cloneGroups copies the groups and returns the old→new group id remap.
func cloneGroups(ctx context.Context, tx *sql.Tx, src, dst int64) (map[int64]int64, error) {
	type group struct {
		id        int64
		position  int
		name      string
		typ       string
		url       string
		interval  int
		tolerance int
		lazy      int
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id,position,name,type,url,interval,tolerance,lazy
		   FROM mihomo_proxy_groups WHERE config_id=? ORDER BY position`, src)
	if err != nil {
		return nil, err
	}

	var groups []group

	for rows.Next() {
		var g group
		if err := rows.Scan(&g.id, &g.position, &g.name, &g.typ, &g.url, &g.interval, &g.tolerance, &g.lazy); err != nil {
			rows.Close()
			return nil, err
		}

		groups = append(groups, g)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}

	rows.Close()

	idMap := make(map[int64]int64, len(groups))

	for _, g := range groups {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_proxy_groups(config_id,position,name,type,url,interval,tolerance,lazy)
			 VALUES(?,?,?,?,?,?,?,?)`,
			dst, g.position, g.name, g.typ, g.url, g.interval, g.tolerance, g.lazy)
		if err != nil {
			return nil, err
		}

		newID, _ := res.LastInsertId()
		idMap[g.id] = newID
	}

	return idMap, nil
}

// cloneMembers copies members, remapping group_id and ref_group_id to the clones.
func cloneMembers(ctx context.Context, tx *sql.Tx, src int64, idMap map[int64]int64) error {
	if len(idMap) == 0 {
		return nil
	}

	type member struct {
		groupID   int64
		position  int
		kind      string
		inboundID sql.NullInt64
		refGroup  sql.NullInt64
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT group_id,position,kind,inbound_id,ref_group_id
		   FROM mihomo_proxy_group_members
		  WHERE group_id IN (SELECT id FROM mihomo_proxy_groups WHERE config_id=?)
		  ORDER BY group_id,position`, src)
	if err != nil {
		return err
	}

	var members []member

	for rows.Next() {
		var m member
		if err := rows.Scan(&m.groupID, &m.position, &m.kind, &m.inboundID, &m.refGroup); err != nil {
			rows.Close()
			return err
		}

		members = append(members, m)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}

	rows.Close()

	for _, m := range members {
		newGroup, ok := idMap[m.groupID]
		if !ok {
			continue
		}

		var refGroup any
		if m.refGroup.Valid {
			if mapped, ok := idMap[m.refGroup.Int64]; ok {
				refGroup = mapped
			}
		}

		var inbound any
		if m.inboundID.Valid {
			inbound = m.inboundID.Int64
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_proxy_group_members(group_id,position,kind,inbound_id,ref_group_id)
			 VALUES(?,?,?,?,?)`,
			newGroup, m.position, m.kind, inbound, refGroup); err != nil {
			return err
		}
	}

	return nil
}

// cloneRules copies the rules, remapping target_group_id to the cloned groups.
func cloneRules(ctx context.Context, tx *sql.Tx, src, dst int64, idMap map[int64]int64) error {
	type rule struct {
		position  int
		typ       string
		value     string
		noResolve int
		kind      string
		inboundID sql.NullInt64
		groupID   sql.NullInt64
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT position,type,value,no_resolve,target_kind,inbound_id,target_group_id
		   FROM mihomo_routing_rules WHERE config_id=? ORDER BY position`, src)
	if err != nil {
		return err
	}

	var rules []rule

	for rows.Next() {
		var ru rule
		if err := rows.Scan(&ru.position, &ru.typ, &ru.value, &ru.noResolve, &ru.kind, &ru.inboundID, &ru.groupID); err != nil {
			rows.Close()
			return err
		}

		rules = append(rules, ru)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}

	rows.Close()

	for _, ru := range rules {
		var inbound, group any
		if ru.inboundID.Valid {
			inbound = ru.inboundID.Int64
		}

		if ru.groupID.Valid {
			if mapped, ok := idMap[ru.groupID.Int64]; ok {
				group = mapped
			}
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_routing_rules(config_id,position,type,value,no_resolve,target_kind,inbound_id,target_group_id)
			 VALUES(?,?,?,?,?,?,?,?)`,
			dst, ru.position, ru.typ, ru.value, ru.noResolve, ru.kind, inbound, group); err != nil {
			return err
		}
	}

	return nil
}

// cloneProviders copies the rule-providers verbatim under the new config_id.
func cloneProviders(ctx context.Context, tx *sql.Tx, src, dst int64) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mihomo_rule_providers(config_id,name,behavior,format,mirror,url,interval,mirror_interval)
		 SELECT ?,name,behavior,format,mirror,url,interval,mirror_interval
		   FROM mihomo_rule_providers WHERE config_id=?`, dst, src); err != nil {
		return err
	}

	return nil
}

// cloneSettings copies the free-form settings (base_yaml) under the new config_id.
func cloneSettings(ctx context.Context, tx *sql.Tx, src, dst int64) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mihomo_settings(config_id,key,value)
		 SELECT ?,key,value FROM mihomo_settings WHERE config_id=?`, dst, src); err != nil {
		return err
	}

	return nil
}
