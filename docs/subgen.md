# subgen — mihomo subscription server

`subgen` serves per-client mihomo (Clash.Meta) subscription configs and ships a
web admin panel. It exists because 3x-ui's built-in Clash subscription only emits
a flat `proxies` list — no proxy-groups, rules, or rule-providers — so it can't
express the routing UX we want. Engineering detail and layout are in
[`../README.md`](../README.md); this page is the design rationale and
operations.

## Why a custom server

We evaluated reusing 3x-ui's clash sub (insufficient: proxies-only, hardcoded
template, first inbound only) and converters like subconverter/Sub-Store (extra
service + template wrestling). A small Go server gives full control of the
selector UX, rule-providers, per-client tokens and auto-propagation, with a tiny
footprint suitable for the 1 GB RU1 node.

## Configuration model

Two clean halves:

- **Bootstrap** → environment / a local `.env` in the repo (`.env`,
  gitignored, next to `.env.example`): listener, TLS cert/key, `SUBGEN_SECRET`,
  admin creds, `SUBGEN_PUBLIC_BASE`, db path. Empty TLS cert/key → plain HTTP
  (local dev).
- **Operational data** → the **SQLite store** (`db/subgen.db`): the node
  registry, the mihomo **proxy-groups + routing rules**, rule-providers, the base
  YAML, and the **user records**. All edited through `/admin`. A fresh store is
  **empty** — no defaults are seeded; the operator fills in everything through the
  panel. There is no `routing.yaml`. Inside the one file, tables are split logically:
  mihomo-config tables are prefixed **`mihomo_`** (`mihomo_proxy_groups`,
  `mihomo_proxy_group_members`, `mihomo_routing_rules`, `mihomo_rule_providers`,
  `mihomo_settings`); subgen-admin tables (`nodes`, `node_inbounds`, `users`,
  `user_connections`) are unprefixed. (SQLite has no in-file schemas, and FKs can't
  cross attached databases — one file keeps the inbound↔rule/member FKs working.)

## Design

```
node registry (SQLite)  +  3x-ui /panel/api/inbounds/list  ─►  BuildFleet  ─►  render mihomo YAML
                                                                              │
client GET /sub/{token}  ──(token = HMAC(secret, subId))──►  resolve subId ───┴─►  YAML + headers
```

- **Source-of-truth split:** 3x-ui holds the raw Xray plumbing (inbounds, client
  UUIDs, REALITY keys), read via the Bearer API. subgen's store owns *who/how
  routes* (nodes, proxy-groups, routing rules, users) and provisions clients into
  3x-ui from it.
- **Auth:** Bearer API token per panel (3x-ui >= 3.2). `Authorization: Bearer
  <token>` — no login/cookie/CSRF. Issue with `x-ui setting -getApiToken`.
- **Routing is operator-defined, typed, per-subscriber:** the operator builds
  mihomo **proxy-groups** (e.g. a `select` switcher `🎯 Подключение`) and **routing
  rules** in the panel. A rule's target and a group's members are the **same typed
  `PolicyRef`** — a built-in policy, an **inbound** (by id), or another group (no
  magic strings; resolved by typed `PolicyKind`). `render` resolves each ref
  for the subscriber, **dropping** an `inbound` ref the subscriber lacks (from
  rules and group members) and falling an empty group back to `DIRECT`. So the
  generated config is always referentially intact and auto-scales as nodes/inbounds
  are added. A proxy's wire-name is the inbound's **label** `<node>-<inbound>`
  (e.g. `🇷🇺 RU1-force`), unique across the fleet.
- **Users are service-owned:** created in the panel (a unique nickname + any number
  of inbounds); subgen mints a `sub_id` and stores one
  `user_connections` row per chosen inbound. On each panel the user is **one
  3x-ui client** (email = nickname, one uuid, the shared sub_id) bound to all its
  inbounds there via a single `addClient(inboundIds=[…])` — 3x-ui's native
  multi-inbound client, so delete-by-email and inbound edits stay unambiguous
  (see AGENTS.md “Client identity model”). Editing re-binds that client,
  preserving its uuid; subgen tracks per-inbound health.
- **Tokens:** `token = HMAC-SHA256(secret, subId)`, so proxy UUIDs never appear in
  the URL. Rotating `SUBGEN_SECRET` invalidates all links.
- **Auto-propagation:** `Profile-Update-Interval` header drives client refresh. A
  rule-provider has **two independent TTLs**: `interval` (the mihomo client's ruleset
  auto-update, always rendered into the YAML) and `mirror_interval` (subgen's mirror
  refresh, used only when mirroring is on) — both edited in the per-provider **edit
  modal** on the Конфиг Mihomo page. A **«проверить»** button there probes the URL
  (`POST /admin/api/config/mihomo/provider/check`, saves nothing): reachable? file
  present? content matches the declared format? — `.mrs` is detected by the **zstd**
  frame magic `28 B5 2F FD` (an `.mrs` is a zstd container, so no zstd dependency is
  pulled in), `yaml` by a parsed `payload:` list, `text` by non-HTML UTF-8 rule lines.
  **Manual prod migration** (no in-code migrations):
  `ALTER TABLE mihomo_rule_providers ADD COLUMN mirror_interval INTEGER NOT NULL DEFAULT 0;`
- **Resilience:** fleet data is cached (`SUBGEN_CACHE_TTL`, default 5m) with
  stale-on-error; rule-provider files are mirrored from GitHub and served from
  `/rules/<name><ext>` (RU networks often can't reach GitHub). The xui client uses
  the default system resolver — RU1's flaky host DNS was fixed at the host level, so
  the old custom-resolver workaround is gone.

## Operations

- **Where:** runs on RU1 as a **Docker container** from the git checkout at
  `/home/server/subgen` (`docker-compose.yml`, `env_file: .env`, store
  bind-mounted from `db/`, TLS via the acme cert mounted read-only, public
  port `2097/tcp`). The legacy systemd unit is stopped/disabled.
- **Edit routing/nodes/users:** all in the `/admin` panel (see below). A single
  Save on the Конфиг Mihomo page persists proxy-groups + rules + providers + base
  YAML in one transaction; it takes effect on the next `/sub` request (the store is
  read live). Node/user actions invalidate the fleet cache so proxies refresh
  immediately.
- **Edit bootstrap/secrets:** change `.env`, then redeploy (or
  `docker compose up -d` on the node). Changing which rule-providers are mirrored
  also needs a restart (the mirror set is fixed at startup).
- **Deploy:** `deploy.sh` — builds the `linux/amd64` image locally, ships it
  over SSH (`docker save | ssh | docker load`, no registry), then `docker compose
  up -d` on the node. First run stops the legacy systemd unit (clean cutover) and
  chowns `db/` to the container's nonroot uid. Go/registry **not** required
  on the node (Docker is). It does **not** push to git. Prompts for sudo.
- **Debug:** `go run ./cmd/subctl -dump-fleet` (lists every subId, its token and proxies);
  `-print <subId>` (renders one config to stdout). Both read `./.env`.
- **Run locally:** `cp .env.example .env`, set `SUBGEN_SECRET`,
  leave TLS empty, `go run ./cmd/service` → http://127.0.0.1:2097/admin.
- **API tests:** `make -C apitest test` spins up two clean 3x-ui panels in
  docker, runs the create/edit/delete/recreate scenarios against them, and tears
  down — turnkey, no risk to prod. See `apitest/README.md` (the scenario
  catalog). They're behind the `apitest` build tag, so `go test ./...` skips them.

## Admin panel

A minimal **Vue 3 SPA** at **`/admin`** (Bootstrap CSS for styling + Vue global
build, no build step; enabled when `SUBGEN_ADMIN_USER`/`SUBGEN_ADMIN_PASSWORD` are
set). The backend serves a static shell (`index.html`) + pure JSON read endpoints
under `/admin/api/*`; the SPA fetches them and posts mutations back as JSON. Login
over the same TLS; session is a 12h HMAC-signed cookie. Three sections:

- **Пользователи** — list (nickname, `subId`, connections, traffic, subscription
  link, health). **Новый пользователь** is a collapsed panel (click to expand): a
  unique nickname (`^[a-z0-9_-]{1,32}$`, also the 3x-ui client email) + any number
  of inbounds (checkboxes, ≥1). **Изменить** opens a modal to re-assign connections —
  `EditUser` reconciles per panel and re-binds that panel's single client to the
  new inbound set (preserving its uuid). **Delete** removes the user's client from
  every panel (by email = nickname) then the store row. Drifted clients get a
  **Пересоздать** button (full per-panel re-bind). **Collision guard:** if a target
  panel already has a client with this nickname (`email`) — an orphan, or a foreign/
  manual client — on a panel the user doesn't yet own, create/edit **abort and change
  nothing** (no store row, no panel write) with an error naming the panel; subgen
  **never deletes a client it doesn't own**, so resolve it on that panel. We only
  delete-and-re-add on panels the user already owns (re-bind / Пересоздать).
  `subId`/`uuid` are random per user and never collide in practice.
- **Узлы** — the node registry (CRUD): name, 3x-ui base URL + path, Bearer token
  (write-only), and an **inbound editor** — add **any number** of
  inbounds, each a `name` + `port` row (`name` is an ASCII `[a-z0-9-]` label like
  `force`/`smart`, unique within the node; the port is likewise unique within the
  node). **Новый узел** / **Изменить**
  open in a **modal popup**. Existing inbounds round-trip by `node_inbounds.id`
  (the form sends it back), so editing a port keeps the id stable and the bound
  users intact. The **Панель** column is a link to the panel UI. Fields are
  validated server-side (`validateNode`): host for clients = bare host **or IP**
  (no scheme/port), base URL = `https://host:port` only (host may be IP; no path),
  ports 1–65535, ≥1 inbound required. Deleting a node — or removing an inbound from
  it — is **blocked while that inbound is still referenced**: by a user connection
  **or** by a mihomo rule / proxy-group member (FK RESTRICT + a pre-check returns a
  clear error naming the node/inbound and the reason); detach those first. A new
  node's inbounds become available as **inbound PolicyRef options** in the Конфиг
  Mihomo constructors (wire them into a group/rule to route to them).
- **Конфиг Mihomo** — two visual **constructors** plus rule-providers and base YAML:
  - **Proxy-groups** — operator-defined mihomo proxy-groups (the former hardcoded
    group and the connection selector are now ordinary rows). Each group has
    a name, a type (`select`/`url-test`/`fallback`/`load-balance`/`relay`, with
    health-check url/interval/tolerance shown by type) and an **ordered list of
    members**, each a typed **PolicyRef** (a built-in policy / an inbound /
    another group). Drag-to-reorder (SortableJS).
  - **Правила** — ordered rule rows: a **type** select (mihomo matcher, grouped),
    a **value** (a rule-provider select for `RULE-SET`, hidden for `MATCH`, else a
    text input), a **no-resolve** toggle (IP matchers / `RULE-SET`), and a **target**
    PolicyRef picker. Drag-to-reorder; inline hints flag a missing/misplaced `MATCH`,
    a `RULE-SET` pointing at an unknown provider, or a dangling group reference.
  - **mrs rule-providers** (add/del rows) and the **base YAML** textarea (everything
    except the generated sections; save rejects `proxies`/`proxy-groups`/`rules`/
    `rule-providers`).
  No magic strings: a rule/member target is a **typed `PolicyRef`**
  (`PolicyKind` + an optional inbound id / group id) resolved per-subscriber at
  render (`inbound`→the client's proxy for that inbound,
  dropped if the client lacks it; group/built-ins always resolve). Group references
  travel as **array indices** (the API never exposes raw group ids). One **Save**
  applies groups + rules + providers + base in a single transaction; it takes effect
  on the next `/sub` request (the store is read live).
  **Schema-driven UI:** the frontend hardcodes nothing — it fetches
  **`GET /admin/api/config/mihomo/schema`** (built from the `mihomo` catalogs) and
  renders every select/toggle/picker from it. The schema is per-section:
  `actions` (built-in policies, with labels), `ruleProvider` (`behaviors`/`formats`),
  `proxyGroup.types[]` (each: type, `usesHealthCheck`/`usesTolerance`, and **`items`**
  — the reference categories its members may point at), `rules.types[]` (each: type,
  `takesProvider`/`supportsNoResolve`/`isMatch`, and **`destinations`** — the
  categories its target may point at), and `generatedKeys`. A **reference category**
  is `actions` / `inbounds` / `groups`; the `policy-picker` renders only the
  categories the schema declares for the current type (the `inbounds` category =
  all fleet inbounds, with labels), so the taxonomy isn't baked into the UI. The
  mihomo-config endpoints live under **`/admin/api/config/mihomo`** (read), `…/schema`,
  `…/save`.

The user form (a modal) lists **one checkbox per inbound** across all nodes (≥1),
labelled by the inbound **label** `<node>-<inbound>`. Options render node
**names**/inbound names but submit **inbound
ids** (`node_inbounds.id`, from `/api/nodes`' per-node `inbounds[]`) — the backend
resolves by id, never by the mutable node name. All mutating actions go
through **`fetch` + JSON**: success shows a toast (bottom-centre, with a fade/slide
transition) and the SPA re-fetches the affected list; while a mutation is in flight
every row action is disabled (the acting row shows a spinner); an error toasts and
leaves the form untouched, so nothing is lost. The SPA (`index.html`, `app.js`, `app.css`) + vendored
Vue/Bootstrap/SortableJS/js-yaml are served from the binary's **embedded** copy by
default (no CDN — RU1 DNS/censorship), or **live from disk** when `SUBGEN_STATIC_DIR`
points at the assets dir (edit + browser-reload, no Go rebuild — for local dev); the
YAML editor (Monaco) loads from a CDN. Rotate the admin password by editing
`SUBGEN_ADMIN_PASSWORD` in `.env` + `restart`.

## Issuing a subscription to a client

1. `/admin` → **Пользователи** → create (name + one or more inbounds) → **Копировать**
   the link, or read it from the row.
2. Hand them `https://ru1.freedom.postlog.ru:2097/sub/<token>`.
3. They add it as a subscription in ClashMi (see [clash-clients.md](https://github.com/Postlog/vpn-toolchain/blob/main/docs/clash-clients.md)).

**Пересоздать** keeps the same `subId` (it only re-binds the panel clients), so
the subscription link stays valid. Only deleting and creating a new user mints a
new `subId` → a new link.

## Adding a node to the subscription

1. `/admin` → **Узлы** → add the panel (base URL + secret path + Bearer token) and
   one or more inbounds (`name` + `port`).
2. `/admin` → **Конфиг Mihomo** → reference the new inbound where you want it: as an
   **inbound** member of your `🎯 Подключение` switcher group, and/or as a rule target.
   (Unlike the old auto-built selector, group membership is now explicit data.)
3. Save — it takes effect on the next `/sub` request; no file edits, no restart. The
   proxy's wire-name is the inbound's label `<node>-<inbound>`, unique across the fleet.
