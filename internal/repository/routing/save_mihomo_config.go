package routing

import (
	"context"
	"fmt"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/repository/dberr"
)

// SaveMihomoConfig atomically replaces one config's proxy-groups, routing rules,
// rule-providers, base_yaml setting and profile row in a single transaction
// (all-or-nothing). Only rows scoped to configID are touched — other configs are
// untouched.
//
// Group references are resolved by INDEX: in a group member or a rule target whose
// kind is "group", PolicyRef.GroupID holds the 0-based index into the groups slice
// (the persisted ids are assigned here, so the caller can't know them yet). Force
// references use the real node_inbounds.id in PolicyRef.InboundID.
func (r *Repository) SaveMihomoConfig(ctx context.Context, configID int64, rules []mihomo.RoutingRule, groups []mihomo.ProxyGroup, rps []mihomo.RuleProvider, baseYAML string, profile mihomo.Profile) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	// Drop this config's rows in FK order: rules and members reference groups/inbounds,
	// so go first. Members have no config_id — scope them via their group.
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

	// Insert groups first; record the assigned id per index so member/rule group
	// references (carried as indices) can be resolved.
	groupID := make([]int64, len(groups))

	for i, g := range groups {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_proxy_groups(config_id,position,name,type,url,interval,tolerance,lazy)
			 VALUES(?,?,?,?,?,?,?,?)`,
			configID, i, g.Name, g.Type, g.URL, g.Interval, g.Tolerance, boolInt(g.Lazy))
		if err != nil {
			return err
		}

		groupID[i], _ = res.LastInsertId()
	}

	for i, g := range groups {
		for j, m := range g.Members {
			inbound, group, err := refColumns(m, groupID)
			if err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx,
				`INSERT INTO mihomo_proxy_group_members(group_id,position,kind,inbound_id,ref_group_id)
				 VALUES(?,?,?,?,?)`,
				groupID[i], j, m.Kind, inbound, group); err != nil {
				return err
			}
		}
	}

	for i, rule := range rules {
		inbound, group, err := refColumns(rule.Target, groupID)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_routing_rules(config_id,position,type,value,no_resolve,target_kind,inbound_id,target_group_id)
			 VALUES(?,?,?,?,?,?,?,?)`,
			configID, i, rule.Type, rule.Value, boolInt(rule.NoResolve), rule.Target.Kind, inbound, group); err != nil {
			return err
		}
	}

	for _, rp := range rps {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_rule_providers(config_id,name,behavior,format,mirror,url,interval,mirror_interval) VALUES(?,?,?,?,?,?,?,?)`,
			configID, rp.Name, rp.Behavior, rp.Format, boolInt(rp.Mirror), rp.URL, rp.Interval, rp.MirrorInterval); err != nil {
			// Groups are pre-validated in-memory, so a uniqueness violation here is the
			// rule-provider name PK (1555). Translate from the constraint, no pre-check.
			if dberr.IsUniqueViolation(err) {
				return entity.ErrRuleProviderNameTaken
			}

			return err
		}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mihomo_settings(config_id,key,value) VALUES(?,'base_yaml',?)
		 ON CONFLICT(config_id,key) DO UPDATE SET value=excluded.value`, configID, baseYAML); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mihomo_profile(config_id,title,filename,update_interval) VALUES(?,?,?,?)
		 ON CONFLICT(config_id) DO UPDATE SET
		   title=excluded.title, filename=excluded.filename, update_interval=excluded.update_interval`,
		configID, profile.Title, profile.Filename, profile.UpdateInterval); err != nil {
		return err
	}

	return tx.Commit()
}

// refColumns maps a save-time PolicyRef to the (inbound_id, ref/target group id)
// column values: inbound → the real inbound id; group → the persisted id at the
// referenced index; built-ins → both nil.
func refColumns(ref mihomo.PolicyRef, groupID []int64) (inbound, group any, err error) {
	switch ref.Kind {
	case mihomo.PolicyInbound:
		if ref.InboundID == nil {
			return nil, nil, fmt.Errorf("inbound ref without inbound id")
		}

		return *ref.InboundID, nil, nil
	case mihomo.PolicyGroup:
		if ref.GroupID == nil || *ref.GroupID < 0 || int(*ref.GroupID) >= len(groupID) {
			return nil, nil, fmt.Errorf("group ref index out of range: %v", ref.GroupID)
		}

		return nil, groupID[*ref.GroupID], nil
	default:
		return nil, nil, nil
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}

	return 0
}
