package render

import "github.com/postlog/subgen/internal/entity"

// proxyToMap renders one proxy as a mihomo map. Only relevant keys are set. VLESS is the
// default; a hysteria2 proxy (the China dual-stack outer transport) is rendered separately.
func proxyToMap(p entity.Proxy) map[string]any {
	if p.Protocol == "hysteria2" {
		return hysteria2ToMap(p)
	}

	m := map[string]any{
		"name":   p.Name,
		"type":   "vless",
		"server": p.Server,
		"port":   p.Port,
		"uuid":   p.UUID.String(),
		"udp":    true,
	}

	network := p.Network
	if network == "" {
		network = "tcp"
	}

	m["network"] = network
	if p.Flow != "" {
		m["flow"] = p.Flow
	}

	switch p.Security {
	case "reality":
		m["tls"] = true
		m["servername"] = p.ServerName

		if p.Fingerprint != "" {
			m["client-fingerprint"] = p.Fingerprint
		}

		ro := map[string]any{"public-key": p.PublicKey}
		if p.ShortID != "" {
			ro["short-id"] = p.ShortID
		}

		m["reality-opts"] = ro
	case "tls":
		m["tls"] = true

		sni := p.SNI
		if sni == "" {
			sni = p.Server // panel left serverName blank -> SNI is the dialed host
		}

		m["servername"] = sni
		if len(p.ALPN) > 0 {
			m["alpn"] = p.ALPN
		}
	}

	switch network {
	case "ws":
		ws := map[string]any{}
		if p.WSPath != "" {
			ws["path"] = p.WSPath
		}

		if p.WSHost != "" {
			ws["headers"] = map[string]any{"Host": p.WSHost}
		}

		m["ws-opts"] = ws
	case "grpc":
		if p.GRPCService != "" {
			m["grpc-opts"] = map[string]any{"grpc-service-name": p.GRPCService}
		}
	}

	// Chain through another proxy/group (the inner hop rides the outer hysteria2).
	if p.DialerProxy != "" {
		m["dialer-proxy"] = p.DialerProxy
	}

	return m
}

// hysteria2ToMap renders the Hysteria2 outer transport (China dual-stack). Identity is
// NOT here — it rides the inner VLESS hop; this is only the QUIC crossing (Salamander
// obfs + Brutal up/down hints). See docs/hysteria2.md.
func hysteria2ToMap(p entity.Proxy) map[string]any {
	m := map[string]any{
		"name":     p.Name,
		"type":     "hysteria2",
		"server":   p.Server,
		"port":     p.Port,
		"password": p.Password,
	}

	if p.Obfs != "" {
		m["obfs"] = p.Obfs
		if p.ObfsPassword != "" {
			m["obfs-password"] = p.ObfsPassword
		}
	}

	sni := p.SNI
	if sni == "" {
		sni = p.ServerName
	}

	if sni != "" {
		m["sni"] = sni
	}

	if p.Up != "" {
		m["up"] = p.Up
	}

	if p.Down != "" {
		m["down"] = p.Down
	}

	// An outer transport normally isn't itself chained, but support it for symmetry.
	if p.DialerProxy != "" {
		m["dialer-proxy"] = p.DialerProxy
	}

	return m
}
