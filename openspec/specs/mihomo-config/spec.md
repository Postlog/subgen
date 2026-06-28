# Mihomo Config Spec

## Purpose

The routing configuration domain for the mihomo engine: proxy groups, routing rules (including
recursive logical rules), rule providers, a base YAML preamble, and profile metadata. The model
is strongly typed — routing targets and group members are a single typed `PolicyRef`
(`direct`/`reject`/…/`inbound`/`group`), never a magic string. The config is decoded and
validated in `internal/mihomo` with sentinel errors; the admin handler maps each sentinel to a
typed `400` with a local message. Group references cross the HTTP boundary by array index, not
database id. The admin endpoints read, save, list, create, and delete configs, and probe a rule
provider.

## Requirements

### Requirement: Typed policy references and catalogs

The config model SHALL resolve routing targets and group members through a typed `PolicyRef`
(kind plus optional inbound/group id) and MUST expose the reference taxonomy via a schema
endpoint so the frontend hardcodes nothing.

#### Scenario: Schema catalog served

- **WHEN** `GET /admin/api/config/mihomo/schema` is called
- **THEN** the response is `200` describing `actions`, `ruleProvider` behaviors/formats, `proxyGroup` types (with allowed member categories), `rules` types (with allowed destination categories and per-type flags), and `generatedKeys`
- **AND** option lists are sorted by name

#### Scenario: Exactly one reference mode per PolicyRef

- **WHEN** a `PolicyRef` is validated
- **THEN** a built-in kind carries no inbound/group id, an inbound kind carries only an inbound id, and a group kind carries only a group index

### Requirement: Read effective config

`GET /admin/api/config/mihomo` SHALL return the base config when no user scope is given, the
user's custom config when `?user=<id>` is given, and `404` when that user has no custom config.

#### Scenario: Read base

- **WHEN** `GET /admin/api/config/mihomo` is called without `user`
- **THEN** the response is `200` with the base config (an empty config if none was ever saved)

#### Scenario: Read existing custom

- **WHEN** `GET /admin/api/config/mihomo?user=<id>` is called for a user that has a custom config
- **THEN** the response is `200` with that user's custom config

#### Scenario: Read missing custom

- **WHEN** `GET /admin/api/config/mihomo?user=<id>` is called for a user with no custom config
- **THEN** the response is `404`

### Requirement: Save config with full validation

`POST /admin/api/config/mihomo/save` SHALL decode and validate the whole config and persist it
atomically, mapping any validation failure to a `400` with a specific message; an absent `userId`
targets the base config and a positive `userId` targets that user's custom config.

#### Scenario: Save base

- **WHEN** a valid config is posted with no `userId`
- **THEN** the response is `200` and the base config is replaced atomically

#### Scenario: Save custom requires an existing custom

- **WHEN** a config is posted with a positive `userId` for a user that has no custom config
- **THEN** the response is `400` reporting the user has no custom config (`entity.ErrUserConfigNotFound`)

#### Scenario: Proxy-group validation

- **WHEN** a group has an empty or duplicate name, an unknown type, no members, a field not allowed for its type, or the groups form a cycle
- **THEN** the response is `400` reporting that specific group error

#### Scenario: Routing-rule validation

- **WHEN** a rule has an unknown type, a `MATCH` that is not last or is nested, a missing value where required (or a value where not allowed), `no-resolve` on an unsupported type, an out-of-range provider reference, a top-level rule without a target, or a nested rule that carries a target
- **THEN** the response is `400` reporting that specific rule error

#### Scenario: Logical-rule arity

- **WHEN** a `NOT` rule does not have exactly one child, or an `AND`/`OR` rule has fewer than two children
- **THEN** the response is `400` reporting the arity error

#### Scenario: Rule-provider validation

- **WHEN** a provider has an empty name or URL, an unknown behavior, or an unknown format
- **THEN** the response is `400` reporting that specific provider error

#### Scenario: Duplicate provider name

- **WHEN** two providers in the config share a name
- **THEN** the save fails `400` reporting the provider name is taken (`entity.ErrRuleProviderNameTaken`, from the database constraint)

#### Scenario: Base YAML guard

- **WHEN** the base YAML does not parse, or contains a generated key (`proxies`, `proxy-groups`, `rules`, `rule-providers`, `sub-rules`)
- **THEN** the response is `400` reporting the invalid YAML or the generated key

#### Scenario: Profile validation

- **WHEN** the profile title or filename is empty, the filename contains `/`, `\`, or control characters, or the update interval is not a positive number
- **THEN** the response is `400` reporting that specific profile error

### Requirement: List custom configs

`GET /admin/api/config/mihomo/customs` SHALL return the set of users that have a custom config
alongside the full user list, so the admin can scope edits.

#### Scenario: Customs and users returned

- **WHEN** `GET /admin/api/config/mihomo/customs` is called
- **THEN** the response is `200` with `{ customs, users }`, where `customs` is a subset of `users`

### Requirement: Create custom config by cloning the base

`POST /admin/api/config/mihomo/custom/create` SHALL create a user's custom config as an
independent clone of the base, refusing if one already exists.

#### Scenario: Clone created

- **WHEN** a `userId` with no existing custom config is posted
- **THEN** the response is `200` and a custom config is created from a snapshot of the base

#### Scenario: Custom already exists

- **WHEN** a `userId` that already has a custom config is posted
- **THEN** the response is `400` reporting the custom already exists (`entity.ErrUserConfigExists`)

### Requirement: Delete custom config

`POST /admin/api/config/mihomo/custom/delete` SHALL delete a user's custom config (reverting them
to the base), refusing if none exists.

#### Scenario: Custom deleted

- **WHEN** a `userId` that has a custom config is posted
- **THEN** the response is `200` and the custom config is removed

#### Scenario: No custom to delete

- **WHEN** a `userId` with no custom config is posted
- **THEN** the response is `400` reporting the user has no custom config (`entity.ErrUserConfigNotFound`)

### Requirement: Probe a rule provider

`POST /admin/api/config/mihomo/provider/check` SHALL fetch a provider URL read-only and report a
typed outcome without persisting anything.

#### Scenario: Reachable and matching

- **WHEN** the URL returns `200` with content matching the declared format
- **THEN** the response is `200` describing the format and size

#### Scenario: Unreachable, HTTP error, empty, or format mismatch

- **WHEN** the URL cannot be reached, returns a non-`200` status, returns an empty body, or returns content not matching the declared format
- **THEN** the response is `400` reporting that specific outcome
