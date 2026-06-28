# Admin Users Spec

## Purpose

The admin-facing management of subscription users: listing with filters and pagination,
creating, editing, deleting, and recreating users, plus exposing each user's ready-to-use
subscription links. Value validation lives in the service and surfaces as typed `4xx`
responses; uniqueness conflicts surface as `409`. This capability owns the HTTP contract;
the cross-panel client lifecycle it triggers is specified in `user-provisioning`.

## Requirements

### Requirement: List users with filters and pagination

`GET /admin/api/users` SHALL return a page of users with their connections and subscription
links, and MUST clamp pagination to safe bounds.

#### Scenario: Default page

- **WHEN** `GET /admin/api/users` is called without pagination parameters
- **THEN** the response is `200` with `{ users, total, page, perPage }`
- **AND** `page` defaults to 1 and `perPage` defaults to 50

#### Scenario: Pagination clamping

- **WHEN** `perPage` is greater than 200, or `page`/`perPage` is below 1
- **THEN** `perPage` is clamped to at most 200, `page` below 1 becomes 1, and `perPage` below 1 becomes 50

#### Scenario: Text filter

- **WHEN** the `q` query parameter is provided
- **THEN** only users whose name matches the substring are returned

#### Scenario: Inbound filter is OR-matched

- **WHEN** the `inbound` parameter lists one or more inbound ids
- **THEN** a user is included if it is bound to at least one of those inbounds

#### Scenario: Each user carries connections and links

- **WHEN** a user is returned
- **THEN** it includes its `inbounds` (each with `id`, `label`, `port`, and a `missing` flag)
- **AND** it includes a `sub.links` array and traffic `stats`

### Requirement: Create user

`POST /admin/api/users/create` SHALL create a user from a name, an optional description, and a
set of inbound ids, validating the input in the service and mapping each domain error to a typed
`4xx`.

#### Scenario: Successful creation

- **WHEN** a valid name, optional description, and at least one existing inbound id are submitted
- **THEN** the response is `200` with a success message
- **AND** the user is provisioned across the selected inbounds' panels

#### Scenario: Invalid name

- **WHEN** the name does not match the allowed format (lowercase letters, digits, `_`, `-`, 1–32 chars)
- **THEN** the response is `400` reporting an invalid name (`entity.ErrInvalidUserName`)

#### Scenario: No connection selected

- **WHEN** the inbound id list is empty
- **THEN** the response is `400` reporting that at least one connection is required (`entity.ErrNoConnectionSelected`)

#### Scenario: Description too long

- **WHEN** the description exceeds 500 characters
- **THEN** the response is `400` reporting the description is too long (`entity.ErrDescriptionTooLong`)

#### Scenario: Unknown inbound or node

- **WHEN** an inbound id (or its node) does not exist
- **THEN** the response is `400` reporting the missing inbound/node (`entity.ErrInboundNotFound` / `entity.ErrNodeNotFound`)

#### Scenario: Name already taken

- **WHEN** the name collides with an existing user
- **THEN** the response is `409` reporting the name is taken (`entity.ErrNameTaken`)

#### Scenario: Email already present on a panel

- **WHEN** a panel already holds a client with the same email (nickname)
- **THEN** the response is `409` reporting the panel collision (`entity.PanelClientExistsError`, naming the node)

### Requirement: Edit user

`POST /admin/api/users/edit` SHALL replace a user's inbound set and description, applying the
same validation as creation and re-binding the user across panels.

#### Scenario: Successful edit

- **WHEN** an existing user id, a non-empty set of existing inbound ids, and an optional description are submitted
- **THEN** the response is `200` with a success message
- **AND** the user's connections and description are updated and panels re-bound

#### Scenario: Edit validation mirrors create

- **WHEN** the new inbound set is empty, the description is too long, or an inbound does not exist
- **THEN** the response is `400` with the corresponding validation error

### Requirement: Delete user

`POST /admin/api/users/delete` SHALL remove a user and best-effort remove its panel clients.

#### Scenario: Successful deletion

- **WHEN** an existing user id is submitted
- **THEN** the response is `200` with a success message
- **AND** the user's clients are removed from its panels where reachable, and the user row is deleted

### Requirement: Recreate user clients

`POST /admin/api/users/recreate` SHALL re-provision a user's panel clients from the stored
connections, to repair drift.

#### Scenario: Successful recreate

- **WHEN** an existing user id is submitted
- **THEN** the response is `200` with a success message
- **AND** the user's clients are re-bound on all its panels from the stored connection set

### Requirement: Subscription links are backend-owned

Each user's `sub.links` SHALL be produced by the backend link catalog as ordered
`{ title, value }` pairs, so the frontend renders them verbatim without hardcoding any link.

#### Scenario: Links rendered from the catalog

- **WHEN** a user is listed
- **THEN** `sub.links` contains one entry per catalog link, each with a `title` and a `value` URL/deeplink
- **AND** the titles and formats come from the backend, not the client
