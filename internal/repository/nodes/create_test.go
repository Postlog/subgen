//go:build integration

package nodes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
)

func TestRepository_Create(t *testing.T) {
	tt := []struct {
		name string

		// seedFirst, when set, creates a node before the case runs (to force a
		// uniqueness collision with it).
		seedFirst *entity.Node
		in        entity.Node

		err     error
		wantErr bool // for the inline "token is required" (no sentinel)
	}{
		{
			name: "success.with_inbounds",
			in: entity.Node{
				Name: "RU1", VPNHost: "ru1.example",
				PanelBaseURL: "https://ru1:2053", PanelBasePath: "/", Token: "tok",
				Inbounds: []entity.Inbound{{Name: "smart", Port: 4433}, {Name: "force", Port: 8443}},
			},
		},
		{
			name: "success.blank_inbounds_skipped",
			in: entity.Node{
				Name: "RU1", VPNHost: "ru1.example",
				PanelBaseURL: "https://ru1:2053", PanelBasePath: "/", Token: "tok",
				// name=="" or port==0 rows are dropped by insertInbounds.
				Inbounds: []entity.Inbound{{Name: "smart", Port: 4433}, {Name: "", Port: 0}, {Name: "blank", Port: 0}},
			},
		},
		{
			name:    "error.empty_token",
			in:      entity.Node{Name: "RU1", VPNHost: "ru1.example"},
			wantErr: true,
		},
		{
			name:      "error.duplicate_name",
			seedFirst: &entity.Node{Name: "RU1", VPNHost: "a", Token: "t"},
			in:        entity.Node{Name: "RU1", VPNHost: "b", Token: "t"},
			err:       entity.ErrNodeNameTaken,
		},
		{
			name: "error.duplicate_inbound_name",
			in: entity.Node{
				Name: "RU1", VPNHost: "a", Token: "t",
				Inbounds: []entity.Inbound{{Name: "dup", Port: 4433}, {Name: "dup", Port: 8443}},
			},
			err: entity.ErrInboundDuplicate,
		},
		{
			name: "error.duplicate_inbound_port",
			in: entity.Node{
				Name: "RU1", VPNHost: "a", Token: "t",
				Inbounds: []entity.Inbound{{Name: "a", Port: 4433}, {Name: "b", Port: 4433}},
			},
			err: entity.ErrInboundDuplicate,
		},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := nodes.New(dbtest.OpenDB(t))

			if tc.seedFirst != nil {
				_, err := repo.Create(t.Context(), *tc.seedFirst)
				require.NoError(t, err)
			}

			id, err := repo.Create(t.Context(), tc.in)

			if tc.wantErr {
				require.ErrorContains(t, err, "token is required")
				return
			}

			require.ErrorIs(t, err, tc.err)

			if tc.err != nil {
				return
			}

			// Success: the node round-trips and only the non-blank inbounds persisted,
			// each with an assigned id.
			require.NotZero(t, id)

			got, err := repo.Get(t.Context(), id)
			require.NoError(t, err)
			assert.Equal(t, tc.in.Name, got.Name)
			assert.Equal(t, tc.in.VPNHost, got.VPNHost)

			var wantInbounds []entity.Inbound
			for _, in := range tc.in.Inbounds {
				if in.Name != "" && in.Port != 0 {
					wantInbounds = append(wantInbounds, in)
				}
			}
			assert.Len(t, got.Inbounds, len(wantInbounds))
			for _, in := range got.Inbounds {
				assert.NotZero(t, in.ID)
			}
		})
	}
}
