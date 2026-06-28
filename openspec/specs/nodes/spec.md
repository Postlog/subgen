# Nodes Spec

## Purpose

The registry of VPN nodes and their inbounds. A node carries its public VPN host and the
coordinates of its 3x-ui panel; each node owns one or more inbounds identified by port. Node and
inbound shapes are validated in the service with sentinel errors mapped to typed `4xx`;
uniqueness and referential integrity are enforced by the database. Inbounds are referenced by
numeric id everywhere except when matching the external 3x-ui inbound by port.

## Requirements

### Requirement: List nodes

`GET /admin/api/nodes` SHALL return all nodes with their panel coordinates and inbounds.

#### Scenario: Nodes returned with inbounds

- **WHEN** `GET /admin/api/nodes` is called
- **THEN** the response is `200` with `{ nodes }`, each node carrying `id`, `name`, `vpnHost`, `panelBaseURL`, `panelBasePath`, and its `inbounds` (each `id`, `name`, `port`)

### Requirement: Save node with field validation

`POST /admin/api/nodes/save` SHALL create a node when no id is given and update it otherwise,
validating every field in the service and mapping each failure to a `400`.

#### Scenario: Create vs update

- **WHEN** the request has no id (or id `0`)
- **THEN** a new node is created; **WHEN** the id is positive, the existing node is updated
- **AND** on success the response is `200` with a confirmation message

#### Scenario: Inputs trimmed and blank inbound rows skipped

- **WHEN** a save includes surrounding whitespace and inbound rows with port `0`
- **THEN** string fields are trimmed and the blank inbound rows are ignored

#### Scenario: Invalid node fields

- **WHEN** the node name, VPN host, panel base URL, or panel base path is malformed
- **THEN** the response is `400` reporting the specific field (node name / host / panel URL / base path)

#### Scenario: Inbound list and inbound fields

- **WHEN** the node has no inbounds, or an inbound has an invalid name or a port outside 1–65535
- **THEN** the response is `400` reporting the specific inbound validation error

#### Scenario: Per-node inbound uniqueness

- **WHEN** two inbounds on the node share a name or a port
- **THEN** the response is `400` reporting the duplicate inbound name or port

#### Scenario: Node name already taken

- **WHEN** the node name collides with another node
- **THEN** the response is `409` reporting the name is taken (`entity.ErrNodeNameTaken`)

### Requirement: Delete node guarded by references

`POST /admin/api/nodes/delete` SHALL delete a node and MUST refuse when any inbound is still
referenced by a user or a routing rule.

#### Scenario: Node not found

- **WHEN** the id does not match any node
- **THEN** the response is `400` reporting the node was not found (`entity.ErrNodeNotFound`)

#### Scenario: Inbound still referenced

- **WHEN** an inbound of the node is still referenced by a user connection or a rule/member (FK RESTRICT)
- **THEN** the response is `400` reporting the node is in use (`entity.ErrInboundReferenced`) and nothing is deleted

#### Scenario: Successful deletion

- **WHEN** the node exists and none of its inbounds are referenced
- **THEN** the response is `200` with a confirmation message and the node is removed
