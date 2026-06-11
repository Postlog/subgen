//go:build integration

package routing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/routing"
)

// Profile reads a config's subscription-profile row (title, filename, update interval),
// scoped by config id. The row is written through SaveMihomoConfig (the single writer);
// a config with no row yet returns a zero Profile (sql.ErrNoRows swallowed) so the
// caller can substitute defaults.
func TestRepository_Profile(t *testing.T) {
	tt := []struct {
		name string

		set  bool // write a profile first (via SaveMihomoConfig)
		want mihomo.Profile
	}{
		{
			name: "missing.returns_zero",
			set:  false,
			want: mihomo.Profile{}, // sql.ErrNoRows swallowed into the zero value
		},
		{
			name: "present.returns_values",
			set:  true,
			want: mihomo.Profile{Title: "My VPN", Filename: "vpn.yaml", UpdateInterval: 6},
		},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := dbtest.OpenDB(t)
			repo := routing.New(db)
			cfg := dbtest.SeedConfig(t, db)

			if tc.set {
				require.NoError(t, repo.SaveMihomoConfig(t.Context(), cfg, dbtest.Draft(nil, nil, nil, "", tc.want)))
			}

			got, err := repo.Profile(t.Context(), cfg)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
