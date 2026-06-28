# Config Ownership Spec

## Purpose

Who owns a subscription config, independent of which engine renders it. A single anchor table
`subscription_configs(id, user_id, kind, created_at)` records ownership: `user_id NULL` is the
base config shared by everyone, a non-null `user_id` is that user's personal custom. `kind` is a
typed `entity.ConfigKind` (the engine — `mihomo` today, later xray/sing-box), never a magic
string. Each kind has one base plus at most one custom per user. The engine's content hangs off
the anchor by `config_id`; the anchor itself knows nothing about mihomo. A custom is a snapshot —
once cloned it is independent of the base.

## Requirements

### Requirement: Base and per-user custom per kind

The anchor SHALL hold exactly one base config (`user_id NULL`) per kind and at most one custom
config per user per kind, enforced by a unique index over `(COALESCE(user_id,0), kind)`.

#### Scenario: One base per kind

- **WHEN** configs exist for a kind
- **THEN** there is a single base config (`user_id NULL`) for that kind

#### Scenario: At most one custom per user per kind

- **WHEN** a user already has a custom config for a kind
- **THEN** attempting to create a second one for the same user and kind is rejected by the unique index

### Requirement: Typed config kind

A config's engine SHALL be a typed `entity.ConfigKind` constant, validated against the supported
kinds rather than compared as a string.

#### Scenario: Unsupported kind

- **WHEN** an operation targets a kind that is not registered/supported
- **THEN** it is rejected as an unknown kind (no string matching)

### Requirement: Custom is an independent snapshot

Creating a custom config SHALL clone the base content into the new config so the custom is
independent — later edits to the base do not reach existing customs, in one transaction with the
anchor row.

#### Scenario: Edits to base do not leak into a custom

- **WHEN** a custom is created from the base and the base is later edited
- **THEN** the custom keeps its cloned content unchanged

#### Scenario: Clone from an empty base

- **WHEN** a custom is created while the base has no content
- **THEN** the custom is created empty (nothing to copy) and is still independent

### Requirement: Atomic content save scoped by config

Saving engine content SHALL replace all of that config's content atomically, scoped to its
`config_id`, so a failed save leaves the previous content intact.

#### Scenario: All-or-nothing save

- **WHEN** a content save runs for a config
- **THEN** the config's groups, members, rules, and providers are replaced in a single transaction
- **AND** if any step fails, nothing is recorded for that config
