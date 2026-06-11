//go:build apitest

package users_test

import (
	"github.com/postlog/subgen/internal/handlers/user_create"
	"github.com/postlog/subgen/internal/handlers/user_delete"
	"github.com/postlog/subgen/internal/handlers/user_edit"
	"github.com/postlog/subgen/internal/handlers/user_recreate"
)

// User-facing messages the user endpoints return. Black-box tests assert the exact text
// subgen produces — but rather than re-stating it here, these alias the exported handler
// constants, so the text lives in exactly one place (the handler). An absent inbound-id
// list is still rejected as a generic 400 by the kept `required` (a null array), so the
// no-connection cases assert api.MsgBadRequest, not a constant here.
const (
	// Validation (handler ← entity sentinels).
	msgInvalidUserName = user_create.MsgInvalidName
	msgNameTaken       = user_create.MsgNameTaken
	msgInboundNotFound = user_create.MsgInboundNotFound

	// Per-handler success messages.
	msgCreated   = user_create.MsgCreated
	msgUpdated   = user_edit.MsgUpdated
	msgDeleted   = user_delete.MsgDeleted
	msgRecreated = user_recreate.MsgRecreated
)
