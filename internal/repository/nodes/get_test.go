//go:build integration

package nodes_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
)

func TestRepository_Get(t *testing.T) {
	tt := []struct {
		name string

		seed    bool // create the seeded node and Get it; else Get a missing id
		wantErr error
	}{
		{name: "success.with_inbounds", seed: true},
		{name: "error.not_found", seed: false, wantErr: sql.ErrNoRows},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := nodes.New(dbtest.OpenDB(t))

			var id int64 = 4242 // a non-existent id for the not-found case
			if tc.seed {
				id = dbtest.SeedNode(t, repo).NodeID
			}

			got, err := repo.Get(t.Context(), id)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "RU1", got.Name)
			assert.Equal(t, "ru1.example", got.VPNHost)
			assert.Equal(t, "tok-ru1", got.Token)
			// Inbounds come back ordered by name ("force" < "smart").
			require.Len(t, got.Inbounds, 2)
			assert.Equal(t, "force", got.Inbounds[0].Name)
			assert.Equal(t, "smart", got.Inbounds[1].Name)
		})
	}
}
