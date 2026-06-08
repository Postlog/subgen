package users

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/postlog/subgen/internal/entity"
)

// ListPage returns one page of users (with their connections), filtered by name
// substring and/or inbound membership and ordered by name, plus the total count
// matching the filter. Each filter combination is its own complete, fixed query —
// nothing is string-built: the name is a bound value wrapped DB-side ('%'||?||'%'),
// and the inbound set is a JSON array expanded by json_each. So the only thing that
// ever varies is which constant query (and which bound args) we pick.
func (r *Repository) ListPage(ctx context.Context, p entity.UserListParams) (entity.UserPage, error) {
	name := strings.TrimSpace(p.NameQuery)
	hasName := name != ""
	hasInbound := len(p.InboundIDs) > 0

	var (
		countQuery string
		pageQuery  string
		filterArgs []any
	)

	switch {
	case hasName && hasInbound:
		countQuery = `SELECT COUNT(*) FROM users WHERE name LIKE '%' || ? || '%' ESCAPE '\' AND id IN (SELECT user_id FROM user_connections WHERE inbound_id IN (SELECT value FROM json_each(?)))`
		pageQuery = `SELECT id,name,sub_id,created_at FROM users WHERE name LIKE '%' || ? || '%' ESCAPE '\' AND id IN (SELECT user_id FROM user_connections WHERE inbound_id IN (SELECT value FROM json_each(?))) ORDER BY name LIMIT ? OFFSET ?`
		filterArgs = []any{escapeLike(name), idsJSON(p.InboundIDs)}
	case hasName:
		countQuery = `SELECT COUNT(*) FROM users WHERE name LIKE '%' || ? || '%' ESCAPE '\'`
		pageQuery = `SELECT id,name,sub_id,created_at FROM users WHERE name LIKE '%' || ? || '%' ESCAPE '\' ORDER BY name LIMIT ? OFFSET ?`
		filterArgs = []any{escapeLike(name)}
	case hasInbound:
		countQuery = `SELECT COUNT(*) FROM users WHERE id IN (SELECT user_id FROM user_connections WHERE inbound_id IN (SELECT value FROM json_each(?)))`
		pageQuery = `SELECT id,name,sub_id,created_at FROM users WHERE id IN (SELECT user_id FROM user_connections WHERE inbound_id IN (SELECT value FROM json_each(?))) ORDER BY name LIMIT ? OFFSET ?`
		filterArgs = []any{idsJSON(p.InboundIDs)}
	default:
		countQuery = `SELECT COUNT(*) FROM users`
		pageQuery = `SELECT id,name,sub_id,created_at FROM users ORDER BY name LIMIT ? OFFSET ?`
		filterArgs = nil
	}

	var page entity.UserPage

	if err := r.db.QueryRowContext(ctx, countQuery, filterArgs...).Scan(&page.Total); err != nil {
		return entity.UserPage{}, err
	}

	rows, err := r.db.QueryContext(ctx, pageQuery, append(filterArgs, p.Limit, p.Offset)...)
	if err != nil {
		return entity.UserPage{}, err
	}

	defer rows.Close()

	var (
		out []entity.User
		ids []int64
	)

	for rows.Next() {
		var u entity.User
		if err := rows.Scan(&u.ID, &u.Name, &u.SubID, &u.CreatedAt); err != nil {
			return entity.UserPage{}, err
		}

		out = append(out, u)
		ids = append(ids, u.ID)
	}

	if err := rows.Err(); err != nil {
		return entity.UserPage{}, err
	}

	byID := make(map[int64]*entity.User, len(out))
	for i := range out {
		byID[out[i].ID] = &out[i]
	}

	if err := r.loadConnectionsForIDs(ctx, byID, ids); err != nil {
		return entity.UserPage{}, err
	}

	page.Users = out

	return page, nil
}

// escapeLike escapes the LIKE wildcards (and the escape char) in the search value so a
// literal _ or % matches literally under ESCAPE '\'. It transforms the bound value
// only — the query text stays fixed. Names are stored lowercase, so it lowercases too.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(s))
}

// idsJSON encodes an id set as a JSON array, bound as one placeholder and expanded
// DB-side by json_each — so an IN list needs no string-built placeholders.
func idsJSON(ids []int64) string {
	b, _ := json.Marshal(ids)
	return string(b)
}
