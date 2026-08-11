package fleet

import (
	"github.com/postlog/subgen/internal/entity"
)

// panelSnapshot is one node's configured inbounds plus what its panel returned.
type panelSnapshot struct {
	node     entity.Node
	inbounds []entity.PanelInbound
}

// buildFleet turns panel snapshots into the normalized fleet. For each configured
// node inbound it matches the panel inbound by port and emits one Proxy per
// enabled client, grouped under that client's subId. The proxy name is the inbound
// label "<node name>-<inbound name>" (unique across the fleet).
func buildFleet(snaps []panelSnapshot) *entity.Fleet {
	fleet := &entity.Fleet{
		Subs:             map[string]*entity.Subscriber{},
		ClientsByInbound: map[int64]map[string]bool{},
	}

	for _, s := range snaps {
		for _, cfg := range s.node.Inbounds {
			pi := findByPort(s.inbounds, cfg.Port)

			// Record raw settings.clients presence for every inbound of this (reachable)
			// node — even disabled / not-on-panel ones — so health matches the prior
			// live check (which ignored Enable and keyed on email). A present-but-empty
			// set still marks the inbound as observed.
			set := fleet.ClientsByInbound[cfg.ID]
			if set == nil {
				set = map[string]bool{}
				fleet.ClientsByInbound[cfg.ID] = set
			}

			if pi != nil {
				for _, c := range pi.Clients {
					set[c.Email] = true
				}
			}

			if pi == nil || !pi.Enable {
				continue
			}

			base := streamToProxy(s.node.InboundLabel(cfg), s.node.VPNHost, pi.Port, pi.Stream)
			base.InboundID = cfg.ID

			// flow lives on the client, not the stream — index it by email.
			flowByEmail := map[string]string{}
			for _, c := range pi.Clients {
				flowByEmail[c.Email] = c.Flow
			}

			for _, cs := range pi.Stats {
				if !cs.Enable || cs.SubID == "" {
					continue
				}

				// per-client copy of the VLESS params — uuid/flow differ per client.
				v := *base.VLESS
				v.UUID = cs.UUID
				v.Flow = flowByEmail[cs.Email]
				p := entity.Proxy{Name: base.Name, InboundID: base.InboundID, VLESS: &v}

				sub := fleet.Subs[cs.SubID]
				if sub == nil {
					sub = &entity.Subscriber{SubID: cs.SubID}
					fleet.Subs[cs.SubID] = sub
				}

				sub.AddEmail(cs.Email)
				sub.Proxies = append(sub.Proxies, p)
				sub.Up += cs.Up
				sub.Down += cs.Down

				if cs.Total > sub.Total {
					sub.Total = cs.Total
				}

				if cs.Expiry > sub.Expiry {
					sub.Expiry = cs.Expiry
				}
			}
		}
	}

	return fleet
}

// addHysteria2Inbounds adds proxies for hysteria2 inbounds. Access is granted exactly like
// VLESS — via user_connections (the admin decides who gets which inbound); the ONLY
// difference is the source of the proxy PARAMS: 3x-ui has no hysteria2, so instead of live
// panel client stats these come from the inbound's stored creds, and the subscribers from
// user_connections directly (subsByInbound) rather than from panel clients. One proxy per
// (subscriber, hysteria2 inbound).
func addHysteria2Inbounds(fleet *entity.Fleet, nodes []entity.Node, subsByInbound map[int64][]string) {
	for _, n := range nodes {
		for _, in := range n.Inbounds {
			if !in.IsHysteria2() || in.Hysteria2 == nil {
				continue
			}

			base := hysteria2Proxy(n, in)

			for _, subID := range subsByInbound[in.ID] {
				sub := fleet.Subs[subID]
				if sub == nil {
					sub = &entity.Subscriber{SubID: subID}
					fleet.Subs[subID] = sub
				}

				h := *base.Hysteria2 // copy the params per subscriber
				sub.Proxies = append(sub.Proxies, entity.Proxy{Name: base.Name, InboundID: base.InboundID, Hysteria2: &h})
			}
		}
	}
}

// hysteria2Proxy renders a static hysteria2 inbound as a plain mihomo hysteria2 node
// (Design A — identity is server-side, so no UUID). Server = the node's VPNHost, port =
// the inbound port, SNI defaults to VPNHost.
func hysteria2Proxy(n entity.Node, in entity.Inbound) entity.Proxy {
	h := in.Hysteria2

	sni := h.SNI
	if sni == "" {
		sni = n.VPNHost
	}

	return entity.Proxy{
		Name:      n.InboundLabel(in),
		InboundID: in.ID,
		Hysteria2: &entity.Hysteria2Proxy{
			Server: n.VPNHost, Port: in.Port,
			Password: h.Password, Obfs: h.Obfs, ObfsPassword: h.ObfsPassword,
			SNI: sni, Up: h.Up, Down: h.Down,
		},
	}
}

func findByPort(inbounds []entity.PanelInbound, port int) *entity.PanelInbound {
	for i := range inbounds {
		if inbounds[i].Port == port {
			return &inbounds[i]
		}
	}

	return nil
}

// streamToProxy maps an inbound's decoded stream info onto a VLESS proxy template.
func streamToProxy(name, host string, port int, st entity.StreamInfo) entity.Proxy {
	return entity.Proxy{
		Name: name,
		VLESS: &entity.VLESSProxy{
			Server: host, Port: port,
			Network: st.Network, Security: st.Security,
			PublicKey: st.PublicKey, ShortID: st.ShortID, ServerName: st.ServerName, Fingerprint: st.Fingerprint,
			SNI: st.SNI, ALPN: st.ALPN,
			WSPath: st.WSPath, WSHost: st.WSHost, GRPCService: st.GRPCService,
		},
	}
}
