# 0005 — Strict typed references in the mihomo config (RULE-SET → provider by id)

- **Status:** Accepted
- **Date:** 2026-06-11
- **PR:** #17

## Context

`RoutingRule` addressed a rule-provider dirtily: for `RULE-SET` the provider name lay as a
string in `RoutingRule.Value` (validation — a match by name, render — the name went into the YAML
as is). This violated the project rule «a reference to an entity = by id; a string — only
if it is itself a string payload» (AGENTS.md, «entity — self-documenting types»):
a provider is an entity, and `Value` must be a pure string payload (domain/ip/
port), empty for `RULE-SET` and `MATCH`.

Groups and inbounds were already addressed by id (`PolicyRef.GroupID`/`InboundID`), but a group
had a hidden wart: the domain field `PolicyRef.GroupID` carried a DIFFERENT meaning by direction —
on input into `SaveMihomoConfig` it is an index into the array (ids are not assigned yet),
on read from the DB it is a real id. One field — two meanings.

mihomo has no numeric ids: in the output YAML proxy-groups and rule-providers are addressed by
NAME (rule-providers are a map, the key = the name). That is, the name is the PK of the output
document. Inside subgen, which edits this document, the name is a mutable form field.

## Considered Options

- **Keep the name as the reference, but type it** (`ProviderRef{Name}` instead of a string in
  `Value`) — honest to the fact that the name is the PK of the mihomo document; but the join key inside the
  editor becomes a mutable string: renaming a provider breaks the reference (this was the current
  behavior + the warning «non-existent provider»).
- **A round-trip for an id on every «+»** (the new-provider endpoint returns an id) — gives real
  ids at the editing stage, but breaks atomicity (save is currently a wholesale replace in
  one transaction), leaves half-assembled configs in the DB (orphan GC is needed), and the ids are
  ephemeral anyway (the next full save recreates them). Chattiness without a durable benefit.
- **A surrogate id + a reference by index on the wire** *(chosen)* — the provider gets a
  surrogate `id`, `RULE-SET` references it; the name is resolved into the YAML key only on
  render (as already done for groups). At the boundary with the frontend the reference travels as an array
  index (a correlation id), resolved into an id inside `SaveMihomoConfig`. Rename-safe, gives
  FK integrity, symmetric to groups.
- **Switch to incremental-diff persistence** (durable ids, in-place UPDATE) — would remove the
  id ephemerality and the index/id wart itself; but it complicates save (the diff, deletions, FK order) and
  at the config sizes (a handful of groups/providers) does not pay off.

## Decision

We take a surrogate id + a reference by index, and at the same time **split the types of the save input and
domain/read** in order to remove the field's double meaning:

- **`RuleProvider`** += `ID int64`; **`RoutingRule`** += `ProviderID *int64` (nil except for
  `RULE-SET`), `Value` — a pure payload.
- The domain types (`RoutingRule`/`ProxyGroup`/`RuleProvider`/`PolicyRef`) carry only
  real ids — they are consumed by reads from the DB and the render.
- A new **draft** type family (`ConfigDraft`/`RuleDraft`/`GroupDraft`/`RefDraft`)
  carries indices — it is produced by `DecodeConfig` and consumed by `SaveMihomoConfig`. Index→
  id is resolved inside save (in local slices), not in the type.
- DB: `mihomo_rule_providers` gets `id INTEGER PRIMARY KEY AUTOINCREMENT` +
  `UNIQUE(config_id,name)`; `mihomo_routing_rules` += `provider_id`; the optional columns
  (`value`/`interval`/`tolerance`/`lazy`) become nullable. The schema travels via the migration
  runner — `migrations/0003-strict-mihomo-refs.notx.sql`: a rebuild of the three tables +
  a backfill of `provider_id` from names + a cleanup of `value`. Since the rebuild requires
  `PRAGMA foreign_keys=OFF` (a no-op inside a transaction), the runner was extended with a **`.notx` mode**
  (the migration runs outside a transaction, writes itself into `schema_migrations`) — a departure
  from ADR-0002 «each in a transaction», deliberate and documented.
- The wire does not split (it is already index-based both ways) — only `providerIdx` was added to
  `MihomoRule`. The provider name is not used on the wire as a reference.

Why an index/correlation id, and not the name: with a wholesale replace providers/rules are
recreated by every save, there is no durable id in principle, so a reference at the editing
stage must be either by name (mutable) or by position (a correlation
id). Position is rename-safe and symmetric to groups.

## Consequences

- Renaming a provider no longer breaks RULE-SET references (as with groups).
- FK integrity `rule → provider` within a snapshot appears; a duplicate name is caught
  as `UNIQUE` (2067), translated into `entity.ErrRuleProviderNameTaken` (the detector matches both
  PK 1555 and UNIQUE 2067 — the behavior is preserved).
- Draft and domain never coexist in one graph (save: wire→ConfigDraft→DB; read:
  DB→domain), so no field carries «an index on input / an id on output». The cost — a
  parallel family of draft types (introduced only where there is a reference by index; the
  provider carries no references → it reuses the domain `RuleProvider` with an empty ID).
- The prod migration is manual and one-off, requires a backup and a read-only check (whether there are
  RULE-SET rules, whether the names match); the order and the checks are described in the migration file itself.
- The cleanliness of the separation rests on the wholesale replace; switching to incremental-diff
  would require a hybrid ref type (either an index or an id) — deliberately deferred.
- AND/OR/NOT (logical rules) will land on this cleaned-up model as separate work.
