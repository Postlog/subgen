# AGENTS.md — subgen code style and rules

This document describes the structure and stylistic rules of the `subgen` service. The
service has been **brought** to this layout (entity / clients / repository / service / handlers,
`contract.go`+mockgen, table-driven tests). Write any new code by these rules;
if you touch old code — bring it up to them in the same change.

The composition root is `cmd/service/main.go`: it loads config, opens
repositories, constructs clients/services and per-action handlers with their dependencies,
assembles them into the `internal/handlers/api` composite and brings up the ogen server (which is also
the only `http.Handler`; on the stdlib `http.ServeMux` alongside it there is only static content). **There is no
`App` oracle** — data flows bottom-up
(`repository`/`clients` → `service` → `handler`), the cache is narrow (inside a specific
service), without global atomic snapshots. HTTP handlers — one package per action in
`internal/handlers/<action>`, shared HTTP tooling — `internal/handlers/web`.

The design and operation of subgen are in [`docs/subgen.md`](docs/subgen.md) and
[`README.md`](README.md). Infrastructure facts that the **code** depends on (the
3x-ui API contract, secrets, dev gotchas) are in the «Infrastructure, 3x-ui API and secrets» section below.
The rest of the document is about the **code** of the Go service.

> subgen is a standalone product (a mihomo/Clash.Meta subscription server), split off
> from the fleet monorepo [`Postlog/vpn-toolchain`](https://github.com/Postlog/vpn-toolchain);
> the fleet topology, nodes and observability live there.

## Language — everything in English

**All text in this repository is English, no exceptions.** This covers code identifiers and
comments, log/error messages, user-facing strings and the admin UI, the docs (`README.md`,
`AGENTS.md`, `docs/`, ADRs), the `CHANGELOG`, commit messages, and **pull-request titles and
descriptions**. Non-English text (e.g. Cyrillic) must not appear anywhere in the tree — the
only exception is operator-entered data that lives in the running store, not in the repo.
Quick guard: `grep -rI '[А-Яа-я]'` over the tree comes back empty. See
[ADR-0009](docs/decisions/0009-public-ready-and-english-docs.md).

Structure references (look at as a model):
`go.avito.ru/av/service-listing-admin`, `go.avito.ru/av/service-mnz-sf`.

## Infrastructure, 3x-ui API and secrets

This is not «about style», but subgen's code is tied to these facts — keep them in mind when editing
the client/provisioning/deploy. The full design is in [`docs/subgen.md`](docs/subgen.md).

### Secrets — never in git

Panel passwords, `SUBGEN_SECRET` (HMAC), admin creds, client UUIDs/keys and the rendered
per-client configs are **not committed**. Bootstrap secrets — in `.env` (gitignored, next to
`.env.example`); `db/` (the SQLite store) is gitignored too. Only the examples go into git.

### 3x-ui API (both fleet panels are 3.2.6) — `internal/clients/xui`

- **Auth = Bearer API token.** `Authorization: Bearer <token>` (issue it: `x-ui setting
  -getApiToken` or Settings → API tokens in the panel). Token calls bypass CSRF, login/cookie
  are not needed — this is subgen's machine path. A browser login on 3.x requires a CSRF token
  (`<meta name="csrf-token">` → `X-CSRF-Token`) — we do not use it for the service.
- **Client management has moved** to `/panel/api/clients/*` (`add`, `update/:email`,
  `del/:email`). The old `/panel/api/inbounds/addClient` returns **404** on 3.2.x. The add body:
  `{"client":{…,"tgId":0},"inboundIds":[…]}` — `tgId` is an **int** (0), not a string, otherwise 400.
- **Client identity model (important).** A 3x-ui client = one `uuid` (VLESS credential);
  `email` and `subId` are labels on that uuid. **One client can hang on many inbounds** —
  pass several ids in `inboundIds`; the uuid/email/subId stay the same for all of them. Pitfalls
  learned the hard way: (a) `del/:email` resolves email→one uuid and removes it from all inbounds,
  but if **one email is on two inbounds with different uuids** (provisioned by separate `add` calls,
  each minting a new uuid) — it fails with `Client Not Found In Inbound For ID: <uuid>` and **does
  not delete anything**; (b) `subId` is bound to one email — reusing a subId across two
  emails → `subId already in use`. So a user is provisioned as **one client per
  panel** (one uuid, email = nickname, shared subId), bound to all of its inbounds
  with a **single** `add`; an edit = a re-bind of the same client (we keep the uuid). There is **no**
  per-inbound delete route in this build (`/panel/api/inbounds/:id/delClient/:id` → 404); only
  `del/:email`.
- `settings`/`streamSettings` arrive as **JSON objects** (on 3.x; before 3.x they were
  a JSON string inside JSON) — the client parses both via `json.RawMessage`.
- Tags of new inbounds — `in-<port>-<net>` (e.g. `in-8443-tcp`).
- DNS RU1 is fixed at the host level — the custom resolver workaround in the xui client has been removed,
  the system resolver is used.

### Dev gotchas (when poking panels/nodes from the operator's Mac)

- **There is no `sqlite3` on the nodes.** To peek into `/etc/x-ui/x-ui.db` — copy it locally or
  go via the 3x-ui HTTP API.
- **The operator's Mac has an HTTP(S) proxy** (`HTTPS_PROXY`) that kills non-standard ports (e.g.
  61001). For curl/go against the panels, prefix with
  `env -u HTTPS_PROXY -u https_proxy -u HTTP_PROXY -u http_proxy`. (subgen on the node is not affected —
  its HTTP transport does not set a proxy.)
- The prod node is live and shared with real users: **read-only recon first**, and
  **confirm externally-visible/irreversible actions** (deploy, Xray restart, client edits).

### Deploy

Docker (not systemd). Prod deploy — a manual GitHub Actions workflow
(`.github/workflows/deploy.yml`, `workflow_dispatch`): the image is built on the runner (the node is
RAM-starved — Go/registry are not needed on the node), shipped over SSH (`docker save | ssh | docker load`),
`.env` is rendered from the `production` Environment secrets, `docker compose up -d`. Details —
`README.md` / `docs/subgen.md`. The legacy systemd deploy has been removed (it was `systemd/`). There is no SIGHUP reload —
the config flows bottom-up from the store on every request. **DB migrations — an ordered runner
on start** (`migrations.Apply`: the `0001-init.sql` baseline + `NNNN-*.sql` by name, tracked in
`schema_migrations`; see [ADR-0002](docs/decisions/0002-ordered-migration-runner.md)).

## Target rules (layer inversion)

Below are the hard rules of the second pass. They refine the sections below; on conflict
these formulations take priority. Any new code follows them; if you touch old code — bring it up.

### External-API clients — thin adapters

- A client = a thin adapter layer to `entity`. **Zero business logic.** No
  internal state except what is needed to connect to the resource (`http.Client`,
  timeouts). The node name, public host, a specific token — are **not** client state.
- No «fat» methods: a method does not pull the general list (`ListInbounds`) and does not
  compute something from it for a specific task (finding an id by port, a uuid by email) —
  that is business logic, its place is in the service. The client returns raw domain data.
- If different nodes specify different connection creds — **the creds are passed as a method
  parameter**, not into the constructor. One client per process, the call target is an argument.
- **One method — one `.go` file** (`list_inbounds.go`, `add_client.go`, `del_client.go`).
- Per method (= file) — a separate unit test; HTTP is mocked (`httptest.Server` /
  `http.RoundTripper` mock), without live network.

### Config — static only, does not flow through layers

- `config` = a package with `Load() (Config, error)`: it reads the environment/`.env` (via tags,
  with a library) and validates. Period.
- `Config` has 0 methods (at most simple getters). It is «special»: you may leave it out
  of `entity`.
- `Config` is **not passed** through layers. Only the
  concrete primitive fields go into the constructors of services/handlers (no `New(cfg *config.Config)`). Interacting with the
  `Config` struct — at most in `main`.
- **No config from the DB** (`FromStore`-like code must not exist — it is a data-flow
  violation: operational data flows bottom-up from the repositories).
- **No seed.** No data in the store means no data; there are no default
  configs/rules/providers in the code.

### entity — self-documenting types

- Traits — via types and constants, not «magic» strings.
  The reference is `mihomo.PolicyRef`/`PolicyKind` and `RuleType`/
  `ProxyGroupType` (in the `internal/mihomo` package): the routing target is resolved by a
  typed `Kind`, **never** by a substring like
  `strings.HasPrefix(name, "<...>")`.
- Strong typing: client id → `github.com/google/uuid.UUID`. Where a built-in type
  is inconvenient — justify it in a comment (e.g. `Node.PanelBaseURL` stays a `string`:
  it travels through SQLite text/HTML forms and is only concatenated with a path — `url.URL`
  buys nothing here).
- **References to entities — by numeric id, not by name/port.** A name (of a node, etc.) is
  mutable and is good only for display and label names (`<node>-<inbound>`). Everything that
  crosses a boundary (API ↔ frontend) or is used as a search/diff key is an id
  (`node_inbounds.id` for selecting a connection, `node.id` for grouping). The exception is
  matching `inbound_port` to an **external** 3x-ui inbound: the port is an identifier on the
  3x-ui side, not our id.
- **Comparing string contents is forbidden** (`strings.Contains(name, "...")` and the like).
  If you need a trait — introduce a boolean flag or a typed constant. Exceptions are allowed,
  but only with a justification in a comment.

### Errors — sentinel, without interpolation

- Domain errors — sentinel constants in `entity`:
  `var ErrNameTaken = errors.New("name already taken")`. Return them
  (`return entity.ErrNameTaken`), **do not** substitute the name/value into the error text — it is already
  in the caller's context.
- Lower layers — wrapped technical errors (`fmt.Errorf("dep.Method: %w", err)`), without
  human-readable text.
- **Not a single `fmt.Errorf` with Russian/human text in `repository`/`service`/`clients`.**
  Understandable messages (including Russian ones) — only constants on the handler layer.
- **Error classification — in the handler itself, by specific sentinels.** The handler knows
  what it calls and which typed errors it returns: against them — an explicit
  `errors.Is`/`switch`. A known domain sentinel → a typed 4xx response with a
  **local text constant of this handler** (`slog.Warn`); **everything else**
  (DB/panel/marshal — infrastructure) → `return nil, err` (becomes a 5xx) + `slog.Error` with
  context. **A DB error must not be returned as 400.** No general «mapper»
  `UserMessage(err) → text` across all handlers: the texts live as constants in each handler,
  at the place of the check.
- **Uniqueness — from the DB response, without pre-check SELECTs.** A duplicate is caught by a typed
  constraint code (`internal/repository/dberr.IsUniqueViolation`: `errors.As` →
  `*sqlite.Error.Code()` ∈ {`SQLITE_CONSTRAINT_UNIQUE` 2067, `SQLITE_CONSTRAINT_PRIMARYKEY`
  1555}; modernc enables extended codes on every connection — **no string comparison**),
  and the repository translates it into a domain sentinel (`entity.ErrNameTaken` / `ErrNodeNameTaken` /
  `ErrInboundDuplicate` / `ErrRuleProviderNameTaken`). `users.NameTaken`-like pre-checks
  must not exist. A PK gives 1555, an ordinary UNIQUE — 2067; the detector matches both.

### Handlers — ogen from openapi, typed dependencies, structured log

- **The endpoints' contract is described in the OpenAPI schema (`openapi/`), the code is generated by ogen
  (`internal/oas`).** Adding/changing an endpoint = editing the spec (`openapi/<endpoint>.yaml`
  + `$ref` in `openapi/openapi.yaml`), then `go generate ./internal/oas/`, then writing
  a handler against the generated interface. **We do not register routes by hand** — the
  path+method→operation mapping is owned by the generated router. One package per operation in
  `internal/handlers/<action>`, implementing the `oas.Handler` method; a thin composite
  `internal/handlers/api` forwards each operation to its handler (without
  `UnimplementedHandler` — the compiler enforces completeness) and holds the shared
  `SecurityHandler` (cookie `subgen_admin`) and `ErrorHandler`.
- **The ogen server is the only `http.Handler` (the root).** In `cmd/service` it is mounted
  at `/` on a stdlib `http.ServeMux`; alongside it — the **only** external route `/admin/static/*`
  (a file handler, not a typed endpoint — per the ogen guide on the static router).
  **`gorilla/mux` is not used.** Even browser pages (the login page
  `GET /admin/login`, the SPA shell `GET /admin` and `GET /admin/{view}`) are ogen operations
  that return raw HTML or `302`; their session cookie is an **ogen parameter** (`in: cookie`),
  validated by `web.Session.Valid` (pages need a redirect, not a 401 from the security scheme).
- **The response contract is idiomatic:** mutations — `2xx {message}` / `4xx {errMessage}`
  (`common.yaml`: `MessageResponse`/`ErrorResponse`); read endpoints — typed JSON.
  The `/admin/api/*` gate → `401` via the ogen `SecurityHandler`. **Admin is always on**
  (`SUBGEN_ADMIN_PASSWORD` is mandatory in the config; there is no `AdminEnabled`/optional panel).
  Login — `POST /admin/api/login` (200 + `Set-Cookie`), logout — `POST /admin/api/logout`
  (204). No `{ok,msg|err}` envelopes and no server redirects on JSON paths.
- **Value validation — in the service, not in the schema.** Into OpenAPI we put only the shape of the contract —
  `required`/`type`/`format` (ogen generates decode/types and a presence check from them). But
  **value constraints** (`minLength`/`minItems`/`minimum`/`maxLength`/`pattern`/…) we do **not**
  put into the schema: on them ogen returns a generic vague `400 "Bad request"`. Instead
  we validate in the **service layer** with sentinel errors (`entity.ErrValidation*`); the handler is
  thin — it maps the sentinel into a typed 4xx with a local text constant. No service —
  we create one (`internal/service/nodes` owns node validation). **Surrogate ids (PK) we do not
  validate** — a non-existent id (whether negative or valid-but-absent)
  yields not-found either way, a separate `id≥1` check is meaningless. See
  [ADR-0003](docs/decisions/0003-validation-in-code.md).
- A handler's dependency — a concrete interface for the data it needs.
  **The anti-pattern `cfgReader{ Cfg() *config.Config }` is forbidden** — if you need a concrete field,
  pass the concrete field.
- `slog` at the handler level: the message in the form `"handler <name>: <event>"`; variables —
  **only as log fields**, not in the message text
  (`slog.Warn("handler node_delete: delete failed", "id", id, "err", err)`). A domain
  4xx — `Warn`, an infrastructure 5xx — `Error` (see «Errors»). Lower layers do not log.
  **The central `ErrorHandler` (`internal/handlers/api`) logs only what slipped past the
  handlers** — security/request-decoding errors; it does not re-log
  handler-level 5xx (those were already logged by the handler itself with the operation context).
- The backend — **pure JSON endpoints** (`/admin/api/*`) + serving static content; there are no server templates.
  The frontend — a minimal SPA on Vue 3 (global build, no bundler) in
  `internal/handlers/web/static/` (`index.html` + `app.js` + `app.css`), data is
  pulled by fetch. **Serving static content (`render.go`):** by default from the **embed** copy
  (`//go:embed static`, a self-contained prod image), or **live from disk** if
  `SUBGEN_STATIC_DIR` is set (a path to the directory relative to cwd) — then CSS/JS edits are visible on
  reload without rebuilding Go (local development; `assetFS()` picks the source).
  **Libs:** locally vendored (`vue.global.prod.js`, `Sortable.min.js`, `js-yaml.min.js`)
  + **from a CDN** — Monaco (`monaco-editor@0.52.2`, AMD loader). Mind the script order: UMD libs (`js-yaml`) load **before**
  the Monaco loader, otherwise their UMD sees `define.amd` and registers itself as a module instead of
  exposing the global. The base-YAML field — a `yaml-editor` component on **Monaco**
  (`loadMonaco()` lazily brings up the engine from the CDN, `defineSubgenTheme` — a dark theme matching the
  palette; language `yaml`, highlighting/Tab/current-line out of the box). **Live syntax
  validation — `jsyaml.load`** with debounce: the error is placed as a marker in Monaco
  (`setModelMarkers`, squiggle+hover) + a status line (line:col + reason).
  **The admin theme mirrors 3x-ui v3+** (React + Ant Design 6 dark) — it is a «skin» of Ant tokens
  over Bootstrap in `app.css`: bg `#1a1b1f` / card `#23252b` (radius 12) / header `#15161a`
  / modal `#2d2f37`, primary Ant-blue `#1668dc`, borders `rgba(255,255,255,.06–.12)`,
  **system fonts** (no webfonts). Keep any new UI within these tokens.
  Read endpoints (`users_get`/`nodes_get`/`config_get`) return typed JSON;
  mutations (`user_*`/`node_*`/`config_save`) accept JSON and respond with `2xx {message}` /
  `4xx {errMessage}` (the frontend reads by status + field).

### Composition and layers

- **No `App` oracle.** Dependency composition and router assembly — in `cmd/service`.
- Data flows **bottom-up**: `repository`/`clients` → `service` → `handler`. The lower
  layer does not know about the upper one.
- **The cache is a narrow layer** around a specific repository/client (or inside a specific
  service), not a global snapshot of everything.
- `ruleset/mirror`, `fleet/build` and similar logic is the service layer
  (`internal/service/*`); generating mihomo YAML (resolving `PolicyRef`, assembling
  proxy-groups/rules) — `internal/mihomo/render`. Not «magic on the side».

### Infrastructure

- There is no rate limit (dropped). The TLS cert reloader — `internal/cert`. Deploy — Docker
  (not systemd).

## Directory structure

```
cmd/service/main.go            — the service itself (entrypoint)
cmd/<tool>/main.go             — other binaries (CLI utilities, workers, cron)
internal/config/               — loading/validating the config (env + .env)
internal/clients/<dep>/        — clients to external network dependencies (xui, …)
internal/repository/<entity>/  — repositories, split by entity (users, nodes, …)
internal/service/<area>/       — the service layer (business logic)
internal/handlers/<do_some>/handler.go — HTTP handlers (one package per action)
internal/entity/               — shared domain kernel types (layer in/out), without I/O
internal/mihomo/               — the mihomo-config subdomain (schema + decode/validate), without I/O and net/http
internal/mihomo/render/        — generating mihomo YAML from the schema + subscriber
migrations/0001-init.sql       — the schema baseline (the first migration; CREATE … IF NOT EXISTS)
migrations/NNNN-*.sql          — subsequent migrations (ALTER/CREATE), by name = by order
migrations/{embed,run}.go      — the Apply() runner: applying by name, tracking in schema_migrations
```

Layer rules:
- **Dependency flow top-down:** `handlers → service → repository | clients`.
  The lower layer does not know about the upper one. The handler depends on the service, the service — on
  the repositories/clients.
- **One package per action/entity.** An action's handler is a separate package
  `internal/handlers/do_some/`; an entity's repository — `internal/repository/users/`.
- **The cache is a repository layer** for a specific entity (the same contract as
  the «real» repository; the cache wraps/implements it). Not separate
  «magic» on the side.
- **`internal/entity`** — shared domain kernel structs (`Node`, `Inbound`, `User`,
  `Proxy`, `Subscriber`, `Panel*`, `Fleet`, `Connection`); without network calls and without I/O.
- **`internal/mihomo`** — a carved-out mihomo-config subdomain: the schema model
  (`RoutingRule`/`ProxyGroup`/`PolicyRef`/`RuleProvider` + catalogs), its
  decode/validate (form→types, sentinel errors) and `render/` (YAML). This is a
  **deliberate exception** to the «single flat `entity`»: the mihomo-config schema is a
  separate cohesive domain that flows both into the DB (`mihomo_` tables), into the
  admin schema, and into the render. The package's hard rules: `mihomo` does **not import**
  `entity` and `net/http`; references to an inbound/group — only by `int64` id (hence
  no cycle); there is no human error text in `mihomo` — only sentinel constants
  (`ErrGroupCycle`, `ErrMatchNotLast`, …), the mapping into Russian text — on the handler
  (`web.UserMessage`). `render/` — the only one allowed to import both
  `entity` (Proxy/Subscriber) and `mihomo`.

The layout has already been brought to this target: clients — `internal/clients/xui` (a thin
adapter, one method — one file); repositories —
`internal/repository/{users,nodes,routing,configs}` (one method — one file; `configs`
— a type-agnostic config-ownership anchor, see below); services —
`internal/service/{fleet,ruleset,provisioning}`
(`fleet` owns the fleet TTL cache; `ruleset` — the provider mirror); handlers —
`internal/handlers/<action>`; the TLS reloader — `internal/cert`; composition (the ogen server
from `internal/oas` + static content on the stdlib `http.ServeMux`) — in `cmd/service`. The packages
`internal/server`/`App`, `internal/{model,cache,ruleset}` and `config.FromStore`/seed no longer exist.
**There is no `repository.Store` bundle either** — `repository.Open()` returns a `*sql.DB`,
and the per-entity repositories (`users.New(db)`, …) are assembled in the composition root.

**The mihomo config (routing) — structured data, not strings.** The domain types are in
`internal/mihomo`: `RoutingRule`, `ProxyGroup`(an added element), and a single typed
**`PolicyRef`** {`PolicyKind` direct|reject|…|inbound|group, `InboundID`,
`GroupID`} — shared by the rule target and the group element; `RuleType`/`ProxyGroupType` —
type-constants. **No magic strings** — the resolution into a proxy name
by type/id is done by the per-subscriber resolver `internal/mihomo/render/policy.go`
(it sets `entity.Proxy.InboundID` in `fleet/build.go`); inbounds unavailable to the
client are dropped, an empty group → `DIRECT`. `repository/routing` writes everything
atomically (`SaveMihomoConfig(configID, …)`: groups+elements+rules+providers+base);
the mihomo-config tables — with a `mihomo_` prefix, **scoped by `config_id`** (FK to
`subscription_configs` CASCADE; to `node_inbounds` — RESTRICT). References to a
group at the HTTP/save boundary — by **array index** (real ids do not leak outside);
decoding the form (`mihomo.DecodeConfig(raw json.RawMessage)` — the handler extracts the raw
body) and validating it (incl. the acyclicity of the group graph) live in `internal/mihomo`
(`decode.go`/`validate.go`) and return sentinel errors.
The frontend — two visual builders (groups and rules) with a shared `policy-picker` and
drag-n-drop (vendored SortableJS) in `internal/handlers/web/static`. **The frontend hardcodes
nothing** — including the **reference taxonomy**: what the rule target / group element
can point to is declared by the schema per-type. The catalogs live in `mihomo`
(`RuleTypeCatalog`/`ProxyGroupTypeCatalog`/`BuiltinPolicyKinds`/`RuleProviderBehaviors`/
`RuleProviderFormats`/`GeneratedKeys` + `PolicyCategory`/`PolicyCategories` — a single
source) and are served by the `config_schema` handler (`GET /admin/api/config/mihomo/schema`,
sorting the options by name — in the handler) by sections: `actions` (built-in
with labels), `ruleProvider`,
`proxyGroup.types[]` (options + `items` — element categories), `rules.types[]` (options +
`destinations` — target categories), `generatedKeys`. **A reference category** is
`actions`/`inbounds`/`groups`; `policy-picker` draws only the declared ones (the `inbounds`
category = all fleet inbounds, with labels).
All mihomo endpoints — under `/admin/api/config/mihomo` (read / `…/schema` / `…/save` /
`…/customs` / `…/custom/create` / `…/custom/delete`).

**A base + per-user custom configs; the engine is typed, not assumed to be the
only one.** Config ownership — a generalized anchor `subscription_configs(id,
user_id, kind, created_at)`: `user_id NULL` = base (for everyone), otherwise a personal
custom; `kind` — `entity.ConfigKind` (the engine: `mihomo` now, later xray/sing-box) —
**a typed constant, not a magic string**. Each `kind` has its own base +
at most one custom per user (the unique index `COALESCE(user_id,0), kind`). The engine's
content (`mihomo_*`) hangs on the anchor via `config_id`. The layers:
- **`internal/repository/configs`** — a type-agnostic anchor (Base/User ConfigID,
  Ensure, List, Create, Delete), parameterized by `entity.ConfigKind`, **knowing nothing about
  mihomo**. Cloning the base content into a new custom — is delegated to the
  content repository via a narrow `cloner` contract (`routing.CloneConfig`,
  in a shared tx). A custom = a **snapshot**: after the clone it is independent, edits to the base do not reach it.
- **`internal/repository/routing`** — the mihomo content, all reads/`SaveMihomoConfig`
  scoped by `configID`; `AllRuleProviders` (across all configs) — for the mirror.
- **Subscription** — the route `/sub/{kind}/{token}`; `kind` is validated against the renderer registry
  `map[entity.ConfigKind]sub.EngineRenderer` (assembled in `cmd/service`,
  currently mihomo is registered). The handler: token → user (`users.IDBySubID`) → its
  custom for `kind`, otherwise the base → `EngineRenderer.Render(sub, configID)`. The reads of
  mihomo content are hidden **inside** `sub.MihomoRenderer`, the shared handler is
  engine-generic. Adding xray = a new `EngineRenderer` + `xray_*` tables +
  a content repository + one registration line; the anchor/router/admin API do not change.
- **Admin** — the Config tab carries a scope (`?user=<id>` on read; `userId` in the save body);
  the frontend — a selector «Users: All | <nickname>» + «Add custom config…» (clone),
  a custom banner with «Delete». The engine URL is in the path (`/config/mihomo/*`).

**DB migrations — an ordered runner, not by hand.** `migrations.Apply(ctx, db)` (the package
`migrations`: `embed.go` + `run.go`) is called from `repository.Open` on start: `0001-init.sql`
— an immutable **baseline**, then `0002-*.sql`, …; all files are `NNNN-`-prefixed, so
an ordinary name sort = the apply order (without special logic). Each file is applied exactly
once (tracked in `schema_migrations`), in its own transaction; on error — `slog` + crash
(`main` does `log.Fatal`). A structural schema change = a **new `NNNN-*.sql`** — not an
edit of the baseline (otherwise it diverges from already-adopted bases), not in-code migrations, not
`*.manual.sql` (the pattern was removed). Migrations are **pure DDL**: connection PRAGMA
(`journal_mode=WAL`, `foreign_keys`, `busy_timeout`) live in the DSN (`open.go`), since `PRAGMA
journal_mode` cannot be executed inside the runner's transaction. There are no rollbacks (forward-only). If
a migration rebuilds a table via RENAME — set `PRAGMA legacy_alter_table=ON` before the
RENAME (otherwise SQLite rewrites FKs in other tables and leaves dangling references). See
[ADR-0002](docs/decisions/0002-ordered-migration-runner.md).

## Dependencies: `contract.go` + mockgen

An entity's (handler's, service's, …) dependencies are declared as a **private interface
in the package where they are used**, in a `contract.go` file, with a mock-generation
directive. The interface describes exactly the methods this package needs (interface
segregation), and is **named after the concrete dependency it points at** (not an abstract
role). The name must make clear what it is: a repository → `<entity>Repo` (`usersRepo`,
`nodesRepo`, `configsRepo`, `routingRepo`), a service → `<entity>Service` (`fleetService`,
`provisioningService`, `sublinksService`), a client → `<entity>Client` (`panelClient`,
`itemPlatformClient`). Role names that don't reveal repo/service/client (`subLinker`,
`configResolver`, `mihomoReader`, `creator`, `deleter`) are **forbidden** — they make the
code harder to read.

```go
// contract.go
//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package $GOPACKAGE
package curl_generation_core

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

type itemPlatformClient interface {
	Get(ctx context.Context, in entity.ItemGetIn) (map[int64]entity.Item, error)
}

type curlService interface {
	Generate(ctx context.Context, pl entity.CURLGeneratorPayload) ([]entity.CURL, error)
}
```

- `contract.go` + mockgen — for **services, entity and clients** (a client — by
  its generated SDK). The mocks lie alongside (`contract_mocks.go`), generated by
  `go generate ./...` (mockgen is wired in as a `go tool`).
- In service/entity tests, **only the local `Mock*`** from one's own
  `contract.go` are used. **Do not** drag another package's `clients.MockClient` / `clients.New(mock)`
  into a service test — the service has its own private contract for the client.
- The constructor takes dependencies as interfaces: `func New(c itemPlatformClient, …) *Service`.

## External-API clients: DTO → domain

A client in `internal/clients/<dep>/` holds **private wire DTOs** with json tags matching the
dependency's response «as is» (with all its quirks: nesting, string-in-string,
foreign field names) and **maps them into domain types** (`internal/entity`) on output.
Outward, the client returns only `entity.*`, while the DTOs/decoding are a private detail of the package
(anti-corruption boundary). Example: `clients/xui` unmarshals into private
`inbound`/`streamSettings` (settings arrive as a JSON string inside JSON — `decode()`
unwraps them) and converts to `entity.PanelInbound`. This way the domain does not know about the
panel's format, and the service layer is trivially mocked via `contract.go`.

## Public API

- Only exported methods are **tested and called from the outside**. Private
  helpers/renderers are checked **through** the public method that uses them.

## Error wrapping and logging

- **Wrapping — always.** When calling a dependency, wrap:
  `fmt.Errorf("<field/dependency name>.<Method>: %w", err)` — e.g.
  `fmt.Errorf("economicEntitiesClient.GetByUserIDs: %w", err)`. This way the stack reads
  along the call chain.
- Errors from **private methods of the same package** propagate **without re-wrapping**
  (they are already wrapped inside).
- **User-facing error texts are formed only on the presentation layer
  (handler).** The repository/service return «technical» wrapped errors and
  do **not** generate human-readable texts. Understandable messages — constants in
  the handlers package (see `error_messages.go` in the references:
  `MessageEntityNotFound`, `MessageInternalError`, …).
- **Logging — `slog`, at the handler level.** The handler logs the error (with
  the request context) and returns an understandable text to the user. Lower layers do not
  log — they only return a wrapped error.

## Unit tests

A strict table-driven structure. One `Test{Type}_{Method}` per **method** (do not
split into `Test*_Success` / `Test*_Error`).

```go
func TestClient_Method(t *testing.T) {
	targetErr := errors.New("test")

	tt := []struct {
		name string
		// input
		buildMock func(mock *MockClient) // buildMocks(...) if there are several dependencies
		result    SomeOut
		err       error
		wantErr   bool // only if err is not a sentinel
	}{
		{name: "empty"},
		{name: "success.one_user", buildMock: func(m *MockClient) { /* EXPECT */ }, result: SomeOut{ /*…*/ }},
		{name: "error.downstream", buildMock: func(m *MockClient) { m.EXPECT().X(gomock.Any()).Return(nil, targetErr) }, err: targetErr},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mock := NewMockClient(ctrl)
			if tc.buildMock != nil {
				tc.buildMock(mock)
			}

			c := New(mock)
			result, err := c.Method(context.Background(), /* in */)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, result)
		})
	}
}
```

| Rule | Essence |
|---|---|
| `contract.go` + mockgen | Dependency interfaces — in the package's `contract.go`; `//go:generate go tool mockgen -source=contract.go` for services, entity and clients |
| Public API | We test only exported methods; private helpers — through the public one |
| Table-driven | One `Test{Type}_{Method}` per method; `tt []struct{ name, … }`; do **not** split into `*_Success`/`*_Error` |
| Case naming | `name` **without spaces**: `success.one_user`, `error.invalid_user_id`, `empty_fields` |
| Parallelism | `t.Parallel()` on the table **and** in `t.Run`. Exception: if the dependency codegen is not thread-safe (a data race in the generated `New()`/declaration) — without parallel construction |
| Branches | At a minimum: `empty` (if applicable), `success` with a mapping check, `error` from downstream; domain edge cases — by sense |
| Mocks | `mockgen` by one's own package `contract.go` → `Mock*`; do **not** drag a foreign `clients.MockClient` into a service/entity test |
| Assertions | `require.ErrorIs` / `ErrorAs` / `NoError` for errors; `assert.Equal` for the result; `wantErr` / `ErrorContains` — only when there is no specific sentinel |

## Integration / API tests

They live separately in `apitest/` (see `apitest/README.md`): a testify suite against
a real 3x-ui in docker, under the `apitest` build tag, a reusable `Base` +
a suite per area (`UserSuite`, then `NodeSuite`/`ConfigSuite`), one file per
scenario. Turnkey run: `make -C apitest test`.

- **This is a black box — request bodies are NOT built from generated structs (`internal/oas`).**
  Only `map[string]any` / hand-rolled structs / raw JSON. Otherwise the test would send with the same
  type the server decodes with, and a bug in request mapping (a renamed field, a wrong
  tag) would be invisible — encode+decode with one type give a consistent but wrong pair.
  This way apitest checks the real wire contract (field names, status codes, error texts).
- Required arrays (`groups`/`rules`/`providers`/`inbounds`/…) are decoded strictly by the server: `null`
  (what a nil slice is JSON-encoded into) is rejected — send empty `[]`.

## Working on a change — worktree, branch, PR

Every feature/fix is developed in its **own git worktree on its own branch** — never
directly on `main` in the main checkout. This keeps `main` clean and switchable, isolates
parallel work, and makes branch → commit → push → PR the only path into `main`.

- **Create a worktree** off a clean `main`:
  `git worktree add -b <type>/<slug> .worktrees/<slug>` — `<type>` ∈ `feat`/`fix`/`chore`
  (e.g. `feat/subscription-link-catalog`). Worktrees live **in-repo** under `.worktrees/`,
  which is gitignored, so the checkouts never show up in `git status`.
- **Do all development and commits in the worktree** (`cd .worktrees/<slug>`); the main
  checkout stays untouched.
- **Run the full check before pushing** (`make all` — build+vet+lint+unit+integration+
  apitest) and confirm it is green; a green push is not a substitute for the gate.
- **Push the branch and open a PR.** Do **not** push to or merge straight into `main` —
  changes land via a reviewed, approved PR (squash-merge). Each PR carries its CHANGELOG
  entry (and an ADR for non-trivial changes — see below).
- **After merge**, clean up: `git worktree remove .worktrees/<slug>` and delete the branch.

## Documenting changes — CHANGELOG + ADR

Every PR leaves a trace, so that «what and why we changed» does not get lost in the
git/GitHub history. This is a repository rule, not an option:

- **`CHANGELOG.md`** (root) — **one entry per PR**, reverse-chronologically.
  Format: `## YYYY-MM-DD — <short title> (#<PR>)` + 1–2 lines of essence + a link to
  an ADR, if there is one. **No version sections** — the service has no releases/tags, the deploy is
  continuous. The entry is added in the same PR as the change.
- **ADR** — for **non-trivial** changes (there is a design decision, a choice between
  options, a non-obvious trade-off): `docs/decisions/NNNN-<slug>.md`, a continuous
  4-digit numbering, by the template `docs/decisions/0000-template.md` (sections **Context /
  Considered Options / Decision / Consequences**) — the problem, the considered options,
  the rationale for the choice. The CHANGELOG entry links to the ADR.
- **Trivial** changes (a typo, a dependency bump, a small thing without forks) — only
  a line in CHANGELOG, without an ADR.
- An ADR is **immutable after merge**. A decision that cancels a previous one is a new ADR with a
  `Supersedes 000X` link; the old one's status changes to `Superseded by 000Y`.
