// Package web holds the shared HTTP plumbing for the admin/sub handlers: user-facing
// message mapping (lower layers return technical/sentinel errors only), the admin
// session/auth middleware, and the static HTML renderer (embedded shell/login pages +
// static assets). Each action lives in its own internal/handlers/<action> package; the
// JSON request/response shapes are owned by the generated ogen layer (internal/oas), not
// this package.
package web

import (
	"fmt"
	"strings"

	"github.com/postlog/subgen/internal/entity"
)

// InboundsBlockedMessage renders the user-facing message for the nodes service's
// structured entity.InboundsBlockedError: an inbound can't be removed (or its node
// deleted) while still referenced by user connections or mihomo rules/group members.
// Human text lives at the presentation layer; the service only carries the counts.
func InboundsBlockedMessage(e entity.InboundsBlockedError) string {
	var parts []string

	for _, in := range e.Inbounds {
		var reasons []string

		if in.Users > 0 {
			reasons = append(reasons, fmt.Sprintf("%d польз.", in.Users))
		}

		if in.Refs > 0 {
			reasons = append(reasons, fmt.Sprintf("%d правил/групп", in.Refs))
		}

		if len(reasons) > 0 {
			parts = append(parts, in.Label+" — "+strings.Join(reasons, ", "))
		}
	}

	return "сначала отвяжите от инбаундов: " + strings.Join(parts, "; ")
}
