//go:build apitest

package nodes_test

import (
	"github.com/postlog/subgen/internal/handlers/node_delete"
	"github.com/postlog/subgen/internal/handlers/node_save"
)

// User-facing messages the node endpoints return. Black-box tests assert the exact text
// subgen produces — but rather than re-stating it here, these alias the exported handler
// constants (node_save / node_delete), so the text lives in exactly one place. Node/inbound
// validation lives in the nodes service (entity.ErrValidation* sentinels, ADR-0003), mapped
// to these constants in the handlers.
const (
	// Duplicate node name (entity.ErrNodeNameTaken → handler const).
	msgNodeNameTaken = node_save.MsgNodeNameTaken

	// Field-validation messages (one per entity.ErrValidation* sentinel).
	msgHost        = node_save.MsgHost
	msgNoInbounds  = node_save.MsgNoInbounds
	msgInboundName = node_save.MsgInboundName
	msgNodeName    = node_save.MsgNodeName

	// In-payload duplicate inbound (validateNode catches these before the DB; the per-node
	// UNIQUE(node_id,name|port) sentinel is unreachable via a single save because validation
	// rejects the payload first).
	msgInboundNameUq = node_save.MsgInboundNameUq
	msgInboundPortUq = node_save.MsgInboundPortUq

	// A node whose inbound is still referenced can't be deleted (FK → entity.ErrInboundReferenced).
	msgDeleteReferenced = node_delete.MsgInboundReferenced

	// Deleting an id that isn't there (entity.ErrNodeNotFound → handler).
	msgNodeNotFound = node_delete.MsgNotFound
)
