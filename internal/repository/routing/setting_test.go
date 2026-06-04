//go:build integration

package routing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/routing"
)

func TestRepository_Setting(t *testing.T) {
	tt := []struct {
		name string

		set  bool // write the key first
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
			repo := routing.New(dbtest.OpenDB(t))

			if tc.set {
				require.NoError(t, repo.SetSetting(t.Context(), tc.key, tc.want))
			}

			got, err := repo.Setting(t.Context(), tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
