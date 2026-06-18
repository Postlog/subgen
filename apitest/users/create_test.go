//go:build apitest

package users_test

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/postlog/subgen/apitest/api"
	"github.com/postlog/subgen/internal/entity"
	userCreateHandler "github.com/postlog/subgen/internal/handlers/user_create"
)

// Corner cases considered for POST /admin/api/users/create:
//   - happy.smart_only           — one smart inbound → one client on that inbound.
//   - happy.force_only           — one force inbound, no smart.
//   - happy.smart_plus_force_same_node — smart+force on N1 → ONE client (one uuid) on
//                                  both inbounds (the multi-inbound model / the old bug).
//   - happy.cross_node           — inbounds on N1 and N2 → one client per panel, shared subId.
//   - err.empty_name             — "" → handler's validateName → friendly 400, nothing provisioned.
//   - err.name_bad_chars         — spaces / "!" → friendly charset message (reaches the handler).
//   - err.name_too_long          — >32 chars → friendly charset message (no maxLength in schema).
//   - err.no_connections         — absent inbound-id list (null) → generic 400 (kept `required`).
//   - err.unknown_inbound_id     — id with no node_inbounds row → "inbound not found".
//   - err.duplicate_name         — second create with same nickname → "name already taken" (store PK).
//   - err.email_exists_on_panel  — a foreign client already owns the email on a target
//                                  panel → PanelClientExistsError naming the node; the
//                                  foreign client is left untouched.
//   - err.malformed_json         — non-JSON body → generic 400, no provisioning.
//
// Validation/duplicate/collision cases also assert the panel was NOT mutated.

// TestCreate covers the happy paths: each asserts the {ok} envelope AND the real client
// landing on the right inbound(s) with the right uuid sharing.
func (s *UserSuite) TestCreate() {
	s.Run("smart_only", func() {
		u := s.createUser(s.userName(), "N1")
		s.RequireClient(s.Pan1(), api.N1Smart, u.Name)
		s.RequireNoClient(s.Pan1(), api.N1Force, u.Name)
	})

	s.Run("force_only", func() {
		name := s.userName()
		res, err := s.API().CreateUser(name, []int64{s.InboundID("N1", "force")})
		s.Require().NoError(err)
		s.Require().True(res.OK, "force-only create: %s", res.Message())
		s.T().Cleanup(func() { u, _ := s.API().FindUser(name); s.deleteIfFound(u) })

		s.RequireClient(s.Pan1(), api.N1Force, name)
		s.RequireNoClient(s.Pan1(), api.N1Smart, name)
	})

	s.Run("smart_plus_force_same_node", func() {
		u := s.createUser(s.userName(), "N1", "N1") // smart + force on N1
		uuidSmart := s.RequireClient(s.Pan1(), api.N1Smart, u.Name)
		uuidForce := s.RequireClient(s.Pan1(), api.N1Force, u.Name)
		s.Equal(uuidSmart, uuidForce, "smart+force on one node must be a single client (one uuid)")
	})

	s.Run("cross_node", func() {
		u := s.createUser(s.userName(), "N1", "N2") // smart=N1, force=N2
		s.RequireClient(s.Pan1(), api.N1Smart, u.Name)
		s.RequireClient(s.Pan2(), api.N2Force, u.Name)
	})
}

// TestCreateValidation covers every rejected create. Each asserts {ok:false}, the exact
// friendly message, and that nothing was provisioned.
func (s *UserSuite) TestCreateValidation() {
	smartN1 := s.InboundID("N1", "smart")

	s.Run("empty_name", func() {
		// Schema no longer carries minLength; the empty name reaches validateName → friendly
		// charset message (ADR-0003: validation in code).
		res, err := s.API().CreateUser("", []int64{smartN1})
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, res.Status)
		s.False(res.OK)
		s.Equal(userCreateHandler.MsgInvalidName, res.Err)
	})

	s.Run("name_bad_chars", func() {
		for _, bad := range []string{"bad name", "bad!char", "naïve"} {
			res, err := s.API().CreateUser(bad, []int64{smartN1})
			s.Require().NoError(err)
			s.False(res.OK, "nickname %q must be rejected", bad)
			s.Equal(userCreateHandler.MsgInvalidName, res.Err)
		}
	})

	s.Run("name_too_long", func() {
		// 33 chars — one past the 32 limit.
		res, err := s.API().CreateUser("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", []int64{smartN1})
		s.Require().NoError(err)
		s.False(res.OK)
		s.Equal(userCreateHandler.MsgInvalidName, res.Err)
	})

	s.Run("no_connections", func() {
		// An empty inbound-id list trips the schema's minItems:1 → 400 generic, before
		// the handler's own "select a connection" check.
		name := s.userName()
		res, err := s.API().CreateUser(name, nil)
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, res.Status)
		s.False(res.OK)
		s.Equal(api.MsgBadRequest, res.Err)

		gone, err := s.API().FindUser(name)
		s.Require().NoError(err)
		s.Nil(gone, "a rejected create must not persist a user")
	})

	s.Run("unknown_inbound_id", func() {
		res, err := s.API().CreateUser(s.userName(), []int64{999999})
		s.Require().NoError(err)
		s.False(res.OK)
		s.Equal(userCreateHandler.MsgInboundNotFound, res.Err)
	})

	s.Run("duplicate_name", func() {
		u := s.createUser(s.userName(), "N1") // provisioned on N1-smart

		// Re-create the same nickname selecting an inbound on N2, where that email is NOT
		// on the panel — so the panel pre-check passes and it's the users.name DB
		// constraint that rejects it (→ "Name already taken"). The same-node panel-collision path
		// is covered separately by TestCreateRejectsExistingEmail.
		res, err := s.API().CreateUser(u.Name, []int64{s.InboundID("N2", "force")})
		s.Require().NoError(err)
		s.False(res.OK, "duplicate nickname must be rejected")
		s.Equal(userCreateHandler.MsgNameTaken, res.Err)
		s.RequireNoClient(s.Pan2(), api.N2Force, u.Name)
	})

	s.Run("malformed_json", func() {
		res, err := s.API().PostRaw("/admin/api/users/create", "application/json", []byte("{not json"))
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, res.Status, "a malformed body is a 400")
		decoded, err := api.DecodeResult(res.Body)
		s.Require().NoError(err)
		s.False(decoded.OK)
		s.Equal(api.MsgBadRequest, decoded.Err)
	})
}

// TestCreateRejectsExistingEmail covers the panel-collision corner case: a foreign
// client already owns the email (= nickname) on a target panel out-of-band (an orphan
// from a half-finished delete, or a manually-created client) with a different
// uuid/subId. subgen's store says the nickname is free (NameTaken checks the store, not
// the panel), so create proceeds to provision — and the API must REFUSE, naming the
// panel, and change NOTHING (never delete a client subgen doesn't own).
func (s *UserSuite) TestCreateRejectsExistingEmail() {
	name := s.userName()

	// Seed a foreign client with our email on N1's smart inbound; clean it up after.
	orphan := uuid.New()
	s.Require().NoError(s.XC().AddClient(api.Ctx, s.Pan1().Target, []int{s.PanelInboundID(s.Pan1(), api.N1Smart)},
		entity.ClientSpec{ID: orphan, Email: name, SubID: "orphansubid00000"}))
	s.T().Cleanup(func() { _ = s.XC().DelClient(api.Ctx, s.Pan1().Target, name) })
	s.Require().Equal(orphan.String(), s.RequireClient(s.Pan1(), api.N1Smart, name), "orphan must be present first")

	// CreateUser (smart on N1) must be rejected with a message naming N1.
	res, err := s.API().CreateUser(name, []int64{s.InboundID("N1", "smart")})
	s.Require().NoError(err)
	s.False(res.OK, "create must be rejected when a foreign client owns the email")
	s.Contains(res.Err, "N1", "rejection must name the offending panel")
	s.Contains(res.Err, "already has a client", "rejection must use the friendly panel-collision text")

	// …and the foreign client must be left intact (same uuid — not deleted/re-added).
	s.Equal(orphan.String(), s.RequireClient(s.Pan1(), api.N1Smart, name), "foreign client must be untouched")

	// No user row should have been created either.
	gone, err := s.API().FindUser(name)
	s.Require().NoError(err)
	s.Nil(gone, "rejected create must not leave a user row")
}

// deleteIfFound deletes a user if non-nil (cleanup helper for ad-hoc creates).
func (s *UserSuite) deleteIfFound(u *api.User) {
	if u != nil {
		_, _ = s.API().DeleteUser(u.ID)
	}
}
