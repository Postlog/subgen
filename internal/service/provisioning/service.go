// Package provisioning is the service that owns subscribers and their 3x-ui
// clients: it creates/edits/deletes users and reconciles the panel side (one
// client per panel, email = nickname, bound to all the user's inbounds). It talks
// to panels through a stateless xui client, resolving each node's credentials from
// the node registry and passing them as the per-call target.
//
// All entity references are by numeric id: a user's selection is a set of
// node_inbounds ids (immutable), nodes are keyed by id, and edits diff inbound-id
// sets. Node/inbound names are used only for display and for the mihomo proxy
// wire-name (the inbound label "<node>-<inbound>"). The one port-based match is our
// inbound_port → the EXTERNAL 3x-ui inbound (panelLookup) — the port is how 3x-ui
// identifies its inbound, not our id.
package provisioning

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/postlog/subgen/internal/entity"
)

// Service provisions users into 3x-ui from the store.
type Service struct {
	users  usersRepo
	nodes  nodesRepo
	client panelClient

	// Random-id sources, injected so tests can make them deterministic (defaulted in
	// New): genID mints a subscriber id, genUUID a 3x-ui client uuid.
	genID   func(int) string
	genUUID func() uuid.UUID
}

// New builds the provisioning service from its dependencies.
func New(users usersRepo, nodes nodesRepo, client panelClient) *Service {
	return &Service{
		users: users, nodes: nodes, client: client,
		genID:   randID,
		genUUID: uuid.New,
	}
}

// nameRe is the allowed nickname charset (also the prefix of every client email).
var nameRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// validateName normalises and checks a user nickname.
func validateName(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !nameRe.MatchString(name) {
		return "", entity.ErrInvalidUserName
	}

	return name, nil
}

// maxDescriptionLen bounds the optional free-text description (in runes — it is UTF-8
// admin text). Over-length input is rejected with entity.ErrDescriptionTooLong.
const maxDescriptionLen = 500

// validateDescription trims the optional description, collapses a nil-or-blank value to
// nil (so "no description" has a single representation, NULL, end to end), and enforces
// the length bound. Returns the normalised value or entity.ErrDescriptionTooLong.
func validateDescription(d *string) (*string, error) {
	if d == nil {
		return nil, nil
	}

	t := strings.TrimSpace(*d)
	if t == "" {
		return nil, nil
	}

	if utf8.RuneCountInString(t) > maxDescriptionLen {
		return nil, entity.ErrDescriptionTooLong
	}

	return &t, nil
}

// connTarget is one inbound a user should be bound to.
type connTarget struct {
	NodeID    int64
	Node      string // node name — display only
	Port      int    // inbound_port — bridge to the 3x-ui inbound
	InboundID int64  // node_inbounds.id — the canonical reference
}

// inboundRef pairs an inbound with its owning node (for id → target resolution).
type inboundRef struct {
	node entity.Node
	in   entity.Inbound
}

// nodeIndex loads the registry once and indexes nodes by id and inbounds by id.
func (s *Service) nodeIndex(ctx context.Context) (byID map[int64]entity.Node, inboundByID map[int64]inboundRef, err error) {
	nodes, err := s.nodes.List(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("nodes.List: %w", err)
	}

	byID = make(map[int64]entity.Node, len(nodes))
	inboundByID = map[int64]inboundRef{}

	for _, n := range nodes {
		byID[n.ID] = n

		for _, in := range n.Inbounds {
			inboundByID[in.ID] = inboundRef{node: n, in: in}
		}
	}

	return byID, inboundByID, nil
}

// desiredTargets resolves a selection of inbound ids into connection targets. Each id
// must be a real (positive, existing) inbound; any number may be selected. Duplicate ids
// are ignored. A non-positive id (the moved-from-schema minimum:1 guard) has no match and
// so yields ErrInboundNotFound, same as any unknown id.
func (s *Service) desiredTargets(inboundByID map[int64]inboundRef, inboundIDs []int64) ([]connTarget, error) {
	var ts []connTarget

	seen := map[int64]bool{}

	for _, id := range inboundIDs {
		if seen[id] {
			continue
		}

		seen[id] = true

		ref, ok := inboundByID[id]
		if !ok {
			return nil, entity.ErrInboundNotFound
		}

		ts = append(ts, connTarget{
			NodeID: ref.node.ID, Node: ref.node.Name, Port: ref.in.Port, InboundID: ref.in.ID,
		})
	}

	return ts, nil
}

// CreateUser validates the name, mints a sub_id, stores the user with its connections
// (and the optional free-text description), and provisions one 3x-ui client per panel
// (email = name).
func (s *Service) CreateUser(ctx context.Context, p entity.UserCreateParams) (*entity.User, error) {
	name, err := validateName(p.Name)
	if err != nil {
		return nil, err
	}

	if len(p.InboundIDs) == 0 {
		return nil, entity.ErrNoConnectionSelected
	}

	desc, err := validateDescription(p.Description)
	if err != nil {
		return nil, err
	}
	// Name uniqueness is enforced by the users.name constraint: users.Create returns
	// entity.ErrNameTaken on a duplicate (no pre-check SELECT).
	byID, inboundByID, err := s.nodeIndex(ctx)
	if err != nil {
		return nil, err
	}

	targets, err := s.desiredTargets(inboundByID, p.InboundIDs)
	if err != nil {
		return nil, err
	}
	// Refuse to clobber a pre-existing client with this nickname on any target panel
	// (orphan or foreign). Abort before creating anything — the operator resolves it.
	if err := s.ensureEmailFree(ctx, byID, nodeIDs(targets), name); err != nil {
		return nil, err
	}

	u := &entity.User{Name: name, SubID: s.genID(16), Description: desc}
	for _, t := range targets {
		u.Connections = append(u.Connections, entity.Connection{InboundID: t.InboundID})
	}

	if err := s.users.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("users.Create: %w", err)
	}

	if err := s.syncPanels(ctx, byID, u, nil, targets, false); err != nil {
		return u, err // user row created; provisioning may be partial
	}

	return u, nil
}

// EditUser reconciles a user's inbound set to the new selection and updates the optional
// free-text description, then re-binds the affected panels' client to match. Identity
// (name/sub_id, and per-panel uuid where possible) is preserved.
func (s *Service) EditUser(ctx context.Context, p entity.UserEditParams) error {
	if len(p.InboundIDs) == 0 {
		return entity.ErrNoConnectionSelected
	}

	desc, err := validateDescription(p.Description)
	if err != nil {
		return err
	}

	u, err := s.users.Get(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("users.Get: %w", err)
	}

	byID, inboundByID, err := s.nodeIndex(ctx)
	if err != nil {
		return err
	}

	desired, err := s.desiredTargets(inboundByID, p.InboundIDs)
	if err != nil {
		return err
	}

	old := u.Connections
	// Guard only panels the user is being NEWLY added to — on panels it already owns
	// the client with this email is ours. Abort before touching the store/panels.
	if err := s.ensureEmailFree(ctx, byID, addedNodeIDs(old, desired), u.Name); err != nil {
		return err
	}

	inbIDs := make([]int64, 0, len(desired))
	for _, t := range desired {
		inbIDs = append(inbIDs, t.InboundID)
	}

	if err := s.users.ReplaceConnections(ctx, u.ID, inbIDs); err != nil {
		return fmt.Errorf("users.ReplaceConnections: %w", err)
	}

	if err := s.users.SetDescription(ctx, u.ID, desc); err != nil {
		return fmt.Errorf("users.SetDescription: %w", err)
	}

	return s.syncPanels(ctx, byID, u, old, desired, false)
}

// DeleteUser removes the user's client from every panel it's on (best-effort, by
// email = name), then removes the user (connections cascade in the store).
func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	u, err := s.users.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("users.Get: %w", err)
	}

	byID, _, err := s.nodeIndex(ctx)
	if err != nil {
		return err
	}

	for _, nid := range connNodeIDs(u.Connections) {
		if n, ok := byID[nid]; ok {
			_ = s.client.DelClient(ctx, target(n), u.Name) // best-effort
		}
	}

	if err := s.users.Delete(ctx, id); err != nil {
		return fmt.Errorf("users.Delete: %w", err)
	}

	return nil
}

// RecreateUser re-binds the user's client on every panel from scratch (fixes drift).
func (s *Service) RecreateUser(ctx context.Context, id int64) error {
	u, err := s.users.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("users.Get: %w", err)
	}

	byID, _, err := s.nodeIndex(ctx)
	if err != nil {
		return err
	}

	var desired []connTarget
	for _, c := range u.Connections {
		desired = append(desired, connTarget{NodeID: c.NodeID, Node: c.Node, Port: c.Port, InboundID: c.InboundID})
	}

	return s.syncPanels(ctx, byID, u, u.Connections, desired, true)
}

// syncPanels makes each panel host exactly one client (a single uuid, email =
// u.Name, subId = u.SubID) bound to all of the user's desired inbounds there —
// 3x-ui's native multi-inbound client. Everything is keyed by node id; `old` is the
// user's current connections (for diffing which panels changed and to preserve the
// uuid on re-bind); forceAll re-binds every panel even if unchanged (used by
// Recreate). It deletes a client only on panels the user already owns.
func (s *Service) syncPanels(ctx context.Context, byID map[int64]entity.Node, u *entity.User, old []entity.Connection, desired []connTarget, forceAll bool) error {
	email := u.Name

	desiredByNode := map[int64][]connTarget{}
	for _, t := range desired {
		desiredByNode[t.NodeID] = append(desiredByNode[t.NodeID], t)
	}

	oldByNode := map[int64][]entity.Connection{}
	for _, c := range old {
		oldByNode[c.NodeID] = append(oldByNode[c.NodeID], c)
	}

	touched := map[int64]bool{}
	for nid := range desiredByNode {
		touched[nid] = true
	}

	for nid := range oldByNode {
		touched[nid] = true
	}

	var failures []string

	for nodeID := range touched {
		n, ok := byID[nodeID]
		want := desiredByNode[nodeID]

		if !ok {
			if len(want) > 0 {
				failures = append(failures, fmt.Sprintf("node %d: not in registry", nodeID))
			}

			continue
		}

		t := target(n)
		had := oldByNode[nodeID]

		if len(want) == 0 {
			// user no longer on this panel — drop the whole client
			if err := s.client.DelClient(ctx, t, email); err != nil {
				failures = append(failures, fmt.Sprintf("%s: DelClient: %v", n.Name, err))
			}

			continue
		}

		if !forceAll && sameInboundIDs(connInboundIDs(had), targetInboundIDs(want)) {
			continue // inbound set unchanged on this panel
		}
		// Read the panel once: inbound ids by port + the existing uuid for email.
		idByPort, uuidByEmail, err := s.panelLookup(ctx, t)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: ListInbounds: %v", n.Name, err))
			continue
		}
		// Re-bind: delete-then-add only on a panel we already own (a prior binding of
		// this user, or a forced recreate), preserving the existing uuid. We never
		// delete a client we don't own — collisions on a newly-added panel are caught
		// up front by ensureEmailFree, not reclaimed here.
		cid := s.genUUID()

		if len(had) > 0 || forceAll {
			if existing, ok := uuidByEmail[email]; ok {
				cid = existing
			}

			_ = s.client.DelClient(ctx, t, email) // our own client; ignore "not found"
		}

		ids := make([]int, 0, len(want))
		missing := false

		for _, w := range want {
			id, ok := idByPort[w.Port]
			if !ok {
				failures = append(failures, fmt.Sprintf("%s: no inbound on port %d", n.Name, w.Port))
				missing = true

				break
			}

			ids = append(ids, id)
		}

		if missing {
			continue
		}

		if err := s.client.AddClient(ctx, t, ids, entity.ClientSpec{ID: cid, Email: email, Flow: "", SubID: u.SubID}); err != nil {
			failures = append(failures, fmt.Sprintf("%s: AddClient: %v", n.Name, err))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}

	return nil
}

// panelLookup reads the panel's inbounds once and returns the inbound id per port
// and the uuid per client email (settings.clients is authoritative). The port is
// the bridge to the EXTERNAL 3x-ui inbound — that's 3x-ui's identifier, not ours.
func (s *Service) panelLookup(ctx context.Context, t entity.PanelTarget) (idByPort map[int]int, uuidByEmail map[string]uuid.UUID, err error) {
	inbs, err := s.client.ListInbounds(ctx, t)
	if err != nil {
		return nil, nil, err
	}

	idByPort = make(map[int]int, len(inbs))
	uuidByEmail = map[string]uuid.UUID{}

	for _, in := range inbs {
		idByPort[in.Port] = in.ID

		for _, sc := range in.Clients {
			if sc.UUID != uuid.Nil {
				uuidByEmail[sc.Email] = sc.UUID
			}
		}
	}

	return idByPort, uuidByEmail, nil
}

// ensureEmailFree verifies that none of the given panels already host a client with
// this email (= nickname). The first that does yields PanelClientExistsError naming
// it. Fail-closed: if a panel can't be listed, return that error rather than risk
// provisioning over an unverified panel.
func (s *Service) ensureEmailFree(ctx context.Context, byID map[int64]entity.Node, nodeIDList []int64, email string) error {
	for _, nid := range nodeIDList {
		n, ok := byID[nid]
		if !ok {
			return entity.ErrNodeNotFound
		}

		exists, err := s.emailExistsOnPanel(ctx, target(n), email)
		if err != nil {
			return fmt.Errorf("ensureEmailFree %s: %w", n.Name, err)
		}

		if exists {
			return entity.PanelClientExistsError{Node: n.Name}
		}
	}

	return nil
}

// emailExistsOnPanel reports whether any client with the given email is present on
// the panel (raw settings.clients scan — independent of uuid validity).
func (s *Service) emailExistsOnPanel(ctx context.Context, t entity.PanelTarget, email string) (bool, error) {
	inbs, err := s.client.ListInbounds(ctx, t)
	if err != nil {
		return false, err
	}

	for _, in := range inbs {
		for _, sc := range in.Clients {
			if sc.Email == email {
				return true, nil
			}
		}
	}

	return false, nil
}

// target builds the per-call panel credentials from a node.
func target(n entity.Node) entity.PanelTarget {
	return entity.PanelTarget{BaseURL: n.PanelBaseURL, BasePath: n.PanelBasePath, Token: n.Token}
}

// nodeIDs returns the distinct node ids across targets.
func nodeIDs(targets []connTarget) []int64 {
	seen := map[int64]bool{}

	var out []int64

	for _, t := range targets {
		if !seen[t.NodeID] {
			seen[t.NodeID] = true

			out = append(out, t.NodeID)
		}
	}

	return out
}

// connNodeIDs returns the distinct node ids across connections.
func connNodeIDs(conns []entity.Connection) []int64 {
	seen := map[int64]bool{}

	var out []int64

	for _, c := range conns {
		if !seen[c.NodeID] {
			seen[c.NodeID] = true

			out = append(out, c.NodeID)
		}
	}

	return out
}

// addedNodeIDs returns the distinct node ids in desired the user is not already on.
func addedNodeIDs(old []entity.Connection, desired []connTarget) []int64 {
	had := map[int64]bool{}
	for _, c := range old {
		had[c.NodeID] = true
	}

	seen := map[int64]bool{}

	var out []int64

	for _, t := range desired {
		if had[t.NodeID] || seen[t.NodeID] {
			continue
		}

		seen[t.NodeID] = true

		out = append(out, t.NodeID)
	}

	return out
}

func connInboundIDs(conns []entity.Connection) []int64 {
	out := make([]int64, 0, len(conns))
	for _, c := range conns {
		out = append(out, c.InboundID)
	}

	return out
}

func targetInboundIDs(ts []connTarget) []int64 {
	out := make([]int64, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.InboundID)
	}

	return out
}

// sameInboundIDs reports whether two inbound-id lists are the same set.
func sameInboundIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}

	m := map[int64]int{}
	for _, x := range a {
		m[x]++
	}

	for _, x := range b {
		m[x]--
	}

	for _, v := range m {
		if v != 0 {
			return false
		}
	}

	return true
}

func randID(n int) string {
	const al = "abcdefghijklmnopqrstuvwxyz0123456789"

	b := make([]byte, n)
	_, _ = rand.Read(b)

	for i := range b {
		b[i] = al[int(b[i])%len(al)]
	}

	return string(b)
}
