//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package provider_check
package provider_check

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// rulesetChecker probes a rule-provider URL (reachable / present / right format).
type rulesetChecker interface {
	Check(ctx context.Context, url, format string) entity.RulesetCheckResult
}
