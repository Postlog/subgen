//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package user_delete
package user_delete

import "context"

// deleter removes a user (the provisioning service satisfies it).
type deleter interface {
	DeleteUser(ctx context.Context, id int64) error
}
