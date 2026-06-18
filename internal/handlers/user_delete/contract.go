//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package user_delete
package user_delete

import "context"

// provisioningService removes a user (the provisioning service satisfies it).
type provisioningService interface {
	DeleteUser(ctx context.Context, id int64) error
}
