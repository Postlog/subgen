package users

import (
	"context"
	"strings"

	"github.com/postlog/subgen/internal/entity"
)

// ListPage returns one page of users (with their connections), filtered by name
// substring and/or inbound membership and ordered by name, plus the total count
// matching the filter. Filtering/paging runs in SQL so the store never materialises
// every user.
func (r *Repository) ListPage(ctx context.Context, p entity.UserListParams) (entity.UserPage, error) {
	where, args := userFilter(p)

	var page entity.UserPage

	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`+where, args...).Scan(&page.Total); err != nil {
		return entity.UserPage{}, err
	}

	//nolint:gosec // G202: only constant fragments and "?" placeholders are concatenated; values are bound via args.
	q := `SELECT id,name,sub_id,created_at FROM users` + where + ` ORDER BY name LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, q, append(args, p.Limit, p.Offset)...)
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

// userFilter builds the shared WHERE clause (and its args) for the count and page
// queries from the list params. An empty filter yields an empty clause.
func userFilter(p entity.UserListParams) (string, []any) {
	var (
		conds []string
		args  []any
	)

	if q := strings.TrimSpace(p.NameQuery); q != "" {
		// Names are stored lowercase; lower the query and escape LIKE metacharacters so a
		// literal _ or % in the query isn't treated as a wildcard.
		conds = append(conds, `name LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(strings.ToLower(q))+"%")
	}

	if len(p.InboundIDs) > 0 {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(p.InboundIDs)), ",")
		conds = append(conds, `id IN (SELECT user_id FROM user_connections WHERE inbound_id IN (`+ph+`))`)

		for _, id := range p.InboundIDs {
			args = append(args, id)
		}
	}

	if len(conds) == 0 {
		return "", nil
	}

	return " WHERE " + strings.Join(conds, " AND "), args
}

// escapeLike escapes the LIKE wildcards (and the escape char) so the value matches
// literally under ESCAPE '\'.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
