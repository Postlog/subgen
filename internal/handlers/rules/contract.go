//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package rules
package rules

// ruleFiles serves mirrored rule-provider files from memory.
type ruleFiles interface {
	Get(file string) ([]byte, string, bool)
}
