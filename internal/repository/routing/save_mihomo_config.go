package routing

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/repository/dberr"
	"github.com/postlog/subgen/internal/utils"
)

// SaveMihomoConfig atomically replaces one config's proxy-groups, routing rules,
// rule-providers, base_yaml setting and profile row in a single transaction
// (all-or-nothing). Only rows scoped to configID are touched — other configs are
// untouched.
//
// The input is a ConfigDraft: group and provider references are carried as array
// INDICES (PolicyRef.GroupIdx into draft.Groups; RuleDraft.ProviderIdx into
// draft.Providers), because the persisted ids are assigned here. Inbound refs use the
// real node_inbounds.id. Groups and providers are inserted first (one batched INSERT
// each), then their ids are read back in order so the member/rule references resolve.
func (r *Repository) SaveMihomoConfig(ctx context.Context, configID int64, draft mihomo.ConfigDraft) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	// Drop this config's rows in FK order: rules reference groups/inbounds/providers,
	// so go first; members have no config_id — scope them via their group.
	for _, stmt := range []string{
		`DELETE FROM mihomo_routing_rules WHERE config_id=?`,
		`DELETE FROM mihomo_proxy_group_members WHERE group_id IN (SELECT id FROM mihomo_proxy_groups WHERE config_id=?)`,
		`DELETE FROM mihomo_proxy_groups WHERE config_id=?`,
		`DELETE FROM mihomo_rule_providers WHERE config_id=?`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, configID); err != nil {
			return err
		}
	}

	// Groups first; read the assigned ids back in position order so member/rule group
	// references (carried as indices) resolve to the persisted id.
	groupRows := make([][]any, len(draft.Groups))
	for i, g := range draft.Groups {
		groupRows[i] = []any{
			configID, i, g.Name, g.Type, g.URL,
			utils.DereferenceOrNil(g.Interval), utils.DereferenceOrNil(g.Tolerance), boolIntPtr(g.Lazy),
		}
	}

	if err := batchInsert(ctx, tx, "mihomo_proxy_groups",
		[]string{"config_id", "position", "name", "type", "url", "interval", "tolerance", "lazy"}, groupRows); err != nil {
		return err
	}

	groupID, err := readIDs(ctx, tx, `SELECT id FROM mihomo_proxy_groups WHERE config_id=? ORDER BY position`, configID)
	if err != nil {
		return err
	}

	// Providers before rules; read ids back (id order = insert order = draft order) so a
	// RULE-SET rule's ProviderIdx resolves to provider_id. An empty source defaults to
	// external (back-compat with drafts predating the source field).
	providerRows := make([][]any, len(draft.Providers))
	for i, rp := range draft.Providers {
		src := rp.Source
		if src == "" {
			src = mihomo.RuleProviderExternal
		}

		providerRows[i] = []any{configID, rp.Name, string(src), rp.Behavior, rp.Format, boolInt(rp.Mirror), rp.URL, rp.Interval, rp.MirrorInterval}
	}

	if err := batchInsert(ctx, tx, "mihomo_rule_providers",
		[]string{"config_id", "name", "source", "behavior", "format", "mirror", "url", "interval", "mirror_interval"}, providerRows); err != nil {
		// Groups are pre-validated in-memory, so a uniqueness violation here is the
		// rule-provider UNIQUE(config_id,name). Translate from the constraint, no pre-check.
		if dberr.IsUniqueViolation(err) {
			return entity.ErrRuleProviderNameTaken
		}

		return err
	}

	providerID, err := readIDs(ctx, tx, `SELECT id FROM mihomo_rule_providers WHERE config_id=? ORDER BY id`, configID)
	if err != nil {
		return err
	}

	// Authored providers carry an in-subgen matcher tree (mihomo_authored_matchers,
	// scoped by provider_id). The provider DELETE above cascades to its old matchers.
	if err := insertAuthoredMatchers(ctx, tx, draft.Providers, providerID); err != nil {
		return err
	}

	// Members (across all groups), refs resolved to columns.
	var memberRows [][]any

	for i, g := range draft.Groups {
		for j, m := range g.Members {
			inbound, group, err := refColumns(m, groupID)
			if err != nil {
				return err
			}

			memberRows = append(memberRows, []any{groupID[i], j, m.Kind, inbound, group})
		}
	}

	if err := batchInsert(ctx, tx, "mihomo_proxy_group_members",
		[]string{"group_id", "position", "kind", "inbound_id", "ref_group_id"}, memberRows); err != nil {
		return err
	}

	// Rules are recursive (a logical rule's sub-rules in the same table via parent_id);
	// insert the whole tree in one batch. The DELETE above removed the whole config's rules
	// (top-level + sub-rules all carry config_id), so no separate cleanup is needed.
	if err := insertRules(ctx, tx, configID, draft.Rules, groupID, providerID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mihomo_settings(config_id,key,value) VALUES(?,'base_yaml',?)
		 ON CONFLICT(config_id,key) DO UPDATE SET value=excluded.value`, configID, draft.BaseYAML); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mihomo_profile(config_id,title,filename,update_interval,proxies_interval) VALUES(?,?,?,?,?)
		 ON CONFLICT(config_id) DO UPDATE SET
		   title=excluded.title, filename=excluded.filename,
		   update_interval=excluded.update_interval, proxies_interval=excluded.proxies_interval`,
		configID, draft.Profile.Title, draft.Profile.Filename, draft.Profile.UpdateInterval, draft.Profile.ProxiesInterval); err != nil {
		return err
	}

	return tx.Commit()
}

// insertAuthoredMatchers inserts every authored provider's matcher tree into
// mihomo_authored_matchers in ONE batched INSERT. Like insertRules, each node's id is
// pre-assigned from the table's current MAX(id) so a sub-matcher's parent_id is known up
// front; this is safe because every insert into this table is an explicit-id batch (save +
// clone). A matcher carries no target/provider — only type, value (NULL for a logical node)
// and children (only for a logical node). providers[i] is persisted at providerID[i].
func insertAuthoredMatchers(ctx context.Context, tx *sql.Tx, providers []mihomo.RuleProvider, providerID []int64) error {
	base, err := maxAuthoredMatcherID(ctx, tx)
	if err != nil {
		return err
	}

	var rows [][]any

	next := base

	var build func(rs []mihomo.RoutingRule, provID int64, parentID any)

	build = func(rs []mihomo.RoutingRule, provID int64, parentID any) {
		for pos, m := range rs {
			next++
			id := next

			rows = append(rows, []any{id, provID, parentID, pos, m.Type, utils.DereferenceOrNil(m.Value)})

			if m.Type.IsLogical() {
				build(m.Children, provID, id)
			}
		}
	}

	for i, rp := range providers {
		if rp.Source == mihomo.RuleProviderAuthored {
			build(rp.Matchers, providerID[i], nil)
		}
	}

	return batchInsert(ctx, tx, "mihomo_authored_matchers",
		[]string{"id", "provider_id", "parent_id", "position", "type", "value"}, rows)
}

// maxAuthoredMatcherID returns the current MAX(id) of mihomo_authored_matchers (0 when
// empty) — the base for pre-assigning ids to a freshly-inserted matcher tree.
func maxAuthoredMatcherID(ctx context.Context, tx *sql.Tx) (int64, error) {
	var maxID int64

	err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) FROM mihomo_authored_matchers`).Scan(&maxID)

	return maxID, err
}

// insertRules inserts the whole rule tree (top-level rules + their logical sub-rules, all
// in the same table via parent_id) in ONE batched INSERT. The tree is walked in memory to
// build the rows; each node's id is pre-assigned from the table's current MAX(id), so a
// sub-rule's parent_id is known up front without a per-row round-trip. Pre-assigned ids
// are safe because every insert into mihomo_routing_rules is an explicit-id batch like
// this one (save + clone) — there are no auto-increment inserts, so MAX(id)+k always stays
// above existing rows and never collides. A top-level rule's Target resolves to
// (target_kind, inbound_id, target_group_id); a sub-rule has none (NULL). A RULE-SET's
// ProviderIdx resolves to provider_id; children exist only for a logical rule.
func insertRules(ctx context.Context, tx *sql.Tx, configID int64, rules []mihomo.RuleDraft, groupID, providerID []int64) error {
	base, err := maxRuleID(ctx, tx)
	if err != nil {
		return err
	}

	var rows [][]any

	next := base

	var build func(rs []mihomo.RuleDraft, parentID any) error

	build = func(rs []mihomo.RuleDraft, parentID any) error {
		for pos, r := range rs {
			var inbound, group, kind any

			if r.Target != nil {
				ib, gp, err := refColumns(*r.Target, groupID)
				if err != nil {
					return err
				}

				inbound, group, kind = ib, gp, string(r.Target.Kind)
			}

			provider, err := providerColumn(r.ProviderIdx, providerID)
			if err != nil {
				return err
			}

			next++
			id := next

			rows = append(rows, []any{
				id, configID, parentID, pos, r.Type, utils.DereferenceOrNil(r.Value), provider,
				boolInt(r.NoResolve != nil && *r.NoResolve), kind, inbound, group,
			})

			if r.Type.IsLogical() {
				if err := build(r.Children, id); err != nil {
					return err
				}
			}
		}

		return nil
	}

	if err := build(rules, nil); err != nil {
		return err
	}

	return batchInsert(ctx, tx, "mihomo_routing_rules",
		[]string{"id", "config_id", "parent_id", "position", "type", "value", "provider_id", "no_resolve", "target_kind", "inbound_id", "target_group_id"},
		rows)
}

// maxRuleID returns the current MAX(id) of mihomo_routing_rules (0 when empty) — the base
// for pre-assigning ids to a freshly-inserted rule tree (see insertRules / cloneRules).
func maxRuleID(ctx context.Context, tx *sql.Tx) (int64, error) {
	var maxID int64

	err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) FROM mihomo_routing_rules`).Scan(&maxID)

	return maxID, err
}

// batchInsert inserts every row into table(cols) in ONE multi-row INSERT (no per-row
// round-trip). An empty rows is a no-op; each row must hold len(cols) values. Configs are
// small, so the total placeholder count stays well under SQLite's variable limit.
func batchInsert(ctx context.Context, tx *sql.Tx, table string, cols []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}

	tuple := "(" + strings.Repeat("?,", len(cols)-1) + "?)"
	tuples := strings.Repeat(tuple+",", len(rows)-1) + tuple

	// Grow by append rather than pre-sizing to len(rows)*len(cols): the product is a
	// capacity hint only, and CodeQL flags the multiplication as a possible overflow.
	// Configs are tiny, so the few reallocs are immaterial.
	var args []any
	for _, row := range rows {
		args = append(args, row...)
	}

	// table/cols are package-internal constants (never user input) and every value is a
	// bound parameter, so the concatenation is safe.
	query := "INSERT INTO " + table + "(" + strings.Join(cols, ",") + ") VALUES " + tuples //nolint:gosec // table/cols constant; values parameterized

	_, err := tx.ExecContext(ctx, query, args...)

	return err
}

// readIDs runs a single id-selecting query and collects the ids in row order.
func readIDs(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var ids []int64

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// refColumns maps a save-time RefDraft to the (inbound_id, ref/target group id) column
// values: inbound → the real inbound id; group → the persisted id at the referenced
// index; built-ins → both nil.
func refColumns(ref mihomo.RefDraft, groupID []int64) (inbound, group any, err error) {
	switch ref.Kind {
	case mihomo.PolicyInbound:
		if ref.InboundID == nil {
			return nil, nil, fmt.Errorf("inbound ref without inbound id")
		}

		return *ref.InboundID, nil, nil
	case mihomo.PolicyGroup:
		if ref.GroupIdx == nil || *ref.GroupIdx < 0 || *ref.GroupIdx >= len(groupID) {
			return nil, nil, fmt.Errorf("group ref index out of range: %v", ref.GroupIdx)
		}

		return nil, groupID[*ref.GroupIdx], nil
	default:
		return nil, nil, nil
	}
}

// providerColumn maps a RULE-SET rule's ProviderIdx to the provider_id column value
// (the persisted id at the referenced index); a nil index → nil column (non-RULE-SET).
func providerColumn(idx *int, providerID []int64) (any, error) {
	if idx == nil {
		return nil, nil
	}

	if *idx < 0 || *idx >= len(providerID) {
		return nil, fmt.Errorf("provider ref index out of range: %d", *idx)
	}

	return providerID[*idx], nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}

	return 0
}

// boolIntPtr maps an optional bool to a nullable integer column: nil → NULL, else 0/1.
func boolIntPtr(b *bool) any {
	if b == nil {
		return nil
	}

	return boolInt(*b)
}
