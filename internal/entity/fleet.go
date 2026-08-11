package entity

import "github.com/google/uuid"

// Proxy is one mihomo proxy node for a subscriber. Exactly one protocol variant is set —
// the renderer switches on it — so there is no shared "fat" struct mixing VLESS and
// hysteria2 fields.
type Proxy struct {
	// Name is the inbound label "<node>-<inbound>" — the proxy name, and the key the
	// renderer resolves an inbound PolicyRef to by id.
	Name string
	// InboundID identifies which node inbound this proxy came from.
	InboundID int64

	// Exactly one of the following is non-nil.
	VLESS     *VLESSProxy
	Hysteria2 *Hysteria2Proxy
}

// VLESSProxy is a VLESS proxy resolved from a 3x-ui inbound + one client.
type VLESSProxy struct {
	Server   string
	Port     int
	UUID     uuid.UUID
	Flow     string
	Network  string // tcp | ws | grpc
	Security string // reality | tls | none

	// REALITY
	PublicKey   string
	ShortID     string
	ServerName  string
	Fingerprint string

	// TLS
	SNI  string
	ALPN []string

	// transports
	WSPath      string
	WSHost      string
	GRPCService string
}

// Hysteria2Proxy is a plain hysteria2 proxy node (China dual-stack, Design A): identity is
// server-side (by daemon port → Xray inbound tag), so there's no UUID. Its params come from
// the inbound's stored creds, not a panel. See docs/hysteria2.md in the vpn-toolchain repo.
type Hysteria2Proxy struct {
	Server       string
	Port         int
	Password     string // plain (the server uses type:password)
	Obfs         string // e.g. "salamander"
	ObfsPassword string
	SNI          string
	Up           string // Brutal CC hint, e.g. "50 Mbps"
	Down         string
}

// Subscriber is everything one subscription token resolves to: the set of
// proxies it can use (one per inbound it appears in) plus aggregate traffic.
type Subscriber struct {
	SubID   string
	Emails  []string // client emails sharing this subId (across inbounds)
	Proxies []Proxy
	Up      int64
	Down    int64
	Total   int64 // 0 = unlimited
	Expiry  int64 // ms epoch, 0 = no expiry
}

// AddEmail records an email once.
func (s *Subscriber) AddEmail(email string) {
	if email == "" {
		return
	}

	for _, e := range s.Emails {
		if e == email {
			return
		}
	}

	s.Emails = append(s.Emails, email)
}

// Fleet indexes subscribers by subId.
type Fleet struct {
	Subs map[string]*Subscriber

	// ClientsByInbound maps a node_inbounds.id to the set of client emails present in
	// that inbound's settings.clients on the panel. A key exists for every inbound of a
	// reachable node (empty set if the inbound has no clients / isn't on the panel) and
	// is absent for inbounds of an unreachable node — so a missing key means "panel not
	// observed", distinct from "observed, client absent". Built once per fleet refresh
	// from the same single panel snapshot, so health needs no per-user panel calls.
	ClientsByInbound map[int64]map[string]bool
}

// ClientMissing reports whether the user's client (by email) is absent from the
// given inbound — the health signal behind the "no panel client" badge, derived from
// the cached fleet instead of live panel calls. A node whose panel wasn't observed
// (no key) is never claimed missing, mirroring the prior live-check behaviour.
func (f *Fleet) ClientMissing(inboundID int64, email string) bool {
	if f == nil {
		return false
	}

	set, ok := f.ClientsByInbound[inboundID]
	if !ok {
		return false
	}

	return !set[email]
}

// Sub returns the subscriber for a subId, or nil.
func (f *Fleet) Sub(subID string) *Subscriber {
	if f == nil {
		return nil
	}

	return f.Subs[subID]
}

// SubIDs returns all known subscription IDs (for HMAC token reverse-lookup).
func (f *Fleet) SubIDs() []string {
	out := make([]string, 0, len(f.Subs))
	for id := range f.Subs {
		out = append(out, id)
	}

	return out
}
