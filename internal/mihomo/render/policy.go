package render

import (
	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
)

// resolver turns a typed PolicyRef into a mihomo policy name for one subscriber.
// Built-in policies and group references are always resolvable; an inbound reference
// is per-client and resolves only when the subscriber actually has that proxy
// (otherwise the member/rule is dropped).
type resolver struct {
	inboundName map[int64]string // node_inbounds.id -> proxy name (label)
	groupName   map[int64]string // proxy_groups.id -> group name
}

// newResolver indexes a subscriber's proxies (by inbound id) and the operator's
// groups (id -> name).
func newResolver(sub *entity.Subscriber, groups []mihomo.ProxyGroup) resolver {
	r := resolver{inboundName: map[int64]string{}, groupName: map[int64]string{}}

	for _, p := range sub.Proxies {
		if p.InboundID != 0 {
			r.inboundName[p.InboundID] = p.Name
		}
	}

	for _, g := range groups {
		r.groupName[g.ID] = g.Name
	}

	return r
}

// resolve returns the mihomo policy name for a ref and whether it is available for
// this subscriber.
func (r resolver) resolve(ref mihomo.PolicyRef) (string, bool) {
	switch ref.Kind {
	case mihomo.PolicyDirect:
		return "DIRECT", true
	case mihomo.PolicyReject:
		return "REJECT", true
	case mihomo.PolicyRejectDrop:
		return "REJECT-DROP", true
	case mihomo.PolicyRejectNoDrop:
		return "REJECT-NO-DROP", true
	case mihomo.PolicyPass:
		return "PASS", true
	case mihomo.PolicyInbound:
		if ref.InboundID == nil {
			return "", false
		}

		name, ok := r.inboundName[*ref.InboundID]

		return name, ok
	case mihomo.PolicyGroup:
		if ref.GroupID == nil {
			return "", false
		}

		name, ok := r.groupName[*ref.GroupID]

		return name, ok
	default:
		return "", false
	}
}
