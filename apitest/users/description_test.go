//go:build apitest

package users_test

import "strings"

// The optional free-text description is pure metadata — it lives only in the store and
// the users read API, and never touches the panels. So these cases assert the wire
// round-trip (create/edit body → users-list field), not any 3x-ui state.
//
// Corner cases for the description field:
//   - set_on_create   — description sent on create surfaces on the users-list row.
//   - trimmed         — surrounding whitespace is stripped server-side.
//   - omitted_is_empty — a create without the field yields an empty description.
//   - edit_replaces   — edit overwrites the stored description (and can clear it).
//   - too_long        — an over-length description is rejected server-side (400), nothing created.

// TestDescription drives the description field through create and edit, reading it back
// through GET /admin/api/users each time.
func (s *UserSuite) TestDescription() {
	sel := s.selection("N1", nil)

	s.Run("set_on_create", func() {
		name := s.userName()
		res, err := s.API().CreateUserWith(name, sel, "work laptop")
		s.Require().NoError(err)
		s.Require().True(res.OK, "create with description: %s", res.Message())
		s.T().Cleanup(func() { u, _ := s.API().FindUser(name); s.deleteIfFound(u) })

		u, err := s.API().MustFindUser(name)
		s.Require().NoError(err)
		s.Equal("work laptop", u.Description)
	})

	s.Run("trimmed", func() {
		name := s.userName()
		res, err := s.API().CreateUserWith(name, sel, "  with spaces  ")
		s.Require().NoError(err)
		s.Require().True(res.OK, "create with padded description: %s", res.Message())
		s.T().Cleanup(func() { u, _ := s.API().FindUser(name); s.deleteIfFound(u) })

		u, err := s.API().MustFindUser(name)
		s.Require().NoError(err)
		s.Equal("with spaces", u.Description)
	})

	s.Run("omitted_is_empty", func() {
		u := s.createUser(s.userName(), "N1") // plain create, no description
		s.Empty(u.Description)
	})

	s.Run("edit_replaces", func() {
		u := s.createUser(s.userName(), "N1")

		res, err := s.API().EditUserWith(u.ID, sel, "after edit")
		s.Require().NoError(err)
		s.Require().True(res.OK, "edit set description: %s", res.Message())

		got, err := s.API().MustFindUser(u.Name)
		s.Require().NoError(err)
		s.Equal("after edit", got.Description)

		// editing with an empty description clears it
		res, err = s.API().EditUserWith(u.ID, sel, "")
		s.Require().NoError(err)
		s.Require().True(res.OK, "edit clear description: %s", res.Message())

		got, err = s.API().MustFindUser(u.Name)
		s.Require().NoError(err)
		s.Empty(got.Description)
	})

	s.Run("too_long", func() {
		name := s.userName()
		res, err := s.API().CreateUserWith(name, sel, strings.Repeat("a", 501))
		s.Require().NoError(err)
		s.Require().False(res.OK, "over-length description must be rejected")
		s.NotEmpty(res.Message())

		// rejected before anything was created
		u, err := s.API().FindUser(name)
		s.Require().NoError(err)
		s.Nil(u)
	})
}
