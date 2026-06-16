package routing

import (
	"context"
	"database/sql"

	"github.com/postlog/subgen/internal/mihomo"
)

// Rules returns the config's routing rules in order, each with its typed target
// (PolicyRef). A logical rule (AND/OR/NOT) also carries its sub-condition tree, read and
// assembled from mihomo_rule_conditions.
func (r *Repository) Rules(ctx context.Context, configID int64) ([]mihomo.RoutingRule, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,position,type,value,provider_id,no_resolve,target_kind,inbound_id,target_group_id
		   FROM mihomo_routing_rules WHERE config_id=? ORDER BY position`, configID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []mihomo.RoutingRule

	ruleIdx := map[int64]int{} // rule id -> index in out

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
		ruleIdx[rule.ID] = len(out)
		out = append(out, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := r.attachConditions(ctx, configID, out, ruleIdx); err != nil {
		return nil, err
	}

	return out, nil
}

// attachConditions reads every sub-condition of the config's rules and assembles the
// per-rule trees into RoutingRule.Conditions. Nodes are first materialised, then linked in
// (parent, position) order so a parent's children land in order; the pointer tree is then
// frozen into the value tree the domain uses. A rule with no conditions is untouched.
func (r *Repository) attachConditions(ctx context.Context, configID int64, rules []mihomo.RoutingRule, ruleIdx map[int64]int) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,rule_id,parent_id,type,value,provider_id
		   FROM mihomo_rule_conditions
		  WHERE rule_id IN (SELECT id FROM mihomo_routing_rules WHERE config_id=?)
		  ORDER BY rule_id, parent_id, position`, configID)
	if err != nil {
		return err
	}

	defer rows.Close()

	// node mirrors a condition row, with pointer children so the tree can be linked before
	// it is frozen into RuleCondition values (appending a value copy would lose later children).
	type node struct {
		cond     mihomo.RuleCondition
		children []*node
	}

	nodes := map[int64]*node{}

	type link struct {
		id, ruleID int64
		parent     sql.Null[int64]
	}

	var links []link

	for rows.Next() {
		var (
			id, ruleID int64
			parent     sql.Null[int64]
			typ        string
			value      sql.Null[string]
			providerID sql.Null[int64]
		)

		if err := rows.Scan(&id, &ruleID, &parent, &typ, &value, &providerID); err != nil {
			return err
		}

		cond := mihomo.RuleCondition{Type: mihomo.RuleType(typ)}

		if value.Valid {
			v := value.V
			cond.Value = &v
		}

		if providerID.Valid {
			pid := providerID.V
			cond.ProviderID = &pid
		}

		nodes[id] = &node{cond: cond}
		links = append(links, link{id: id, ruleID: ruleID, parent: parent})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	roots := map[int64][]*node{} // rule id -> root conditions (in position order)

	for _, l := range links {
		n := nodes[l.id]
		if l.parent.Valid {
			if p, ok := nodes[l.parent.V]; ok {
				p.children = append(p.children, n)
			}

			continue
		}

		roots[l.ruleID] = append(roots[l.ruleID], n)
	}

	var freeze func(n *node) mihomo.RuleCondition

	freeze = func(n *node) mihomo.RuleCondition {
		c := n.cond
		for _, ch := range n.children {
			c.Conditions = append(c.Conditions, freeze(ch))
		}

		return c
	}

	for ruleID, rs := range roots {
		idx, ok := ruleIdx[ruleID]
		if !ok {
			continue
		}

		for _, root := range rs {
			rules[idx].Conditions = append(rules[idx].Conditions, freeze(root))
		}
	}

	return nil
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
