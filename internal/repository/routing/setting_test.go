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

// Setting reads a config's free-form setting (base_yaml today), scoped by config id.
// The value is written through SaveMihomoConfig (the single writer); a missing key
// returns "" (sql.ErrNoRows swallowed).
func TestRepository_Setting(t *testing.T) {
	tt := []struct {
		name string

		set  bool // write base_yaml first (via SaveMihomoConfig)
		key  string
		want string
	}{
		{
			name: "missing.returns_empty",
			set:  false,
			key:  "base_yaml",
			want: "", // sql.ErrNoRows is swallowed into ""
		},
		{
			name: "present.returns_value",
			set:  true,
			key:  "base_yaml",
			want: "port: 7890",
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
				require.NoError(t, repo.SaveMihomoConfig(t.Context(), cfg, nil, nil, nil, tc.want, mihomo.Profile{}))
			}

			got, err := repo.Setting(t.Context(), cfg, tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
