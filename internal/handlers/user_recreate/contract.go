//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package user_recreate
package user_recreate

import "context"

// recreator re-provisions a user's panel clients (the provisioning service satisfies it).
type recreator interface {
	RecreateUser(ctx context.Context, id int64) error
}
