// Package nodes is the service that owns node create/update/delete: it validates the
// node (returning entity.ErrValidation* sentinels) and persists via the nodes repository.
// A removed/deleted inbound that is still referenced is refused by the database FK
// (RESTRICT) and surfaces from the repository as entity.ErrInboundReferenced — there is no
// pre-check here. Conflicts (name/inbound already used on the node) surface as entity
// sentinels from the repo. Handlers stay thin — they map these to user-facing responses.
package nodes

import (
	"context"
	"fmt"

	"github.com/postlog/subgen/internal/entity"
)

// Service owns node operations over the nodes repository.
type Service struct {
	nodes nodeRepo
}

// New builds the service.
func New(nodes nodeRepo) *Service {
	return &Service{nodes: nodes}
}

// Save validates a node and persists it: create when n.ID == 0, else update. Returns the
// node id. A name/inbound clash surfaces as entity.ErrNodeNameTaken / ErrInboundDuplicate;
// dropping an inbound still referenced surfaces as entity.ErrInboundReferenced (the FK).
func (s *Service) Save(ctx context.Context, n entity.Node) (int64, error) {
	if err := validateNode(&n); err != nil {
		return 0, err
	}

	if n.ID <= 0 {
		id, err := s.nodes.Create(ctx, n)
		if err != nil {
			return 0, fmt.Errorf("nodes.Create: %w", err)
		}

		return id, nil
	}

	if err := s.nodes.Update(ctx, n.ID, n, n.Token != ""); err != nil {
		return 0, fmt.Errorf("nodes.Update: %w", err)
	}

	return n.ID, nil
}

// Delete removes a node. A node whose inbound is still referenced is refused by the FK and
// surfaces as entity.ErrInboundReferenced.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.nodes.Delete(ctx, id); err != nil {
		return fmt.Errorf("nodes.Delete: %w", err)
	}

	return nil
}
