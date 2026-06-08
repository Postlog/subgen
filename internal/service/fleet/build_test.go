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
