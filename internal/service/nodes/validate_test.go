package nodes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
)

func TestValidateNode(t *testing.T) {
	base := func() entity.Node {
		return entity.Node{
			Name: "🇷🇺 RU1", VPNHost: "1.2.3.4", PanelBaseURL: "https://1.2.3.4:2096", PanelBasePath: "/p/",
			Inbounds: []entity.Inbound{{Name: "force", Port: 8443, Kind: entity.InboundKindVLESS}},
		}
	}

	// hy2 builds a valid hysteria2 inbound; per-test mutations tweak one field.
	hy2 := func(f func(*entity.Hysteria2Settings)) entity.Inbound {
		s := &entity.Hysteria2Settings{Password: "pw", Obfs: "salamander", ObfsPassword: "ob", SNI: "ru1.example", Up: "50 Mbps", Down: "100 Mbps"}
		if f != nil {
			f(s)
		}

		return entity.Inbound{Name: "hy2", Port: 443, Kind: entity.InboundKindHysteria2, Hysteria2: s}
	}

	tt := []struct {
		name string

		mutate func(*entity.Node)

		err         error  // nil => success; else the expected sentinel (ErrorIs)
		wantBasePat string // expected normalised PanelBasePath on success
	}{
		{
			name:        "success.base_path_normalised",
			mutate:      func(n *entity.Node) { n.PanelBasePath = "secret" },
			wantBasePat: "/secret",
		},
		{
			name: "success.multiple_inbounds",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{
					{Name: "force", Port: 8443, Kind: entity.InboundKindVLESS},
					{Name: "alt", Port: 39129, Kind: entity.InboundKindVLESS},
					{Name: "smart", Port: 4433, Kind: entity.InboundKindVLESS},
				}
			},
			wantBasePat: "/p/",
		},
		{
			name:        "success.hysteria2_full",
			mutate:      func(n *entity.Node) { n.Inbounds = []entity.Inbound{hy2(nil)} },
			wantBasePat: "/p/",
		},
		{
			name: "success.hysteria2_minimal",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{hy2(func(s *entity.Hysteria2Settings) {
					s.Obfs, s.ObfsPassword, s.SNI, s.Up, s.Down = "", "", "", "", ""
				})}
			},
			wantBasePat: "/p/",
		},
		{
			name: "success.hysteria2_bandwidth_bare_number",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{hy2(func(s *entity.Hysteria2Settings) { s.Up, s.Down = "50", "100" })}
			},
			wantBasePat: "/p/",
		},
		{
			name: "success.vless_with_nil_settings",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{{Name: "force", Port: 8443, Kind: entity.InboundKindVLESS}}
			},
			wantBasePat: "/p/",
		},
		{name: "error.no_name", mutate: func(n *entity.Node) { n.Name = "" }, err: entity.ErrValidationNodeName},
		{name: "error.bad_node_name", mutate: func(n *entity.Node) { n.Name = "RU.1" }, err: entity.ErrValidationNodeName},
		{name: "error.bad_host", mutate: func(n *entity.Node) { n.VPNHost = "https://x" }, err: entity.ErrValidationHost},
		{name: "error.bad_url", mutate: func(n *entity.Node) { n.PanelBaseURL = "1.2.3.4:2096" }, err: entity.ErrValidationPanelURL},
		{name: "error.no_path", mutate: func(n *entity.Node) { n.PanelBasePath = "" }, err: entity.ErrValidationBasePath},
		{name: "error.no_inbound", mutate: func(n *entity.Node) { n.Inbounds = nil }, err: entity.ErrValidationNoInbounds},
		{name: "error.empty_inbound_name", mutate: func(n *entity.Node) { n.Inbounds = []entity.Inbound{{Name: "", Port: 1}} }, err: entity.ErrValidationInboundName},
		{name: "error.bad_inbound_name", mutate: func(n *entity.Node) { n.Inbounds = []entity.Inbound{{Name: "no pe", Port: 1}} }, err: entity.ErrValidationInboundName},
		{name: "error.port_out_of_range", mutate: func(n *entity.Node) { n.Inbounds = []entity.Inbound{{Name: "force", Port: 70000}} }, err: entity.ErrValidationInboundPort},
		{
			name: "error.duplicate_name",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{
					{Name: "force", Port: 8443, Kind: entity.InboundKindVLESS},
					{Name: "force", Port: 9000, Kind: entity.InboundKindVLESS},
				}
			},
			err: entity.ErrValidationInboundNameUq,
		},
		{
			name: "error.duplicate_port",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{
					{Name: "force", Port: 8443, Kind: entity.InboundKindVLESS},
					{Name: "smart", Port: 8443, Kind: entity.InboundKindVLESS},
				}
			},
			err: entity.ErrValidationInboundPortUq,
		},

		// Inbound kind + hysteria2 settings.
		{
			name:   "error.unknown_kind",
			mutate: func(n *entity.Node) { n.Inbounds = []entity.Inbound{{Name: "force", Port: 8443, Kind: "trojan"}} },
			err:    entity.ErrValidationInboundKind,
		},
		{
			name:   "error.empty_kind",
			mutate: func(n *entity.Node) { n.Inbounds = []entity.Inbound{{Name: "force", Port: 8443, Kind: ""}} },
			err:    entity.ErrValidationInboundKind,
		},
		{
			name: "error.vless_with_hysteria2_settings",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{{Name: "force", Port: 8443, Kind: entity.InboundKindVLESS, Hysteria2: &entity.Hysteria2Settings{Password: "pw"}}}
			},
			err: entity.ErrValidationInboundSettings,
		},
		{
			name: "error.hysteria2_nil_settings",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{{Name: "hy2", Port: 443, Kind: entity.InboundKindHysteria2}}
			},
			err: entity.ErrValidationInboundSettings,
		},
		{
			name: "error.hysteria2_no_password",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{hy2(func(s *entity.Hysteria2Settings) { s.Password = "  " })}
			},
			err: entity.ErrValidationHysteria2Pass,
		},
		{
			name: "error.hysteria2_bad_obfs",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{hy2(func(s *entity.Hysteria2Settings) { s.Obfs = "xor" })}
			},
			err: entity.ErrValidationHysteria2Obfs,
		},
		{
			name: "error.hysteria2_obfs_without_password",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{hy2(func(s *entity.Hysteria2Settings) { s.ObfsPassword = "" })}
			},
			err: entity.ErrValidationHysteria2Obfs,
		},
		{
			name: "error.hysteria2_obfs_password_without_obfs",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{hy2(func(s *entity.Hysteria2Settings) { s.Obfs = "" })}
			},
			err: entity.ErrValidationHysteria2Obfs,
		},
		{
			name: "error.hysteria2_bad_bandwidth",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{hy2(func(s *entity.Hysteria2Settings) { s.Up = "fast" })}
			},
			err: entity.ErrValidationHysteria2Band,
		},
		{
			name: "error.hysteria2_bad_sni",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{hy2(func(s *entity.Hysteria2Settings) { s.SNI = "https://x" })}
			},
			err: entity.ErrValidationHysteria2SNI,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			n := base()
			tc.mutate(&n)

			err := validateNode(&n)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantBasePat, n.PanelBasePath)
		})
	}
}

func TestIsHostOrIP(t *testing.T) {
	tt := []struct {
		name string
		in   string
		want bool
	}{
		{name: "ipv4", in: "1.2.3.4", want: true},
		{name: "domain", in: "ru1.example.com", want: true},
		{name: "single_label", in: "a", want: true},
		{name: "hyphen_label", in: "vpn-1.node.local", want: true},
		{name: "ipv6_loopback", in: "::1", want: true},
		{name: "ipv6", in: "2001:db8::1", want: true},
		{name: "empty", in: "", want: false},
		{name: "with_scheme", in: "https://x", want: false},
		{name: "with_port", in: "host:8080", want: false},
		{name: "with_space", in: "host name", want: false},
		{name: "leading_hyphen", in: "-bad.com", want: false},
		{name: "trailing_hyphen", in: "bad-.com", want: false},
		{name: "empty_label", in: "a..b", want: false},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isHostOrIP(tc.in))
		})
	}
}

func TestValidPanelURL(t *testing.T) {
	tt := []struct {
		name string
		in   string
		want bool
	}{
		{name: "host_port", in: "https://1.2.3.4:2096", want: true},
		{name: "host_only", in: "https://panel.example.com", want: true},
		{name: "http_scheme", in: "http://10.0.0.1:8080", want: true},
		{name: "no_scheme", in: "1.2.3.4:2096", want: false},
		{name: "wrong_scheme", in: "ftp://1.2.3.4", want: false},
		{name: "has_path", in: "https://1.2.3.4:2096/secret/", want: false},
		{name: "no_host", in: "https://", want: false},
		{name: "bad_port", in: "https://1.2.3.4:99999", want: false},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, validPanelURL(tc.in))
		})
	}
}

func TestIsBandwidth(t *testing.T) {
	tt := []struct {
		name string
		in   string
		want bool
	}{
		{name: "empty", in: "", want: true},
		{name: "bare_number", in: "50", want: true},
		{name: "mbps_space", in: "50 Mbps", want: true},
		{name: "mbps_nospace", in: "100mbps", want: true},
		{name: "gbps_upper", in: "1 GBPS", want: true},
		{name: "bps", in: "500 bps", want: true},
		{name: "kbps", in: "256kbps", want: true},
		{name: "tbps", in: "2 tbps", want: true},
		{name: "word", in: "fast", want: false},
		{name: "no_number", in: "Mbps", want: false},
		{name: "bad_unit", in: "50 gb", want: false},
		{name: "trailing_junk", in: "50 Mbps!", want: false},
		{name: "negative", in: "-5 Mbps", want: false},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isBandwidth(tc.in))
		})
	}
}

func TestIsInboundName(t *testing.T) {
	tt := []struct {
		name string
		in   string
		want bool
	}{
		{name: "alnum", in: "force", want: true},
		{name: "with_hyphen", in: "force-tcp", want: true},
		{name: "digits", in: "in8443", want: true},
		{name: "empty", in: "", want: false},
		{name: "with_space", in: "no pe", want: false},
		{name: "with_underscore", in: "force_tcp", want: false},
		{name: "with_dot", in: "force.tcp", want: false},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isInboundName(tc.in))
		})
	}
}
