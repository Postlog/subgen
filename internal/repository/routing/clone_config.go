package routing

import (
	"context"
	"database/sql"
)

// CloneConfig copies all mihomo content (proxy-groups + members, routing rules,
// rule-providers, settings, profile) from srcConfigID into dstConfigID, within the
// caller's transaction. It is the engine-specific half of configs.CreateUserConfig (the
// generic anchor repo owns the tx and calls this through a narrow cloner contract),
// so a custom config starts as a full snapshot of the base.
//
// Group AND provider ids change on copy, so member ref_group_id / rule target_group_id
// are remapped to the cloned groups and a RULE-SET rule's provider_id to the cloned
// providers. Rows are fully read before any insert: SQLite runs on a single connection,
// so an open query can't overlap writes on the same tx.
func (r *Repository) CloneConfig(ctx context.Context, tx *sql.Tx, srcConfigID, dstConfigID int64) error {
	groupMap, err := cloneGroups(ctx, tx, srcConfigID, dstConfigID)
	if err != nil {
		return err
	}

	if err := cloneMembers(ctx, tx, srcConfigID, groupMap); err != nil {
		return err
	}

	provMap, err := cloneProviders(ctx, tx, srcConfigID, dstConfigID)
	if err != nil {
		return err
	}

	if err := cloneRules(ctx, tx, srcConfigID, dstConfigID, groupMap, provMap); err != nil {
		return err
	}

	if err := cloneSettings(ctx, tx, srcConfigID, dstConfigID); err != nil {
		return err
	}

	return cloneProfile(ctx, tx, srcConfigID, dstConfigID)
}

// cloneGroups copies the groups and returns the old→new group id remap.
func cloneGroups(ctx context.Context, tx *sql.Tx, src, dst int64) (map[int64]int64, error) {
	type group struct {
		id        int64
		position  int
		name      string
		typ       string
		url       string
		interval  sql.Null[int64]
		tolerance sql.Null[int64]
		lazy      sql.Null[int64]
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
		inboundID sql.Null[int64]
		refGroup  sql.Null[int64]
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
			if mapped, ok := idMap[m.refGroup.V]; ok {
				refGroup = mapped
			}
		}

		var inbound any
		if m.inboundID.Valid {
			inbound = m.inboundID.V
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

// cloneRules copies the rules, remapping target_group_id to the cloned groups and a
// RULE-SET's provider_id to the cloned providers. value is nullable (NULL for MATCH/
// RULE-SET).
func cloneRules(ctx context.Context, tx *sql.Tx, src, dst int64, groupMap, provMap map[int64]int64) error {
	type rule struct {
		position   int
		typ        string
		value      sql.Null[string]
		providerID sql.Null[int64]
		noResolve  int
		kind       string
		inboundID  sql.Null[int64]
		groupID    sql.Null[int64]
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT position,type,value,provider_id,no_resolve,target_kind,inbound_id,target_group_id
		   FROM mihomo_routing_rules WHERE config_id=? ORDER BY position`, src)
	if err != nil {
		return err
	}

	var rules []rule

	for rows.Next() {
		var ru rule
		if err := rows.Scan(&ru.position, &ru.typ, &ru.value, &ru.providerID, &ru.noResolve, &ru.kind, &ru.inboundID, &ru.groupID); err != nil {
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
		var inbound, group, provider any
		if ru.inboundID.Valid {
			inbound = ru.inboundID.V
		}

		if ru.groupID.Valid {
			if mapped, ok := groupMap[ru.groupID.V]; ok {
				group = mapped
			}
		}

		if ru.providerID.Valid {
			if mapped, ok := provMap[ru.providerID.V]; ok {
				provider = mapped
			}
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_routing_rules(config_id,position,type,value,provider_id,no_resolve,target_kind,inbound_id,target_group_id)
			 VALUES(?,?,?,?,?,?,?,?,?)`,
			dst, ru.position, ru.typ, ru.value, provider, ru.noResolve, ru.kind, inbound, group); err != nil {
			return err
		}
	}

	return nil
}

// cloneProviders copies the rule-providers under the new config_id and returns the
// old→new provider id remap (a RULE-SET rule's provider_id is remapped through it).
func cloneProviders(ctx context.Context, tx *sql.Tx, src, dst int64) (map[int64]int64, error) {
	type prov struct {
		id             int64
		name           string
		behavior       string
		format         string
		mirror         int
		url            string
		interval       int
		mirrorInterval int
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id,name,behavior,format,mirror,url,interval,mirror_interval
		   FROM mihomo_rule_providers WHERE config_id=? ORDER BY id`, src)
	if err != nil {
		return nil, err
	}

	var provs []prov

	for rows.Next() {
		var p prov
		if err := rows.Scan(&p.id, &p.name, &p.behavior, &p.format, &p.mirror, &p.url, &p.interval, &p.mirrorInterval); err != nil {
			rows.Close()
			return nil, err
		}

		provs = append(provs, p)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}

	rows.Close()

	idMap := make(map[int64]int64, len(provs))

	for _, p := range provs {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_rule_providers(config_id,name,behavior,format,mirror,url,interval,mirror_interval)
			 VALUES(?,?,?,?,?,?,?,?)`,
			dst, p.name, p.behavior, p.format, p.mirror, p.url, p.interval, p.mirrorInterval)
		if err != nil {
			return nil, err
		}

		newID, _ := res.LastInsertId()
		idMap[p.id] = newID
	}

	return idMap, nil
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

// cloneProfile copies the subscription-profile row (title, filename, update interval)
// under the new config_id. A missing source row copies nothing — the clone then falls
// back to defaults like any config without a profile row.
func cloneProfile(ctx context.Context, tx *sql.Tx, src, dst int64) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mihomo_profile(config_id,title,filename,update_interval)
		 SELECT ?,title,filename,update_interval FROM mihomo_profile WHERE config_id=?`, dst, src); err != nil {
		return err
	}

	return nil
}
