package render

import "github.com/postlog/subgen/internal/entity"

// proxyToMap renders one VLESS proxy as a mihomo map. Only relevant keys are set.
func proxyToMap(p entity.Proxy) map[string]any {
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

	return m
}
