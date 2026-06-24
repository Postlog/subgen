# Mihomo Rendering Spec (delta)

## MODIFIED Requirements

### Requirement: Preserve base YAML and append generated sections

Rendering SHALL keep the operator's base YAML preamble and append the generated sections —
`proxy-providers`, `proxy-groups`, `rules`, and `rule-providers` — stripping any of subgen's
generated keys present in the base. The node list is delivered as a `proxy-providers` block, not
inlined as a top-level `proxies:` array.

#### Scenario: Base preserved, generated appended

- **WHEN** a config with a base YAML preamble is rendered
- **THEN** the output contains the base YAML plus the generated sections, with no generated key duplicated in the base
- **AND** the node list appears under `proxy-providers`, not as an inline `proxies:` array

## ADDED Requirements

### Requirement: Deliver nodes as a proxy-provider

Rendering SHALL emit a single auto `proxies` HTTP proxy-provider pointing at the subscriber's
per-token `/sub/{kind}/{token}/proxies` URL, carrying the per-config refresh interval when set, so
the mihomo core re-fetches the node list while connected. A proxy-group's inbound members SHALL be
pulled from that provider via `use: [proxies]` plus an anchored `filter:` that matches exactly the
resolved proxy names, each escaped (`regexp.QuoteMeta`) because operator labels may carry regex
metacharacters. Built-in policies and group references stay inline; a group left with no members
falls back to `DIRECT`.

#### Scenario: Inbound members via the provider

- **WHEN** a group has inbound members the subscriber has proxies for
- **THEN** the group is rendered with `use: [proxies]` and a `filter:` matching exactly those proxy names (escaped, anchored)

#### Scenario: Mixed members

- **WHEN** a group mixes inbound members with built-in policies or group references
- **THEN** the inbound members render via `use:`+`filter:` and the built-in/group members stay inline under `proxies:`

#### Scenario: No resolvable members

- **WHEN** every member resolves to an inbound the subscriber lacks
- **THEN** the group falls back to `DIRECT`

### Requirement: Render authored rule-providers as classical text

Rendering SHALL serve an authored rule-provider's stored matcher tree as classical rule-provider
text — one matcher per line with no target: a leaf as `TYPE,VALUE` (or `TYPE` when it carries no
value), a logical matcher as `TYPE,((child),(child),…)`. No per-subscriber resolution is needed
because authored matchers never reference inbounds, groups, or providers.

#### Scenario: Leaf and logical matchers

- **WHEN** an authored provider's matcher tree is rendered
- **THEN** each leaf becomes a `TYPE,VALUE` line and each logical matcher becomes a `TYPE,((child),…)` line, target-less
