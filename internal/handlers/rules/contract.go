//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package rules
package rules

// rulesetMirror serves mirrored rule-provider files from memory.
type rulesetMirror interface {
	Get(file string) ([]byte, string, bool)
}
