package entity

import "errors"

// Node validation sentinels — node/inbound input validation moved out of the handler into
// the nodes service (see docs/decisions/0003-validation-in-code.md). The service returns
// these; the handler maps each to a user-facing 400 message (constant by handler). They
// carry no interpolated values — the offending field is known at the call site.
var (
	ErrValidationNodeName      = errors.New("invalid node name")
	ErrValidationHost          = errors.New("invalid host")
	ErrValidationPanelURL      = errors.New("invalid panel base url")
	ErrValidationBasePath      = errors.New("panel base path required")
	ErrValidationNoInbounds    = errors.New("no inbounds")
	ErrValidationInboundName   = errors.New("invalid inbound name")
	ErrValidationInboundPort   = errors.New("invalid inbound port")
	ErrValidationInboundNameUq = errors.New("duplicate inbound name")
	ErrValidationInboundPortUq = errors.New("duplicate inbound port")
)
