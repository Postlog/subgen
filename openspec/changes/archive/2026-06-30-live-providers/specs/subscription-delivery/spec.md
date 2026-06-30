# Subscription Delivery Spec (delta)

## ADDED Requirements

### Requirement: Node-list provider endpoint

`GET /sub/{kind}/{token}/proxies` SHALL render the resolved subscriber's node list as a
proxy-provider payload (a YAML document with a top-level `proxies:` array), using the same engine
and token resolution as `GET /sub/{kind}/{token}`, so the mihomo core can re-fetch it on its
interval.

#### Scenario: Valid token

- **WHEN** the token matches a service-owned user under a registered engine kind
- **THEN** the response is `200` with that subscriber's node list as a proxy-provider document

#### Scenario: Unknown kind or token

- **WHEN** the kind has no registered renderer, or the token matches no service-owned user
- **THEN** the response is `404`

### Requirement: Authored rule-provider endpoint

`GET /sub/{kind}/{token}/rules/{name}` SHALL serve the named authored rule-provider of the
resolved subscriber's config as classical rule-provider text (one matcher per line, no target),
with `X-Content-Type-Options: nosniff`, using the same token resolution as the subscription route.

#### Scenario: Authored provider found

- **WHEN** the token resolves and `name` is an authored rule-provider of that subscriber's config
- **THEN** the response is `200` with the provider's classical-text list and `X-Content-Type-Options: nosniff`

#### Scenario: Unknown kind, token, or provider name

- **WHEN** the kind/token does not resolve, or `name` is not an authored provider of the resolved config
- **THEN** the response is `404`
