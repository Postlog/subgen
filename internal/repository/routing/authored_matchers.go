package routing

import (
	"context"
	"database/sql"
	"strings"

	"github.com/postlog/subgen/internal/mihomo"
)

// attachAuthoredMatchers loads the matcher trees of the authored providers in provs (one
// query over their ids) and sets RuleProvider.Matchers in place. External providers are
// left untouched. A matcher is recursive: a logical matcher (AND/OR/NOT) carries its
// sub-matchers in Children via parent_id; a matcher has no target (it is the content of a
// classical rule-provider, not a routing decision).
func (r *Repository) attachAuthoredMatchers(ctx context.Context, provs []mihomo.RuleProvider) error {
	ids := make([]any, 0, len(provs))
	idx := map[int64]int{} // provider id -> index in provs

	for i := range provs {
		if provs[i].Source == mihomo.RuleProviderAuthored {
			ids = append(ids, provs[i].ID)
			idx[provs[i].ID] = i
		}
	}

	if len(ids) == 0 {
		return nil
	}

	placeholders := strings.Repeat("?,", len(ids)-1) + "?"

	// placeholders is a constant run of "?" marks (one per id) and every id is a bound
	// parameter, so the concatenation carries no user input.
	query := `SELECT id,provider_id,parent_id,position,type,value FROM mihomo_authored_matchers WHERE provider_id IN (` + placeholders + `) ORDER BY provider_id, parent_id, position` //nolint:gosec // placeholders constant; ids parameterized

	rows, err := r.db.QueryContext(ctx, query, ids...)
	if err != nil {
		return err
	}

	defer rows.Close()

	// node mirrors a matcher row with pointer children, so the tree can be linked before it
	// is frozen into RoutingRule values.
	type node struct {
		rule     mihomo.RoutingRule
		children []*node
	}

	type link struct {
		id       int64
		provider int64
		parent   sql.Null[int64]
	}

	nodes := map[int64]*node{}

	var links []link

	for rows.Next() {
		var (
			m      mihomo.RoutingRule
			provID int64
			parent sql.Null[int64]
			value  sql.Null[string]
		)

		if err := rows.Scan(&m.ID, &provID, &parent, &m.Position, &m.Type, &value); err != nil {
			return err
		}

		if value.Valid {
			v := value.V
			m.Value = &v
		}

		nodes[m.ID] = &node{rule: m}
		links = append(links, link{id: m.ID, provider: provID, parent: parent})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Link in query order (ORDER BY provider_id, parent_id, position): a parent's children
	// land in position order; roots (parent NULL) are bucketed per provider.
	roots := map[int64][]*node{}

	for _, l := range links {
		n := nodes[l.id]
		if l.parent.Valid {
			if p, ok := nodes[l.parent.V]; ok {
				p.children = append(p.children, n)
			}

			continue
		}

		roots[l.provider] = append(roots[l.provider], n)
	}

	var freeze func(n *node) mihomo.RoutingRule

	freeze = func(n *node) mihomo.RoutingRule {
		out := n.rule
		for _, ch := range n.children {
			out.Children = append(out.Children, freeze(ch))
		}

		return out
	}

	for provID, rs := range roots {
		matchers := make([]mihomo.RoutingRule, 0, len(rs))
		for _, root := range rs {
			matchers = append(matchers, freeze(root))
		}

		provs[idx[provID]].Matchers = matchers
	}

	return nil
}
