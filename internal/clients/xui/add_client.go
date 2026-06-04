package xui

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// AddClient creates one client (a single uuid/email/subId) bound to all the given
// inbounds in one call — 3x-ui's native multi-inbound client. This is what makes
// del-by-email and edits unambiguous: one identity, many inbound bindings.
func (c *Client) AddClient(ctx context.Context, t entity.PanelTarget, inboundIDs []int, cs entity.ClientSpec) error {
	body := map[string]any{
		"client": map[string]any{
			"id":         cs.ID,
			"email":      cs.Email,
			"flow":       cs.Flow,
			"enable":     true,
			"subId":      cs.SubID,
			"totalGB":    0,
			"expiryTime": 0,
			"limitIp":    0,
			"tgId":       0,
			"reset":      0,
			"comment":    "client managed by SubGen",
		},
		"inboundIds": inboundIDs,
	}

	return c.postJSON(ctx, t, "/panel/api/clients/add", body)
}
