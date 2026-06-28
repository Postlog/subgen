# Changelog

subgen changes — one entry per PR, reverse-chronological. Non-trivial changes
link to an ADR in [`docs/decisions/`](docs/decisions/). The rule and format are
in [`AGENTS.md`](AGENTS.md) (section "Documenting changes"). There are no versions/tags:
the service is not released, deploy is continuous.

## 2026-06-28 — Feature worktree workflow (#120)

Documented the per-feature workflow in `AGENTS.md`: every feature/fix is developed in its
own git worktree under `.worktrees/<slug>` on its own branch (branch → commit → push → PR;
never edit `main` directly). Added `.worktrees/` to `.gitignore` so those in-repo checkouts
never pollute `git status`.

## 2026-06-19 — Responsive admin UI: phones, tablets, ultrawide (#115)

Made the admin panel usable on every screen size — mini-phones (320px) through ultrawide.
Almost all of it is static (`internal/handlers/web/static/{app.css,app.js,index.html}`); the
only Go change is a one-line dev tweak (below). The desktop (≥992px) is visually unchanged.
The navbar collapses into a hamburger below `lg` (the tabs became `<button>` for keyboard
use); the Users/Nodes tables turn into `data-label` cards below 992px (two-up on tablets via
CSS-grid, the table returns at ≥992px), with the user description shown inline on a card
rather than a floating tooltip and Bootstrap's row-hover cell tint dropped on cards; the
config constructors stack below 768px; touch gets bigger tap targets while inputs keep their
normal size (no 16px bump — it had blown the dense config selects/inputs out of proportion);
hover popovers open on tap (`:focus-within`); modals scroll their body only (header/footer
pinned); Monaco no longer traps page scroll; and the container widens on ultrawide.
`render.go` now serves the dev `SUBGEN_STATIC_DIR` assets with `Cache-Control: no-cache` so a
live edit shows on a plain reload. Verified with a real browser at
320/360/500/768/850/1024/1280/1600px (no horizontal overflow, no desktop regression).

## 2026-06-18 — public-ready: README overhaul, MIT license, English docs & UI (#117)

Prepared the repository for a public release. Rewrote `README.md` for newcomers (a clear
"what/why" hook, a Features list, a screenshot gallery, an env-var table) and fixed a stale
`gorilla/mux` mention. Added a `LICENSE` (MIT) and `CONTRIBUTING.md`. Translated everything
human-facing to English — `AGENTS.md`, `CHANGELOG.md`, all ADRs, `apitest/README.md`,
`docs/subgen.md`, the admin UI (`internal/handlers/web/static/`), and all user-facing
handler/error messages (unit tests and `apitest` assertions updated in lockstep). See
[ADR-0009](docs/decisions/0009-public-ready-and-english-docs.md).

## 2026-06-17 — Subscription: a popup of links from the backend (raw URL + clashmi deeplink) (#116)

The "Subscription" column in the users list now opens a popup with a list of copyable links
instead of a single Mihomo button: currently the raw Mihomo subscription URL and the deeplink
`clashmi://install-config?url=<enc>&name=<title>&overwrite=false`. The set of links and their
titles come entirely from the backend — a new `internal/service/sublinks` service owns the
catalog and the frontend hardcodes nothing (adding an engine/app = one catalog line); the popup
shows only the title and a "Copy" button (the value is private). The `sub` shape in
`GET /admin/api/users` changed from `{id,url}` to `{links:[{title,value}]}`; the clashmi
deeplink's `name` = the profile title of the user's effective (custom, else base) config. See
[ADR-0008](docs/decisions/0008-subscription-link-catalog.md).

Also in this PR: the "Subscription settings" block was moved to the top of the "Mihomo config"
tab; leftover githooks were removed (the `make hooks` target and the local `core.hooksPath` — the
`.githooks/` directory was deleted earlier); per review, the interface naming in every
`contract.go` was aligned with the concrete dependency (repo → `<entity>Repo`, service →
`<entity>Service`, client → `<entity>Client`) instead of role names
(`subLinker`/`configResolver`/`creator`/…); the rule is recorded in `AGENTS.md`. Linting was
reduced to a single source of truth: `make lint` runs golangci-lint in the pinned Docker image
`golangci/golangci-lint`, and **CI (`ci.yml`/`deploy.yml`) now calls the same `make lint`**
instead of `golangci-lint-action`, so local and CI lint don't drift. The platform is host-native
(CI amd64 / Apple Silicon arm64): all enabled linters are arch-independent on 64-bit, so the
result is identical without emulation (max speed); the module/analysis cache lives in the
gitignored `.lintcache/` (on CI via `actions/cache`). This closed the previous `wsl_v5` mismatch.

## 2026-06-16 — mihomo logical rules (AND/OR/NOT) with a recursive tree UI (#114)

A routing rule can now use the logical operators `AND`/`OR`/`NOT` with arbitrarily
nested sub-rules. The rule was made recursive (`RoutingRule.Children` — of the same
structure, `Target` optional: a sub-rule has none), without a separate "condition" entity;
storage is the self-referential `mihomo_routing_rules` (`parent_id`), not a JSON blob. The renderer emits the
nested syntax verbatim (`AND,((NETWORK,UDP),(DST-PORT,443)),REJECT-DROP`). Four
matchers were added for parity with the wiki (`SRC-IP-ASN`, `SRC-IP-SUFFIX`, `PROCESS-PATH-WILDCARD`,
`PROCESS-NAME-WILDCARD`) and `sub-rules` in `GeneratedKeys` (the operator can no longer set this
section in the base YAML). Sub-rules carry no `no-resolve` (mihomo does not parse it for them). UI —
a recursive tree constructor; SUB-RULE is not implemented. Schema — migration
`migrations/0004-mihomo-rule-children.notx.sql`. See
[ADR-0006](docs/decisions/0006-recursive-routing-rules.md).

## 2026-06-11 — Strict mihomo references: RULE-SET → rule-provider by id (#17)

`RoutingRule` no longer stores the provider name as a string in `value` — `RULE-SET` references a
rule-provider by surrogate id (`provider_id` FK); the save input and domain/read were split into
separate types (a draft with indices vs a domain with real ids), which removes the double meaning of
`PolicyRef.GroupID`. Optional fields (`value`/`interval`/`tolerance`/`lazy`/`noResolve`) are
pointers. The schema is migrated by the runner (`migrations/0003-strict-mihomo-refs.notx.sql` —
rebuild with FK off outside a transaction). See [ADR-0005](docs/decisions/0005-strict-mihomo-refs.md).

## 2026-06-11 — User: optional description for the admin panel (#15)

A user gained an optional free-text description (`*string`, nillable;
visible only in the admin UI): set on create/edit, shown as an icon with a
tooltip in the table. The `users.description` column (nullable) is added by migration
`migrations/0002-users-description.sql` via the runner. Service inputs were moved into the structs
`entity.UserCreateParams` / `entity.UserEditParams` (removed `entity.ConnectionSelection`).
See [ADR-0004](docs/decisions/0004-optional-user-description.md).

## 2026-06-11 — Request validation — in the service layer, not in OpenAPI (#19)

All value-constraints (`minLength`/`minItems`/`minimum`) were removed from `openapi/*.yaml` —
ogen no longer generates server-side value validators (the generic, vague 400). Validation is
in the service layer via sentinel errors (`entity.ErrValidation*`), the handlers are thin. A
`internal/service/nodes` was introduced (node validation + save/delete); node validation and `web.ValidateNode`
moved there. Inbound referential integrity is **not pre-checked** — it is held by the DB FK
(RESTRICT), the repository translates a violation into `entity.ErrInboundReferenced`. An empty
provider-check URL is not a special case (nor is a malformed URL → `RulesetCheckUnreachable`). Surrogate ids
(PK) are **not** validated (a nonexistent id → not-found). Handler message texts were made
public and are imported in apitest (no duplication). In tests `gomock.Any()` was kept
only for the context — the other arguments are checked exactly (matchers for random uuid/subId).
`required`/`type`/`format` were kept (the contract shape). See
[ADR-0003](docs/decisions/0003-validation-in-code.md).

## 2026-06-11 — Ordered migration runner (#18)

Manual `*.manual.sql` was replaced by the runner `migrations.Apply` (`repository.Open` calls it
instead of `ExecContext(Schema)`): `0001-init.sql` — the immutable baseline, then `NNNN-*.sql`
by name, the application fact — in `schema_migrations`, each migration in a transaction,
fail-fast + log. Connection PRAGMA (incl. `journal_mode=WAL`) moved into the DSN. The section on
migrations in `AGENTS.md` was rewritten. See [ADR-0002](docs/decisions/0002-ordered-migration-runner.md).

## 2026-06-11 — Documenting convention: CHANGELOG + ADR (#16)

Introduced `CHANGELOG.md` (this file) and the ADR catalog `docs/decisions/`; the rule is recorded in
`AGENTS.md`. The format "one entry per PR, no versions" was chosen.
See [ADR-0001](docs/decisions/0001-adopt-changelog-and-adr.md).
