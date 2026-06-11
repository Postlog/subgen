package render

import (
	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
)

// entityNameResolver turns a typed PolicyRef into a mihomo policy name for one
// subscriber, and a RULE-SET's provider id into the provider name. Built-in policies,
// group references and rule-providers are always resolvable (config-global); an inbound
// reference is per-client and resolves only when the subscriber actually has that proxy
// (otherwise the member/rule is dropped — the one expected, non-error miss). subID is
// kept for log context when a non-inbound ref fails to resolve (that should never happen).
type entityNameResolver struct {
	subID        string
	inboundName  map[int64]string // node_inbounds.id -> proxy name (label)
	groupName    map[int64]string // proxy_groups.id -> group name
	providerName map[int64]string // rule_providers.id -> provider name (RULE-SET payload)
}

// newEntityNameResolver indexes a subscriber's proxies (by inbound id), the operator's
// groups (id -> name) and rule-providers (id -> name).
func newEntityNameResolver(sub *entity.Subscriber, groups []mihomo.ProxyGroup, providers []mihomo.RuleProvider) entityNameResolver {
	r := entityNameResolver{
		subID:        sub.SubID,
		inboundName:  map[int64]string{},
		groupName:    map[int64]string{},
		providerName: map[int64]string{},
	}

	for _, p := range sub.Proxies {
		if p.InboundID != 0 {
			r.inboundName[p.InboundID] = p.Name
		}
	}

	for _, g := range groups {
		r.groupName[g.ID] = g.Name
	}

	for _, p := range providers {
		r.providerName[p.ID] = p.Name
	}

	return r
}

// resolve returns the mihomo policy name for a ref and whether it is available for
// this subscriber.
func (r entityNameResolver) resolve(ref mihomo.PolicyRef) (string, bool) {
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
