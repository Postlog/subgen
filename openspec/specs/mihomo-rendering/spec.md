# Mihomo Rendering Spec

## Purpose

How a stored mihomo config plus one subscriber's live proxies become the YAML the client
downloads. A per-subscriber resolver turns each typed `PolicyRef` into a concrete proxy name:
built-in policies map to their constant, inbound references map to the subscriber's matching
proxy, group references map to the group name. References the subscriber cannot satisfy are
dropped silently, and an empty group degrades to `DIRECT` on the client. The base YAML preamble
is preserved and the generated sections (`proxies`, `proxy-groups`, `rules`, `rule-providers`,
`sub-rules`) are appended.

## Requirements

### Requirement: Resolve policy references per subscriber

Rendering SHALL resolve each `PolicyRef` against the subscriber's own proxies and the config's
groups/providers: built-in kinds to their uppercase constant, an inbound reference to the
subscriber's proxy for that inbound, a group reference to the group name.

#### Scenario: Built-in policy

- **WHEN** a reference is a built-in policy (e.g. `direct`, `reject`)
- **THEN** it renders to the corresponding policy name, which is always available

#### Scenario: Inbound available to the subscriber

- **WHEN** a reference points to an inbound the subscriber has a proxy for
- **THEN** it renders to that proxy's name

### Requirement: Drop unavailable references

Rendering SHALL omit a member or rule whose inbound, group, or provider reference cannot be
resolved for the subscriber, rather than emitting a dangling name.

#### Scenario: Inbound not available

- **WHEN** a member or rule references an inbound the subscriber has no proxy for
- **THEN** that member/rule is omitted from the rendered YAML

#### Scenario: Subscriber with no proxies

- **WHEN** the subscriber has no proxies at all
- **THEN** every inbound reference drops, leaving groups empty

### Requirement: Empty group degrades to DIRECT

Rendering SHALL emit a proxy group with no remaining members as an empty group, which the mihomo
client treats as `DIRECT`.

#### Scenario: All members dropped

- **WHEN** every member of a group resolves to an unavailable inbound
- **THEN** the group is rendered with no members (the client falls back to `DIRECT`)

### Requirement: Preserve base YAML and append generated sections

Rendering SHALL keep the operator's base YAML preamble and append the generated `proxies`,
`proxy-groups`, `rules`, `rule-providers`, and `sub-rules` sections.

#### Scenario: Base preserved, generated appended

- **WHEN** a config with a base YAML preamble is rendered
- **THEN** the output contains the base YAML plus the generated sections, with no generated key duplicated in the base
