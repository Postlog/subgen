//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package configs
package configs

import (
	"context"
	"database/sql"
)

// cloner copies one engine's content from a source config to a destination config
// within the caller's transaction (the engine-specific half of CreateUserConfig).
// For mihomo this is routing.Repository.CloneConfig.
type cloner interface {
	CloneConfig(ctx context.Context, tx *sql.Tx, srcConfigID, dstConfigID int64) error
}
