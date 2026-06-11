package routing

import (
	"context"
	"database/sql"

	"github.com/postlog/subgen/internal/mihomo"
)

// Rules returns the config's routing rules in order, each with its typed target
// (PolicyRef).
func (r *Repository) Rules(ctx context.Context, configID int64) ([]mihomo.RoutingRule, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,position,type,value,provider_id,no_resolve,target_kind,inbound_id,target_group_id
		   FROM mihomo_routing_rules WHERE config_id=? ORDER BY position`, configID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []mihomo.RoutingRule

	for rows.Next() {
		var (
			rule       mihomo.RoutingRule
			value      sql.Null[string]
			noResolve  int
			kind       string
			providerID sql.Null[int64]
			inboundID  sql.Null[int64]
			groupID    sql.Null[int64]
		)

		if err := rows.Scan(&rule.ID, &rule.Position, &rule.Type, &value,
			&providerID, &noResolve, &kind, &inboundID, &groupID); err != nil {
			return nil, err
		}

		if value.Valid {
			v := value.V
			rule.Value = &v
		}

		if providerID.Valid {
			id := providerID.V
			rule.ProviderID = &id
		}

		if noResolve != 0 {
			t := true
			rule.NoResolve = &t
		}

		rule.Target = policyRef(kind, inboundID, groupID)
		out = append(out, rule)
	}

	return out, rows.Err()
}

// policyRef assembles a PolicyRef from its persisted columns: the kind plus a
// nullable inbound_id (inbound) / group id (group). Shared by rules and group members.
func policyRef(kind string, inboundID, groupID sql.Null[int64]) mihomo.PolicyRef {
	ref := mihomo.PolicyRef{Kind: mihomo.PolicyKind(kind)}

	if inboundID.Valid {
		id := inboundID.V
		ref.InboundID = &id
	}

	if groupID.Valid {
		id := groupID.V
		ref.GroupID = &id
	}

	return ref
}
