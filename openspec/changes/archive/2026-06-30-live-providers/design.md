# Design — Live delivery of nodes and rules via providers

## Context

A mihomo subscription used to be one YAML document: the node list (`proxies:`), proxy-groups
and routing rules were all inlined. Whether an operator's edit reaches a client depends on the
client app re-pulling the whole profile. Reading the ClashMi (`KaringX/clashmi`) source confirmed
that on mobile there is **no background profile-update timer** at all (the periodic timer is
gated on `PlatformUtils.isPC()`, and there is no `workmanager`/`BGTask`): the profile refreshes
only when the app is foregrounded or the tunnel (re)connects. So the `Profile-Update-Interval`
header — which subgen sets correctly — cannot guarantee "edit the config, all connected clients
apply it within an hour" on phones.

mihomo has a second, reliable refresh path: `proxy-providers` and `rule-providers` carry an
`interval` (seconds) and the **core itself** re-fetches them while the tunnel is up, independent
of the app UI. We want subgen to use it so node and rule edits propagate to connected clients
within the provider interval.

## Considered Options

- **A. Keep inline, only tune the header.** Zero work, but does not fix mobile — the header path
  is the broken one.
- **B. Auto-bucket rules by resolved target.** subgen splits the ordered rule list into per-target
  classical rule-providers automatically. Live, but the bucketing/ordering is implicit "magic" and
  the operator can't name or reason about the buckets.
- **C. Explicit authored rule-provider blocks (chosen).** The skeleton `rules:` stays exactly as
  authored (`RULE-SET,<block>,<target>` + `MATCH`); each rule-provider block is operator-named and
  either an external upstream (as before, optionally mirrored) or an **authored** list edited in
  subgen and served as classical text at a per-token URL. Nodes always become one auto
  `proxy-provider`; groups reference it via `use:` + a `filter:` regex.
- **D. Merge external `mrs` + authored into one self-contained list.** Rejected: `mrs` is a
  compiled binary (zstd + succinct trie) that only mihomo (GPL-3.0) can decode, and flattening a
  large geosite/geoip set into classical text defeats `mrs`'s compactness. subgen stays MIT and
  does not decode/transcode `mrs` — external `mrs` is referenced as its own provider.

## Decision

Option C. The node list is delivered as a single `proxy-provider` (`/sub/mihomo/{token}/proxies`,
refreshed on a per-config `proxies_interval`), and proxy-groups switch inline member names to
`use: [proxies]` + an anchored, `regexp.QuoteMeta`-escaped `filter:` for the inbound members
(group/built-in members stay inline; an empty group still falls back to `DIRECT`). A rule-provider
gains a typed `source`: `external` (unchanged) or `authored`. An authored provider stores a
target-less matcher tree (reusing `RoutingRule` with `Target == nil`; leaf + logical AND/OR/NOT,
never MATCH/RULE-SET/SUB-RULE — mihomo forbids those in a classical provider) and is rendered to
classical text at `/sub/mihomo/{token}/rules/{name}`. The skeleton `rules:` and the
`rule-providers:` declarations remain in the base profile (Layer A); only the **content** each
provider fetches is live.

Why C over B: the operator asked for explicit, named blocks they author and wire by hand, not an
implicit auto-split. Why C over D: keeping MIT and `mrs` compactness outweighs the convenience of a
single merged list — composition of "authored + external" toward one target is done with two
`RULE-SET` skeleton lines, which is routing-identical.

## Consequences

- Node/rule-content edits reach a **connected** client within the provider interval, with no
  profile reload — the original goal. Disconnected clients are unaffected (nothing to fetch), and
  *structural* changes (new groups/targets, reordering, adding a whole provider) still need a
  profile reload, because the skeleton and provider declarations live in the base profile.
- New surface: `mihomo_rule_providers.source`, the recursive `mihomo_authored_matchers` table and
  `mihomo_profile.proxies_interval` (migration `0005`, additive); two ogen endpoints
  (`subProxies`/`subRules`) reusing the existing HMAC token resolution; `MihomoProvider.source` +
  `matchers` and `MihomoConfig.proxiesInterval` in the wire contract; an authored-list editor in
  the admin UI (reusing the `rule-node` component, RULE-SET filtered out).
- subgen never imports mihomo and never decodes `mrs`; the node provider's `filter` must escape
  proxy names (operator labels may contain regex metacharacters/emoji). Validation of the authored
  grammar lives in the service (`internal/mihomo`), mapped to per-handler messages.
