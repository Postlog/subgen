package render

import "github.com/postlog/subgen/internal/entity"

// proxyToMap renders one proxy as a mihomo map. Exactly one protocol variant is set on the
// proxy — the renderer switches on it.
func proxyToMap(p entity.Proxy) map[string]any {
	if p.Hysteria2 != nil {
		return hysteria2ToMap(p.Name, p.Hysteria2)
	}

	return vlessToMap(p.Name, p.VLESS)
}

// vlessToMap renders a VLESS proxy. Only relevant keys are set.
func vlessToMap(name string, v *entity.VLESSProxy) map[string]any {
	m := map[string]any{
		"name":   name,
		"type":   "vless",
		"server": v.Server,
		"port":   v.Port,
		"uuid":   v.UUID.String(),
		"udp":    true,
	}

	network := v.Network
	if network == "" {
		network = "tcp"
	}

	m["network"] = network
	if v.Flow != "" {
		m["flow"] = v.Flow
	}

	switch v.Security {
	case "reality":
		m["tls"] = true
		m["servername"] = v.ServerName

		if v.Fingerprint != "" {
			m["client-fingerprint"] = v.Fingerprint
		}

		ro := map[string]any{"public-key": v.PublicKey}
		if v.ShortID != "" {
			ro["short-id"] = v.ShortID
		}

		m["reality-opts"] = ro
	case "tls":
		m["tls"] = true

		sni := v.SNI
		if sni == "" {
			sni = v.Server // panel left serverName blank -> SNI is the dialed host
		}

		m["servername"] = sni
		if len(v.ALPN) > 0 {
			m["alpn"] = v.ALPN
		}
	}

	switch network {
	case "ws":
		ws := map[string]any{}
		if v.WSPath != "" {
			ws["path"] = v.WSPath
		}

		if v.WSHost != "" {
			ws["headers"] = map[string]any{"Host": v.WSHost}
		}

		m["ws-opts"] = ws
	case "grpc":
		if v.GRPCService != "" {
			m["grpc-opts"] = map[string]any{"grpc-service-name": v.GRPCService}
		}
	}

	return m
}

// hysteria2ToMap renders a plain hysteria2 node (China dual-stack, Design A). No UUID and no
// dialer-proxy — identity is server-side (daemon port -> Xray inbound tag). SNI is already
// resolved (defaults to the node host). See docs/hysteria2.md.
func hysteria2ToMap(name string, h *entity.Hysteria2Proxy) map[string]any {
	m := map[string]any{
		"name":     name,
		"type":     "hysteria2",
		"server":   h.Server,
		"port":     h.Port,
		"password": h.Password,
	}

	if h.Obfs != "" {
		m["obfs"] = h.Obfs
		if h.ObfsPassword != "" {
			m["obfs-password"] = h.ObfsPassword
		}
	}

	if h.SNI != "" {
		m["sni"] = h.SNI
	}

	if h.Up != "" {
		m["up"] = h.Up
	}

	if h.Down != "" {
		m["down"] = h.Down
	}

	return m
}
