# 0004 — Optional user description: nillable + input structs

- **Status:** Accepted
- **Date:** 2026-06-11
- **PR:** #15

## Context

The admin needs the ability to attach an arbitrary text note to a user
(«work laptop», «issued on such-and-such date»), visible only in the admin UI and not affecting
anything in the 3x-ui provisioning. The field is optional — most users do not have it. The question: how
to represent «no description» in the domain and in the DB, and how to thread one more optional
parameter through `CreateUser`/`EditUser` without turning them into walls of positional arguments.

## Considered Options

- **`NOT NULL DEFAULT ''` + `Description string`** — the simplest: an empty string = «no
  description». But «optional» is then not reflected in the type — `""` and «not set» are indistinguishable, easy
  to forget the normalization, the domain lies about optionality. (This was the first version of the PR;
  rejected at review.)
- **A nullable column + `Description *string`** *(chosen for the representation)* — `NULL`/`nil`
  = «not set», a non-empty string = a value. «optional ⇒ nillable» honestly in the type; one
  canonical representation of emptiness (NULL).
- **An extra positional argument** `CreateUser(ctx, name, description, sel)` —
  two adjacent `string`s next to each other (name/description) are easy to swap, the signature
  grows. Rejected at review.
- **An input struct** `UserCreateParams`/`UserEditParams` *(chosen)* — named fields,
  extensible without breaking the signature.

## Decision

- **`Description *string`** through the layers (entity → repo → service → handlers): `nil` =
  not set. The column `users.description` is **nullable**; the repository **scans it straight
  into a `*string`** (the standard `database/sql` + modernc put `NULL`→`nil`, a value→a string
  themselves — a `sql.Null*` proxy is not needed). The service normalizes empty/whitespace to `nil` (one
  representation = NULL). The read API (`GET /admin/api/users`) returns `description` **only
  when set** (the field is optional in the schema), the write (`create`/`edit`) accepts
  an optional string.
- **Service inputs — structs** `entity.UserCreateParams{Name, Description, InboundIDs}`
  and `entity.UserEditParams{ID, Description, InboundIDs}` instead of positional arguments.
  The former wrapper `entity.ConnectionSelection` (a relic of the old architecture) was removed — the
  set of inbounds travels as a bare `[]int64`.
- **The length is validated by the service** (`validateDescription` → `entity.ErrDescriptionTooLong`,
  ≤500 runes), the handler maps it to 400 — not by an «invisible» `maxLength` in the OpenAPI schema. This is
  consistent with how `name` is validated (`validateName` in the service).
- The column is added by the migration **`migrations/0002-users-description.sql`** via the runner
  ([ADR-0002](0002-ordered-migration-runner.md)) — not by editing the `0001-init.sql` baseline.

## Consequences

- «No description» has a single representation (NULL/`nil`) — there is no ambiguity of
  `""` vs not set; less chance of forgetting the normalization.
- `CreateUser`/`EditUser` are extensible with new fields without changing the signature (via the struct).
- Length validation — in one place (the service), visible and covered by a unit test; OpenAPI does not
  duplicate it.
- The UI shows the description as an icon with a tooltip next to the nickname; in the create/edit form — a textarea.
- This is the first feature migration on top of the runner (`0002-*.sql`) — it confirms that the schema grows
  by new files, not by editing the baseline.
