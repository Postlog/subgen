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
// The input is a ConfigDraft: group and provider references are carried as array
// INDICES (PolicyRef.GroupIdx into draft.Groups; RuleDraft.ProviderIdx into
// draft.Providers), because the persisted ids are assigned here. Inbound refs use the
// real node_inbounds.id. Groups and providers are inserted first so their ids can
// resolve the rule/member references.
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

	// Insert groups first; record the assigned id per index so member/rule group
	// references (carried as indices) can be resolved.
	groupID := make([]int64, len(draft.Groups))

	for i, g := range draft.Groups {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_proxy_groups(config_id,position,name,type,url,interval,tolerance,lazy)
			 VALUES(?,?,?,?,?,?,?,?)`,
			configID, i, g.Name, g.Type, g.URL, g.Interval, g.Tolerance, boolIntPtr(g.Lazy))
		if err != nil {
			return err
		}

		groupID[i], _ = res.LastInsertId()
	}

	for i, g := range draft.Groups {
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

	// Insert providers before rules; record the assigned id per index so a RULE-SET
	// rule's ProviderIdx can be resolved to provider_id.
	providerID := make([]int64, len(draft.Providers))

	for i, rp := range draft.Providers {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_rule_providers(config_id,name,behavior,format,mirror,url,interval,mirror_interval)
			 VALUES(?,?,?,?,?,?,?,?)`,
			configID, rp.Name, rp.Behavior, rp.Format, boolInt(rp.Mirror), rp.URL, rp.Interval, rp.MirrorInterval)
		if err != nil {
			// Groups are pre-validated in-memory, so a uniqueness violation here is the
			// rule-provider UNIQUE(config_id,name). Translate from the constraint, no pre-check.
			if dberr.IsUniqueViolation(err) {
				return entity.ErrRuleProviderNameTaken
			}

			return err
		}

		providerID[i], _ = res.LastInsertId()
	}

	for i, rule := range draft.Rules {
		inbound, group, err := refColumns(rule.Target, groupID)
		if err != nil {
			return err
		}

		provider, err := providerColumn(rule.ProviderIdx, providerID)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO mihomo_routing_rules(config_id,position,type,value,provider_id,no_resolve,target_kind,inbound_id,target_group_id)
			 VALUES(?,?,?,?,?,?,?,?,?)`,
			configID, i, rule.Type, rule.Value, provider, boolInt(rule.NoResolve), rule.Target.Kind, inbound, group); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mihomo_settings(config_id,key,value) VALUES(?,'base_yaml',?)
		 ON CONFLICT(config_id,key) DO UPDATE SET value=excluded.value`, configID, draft.BaseYAML); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mihomo_profile(config_id,title,filename,update_interval) VALUES(?,?,?,?)
		 ON CONFLICT(config_id) DO UPDATE SET
		   title=excluded.title, filename=excluded.filename, update_interval=excluded.update_interval`,
		configID, draft.Profile.Title, draft.Profile.Filename, draft.Profile.UpdateInterval); err != nil {
		return err
	}

	return tx.Commit()
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
