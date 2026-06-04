package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
)

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

func TestValidatePanelURL(t *testing.T) {
	tt := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "success.host_port", in: "https://1.2.3.4:2096"},
		{name: "success.host_only", in: "https://panel.example.com"},
		{name: "success.http_scheme", in: "http://10.0.0.1:8080"},
		{name: "error.no_scheme", in: "1.2.3.4:2096", wantErr: true},
		{name: "error.wrong_scheme", in: "ftp://1.2.3.4", wantErr: true},
		{name: "error.has_path", in: "https://1.2.3.4:2096/secret/", wantErr: true},
		{name: "error.no_host", in: "https://", wantErr: true},
		{name: "error.bad_port", in: "https://1.2.3.4:99999", wantErr: true},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validatePanelURL(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidateNode(t *testing.T) {
	base := func() entity.Node {
		return entity.Node{
			Name: "🇷🇺 RU1", VPNHost: "1.2.3.4", PanelBaseURL: "https://1.2.3.4:2096", PanelBasePath: "/p/",
			Inbounds: []entity.Inbound{{Name: "force", Port: 8443}},
		}
	}

	tt := []struct {
		name string

		mutate func(*entity.Node)

		wantErr     bool
		wantBasePat string // expected normalised PanelBasePath when no error
	}{
		{
			name:        "success.base_path_normalised",
			mutate:      func(n *entity.Node) { n.PanelBasePath = "secret" },
			wantBasePat: "/secret",
		},
		{
			name: "success.multiple_inbounds",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{{Name: "force", Port: 8443}, {Name: "alt", Port: 39129}, {Name: "smart", Port: 4433}}
			},
			wantBasePat: "/p/",
		},
		{name: "error.no_name", mutate: func(n *entity.Node) { n.Name = "" }, wantErr: true},
		{name: "error.bad_node_name", mutate: func(n *entity.Node) { n.Name = "RU.1" }, wantErr: true},
		{name: "error.bad_host", mutate: func(n *entity.Node) { n.VPNHost = "https://x" }, wantErr: true},
		{name: "error.bad_url", mutate: func(n *entity.Node) { n.PanelBaseURL = "1.2.3.4:2096" }, wantErr: true},
		{name: "error.no_path", mutate: func(n *entity.Node) { n.PanelBasePath = "" }, wantErr: true},
		{name: "error.no_inbound", mutate: func(n *entity.Node) { n.Inbounds = nil }, wantErr: true},
		{name: "error.empty_inbound_name", mutate: func(n *entity.Node) { n.Inbounds = []entity.Inbound{{Name: "", Port: 1}} }, wantErr: true},
		{name: "error.bad_inbound_name", mutate: func(n *entity.Node) { n.Inbounds = []entity.Inbound{{Name: "no pe", Port: 1}} }, wantErr: true},
		{name: "error.port_out_of_range", mutate: func(n *entity.Node) { n.Inbounds = []entity.Inbound{{Name: "force", Port: 70000}} }, wantErr: true},
		{
			name: "error.duplicate_name",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{{Name: "force", Port: 8443}, {Name: "force", Port: 9000}}
			},
			wantErr: true,
		},
		{
			name: "error.duplicate_port",
			mutate: func(n *entity.Node) {
				n.Inbounds = []entity.Inbound{{Name: "force", Port: 8443}, {Name: "smart", Port: 8443}}
			},
			wantErr: true,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			n := base()
			tc.mutate(&n)

			err := ValidateNode(&n)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantBasePat, n.PanelBasePath)
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
