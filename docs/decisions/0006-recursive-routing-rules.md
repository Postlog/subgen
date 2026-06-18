# 0006 — Logical routing rules (AND/OR/NOT) as a recursive rule

- **Status:** Accepted
- **Date:** 2026-06-16
- **PR:** #114

## Context

The routing-rule model was a flat single matcher:
`RoutingRule{Type, Value, ProviderID, NoResolve, Target}` — one type, one value, one
target. It does not express mihomo's logical/composite rules (`AND`/`OR`/`NOT`), whose
payload is a nested list of sub-rules: `LOGIC,((TYPE,VAL),(TYPE,VAL)),TARGET`. This
blocked a real operational need — silencing QUIC

```
AND,((NETWORK,UDP),(DST-PORT,443)),REJECT-DROP
```

(it forces HTTP/3 to fall back to TCP; fixes App Store / YouTube hangs under TUN+fake-ip)
— it could not be assembled from the UI.

Targets are already typed via `PolicyRef` (see [ADR-0005](0005-strict-mihomo-refs.md)) —
the gap was exclusively on the matchers' side. Additionally: the operator could write a
`sub-rules` section into the base YAML and hit a collision with generation (the key was not in `GeneratedKeys`).

Cross-checking the `ruleTypes` registry against the mihomo wiki revealed four more missing simple
matchers: `SRC-IP-ASN`, `SRC-IP-SUFFIX`, `PROCESS-PATH-WILDCARD`, `PROCESS-NAME-WILDCARD`.

Constraints discovered from the mihomo source (`rules/logic`): `NOT` takes exactly one
sub-rule; sub-rules are parsed via `ParseRulePayload(payload, parseParams=false)` —
**parameters (incl. `no-resolve`) inside a logical sub-rule are not extracted**, that is,
a sub-rule does not carry `no-resolve`. `SUB-RULE` (a reference to a named sub-rule group) is
a separate large feature (named sub-rule groups = a second editor); by the
owner's decision in this iteration it is **not implemented**, only the base-YAML hole is closed.

## Considered Options

### The sub-rule type model

- **A. A single recursive type.** `RoutingRule` (and the save-side `RuleDraft`) are recursive: a logical
  rule carries sub-rules in `Children []RoutingRule` — of the same structure; `Target`
  becomes optional (`*PolicyRef`) — the top level has it, a sub-rule does not.
  Pros: one type, no «second entity»; exactly the model mihomo itself has (a rule
  and a sub-rule are one). Cons: a sub-rule carries inapplicable fields (`Target`/`NoResolve`),
  which validation forbids (a positional invariant top-level vs child).
- **B. A separate sub-condition type** (`RuleCondition`/`ConditionDraft`) — only the needed fields
  (`Type/Value/Provider/Children`), without a target. Pros: each type carries only its own. Cons:
  two parallel recursive types + a separate table + duplication of decode/render/clone;
  over-engineering for what is essentially one entity.

### Storage

- **A. A self-referential `mihomo_routing_rules`** (`parent_id` → self, `target_kind` nullable).
  A sub-rule — a row of the same table with `parent_id` and without a target. A CHECK pins the invariant
  `(parent_id IS NULL) = (target_kind IS NOT NULL)`. Pros: one table, save/read/clone
  — one pass; `DELETE … WHERE config_id` removes the whole tree (every row has
  `config_id`). Cons: a rebuild of the table is needed (target_kind → nullable) — a `.notx` migration.
- **B. A separate table** `mihomo_rule_conditions`. Cons: a «second entity» in the DB, three
  places of recursion (save/read/clone) + a separate provider FK; it contradicts the single type.
- **C. A JSON blob** of sub-rules. Cons: references to a provider inside the blob are not FKs; it violates
  the typed-references invariant (AGENTS). Rejected immediately.

## Decision

- **A single recursive type (A) + a self-referential table (A).** `RoutingRule.Children
  []RoutingRule`, `Target *PolicyRef` (nil only on a sub-rule); the save mirror —
  `RuleDraft.Children []RuleDraft`, `Target *RefDraft`. Separate `RuleCondition`/
  `ConditionDraft` and a `mihomo_rule_conditions` table are **not introduced** — that was
  over-engineering (a rule and a sub-rule are one entity, as in mihomo). Storage —
  `mihomo_routing_rules` with `parent_id` (the migration `0004-*.notx.sql`, a rebuild with a nullable
  `target_kind`). The JSON blob was rejected as violating typed references.
- **The invariant — in code, positionally.** `RuleDraft.Valid()` — a position-independent
  per-type check (a logical one carries no value/provider/no-resolve; `Children` only on a
  logical one). A recursive `validateRule(r, top, …)` pins the positional part: a top-level one must
  have a target (`ErrTargetRequired`), a sub-rule — must not (`ErrChildTarget`); `NOT`
  exactly one, `AND`/`OR` ≥2; `MATCH` cannot be a sub-rule (`ErrMatchChild`); the provider index
  in range; a sub-rule does not carry `no-resolve`.
- **`no-resolve` on sub-rules — no.** Matches the mihomo parser and removes the field from the
  sub-rule (only the top-level one carries it).
- **The wire — `MihomoRule` is recursive.** `target` is optional (not `required`), `children[]` —
  a self-`$ref`; there is no separate `MihomoCondition`. The top-level/child invariant — in the service, not
  in the schema (per the AGENTS convention).
- **`sub-rules` → `GeneratedKeys`.** Closes the base-YAML hole (save rejects, render
  strips); subgen does not generate named sub-rule groups. SUB-RULE as a rule type was not
  added.
- **Matcher parity:** + `SRC-IP-ASN`, `SRC-IP-SUFFIX`, `PROCESS-PATH-WILDCARD`,
  `PROCESS-NAME-WILDCARD` (simple, without special options).
- **UI:** a recursive `rule-node` component (a tree with indents). Reordering
  sub-rules is **intentionally not implemented** — AND/OR/NOT are commutative, order does not affect
  matching (and it removes the conflict of nested SortableJS). Dragging top-level
  rules is preserved (a row + its sub-rules are wrapped in a single draggable `.rule-item`).

## Consequences

- An arbitrary recursion `AND(AND(OR(...),NOT(...)),...)` round-trips DB ↔ render ↔
  decode; the QUIC rule is assembled from the UI and rendered verbatim.
- One table for rules and sub-rules: save — a recursive `insertRules` (a per-node
  insert, `LastInsertId` → `parent_id`); read — assembling the tree in memory from a single query;
  clone — one pass over `ORDER BY id` (parent before child) with a remap of
  parent/provider/group. `DELETE … WHERE config_id` removes the whole tree.
- A sub-rule carries inapplicable fields (`Target`/`NoResolve`), which the validator forbids;
  this is a deliberate cost of the single type (fewer types matters more than «every field is applicable»).
- Sub-rules do not reference inbounds (they are matchers, not targets) → the per-subscriber drop
  works only by the rule's `Target`; the only render «failure» of a sub-rule is
  an unresolved RULE-SET provider (a config error, log + drop of the rule).
- Sub-rules do not carry `no-resolve` — if it is ever needed by mihomo's logic in the future, it
  will require both support in the mihomo parser and an extension of the model.
- The sub-rule order in the UI is not editable; if it is ever needed — it is added
  trivially (the model is already ordered by `position`).
- SUB-RULE (named sub-rule groups) remains a future task; `sub-rules`
  is reserved for subgen, so it can be introduced later without a key migration.
