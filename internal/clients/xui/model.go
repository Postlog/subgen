package xui

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/postlog/subgen/internal/entity"
)

// inbound is one row from /panel/api/inbounds/list (wire format). Settings and
// StreamSettings arrive as JSON-encoded *strings* and must be decoded again.
// It is an internal detail of this package — callers get entity.PanelInbound.
type inbound struct {
	ID       int    `json:"id"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Remark   string `json:"remark"`
	Enable   bool   `json:"enable"`
	// settings/streamSettings are JSON-encoded *strings* on 3x-ui < 3.x and plain
	// nested objects on 3.x+. RawMessage lets decode() handle both forms.
	Settings       json.RawMessage `json:"settings"`
	StreamSettings json.RawMessage `json:"streamSettings"`
	ClientStats    []ClientStat    `json:"clientStats"`

	// Decoded lazily by decode().
	clients []SettingsClient
	stream  *streamSettings
}

// ClientStat carries per-client traffic + identity (uuid/subId) and expiry.
type ClientStat struct {
	Email      string `json:"email"`
	UUID       string `json:"uuid"`
	SubID      string `json:"subId"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
	Total      int64  `json:"total"`
	ExpiryTime int64  `json:"expiryTime"` // ms epoch, 0 = no expiry
	Enable     bool   `json:"enable"`
}

// SettingsClient is an entry in settings.clients (carries flow, which clientStats lacks).
type SettingsClient struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Flow       string `json:"flow"`
	SubID      string `json:"subId"`
	TotalGB    int64  `json:"totalGB"`
	ExpiryTime int64  `json:"expiryTime"`
	Enable     bool   `json:"enable"`
}

type inboundSettings struct {
	Clients []SettingsClient `json:"clients"`
}

// streamSettings is the decoded streamSettings string.
type streamSettings struct {
	Network  string           `json:"network"`
	Security string           `json:"security"` // reality | tls | none
	Reality  *RealitySettings `json:"realitySettings"`
	TLS      *TLSSettings     `json:"tlsSettings"`
	WS       *WSSettings      `json:"wsSettings"`
	GRPC     *GRPCSettings    `json:"grpcSettings"`
}

// RealitySettings — note the client-facing publicKey/fingerprint are nested
// one level deeper under "settings".
type RealitySettings struct {
	ServerNames []string `json:"serverNames"`
	ShortIds    []string `json:"shortIds"`
	Settings    struct {
		PublicKey   string `json:"publicKey"`
		Fingerprint string `json:"fingerprint"`
		SpiderX     string `json:"spiderX"`
	} `json:"settings"`
}

// TLSSettings is the decoded tlsSettings.
type TLSSettings struct {
	ServerName string   `json:"serverName"`
	ALPN       []string `json:"alpn"`
}

// WSSettings is the decoded wsSettings.
type WSSettings struct {
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
}

// GRPCSettings is the decoded grpcSettings.
type GRPCSettings struct {
	ServiceName string `json:"serviceName"`
}

// decode parses settings/streamSettings, accepting either a JSON-encoded string
// (3x-ui < 3.x) or a plain object (3.x+). Safe to call multiple times.
func (in *inbound) decode() error {
	if in.stream == nil {
		var s streamSettings
		if err := unwrapJSON(in.StreamSettings, &s); err != nil {
			return err
		}

		in.stream = &s
	}

	if in.clients == nil {
		var set inboundSettings
		if err := unwrapJSON(in.Settings, &set); err != nil {
			return err
		}

		in.clients = set.Clients
	}

	return nil
}

// unwrapJSON unmarshals raw into v. If raw is a JSON-encoded string (older
// 3x-ui), it is first unquoted, then its contents are parsed.
func unwrapJSON(raw json.RawMessage, v any) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" || string(raw) == `""` {
		return nil
	}

	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}

		if strings.TrimSpace(s) == "" {
			return nil
		}

		return json.Unmarshal([]byte(s), v)
	}

	return json.Unmarshal(raw, v)
}

// Stream returns the decoded stream settings (call decode first).
func (in *inbound) Stream() *streamSettings { return in.stream }

// Clients returns the decoded client list (call decode first).
func (in *inbound) Clients() []SettingsClient { return in.clients }

// toPanelInbound converts a decoded wire inbound to the domain type.
func toPanelInbound(in inbound) entity.PanelInbound {
	pi := entity.PanelInbound{ID: in.ID, Port: in.Port, Remark: in.Remark, Enable: in.Enable}
	if st := in.Stream(); st != nil {
		pi.Stream = toStreamInfo(st)
	}

	for _, sc := range in.Clients() {
		pc := entity.PanelClient{Email: sc.Email, Flow: sc.Flow, SubID: sc.SubID}
		pc.UUID, _ = uuid.Parse(sc.ID) // keep email/flow even if the id is non-canonical (uuid.Nil)
		pi.Clients = append(pi.Clients, pc)
	}

	for _, cs := range in.ClientStats {
		id, err := uuid.Parse(cs.UUID)
		if err != nil {
			continue // a stat without a valid credential can't become a usable proxy
		}

		pi.Stats = append(pi.Stats, entity.PanelClientStat{
			Email: cs.Email, UUID: id, SubID: cs.SubID,
			Up: cs.Up, Down: cs.Down, Total: cs.Total, Expiry: cs.ExpiryTime, Enable: cs.Enable,
		})
	}

	return pi
}

func toStreamInfo(st *streamSettings) entity.StreamInfo {
	si := entity.StreamInfo{Network: st.Network, Security: st.Security}

	switch st.Security {
	case "reality":
		if r := st.Reality; r != nil {
			si.PublicKey = r.Settings.PublicKey
			si.Fingerprint = r.Settings.Fingerprint

			if len(r.ShortIds) > 0 {
				si.ShortID = r.ShortIds[0]
			}

			if len(r.ServerNames) > 0 {
				si.ServerName = r.ServerNames[0]
			}
		}
	case "tls":
		if t := st.TLS; t != nil {
			si.SNI = t.ServerName
			si.ALPN = t.ALPN
		}
	}

	switch st.Network {
	case "ws":
		if w := st.WS; w != nil {
			si.WSPath = w.Path
			si.WSHost = w.Headers["Host"]
		}
	case "grpc":
		if g := st.GRPC; g != nil {
			si.GRPCService = g.ServiceName
		}
	}

	return si
}
