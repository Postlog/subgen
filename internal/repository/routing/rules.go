package routing

import (
	"context"
	"database/sql"

	"github.com/postlog/subgen/internal/mihomo"
)

// Rules returns the config's routing rules in order. A rule is recursive: a logical rule
// (AND/OR/NOT) carries its sub-rules in Children, read from the same table via parent_id.
// A top-level rule has a typed target (PolicyRef); a sub-rule has none (Target nil).
func (r *Repository) Rules(ctx context.Context, configID int64) ([]mihomo.RoutingRule, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,parent_id,position,type,value,provider_id,no_resolve,target_kind,inbound_id,target_group_id
		   FROM mihomo_routing_rules WHERE config_id=? ORDER BY parent_id, position`, configID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	// node mirrors a rule row with pointer children, so the tree can be linked before it
	// is frozen into RoutingRule values (appending a value copy would lose later children).
	type node struct {
		rule     mihomo.RoutingRule
		children []*node
	}

	nodes := map[int64]*node{}

	type link struct {
		id     int64
		parent sql.Null[int64]
	}

	var links []link

	for rows.Next() {
		var (
			rule       mihomo.RoutingRule
			parent     sql.Null[int64]
			value      sql.Null[string]
			noResolve  int
			kind       sql.Null[string]
			providerID sql.Null[int64]
			inboundID  sql.Null[int64]
			groupID    sql.Null[int64]
		)

		if err := rows.Scan(&rule.ID, &parent, &rule.Position, &rule.Type, &value,
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

		// A top-level rule carries a target (target_kind set); a sub-rule does not.
		if kind.Valid {
			ref := policyRef(kind.V, inboundID, groupID)
			rule.Target = &ref
		}

		nodes[rule.ID] = &node{rule: rule}
		links = append(links, link{id: rule.ID, parent: parent})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Link in query order (ORDER BY parent_id, position): top-level rules (parent NULL,
	// sorted first) and each parent's children land in position order.
	var roots []*node

	for _, l := range links {
		n := nodes[l.id]
		if l.parent.Valid {
			if p, ok := nodes[l.parent.V]; ok {
				p.children = append(p.children, n)
			}

			continue
		}

		roots = append(roots, n)
	}

	var freeze func(n *node) mihomo.RoutingRule

	freeze = func(n *node) mihomo.RoutingRule {
		out := n.rule
		for _, ch := range n.children {
			out.Children = append(out.Children, freeze(ch))
		}

		return out
	}

	out := make([]mihomo.RoutingRule, 0, len(roots))
	for _, root := range roots {
		out = append(out, freeze(root))
	}

	return out, nil
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
