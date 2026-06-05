# subgen

Per-client **mihomo (Clash.Meta) subscription server**. It renders a full mihomo
YAML config per subscriber and serves it at `/sub/{kind}/{token}`, and ships a small admin
panel for managing nodes, the mihomo config (proxy-groups + routing rules) and users.

There is one shared **base** config plus optional **per-user custom configs** (a full
snapshot of the base that the operator then edits freely); a subscriber is served their
custom config when one exists, else the base. The engine is a URL segment (`{kind}`,
`mihomo` today) so the same token can serve other formats later (xray/sing-box).

It exists because 3x-ui's built-in Clash subscription only emits a flat `proxies`
list — no proxy-groups, rules or rule-providers — so it can't express the routing
UX we want. See [`docs/subgen.md`](docs/subgen.md) for the full design.

## Configuration model

Two clean halves:

- **Bootstrap** (listener, TLS, secret, admin creds, db path) → environment
  variables, loaded from a local [`.env`](.env.example) file. Nothing secret in git.
- **Operational data** (nodes/panels, proxy-groups, routing rules, rule-providers,
  the base YAML, users, per-user custom configs) → the **SQLite store** (`db/subgen.db`),
  edited entirely through the admin panel at `/admin`. A fresh store starts **empty** —
  the operator fills in everything through the panel. No defaults are seeded. Inside the
  one file the tables are split logically: the config-ownership anchor is
  `subscription_configs`, mihomo-config tables are prefixed `mihomo_` (scoped by
  `config_id`), subgen-admin tables (nodes/inbounds/users) are not (SQLite has no in-file
  schemas and FKs can't cross attached DBs, so a single file keeps the inbound↔rule/member
  FKs intact).

There is no `routing.yaml` anymore — it was split into the two halves above.

## Routing config (proxy-groups + rules)

The **Конфиг Mihomo** tab is a visual constructor with two halves, both reordered by
drag-and-drop:

- **Proxy-groups** — operator-defined mihomo proxy-groups (a `select` group named
  e.g. `🎯 Подключение` is the connection switcher; there is no hardcoded group
  anymore). Each group has a type and an ordered list of **members**.
- **Правила** — ordered routing rules; each has a mihomo matcher type, a value, an
  optional `no-resolve`, and a **target**.

A rule target and a group member are the **same typed reference** — a `PolicyRef`:
a built-in policy (`DIRECT`/`REJECT`/…/`PASS`), an **inbound** (by id), or another
**group**. There are no magic strings — the target is resolved by typed `PolicyKind`.
At render time each ref is resolved for the subscriber; an `inbound` ref the subscriber
lacks is **dropped** (from rules and from group members), and a group left empty
falls back to `DIRECT`, so the config always stays referentially intact and
auto-scales as nodes/inbounds are added.

## Per-user custom configs

By default every subscriber renders from the shared **base** config. The **Конфиг
Mihomo** tab has a scope selector (**Пользователи: Все** = base, or a specific user);
**Добавить кастомный конфиг…** clones the base into an independent **custom config**
bound to that user, which you then edit like any other config. It is a **snapshot** —
later base edits do not propagate to existing custom configs. **Удалить** drops the
custom config and the user falls back to the base.

Ownership lives in a small engine-agnostic anchor table `subscription_configs`
(`user_id NULL` = base, one custom per user per engine); the `mihomo_*` content tables
are scoped to it by `config_id`. On a `/sub/{kind}/{token}` request subgen resolves the
token → user → their custom config for that engine, else the base. Engine selection is
a per-`kind` renderer registry (mihomo today) — adding xray/sing-box is a new renderer
+ content tables on the same anchor.

## Run locally

```sh
cp .env.example .env          # then set SUBGEN_SECRET (openssl rand -hex 32)
                              # leave SUBGEN_TLS_* empty → plain HTTP, no cert needed
go run ./cmd/service           # reads ./.env, creates db/subgen.db, listens on 127.0.0.1:2097
```

Open <http://127.0.0.1:2097/admin> (admin / your `SUBGEN_ADMIN_PASSWORD`). Add a
node under **Узлы**, then create a user under **Пользователи** and copy its
subscription link. The store lives in `db/` (gitignored); delete it to start fresh.

Debug helpers:

```sh
go run ./cmd/subctl -dump-fleet        # print every subId, its token and proxies
go run ./cmd/subctl -print <subId>     # render one config to stdout
```

Node, user and routing-config edits take effect immediately — the next `/sub`
request reads the store live, and the fleet cache is invalidated on writes. The
rule-provider **mirror** set is fixed at startup, so changing which providers are
mirrored needs a restart.

## Docker

Production runs subgen as a **Docker container** (`Dockerfile` → multi-stage,
distroless static, nonroot). Run it locally with compose:

```sh
docker compose build                          # static binary → distroless image
mkdir -p db && sudo chown -R 65532:65532 db   # nonroot (uid 65532) writes the SQLite store
docker compose up -d                          # reads ./.env, persists ./db, listens on :2097
```

Set `SUBGEN_LISTEN=0.0.0.0:2097` in `.env`. For TLS, point `SUBGEN_TLS_CERT/KEY`
under `/certs` and mount the cert dir (see `docker-compose.yml`).

## Deploy

Production deploy is a **manual GitHub Actions workflow**
([`.github/workflows/deploy.yml`](.github/workflows/deploy.yml)): the image is built on
the runner (the node is RAM-starved and can't), streamed to the server over SSH, and
run with `docker compose`. Tests gate the deploy; bootstrap secrets are injected into
the server-side `.env` from the `prod` Environment.

```sh
gh workflow run deploy.yml -f ref=main     # or: Actions → Deploy → Run workflow
```

It runs `lint + unit + integration`, then on the server: `docker load` the image,
render `.env`, `docker compose up -d` (with `docker-compose.prod.yml`), and a
`/healthz` check. The `db/` bind-mount (panel tokens, nodes, users) **persists across
deploys** — CD never touches it.

**One-time setup** (operator):

- **GitHub → Settings → Environments → `prod`:**
  - **Secrets:** `SUBGEN_SECRET`, `SUBGEN_ADMIN_USER`, `SUBGEN_ADMIN_PASSWORD` (the
    *live* values — don't rotate `SUBGEN_SECRET`, it would invalidate every subscription
    link; the admin login is treated as a credential too), `DEPLOY_SSH_KEY` (a dedicated
    deploy private key).
  - **Variables:** `DEPLOY_HOST`, `DEPLOY_PORT`, `DEPLOY_USER`, `DEPLOY_DIR` (`subgen`),
    `DEPLOY_KNOWN_HOSTS` (`ssh-keyscan -p <port> <host>`), `SUBGEN_PUBLIC_BASE`,
    `SUBGEN_TLS_CERT`, `SUBGEN_TLS_KEY` (paths under `/certs`), `CERT_HOST_DIR` (host
    cert dir, e.g. `/root/cert/<domain>`).
- **Server:** Docker installed + the deploy user in the `docker` group; append the
  deploy key's public half to `~/.ssh/authorized_keys`; once,
  `mkdir -p ~/subgen/db && sudo chown -R 65532:65532 ~/subgen/db`. No git checkout
  needed — the workflow ships the compose + `.env`.
- **Cert perms:** the image runs **nonroot** (uid 65532), so the TLS privkey in
  `CERT_HOST_DIR` must be world-readable (`chmod 644`) for the container to read it —
  acme's `--reloadcmd` keeps it that way and restarts the container on renewal.

Roll back by re-running the workflow with an older `ref`. Once the node moves to a
beefier host, you can drop the build/ship steps and switch to server-side
`docker compose up -d --build`.

## How it flows

```
node registry (SQLite)  +  3x-ui /panel/api/inbounds/list  ->  BuildFleet  ->  render mihomo YAML
        (Bearer token)                                                            |
client GET /sub/{kind}/{token} --(token=HMAC(secret,subId))-->  resolve subId  ---------+--> YAML + headers
```

- Panels (3x-ui >= 3.2) are read with `Authorization: Bearer <token>` — no login/CSRF.
- `settings` / `streamSettings` may be JSON objects (3.x) or strings (legacy); both handled.
- `token = HMAC-SHA256(secret, subId)` — proxy UUIDs never appear in the URL.
- Fleet data is cached (`SUBGEN_CACHE_TTL`, default 5m) with stale-on-error.
- Mirrored rule-providers are fetched in the background and served from `/rules/<name><ext>`.

## Layout

```
cmd/service/            composition root: load config, wire services, gorilla/mux router, TLS, shutdown
cmd/subctl/             CLI utility: -dump-fleet / -print <subId>
migrations/init.sql     SQLite schema (embedded, applied on open)
migrations/*.manual.sql one-off hand-run migrations (NOT auto-applied; e.g. per_user_configs)
internal/entity/        kernel domain types + sentinel errors (User, Node, Inbound,
                        Fleet, Subscriber, Proxy, …)
internal/mihomo/        mihomo-config subdomain: schema (RoutingRule, ProxyGroup, PolicyRef,
                        RuleProvider, catalogs) + decode/validate (sentinel errors); no entity/net-http import
internal/mihomo/render/ mihomo YAML generation (proxies, proxy-groups, rules; per-subscriber PolicyRef resolver)
internal/config/        .env bootstrap load (env tags) + validation
internal/clients/xui/   3x-ui API client (stateless; panel passed per call; one method per file)
internal/repository/    SQLite: Open() -> *sql.DB; users/ nodes/ routing/ configs/ (per-entity, one method per file)
                        configs/ is the engine-agnostic config-ownership anchor (base vs per-user custom)
internal/service/fleet/        fetch panels + BuildFleet + narrow TTL cache (stale-on-error)
internal/service/ruleset/      background mirror of rule-provider files
internal/service/provisioning/ user create/edit/delete/recreate + panel reconcile
internal/handlers/web/  shared HTTP kit: renderer (static SPA), session, JSON, user-facing message mapping
internal/handlers/<action>/  one package per action (contract.go + handler.go)
internal/cert/          TLS cert reloader (reloads on file change)
internal/token/         HMAC sub tokens
```

Code style & conventions for this layout: **[AGENTS.md](AGENTS.md)**.
