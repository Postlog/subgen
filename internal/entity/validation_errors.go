package entity

import "errors"

// Node validation sentinels — node/inbound input validation moved out of the handler into
// the nodes service (value validation lives in the service layer — see AGENTS.md). The service returns
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

	// Inbound kind + hysteria2 creds.
	ErrValidationInboundKind     = errors.New("invalid inbound kind")
	ErrValidationInboundSettings = errors.New("inbound settings do not match its kind")
	ErrValidationHysteria2Pass   = errors.New("hysteria2 inbound requires a password")
	ErrValidationHysteria2Obfs   = errors.New("hysteria2 obfs must be salamander with an obfs password")
	ErrValidationHysteria2Band   = errors.New("hysteria2 up/down must be a bandwidth")
	ErrValidationHysteria2SNI    = errors.New("hysteria2 sni must be a valid host")
)
