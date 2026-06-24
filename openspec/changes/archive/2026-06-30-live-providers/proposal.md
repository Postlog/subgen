# Live delivery of nodes and rules via providers

## Why

A mihomo subscription used to be one YAML document: the node list (`proxies:`), proxy-groups
and routing rules were all inlined, so whether an operator's edit reaches a client depends on
the client app re-pulling the whole profile. Reading the ClashMi (`KaringX/clashmi`) source
confirmed that on mobile there is **no background profile-update timer** at all — the profile
refreshes only when the app is foregrounded or the tunnel (re)connects. So the
`Profile-Update-Interval` header subgen sets cannot guarantee "edit the config, all connected
clients apply it within an hour" on phones.

mihomo has a second, reliable refresh path: `proxy-providers` and `rule-providers` carry an
`interval` and the **core itself** re-fetches them while the tunnel is up, independent of the
app UI. We want subgen to use it so node and rule edits propagate to connected clients within
the provider interval.

## What Changes

- The node list is delivered as a single auto `proxy-provider` (`proxies`) pointing at a new
  per-token endpoint `GET /sub/{kind}/{token}/proxies`; proxy-groups switch inline inbound
  members to `use: [proxies]` + an anchored, escaped `filter:`. Built-in/group members stay
  inline; an empty group still falls back to `DIRECT`.
- A rule-provider gains a typed `source`: `external` (an upstream URL, as before, optionally
  mirrored) or `authored` (a list edited in subgen, stored as a target-less matcher tree and
  served as classical text at `GET /sub/{kind}/{token}/rules/{name}` with `nosniff`).
- A per-config **nodes update interval** (`proxies_interval`, seconds) drives the
  proxy-provider's refresh.
- The base profile keeps the skeleton `rules:` and the `rule-providers:` declarations; only the
  **content** each provider fetches is live. Structural changes (new groups/targets, reordering)
  still need a profile reload.

## Impact

- Affected specs: `subscription-delivery` (two new public endpoints), `mihomo-rendering` (nodes
  as a provider, `use:`+`filter:` groups, authored rule-provider text), `mihomo-config`
  (rule-provider `source`, authored matcher validation, the nodes update interval).
- Code: new `subProxies`/`subRules` ogen operations; `mihomo_rule_providers.source`, the
  recursive `mihomo_authored_matchers` table and `mihomo_profile.proxies_interval` (migration
  `0005`, additive); `MihomoProvider.source`+`matchers` and `MihomoConfig.proxiesInterval` in
  the wire contract; an authored-list editor in the admin UI.
- subgen never imports mihomo and never decodes `mrs`; the node provider's `filter` escapes
  proxy names (operator labels may carry regex metacharacters/emoji).
