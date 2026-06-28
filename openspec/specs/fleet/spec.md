# Fleet Assembly Spec

## Purpose

The live view of the fleet, built fresh from the panels (no global snapshot). For each node the
service lists the panel's inbounds, matches them to configured inbounds by port, and collects the
enabled clients into per-subscriber proxy sets keyed by `subId`. Configuration data flows
bottom-up on every request; one unreachable panel is tolerated so a single bad node does not take
down subscriptions for everyone.

## Requirements

### Requirement: Match configured inbounds to panel inbounds by port

Fleet assembly SHALL resolve each configured inbound against the panel by its port, since the
port is the identifier on the 3x-ui side.

#### Scenario: Port match

- **WHEN** a configured inbound's port exists on its panel
- **THEN** that panel inbound's stream settings and clients are used for the inbound

#### Scenario: Configured inbound absent on panel

- **WHEN** a configured inbound's port is not present on the panel
- **THEN** the inbound is treated as observed-empty and contributes no proxies

### Requirement: Tolerate a single unreachable panel

Fleet assembly SHALL skip a panel that cannot be reached (with a log) and MUST fail only when no
panel can be reached.

#### Scenario: One panel down

- **WHEN** one node's panel cannot be listed but others can
- **THEN** the fleet is built from the reachable panels and the failed one is skipped

#### Scenario: All panels down

- **WHEN** no panel can be reached
- **THEN** fleet assembly returns an error

### Requirement: Build per-subscriber proxies from enabled clients

Fleet assembly SHALL produce one proxy per enabled client per inbound it appears on, grouped into
a subscriber by `subId`, with the proxy named by the inbound label.

#### Scenario: Enabled client becomes a proxy

- **WHEN** an enabled client with a `subId` is present on an enabled inbound
- **THEN** a proxy is created for it under that inbound, named `<node>-<inbound>`, carrying the client's uuid/flow and stream settings
- **AND** the proxy is grouped under the subscriber identified by the client's `subId`

#### Scenario: Disabled inbound yields no proxies

- **WHEN** a panel inbound is disabled
- **THEN** no proxies are produced from it

### Requirement: Distinguish observed-empty from missing inbound

Fleet assembly SHALL record which clients are present per inbound such that an inbound observed
with no clients is distinct from an inbound that is missing on the panel.

#### Scenario: Observed-empty inbound

- **WHEN** an inbound exists on the panel but has no matching clients
- **THEN** its client set is recorded as empty (present but empty)

#### Scenario: Missing inbound

- **WHEN** an inbound is not present on the panel at all
- **THEN** it has no client-set entry (distinct from an empty set)
