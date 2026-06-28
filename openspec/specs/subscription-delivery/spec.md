# Subscription Delivery Spec

## Purpose

The public subscription endpoint clients poll: `GET /sub/{kind}/{token}`. The `kind` selects an
engine renderer from a registry; the `token` is an HMAC over the user's `subId`. The handler
resolves the token to a service-owned user, picks that user's custom config for the kind (or the
base), renders it for the user's live proxies, and returns the YAML with the metadata headers
mihomo/Clash clients consume. The path is engine-generic: adding an engine is one renderer
registration, no handler change.

## Requirements

### Requirement: Engine-generic subscription route

`GET /sub/{kind}/{token}` SHALL validate `kind` against the renderer registry and render through
the matching engine, returning `404` for an unknown kind.

#### Scenario: Known kind

- **WHEN** `kind` matches a registered engine renderer
- **THEN** the request is rendered by that engine

#### Scenario: Unknown kind

- **WHEN** `kind` has no registered renderer
- **THEN** the response is `404`

### Requirement: Token resolves to a service-owned user

The handler SHALL accept the token only if it HMAC-matches the `subId` of a user owned by the
service, returning `404` otherwise.

#### Scenario: Valid token

- **WHEN** the token matches a service-owned user's `subId` under the configured secret
- **THEN** that user is selected for rendering

#### Scenario: Unmatched token

- **WHEN** the token matches no service-owned user
- **THEN** the response is `404`

#### Scenario: Client provisioned directly on a panel

- **WHEN** a token would correspond to a client created directly on a panel (not via the service)
- **THEN** it is not served (only service-owned users are matched)

### Requirement: Custom-else-base config selection

The handler SHALL render the user's custom config for the requested kind when it exists, and
otherwise the base config.

#### Scenario: User has a custom

- **WHEN** the resolved user has a custom config for the kind
- **THEN** that custom config is rendered

#### Scenario: User has no custom

- **WHEN** the resolved user has no custom config for the kind
- **THEN** the base config is rendered

### Requirement: Subscription metadata headers

A successful subscription response SHALL carry the client-facing metadata headers:
profile update interval, base64 profile title, content-disposition filename, and the user-info
traffic line.

#### Scenario: Headers present on success

- **WHEN** a subscription is served successfully
- **THEN** the response includes `Profile-Update-Interval` (hours), a base64 `Profile-Title`, a `Content-Disposition` filename, and a `Subscription-Userinfo` line
- **AND** the body is the rendered engine config
