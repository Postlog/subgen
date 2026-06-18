# 0003 — Request validation in code, not in the OpenAPI schema

- **Status:** Accepted
- **Date:** 2026-06-11
- **PR:** #19

## Context

ogen generates server validators from schema constraints (`minLength`, `minItems`,
`minimum`, …) and calls them on request decode. On a violation the response is a generic
`400 {"errMessage":"Bad request"}` (the central `ErrorHandler`), **without a binding
to the field**: a user editing a specific field does not understand what is wrong.

Disabling the **generation** of these validators while keeping the constraints themselves in the
`.yaml` as a contract declaration is something ogen cannot do: there is no
`server/request/validation` feature, and `x-ogen-validate` only adds custom validators. So the choice is
binary: either the constraint is in the schema (ogen validates, generic message), or it is not there
(we validate in code, a precise message). An understandable localized message was deemed more
important than declarativeness.

## Considered Options

- **Validation by schema (as it was)** — declarative, free (codegen), cuts off at the edge;
  but the message is generic («Bad request»), bad UX, and some rules cannot be expressed by the schema.
- **Duplicate (schema + code)** — both the contract and a precise message; but with the constraint in the
  schema ogen cuts the request **before** the handler, and the code check on the HTTP path is
  dead → a formal duplicate.
- **Custom ogen templates** — override the request decode so as not to call `.Validate()`;
  global, fragile (a resync on every ogen upgrade), high maintenance.
- **All value validation — in code** *(chosen)* — remove value constraints from the schema,
  check in the handler/service with a sentinel + a localized message.

## Decision

- **All value constraints** (`minLength`, `minItems`,
  `minimum`) were removed from `openapi/*.yaml`. **Kept** are `required`, `type`, `format` — they define the shape of the contract
  and the generated Go types (this is not «value validation», and ogen validates
  presence/type by them, which is what we need).
- **Validation — in the service layer**, with sentinel errors in `entity`; the handlers are thin and map a
  sentinel into a typed 4xx with a local message constant. Where there was no service — it was
  introduced: **`internal/service/nodes`** owns node validation (the
  `entity.ErrValidation*` family — name/host/URL/base-path/inbounds) plus save/delete. **Inbound
  referential integrity we do NOT pre-check**: deleting a node/removing an inbound that still has a
  reference (a user connection or a mihomo rule/group) is rejected by the DB FK (RESTRICT), and the
  repository translates the violation into `entity.ErrInboundReferenced` — the handler maps it to
  400. Some validations were already in the service/domain and were kept: `validateName`,
  `ErrNoConnectionSelected`, `PolicyRef.Valid()`.
- **Surrogate ids (PK) we do NOT validate.** Checking `id ≥ 1` is meaningless: a non-existent id
  (whether `-100` or a valid-but-absent `1233`) yields not-found on read either way —
  a separate «id format validation» carries no meaning. Removed.
- Where the old behavior already correctly rejected the input without an openapi guard — it was kept: empty
  creds → 401 (constant-time compare), empty path segments do not match the route, an empty/unknown
  `PolicyRef.kind` → `validateRef`. An empty provider-check URL we do **not** validate separately: it is
  no different from a malformed URL — both are unreachable and yield `RulesetCheckUnreachable`
  (there is no emptiness guard in the handler).

## Consequences

- Validation errors are precise, localized, bound to the field; in the service, covered by
  unit tests. The handlers stay thin (a service call + sentinel mapping).
- `openapi/*.yaml` stops being a source of enforced value constraints (only the shape of the
  contract). **A convention for the future**: values are validated in the service with sentinel errors,
  into the schema we put only `required`/`type`/`format`; surrogate ids we do not validate.
- Node validation moved from `web` (the handler layer) into `internal/service/nodes`; `web` no longer
  holds the message mapping — the human error text lives in local constants in each
  handler (`node_save`/`node_delete` etc.), exported for apitest.
- A small downside: the limits no longer self-document in the schema for third-party tooling.
- The risk of a «duplicate» and the generic vague 400 on a schema error are removed.
