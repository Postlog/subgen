package entity

import "errors"

// Domain sentinel errors. Lower layers (repository/service/clients) return these,
// wrapping with fmt.Errorf("%w", …) only to add call-chain context. They carry no
// interpolated values — identifying data is already in the caller's context. The
// handler layer maps each sentinel to user-facing text.
var (
	ErrInvalidUserName       = errors.New("invalid name")
	ErrNameTaken             = errors.New("name already taken")
	ErrNodeNameTaken         = errors.New("node name already taken")
	ErrInboundDuplicate      = errors.New("inbound name or port already used on node")
	ErrRuleProviderNameTaken = errors.New("rule-provider name already taken")
	ErrNoConnectionSelected  = errors.New("no connection selected")
	ErrNodeNotFound          = errors.New("node not found")
	ErrInboundNotFound       = errors.New("inbound not found")
)

// PanelClientExistsError means a client with the user's nickname (email) already
// exists on a panel subgen would provision to, for a panel the user does not yet
// own. subgen never deletes it — the operator must resolve it on that panel. It
// carries the node so the handler layer can name it. Compare with errors.As.
type PanelClientExistsError struct{ Node string }

func (e PanelClientExistsError) Error() string {
	return "a client with this email already exists on panel " + e.Node
}
