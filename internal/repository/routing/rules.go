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
		`SELECT id,position,type,value,no_resolve,target_kind,inbound_id,target_group_id
		   FROM mihomo_routing_rules WHERE config_id=? ORDER BY position`, configID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []mihomo.RoutingRule

	for rows.Next() {
		var (
			rule      mihomo.RoutingRule
			noResolve int
			kind      string
			inboundID sql.NullInt64
			groupID   sql.NullInt64
		)

		if err := rows.Scan(&rule.ID, &rule.Position, &rule.Type, &rule.Value,
			&noResolve, &kind, &inboundID, &groupID); err != nil {
			return nil, err
		}

		rule.NoResolve = noResolve != 0
		rule.Target = policyRef(kind, inboundID, groupID)
		out = append(out, rule)
	}

	return out, rows.Err()
}

// policyRef assembles a PolicyRef from its persisted columns: the kind plus a
// nullable inbound_id (inbound) / group id (group). Shared by rules and group members.
func policyRef(kind string, inboundID, groupID sql.NullInt64) mihomo.PolicyRef {
	ref := mihomo.PolicyRef{Kind: mihomo.PolicyKind(kind)}

	if inboundID.Valid {
		id := inboundID.Int64
		ref.InboundID = &id
	}

	if groupID.Valid {
		id := groupID.Int64
		ref.GroupID = &id
	}

	return ref
}
