# Mihomo Config Spec (delta)

## ADDED Requirements

### Requirement: Rule-provider source (external or authored)

A rule-provider SHALL carry a typed `source`: `external` (an upstream URL, as before) or
`authored` (a list edited in subgen and served as classical text). An `external` provider needs a
non-empty URL; an `authored` provider MUST carry no URL and at least one matcher, each a
target-less leaf or logical rule and never `MATCH`, `RULE-SET`, or `SUB-RULE` (mihomo rejects
those in a classical provider). An empty or missing `source` is treated as `external` for
back-compat with rows predating the field. Save maps each violation to a `400` with a specific
message.

#### Scenario: Valid authored provider

- **WHEN** a provider with `source: authored`, no URL, and a non-empty tree of target-less leaf/logical matchers is saved
- **THEN** the response is `200` and the provider is persisted with its matcher tree

#### Scenario: Invalid authored provider

- **WHEN** an authored provider sets a URL, carries no matchers, or includes a `MATCH`/`RULE-SET`/`SUB-RULE` matcher
- **THEN** the response is `400` reporting that specific authored-provider error

#### Scenario: External provider unchanged

- **WHEN** a provider with `source: external` (or no source) is saved
- **THEN** it is validated as before — a non-empty URL, a known behavior, and a known format are required

### Requirement: Per-config nodes update interval

The mihomo profile SHALL carry a `proxiesInterval` (seconds) driving the auto node
proxy-provider's refresh; a non-positive value is rejected at save.

#### Scenario: Invalid nodes update interval

- **WHEN** a config is saved with a `proxiesInterval` that is not a positive number of seconds
- **THEN** the response is `400` reporting the invalid proxies interval (`entity` profile sentinel)
