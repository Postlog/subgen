package entity

import "github.com/google/uuid"

// Proxy is one mihomo proxy node (one 3x-ui inbound, resolved for one client).
type Proxy struct {
	Name string // the inbound label "<node>-<inbound>" — used verbatim as the proxy name
	// InboundID identifies which node inbound this proxy came from, so the renderer
	// can resolve an inbound PolicyRef to this proxy's name by id.
	InboundID int64
	Server    string
	Port      int
	UUID      uuid.UUID
	Flow      string
	Network   string // tcp | ws | grpc
	Security  string // reality | tls | none

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
