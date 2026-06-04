package users

import "context"

// SubIDs returns all known subscription ids (for token reverse-lookup).
func (r *Repository) SubIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT sub_id FROM users`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []string

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		out = append(out, id)
	}

	return out, rows.Err()
}
