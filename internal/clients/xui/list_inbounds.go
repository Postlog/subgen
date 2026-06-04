package xui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/postlog/subgen/internal/entity"
)

// ListInbounds fetches, decodes and returns all inbounds on the target panel as
// domain types.
func (c *Client) ListInbounds(ctx context.Context, t entity.PanelTarget) ([]entity.PanelInbound, error) {
	body, err := c.get(ctx, t, "/panel/api/inbounds/list")
	if err != nil {
		return nil, err
	}

	var r struct {
		Success bool      `json:"success"`
		Msg     string    `json:"msg"`
		Obj     []inbound `json:"obj"`
	}

	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("list: parse: %w", err)
	}

	if !r.Success {
		return nil, fmt.Errorf("list: %s", strings.TrimSpace(r.Msg))
	}

	out := make([]entity.PanelInbound, 0, len(r.Obj))

	for i := range r.Obj {
		if err := r.Obj[i].decode(); err != nil {
			return nil, fmt.Errorf("decode inbound %d: %w", r.Obj[i].Port, err)
		}

		out = append(out, toPanelInbound(r.Obj[i]))
	}

	return out, nil
}
