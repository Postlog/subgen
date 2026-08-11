package fleet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
)

// TestBuildFleet_ClientsByInbound pins the health presence index: a key exists for
// every inbound of a reachable node (with the raw settings.clients emails, ignoring
// Enable and even when the inbound isn't on the panel), and is absent for unobserved
// inbounds — which ClientMissing then reads to decide the badge.
func TestBuildFleet_ClientsByInbound(t *testing.T) {
	t.Parallel()

	node := entity.Node{
		Name: "RU1", VPNHost: "ru1.example",
		Inbounds: []entity.Inbound{
			{ID: 1, Name: "smart", Port: 4433}, // enabled, has clients
			{ID: 2, Name: "force", Port: 8443}, // not on the panel (pi == nil)
			{ID: 3, Name: "off", Port: 9000},   // disabled on the panel, still recorded
		},
	}

	snaps := []panelSnapshot{{
		node: node,
		inbounds: []entity.PanelInbound{
			{
				Port: 4433, Enable: true,
				Clients: []entity.PanelClient{{Email: "amy"}, {Email: "zoe"}},
				Stats:   []entity.PanelClientStat{{Email: "amy", SubID: "s-amy", Enable: true}},
			},
			{
				Port: 9000, Enable: false,
				Clients: []entity.PanelClient{{Email: "ben"}},
			},
		},
	}}

	f := buildFleet(snaps)

	// Presence index: enabled-with-clients, on-panel-absent (empty), disabled (still recorded).
	require.Equal(t, map[string]bool{"amy": true, "zoe": true}, f.ClientsByInbound[1])
	require.Equal(t, map[string]bool{}, f.ClientsByInbound[2])
	require.Equal(t, map[string]bool{"ben": true}, f.ClientsByInbound[3])

	// ClientMissing reads the index: present, absent-but-observed, observed-empty,
	// disabled-but-present, and an entirely unobserved inbound.
	assert.False(t, f.ClientMissing(1, "amy"))
	assert.True(t, f.ClientMissing(1, "ghost"))
	assert.True(t, f.ClientMissing(2, "amy")) // key present but empty → missing
	assert.False(t, f.ClientMissing(3, "ben"))
	assert.False(t, f.ClientMissing(99, "amy")) // no key (unobserved node) → not missing

	// Subscribers still build from clientStats of enabled inbounds only.
	require.NotNil(t, f.Sub("s-amy"))
	assert.Len(t, f.Sub("s-amy").Proxies, 1)
}

// TestAddHysteria2Inbounds renders a hysteria2 inbound from its stored creds for exactly
// the subscribers connected to it (via user_connections), defaulting SNI to the node host,
// and leaves vless inbounds alone (those are built from panel client stats).
func TestAddHysteria2Inbounds(t *testing.T) {
	t.Parallel()

	nodes := []entity.Node{{
		Name: "🇷🇺 RU1", VPNHost: "ru1.example",
		Inbounds: []entity.Inbound{
			{ID: 1, Name: "smart", Port: 12466}, // vless — NOT rendered here
			{ID: 2, Name: "hy2-postlog", Port: 443, Kind: entity.InboundKindHysteria2, Hysteria2: &entity.Hysteria2Settings{
				Password: "pw", Obfs: "salamander", ObfsPassword: "ob", Up: "50 Mbps", Down: "100 Mbps",
			}},
		},
	}}

	// Subscribers per inbound id (from user_connections): s1 is on both, s2 only on the vless.
	subs := map[int64][]string{2: {"s1"}, 1: {"s1", "s2"}}

	f := &entity.Fleet{Subs: map[string]*entity.Subscriber{}}
	addHysteria2Inbounds(f, nodes, subs)

	// Only s1 (on the hy2 inbound) gets a proxy here; the vless inbound is not rendered by
	// this path, so s2 (vless-only) gets nothing and s1 gets exactly the one hy2 proxy.
	require.NotNil(t, f.Sub("s1"))
	require.Len(t, f.Sub("s1").Proxies, 1)
	assert.Nil(t, f.Sub("s2"))

	p := f.Sub("s1").Proxies[0]
	assert.Equal(t, entity.InboundKindHysteria2, p.Protocol)
	assert.Equal(t, "🇷🇺 RU1-hy2-postlog", p.Name)
	assert.Equal(t, "ru1.example", p.Server)
	assert.Equal(t, 443, p.Port)
	assert.Equal(t, int64(2), p.InboundID)
	assert.Equal(t, "pw", p.Password)
	assert.Equal(t, "salamander", p.Obfs)
	assert.Equal(t, "ob", p.ObfsPassword)
	assert.Equal(t, "ru1.example", p.SNI) // defaulted to VPNHost
	assert.Equal(t, "50 Mbps", p.Up)
	assert.Equal(t, "100 Mbps", p.Down)
}
