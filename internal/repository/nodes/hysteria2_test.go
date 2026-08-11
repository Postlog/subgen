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

// A hysteria2 inbound's kind + creds round-trip through Create → Get / List, and an
// Update edits the creds in place keeping the id stable; a plain VLESS inbound stays
// kind=vless with no Hysteria2 settings.
func TestNodes_Hysteria2Inbound_RoundTrip(t *testing.T) {
	t.Parallel()

	repo := nodes.New(dbtest.OpenDB(t))

	hy := &entity.Hysteria2Settings{
		Password: "pw", Obfs: "salamander", ObfsPassword: "obfs",
		SNI: "ru1.example", Up: "50 Mbps", Down: "100 Mbps",
	}

	id, err := repo.Create(t.Context(), entity.Node{
		Name: "RU1", VPNHost: "ru1.example", PanelBaseURL: "https://ru1.example:2053", PanelBasePath: "/", Token: "tok",
		Inbounds: []entity.Inbound{
			{Name: "smart", Port: 12466}, // vless (default kind)
			{Name: "hy2-postlog", Port: 443, Kind: entity.InboundKindHysteria2, Hysteria2: hy},
		},
	})
	require.NoError(t, err)

	got, err := repo.Get(t.Context(), id)
	require.NoError(t, err)

	byName := map[string]entity.Inbound{}
	for _, in := range got.Inbounds {
		byName[in.Name] = in
	}

	// vless inbound: kind defaults, no hysteria2 settings.
	assert.Equal(t, entity.InboundKindVLESS, byName["smart"].Kind)
	assert.False(t, byName["smart"].IsHysteria2())
	assert.Nil(t, byName["smart"].Hysteria2)

	// hysteria2 inbound: kind + all creds round-trip.
	h := byName["hy2-postlog"]
	assert.True(t, h.IsHysteria2())
	require.NotNil(t, h.Hysteria2)
	assert.Equal(t, *hy, *h.Hysteria2)

	// List hydrates it too.
	list, err := repo.List(t.Context())
	require.NoError(t, err)
	require.Len(t, list, 1)

	var listed *entity.Inbound
	for i := range list[0].Inbounds {
		if list[0].Inbounds[i].Name == "hy2-postlog" {
			listed = &list[0].Inbounds[i]
		}
	}
	require.NotNil(t, listed)
	require.NotNil(t, listed.Hysteria2)
	assert.Equal(t, "pw", listed.Hysteria2.Password)

	// Update edits the password in place; the id stays stable (connections reference it).
	updated := *h.Hysteria2
	updated.Password = "pw2"
	require.NoError(t, repo.Update(t.Context(), id, entity.Node{
		Name: "RU1", VPNHost: "ru1.example", PanelBaseURL: "https://ru1.example:2053", PanelBasePath: "/",
		Inbounds: []entity.Inbound{
			{ID: byName["smart"].ID, Name: "smart", Port: 12466},
			{ID: h.ID, Name: "hy2-postlog", Port: 443, Kind: entity.InboundKindHysteria2, Hysteria2: &updated},
		},
	}, false))

	got2, err := repo.Get(t.Context(), id)
	require.NoError(t, err)

	for _, in := range got2.Inbounds {
		if in.Name == "hy2-postlog" {
			require.NotNil(t, in.Hysteria2)
			assert.Equal(t, "pw2", in.Hysteria2.Password)
			assert.Equal(t, h.ID, in.ID)
		}
	}
}
