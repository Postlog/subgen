# User Provisioning Spec

## Purpose

How a subgen user maps onto 3x-ui panel clients across the fleet. The invariant: a user is
**one client per panel** — a single VLESS `uuid` whose `email` is the user's nickname and whose
`subId` is shared across panels — bound to all of the user's inbounds on that panel with a single
`add` call. Edits re-bind the same client (keeping the uuid); deletes and recreates are
best-effort across panels. These rules exist because of hard-won 3x-ui quirks documented in
`AGENTS.md`.

## Requirements

### Requirement: One client per panel, multi-inbound bind

Provisioning SHALL create exactly one 3x-ui client per panel — one `uuid`, `email` = nickname,
shared `subId` — and MUST bind it to all of the user's inbounds on that panel in a single `add`
call.

#### Scenario: User spans multiple inbounds on a panel

- **WHEN** a user selects several inbounds that live on the same panel
- **THEN** one client is added on that panel with one uuid bound to all those inbounds in a single call
- **AND** the same `subId` is used for the user on every panel

#### Scenario: Edit keeps the uuid

- **WHEN** a user's inbound set changes on a panel it already occupies
- **THEN** the existing client is re-bound and its uuid is preserved

### Requirement: Email-collision guard before binding

Before binding a user onto a panel it does not yet occupy, provisioning SHALL verify the
nickname (email) is free on that panel and fail closed if the panel cannot be listed.

#### Scenario: Email already used on a new panel

- **WHEN** a panel being added already has a client whose email equals the user's nickname
- **THEN** provisioning aborts with `entity.PanelClientExistsError` naming that node, making no changes

#### Scenario: Panel unreachable during the guard

- **WHEN** a panel cannot be listed during the collision check
- **THEN** provisioning fails closed (returns the error) rather than risk provisioning blindly

#### Scenario: Already-owned panels are exempt

- **WHEN** a user is edited on a panel it already occupies
- **THEN** the collision guard does not run for that panel (the client there is already the user's)

### Requirement: Best-effort delete across panels

Deleting a user SHALL attempt to remove its client from each panel by email and MUST treat a
client that is already absent as success.

#### Scenario: Client missing on a panel

- **WHEN** a delete runs and the client is not found on a panel
- **THEN** that panel is treated as already-clean (idempotent) and deletion continues

#### Scenario: Panel unreachable during delete

- **WHEN** a panel is unreachable during delete
- **THEN** deletion proceeds on the other panels and the user row is still removed

### Requirement: Recreate re-binds from stored state

Recreating a user SHALL force a full re-bind of its panel clients from the stored connection
set, preserving uuids where the user already owns a client.

#### Scenario: Repair drift

- **WHEN** a recreate runs for a user
- **THEN** each panel in the stored connection set is re-bound for the user
- **AND** uuids are preserved on panels the user already occupies
