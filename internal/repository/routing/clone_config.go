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
// so an open query can't overlap writes on the same tx. Inserts are batched (one
// multi-row INSERT per table, like SaveMihomoConfig); the new ids of the id'd tables
// (groups, providers) are read back in the same order they were inserted to build the
// old→new remap (dst is a fresh config, so its rows are exactly the clones).
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

	ruleMap, err := cloneRules(ctx, tx, srcConfigID, dstConfigID, groupMap, provMap)
	if err != nil {
		return err
	}

	if err := cloneConditions(ctx, tx, srcConfigID, ruleMap, provMap); err != nil {
		return err
	}

	if err := cloneSettings(ctx, tx, srcConfigID, dstConfigID); err != nil {
		return err
	}

	return cloneProfile(ctx, tx, srcConfigID, dstConfigID)
}

// cloneGroups copies the groups (one batched INSERT) and returns the old→new group id
// remap. Source order is by position; the new ids are read back by position too, so the
// i-th old group pairs with the i-th new id.
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

	groupRows := make([][]any, len(groups))
	for i, g := range groups {
		groupRows[i] = []any{dst, g.position, g.name, g.typ, g.url, g.interval, g.tolerance, g.lazy}
	}

	if err := batchInsert(ctx, tx, "mihomo_proxy_groups",
		[]string{"config_id", "position", "name", "type", "url", "interval", "tolerance", "lazy"}, groupRows); err != nil {
		return nil, err
	}

	newIDs, err := readIDs(ctx, tx, `SELECT id FROM mihomo_proxy_groups WHERE config_id=? ORDER BY position`, dst)
	if err != nil {
		return nil, err
	}

	idMap := make(map[int64]int64, len(groups))
	for i, g := range groups {
		idMap[g.id] = newIDs[i]
	}

	return idMap, nil
}

// cloneMembers copies members (one batched INSERT), remapping group_id and ref_group_id
// to the clones.
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

	var memberRows [][]any

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

		memberRows = append(memberRows, []any{newGroup, m.position, m.kind, inbound, refGroup})
	}

	return batchInsert(ctx, tx, "mihomo_proxy_group_members",
		[]string{"group_id", "position", "kind", "inbound_id", "ref_group_id"}, memberRows)
}

// cloneRules copies the rules (one batched INSERT), remapping target_group_id to the
// cloned groups and a RULE-SET's provider_id to the cloned providers. value is nullable
// (NULL for MATCH/RULE-SET). It returns the old→new rule id remap (a sub-condition's
// rule_id is remapped through it): source order is by position, the new ids are read back
// by position too, so the i-th old rule pairs with the i-th new id.
func cloneRules(ctx context.Context, tx *sql.Tx, src, dst int64, groupMap, provMap map[int64]int64) (map[int64]int64, error) {
	type rule struct {
		id         int64
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
		`SELECT id,position,type,value,provider_id,no_resolve,target_kind,inbound_id,target_group_id
		   FROM mihomo_routing_rules WHERE config_id=? ORDER BY position`, src)
	if err != nil {
		return nil, err
	}

	var rules []rule

	for rows.Next() {
		var ru rule
		if err := rows.Scan(&ru.id, &ru.position, &ru.typ, &ru.value, &ru.providerID, &ru.noResolve, &ru.kind, &ru.inboundID, &ru.groupID); err != nil {
			rows.Close()
			return nil, err
		}

		rules = append(rules, ru)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}

	rows.Close()

	ruleRows := make([][]any, len(rules))

	for i, ru := range rules {
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

		ruleRows[i] = []any{dst, ru.position, ru.typ, ru.value, provider, ru.noResolve, ru.kind, inbound, group}
	}

	if err := batchInsert(ctx, tx, "mihomo_routing_rules",
		[]string{"config_id", "position", "type", "value", "provider_id", "no_resolve", "target_kind", "inbound_id", "target_group_id"}, ruleRows); err != nil {
		return nil, err
	}

	newIDs, err := readIDs(ctx, tx, `SELECT id FROM mihomo_routing_rules WHERE config_id=? ORDER BY position`, dst)
	if err != nil {
		return nil, err
	}

	idMap := make(map[int64]int64, len(rules))
	for i, ru := range rules {
		idMap[ru.id] = newIDs[i]
	}

	return idMap, nil
}

// cloneConditions copies every logical rule's sub-condition tree, remapping rule_id to the
// cloned rules, provider_id to the cloned providers, and parent_id to the cloned
// conditions. Rows are read ordered by id (a parent is always inserted before its children
// — insertConditions writes depth-first, so a parent's id is always less than its
// children's), so the running old→new condition map is populated before any child needs it.
func cloneConditions(ctx context.Context, tx *sql.Tx, src int64, ruleMap, provMap map[int64]int64) error {
	if len(ruleMap) == 0 {
		return nil
	}

	type cond struct {
		id         int64
		ruleID     int64
		parentID   sql.Null[int64]
		position   int
		typ        string
		value      sql.Null[string]
		providerID sql.Null[int64]
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id,rule_id,parent_id,position,type,value,provider_id
		   FROM mihomo_rule_conditions
		  WHERE rule_id IN (SELECT id FROM mihomo_routing_rules WHERE config_id=?)
		  ORDER BY id`, src)
	if err != nil {
		return err
	}

	var conds []cond

	for rows.Next() {
		var c cond
		if err := rows.Scan(&c.id, &c.ruleID, &c.parentID, &c.position, &c.typ, &c.value, &c.providerID); err != nil {
			rows.Close()
			return err
		}

		conds = append(conds, c)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}

	rows.Close()

	condMap := map[int64]int64{} // old condition id -> new condition id

	for _, c := range conds {
		newRule, ok := ruleMap[c.ruleID]
		if !ok {
			continue
		}

		var parent, provider any

		if c.parentID.Valid {
			if mapped, ok := condMap[c.parentID.V]; ok {
				parent = mapped
			}
		}

		if c.providerID.Valid {
			if mapped, ok := provMap[c.providerID.V]; ok {
				provider = mapped
			}
		}

		res, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_rule_conditions(rule_id,parent_id,position,type,value,provider_id) VALUES(?,?,?,?,?,?)`,
			newRule, parent, c.position, c.typ, c.value, provider)
		if err != nil {
			return err
		}

		newID, err := res.LastInsertId()
		if err != nil {
			return err
		}

		condMap[c.id] = newID
	}

	return nil
}

// cloneProviders copies the rule-providers (one batched INSERT) under the new config_id
// and returns the old→new provider id remap (a RULE-SET rule's provider_id is remapped
// through it). Source order is by id; the new ids are read back by id too, so the i-th
// old provider pairs with the i-th new id.
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

	provRows := make([][]any, len(provs))
	for i, p := range provs {
		provRows[i] = []any{dst, p.name, p.behavior, p.format, p.mirror, p.url, p.interval, p.mirrorInterval}
	}

	if err := batchInsert(ctx, tx, "mihomo_rule_providers",
		[]string{"config_id", "name", "behavior", "format", "mirror", "url", "interval", "mirror_interval"}, provRows); err != nil {
		return nil, err
	}

	newIDs, err := readIDs(ctx, tx, `SELECT id FROM mihomo_rule_providers WHERE config_id=? ORDER BY id`, dst)
	if err != nil {
		return nil, err
	}

	idMap := make(map[int64]int64, len(provs))
	for i, p := range provs {
		idMap[p.id] = newIDs[i]
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
