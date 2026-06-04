package xui

import (
	"context"
	"net/url"

	"github.com/postlog/subgen/internal/entity"
)

// DelClient deletes a client by email on the target panel.
func (c *Client) DelClient(ctx context.Context, t entity.PanelTarget, email string) error {
	return c.postJSON(ctx, t, "/panel/api/clients/del/"+url.PathEscape(email), nil)
}
