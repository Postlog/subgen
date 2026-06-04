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
	fleet := &entity.Fleet{Subs: map[string]*entity.Subscriber{}}

	for _, s := range snaps {
		for _, cfg := range s.node.Inbounds {
			pi := findByPort(s.inbounds, cfg.Port)
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

				p := base // copy
				p.UUID = cs.UUID
				p.Flow = flowByEmail[cs.Email]

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

func findByPort(inbounds []entity.PanelInbound, port int) *entity.PanelInbound {
	for i := range inbounds {
		if inbounds[i].Port == port {
			return &inbounds[i]
		}
	}

	return nil
}

// streamToProxy maps an inbound's decoded stream info onto a proxy template.
func streamToProxy(name, host string, port int, st entity.StreamInfo) entity.Proxy {
	return entity.Proxy{
		Name: name, Server: host, Port: port,
		Network: st.Network, Security: st.Security,
		PublicKey: st.PublicKey, ShortID: st.ShortID, ServerName: st.ServerName, Fingerprint: st.Fingerprint,
		SNI: st.SNI, ALPN: st.ALPN,
		WSPath: st.WSPath, WSHost: st.WSHost, GRPCService: st.GRPCService,
	}
}
