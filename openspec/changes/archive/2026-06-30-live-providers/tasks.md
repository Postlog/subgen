# Tasks — Live delivery of nodes and rules via providers

## 1. Schema & storage

- [x] 1.1 Migration `0005-live-providers.sql` (additive): `mihomo_rule_providers.source`, the
      recursive `mihomo_authored_matchers` table, `mihomo_profile.proxies_interval`.
- [x] 1.2 `repository/routing` reads/writes the `source`, authored matcher trees and
      `proxies_interval`; `AllRuleProviders` and `CloneConfig` carry the new columns.

## 2. Domain (`internal/mihomo`)

- [x] 2.1 `RuleProvider.Source` (`external`/`authored`) + `Matchers []RoutingRule`;
      `MihomoProfile.ProxiesInterval`.
- [x] 2.2 Decode the `source`/`matchers`/`proxiesInterval` from the save form.
- [x] 2.3 Validate per source: authored carries no URL, ≥1 matcher, each matcher a target-less
      leaf/logical rule (no MATCH/RULE-SET/SUB-RULE); sentinel errors.

## 3. Rendering (`internal/mihomo/render`)

- [x] 3.1 Emit the auto `proxies` proxy-provider at the per-token `/proxies` URL with the
      interval; group inbound members become `use: [proxies]` + an escaped, anchored `filter:`.
- [x] 3.2 `RenderProxiesPayload` (node list) and `RenderAuthoredProvider` (classical text).

## 4. Endpoints (ogen)

- [x] 4.1 `openapi/sub_proxies.yaml`, `openapi/sub_rules.yaml` + `$ref`; regenerate `internal/oas`.
- [x] 4.2 `handlers/sub` implements `SubProxies`/`SubRules` reusing the token resolution;
      `MihomoRenderer` gains `RenderProxies`/`RenderRuleProvider`.

## 5. Admin UI

- [x] 5.1 Rule-provider `source` selector; authored-list editor (reuse `rule-node`, RULE-SET
      filtered out); nodes-update-interval field.

## 6. Docs & checks

- [x] 6.1 CHANGELOG entry; this OpenSpec change.
- [x] 6.2 `make all` green (unit + integration + apitest).
