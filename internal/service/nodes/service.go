// Package nodes is the service that owns node create/update/delete: it validates the
// node (returning entity.ErrValidation* sentinels), refuses to drop or delete an inbound
// still referenced (entity.InboundsBlockedError), and persists via the nodes repository.
// Conflicts (name/inbound already used on the node) surface as entity sentinels from the
// repo. Handlers stay thin — they map these to user-facing responses.
package nodes

import (
	"context"
	"fmt"

	"github.com/postlog/subgen/internal/entity"
)

// Service owns node operations over the nodes + routing repositories.
type Service struct {
	nodes   nodeRepo
	routing routingRepo
}

// New builds the service.
func New(nodes nodeRepo, routing routingRepo) *Service {
	return &Service{nodes: nodes, routing: routing}
}

// Save validates a node and persists it: create when n.ID == 0, else update. On update it
// refuses to drop an inbound still referenced (entity.InboundsBlockedError). Returns the
// node id. A name/inbound clash surfaces as entity.ErrNodeNameTaken / ErrInboundDuplicate.
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

	if err := s.blockRemovedInbounds(ctx, n); err != nil {
		return 0, err
	}

	if err := s.nodes.Update(ctx, n.ID, n, n.Token != ""); err != nil {
		return 0, fmt.Errorf("nodes.Update: %w", err)
	}

	return n.ID, nil
}

// Delete removes a node, refusing if any of its inbounds is still referenced.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.blocked(ctx, id, nil); err != nil {
		return err
	}

	if err := s.nodes.Delete(ctx, id); err != nil {
		return fmt.Errorf("nodes.Delete: %w", err)
	}

	return nil
}

// blockRemovedInbounds blocks an update that drops an inbound (a current id absent from
// the submission) still referenced. If the node can't be read, the check is skipped — the
// Update below surfaces any real failure.
func (s *Service) blockRemovedInbounds(ctx context.Context, n entity.Node) error {
	cur, err := s.nodes.Get(ctx, n.ID)
	if err != nil {
		return nil //nolint:nilerr // can't pre-check; Update will surface real errors
	}

	kept := map[int64]bool{}

	for _, in := range n.Inbounds {
		if in.ID > 0 {
			kept[in.ID] = true
		}
	}

	var removed []int64

	for _, in := range cur.Inbounds {
		if !kept[in.ID] {
			removed = append(removed, in.ID)
		}
	}

	if len(removed) == 0 {
		return nil
	}

	return s.blocked(ctx, n.ID, removed)
}

// blocked returns entity.InboundsBlockedError if any of the given node-inbound ids (all of
// the node's inbounds when inboundIDs == nil) is still referenced by a user connection or
// a mihomo rule / proxy-group member; otherwise nil. It gathers the per-inbound counts —
// the handler layer formats the message.
func (s *Service) blocked(ctx context.Context, nodeID int64, inboundIDs []int64) error {
	n, err := s.nodes.Get(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("nodes.Get: %w", err)
	}

	label := map[int64]string{}
	for _, in := range n.Inbounds {
		label[in.ID] = fmt.Sprintf("%s:%d", n.InboundLabel(in), in.Port)
	}

	ids := inboundIDs
	if ids == nil {
		for _, in := range n.Inbounds {
			ids = append(ids, in.ID)
		}
	}

	users, err := s.nodes.ConnectionCountsByInbound(ctx, ids)
	if err != nil {
		return fmt.Errorf("nodes.ConnectionCountsByInbound: %w", err)
	}

	refs, err := s.routing.InboundRefCounts(ctx, ids)
	if err != nil {
		return fmt.Errorf("routing.InboundRefCounts: %w", err)
	}

	if len(users) == 0 && len(refs) == 0 {
		return nil
	}

	var blocked []entity.BlockedInbound

	for _, id := range ids {
		if u, r := users[id], refs[id]; u > 0 || r > 0 {
			blocked = append(blocked, entity.BlockedInbound{Label: label[id], Users: u, Refs: r})
		}
	}

	if len(blocked) == 0 {
		return nil
	}

	return entity.InboundsBlockedError{Inbounds: blocked}
}
