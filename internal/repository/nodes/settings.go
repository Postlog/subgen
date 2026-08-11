package nodes

import (
	"encoding/json"

	"github.com/postlog/subgen/internal/entity"
)

// inboundKindSettings normalises an inbound for storage: the `kind` column (defaulting to
// vless) and the JSON `settings` blob (the hysteria2 creds; empty for vless).
func inboundKindSettings(in entity.Inbound) (kind, settings string) {
	kind = in.Kind
	if kind == "" {
		kind = entity.InboundKindVLESS
	}

	if kind == entity.InboundKindHysteria2 && in.Hysteria2 != nil {
		b, _ := json.Marshal(in.Hysteria2)
		settings = string(b)
	}

	return kind, settings
}

// applyKindSettings decodes a stored (kind, settings) pair onto an inbound.
func applyKindSettings(in *entity.Inbound, kind, settings string) {
	in.Kind = kind

	if kind == entity.InboundKindHysteria2 && settings != "" {
		var h entity.Hysteria2Settings
		if json.Unmarshal([]byte(settings), &h) == nil {
			in.Hysteria2 = &h
		}
	}
}
