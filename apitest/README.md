# subgen API tests (black box: a real server + a real 3x-ui)

Integration tests that drive the **real subgen server** through its HTTP API and
check both the API responses and the actual state of clients on the inbounds of **real**
3x-ui 3.2.6 panels brought up in docker.

This is an honest black box: the test builds the subgen binary (`go build`), starts it as a
**separate process** (a temporary SQLite DB, test admin creds, a free
loopback port, plain HTTP), logs in and hits the real endpoints
(`/admin/api/...`, `/sub/{kind}/{token}`, `/rules/{file}`, `/healthz`). No access to the
services/repositories from the inside — the only entry is the same API the operator and the SPA use.
This way exactly those bugs are caught that unit tests cannot (the semantics of `del/:email`,
a multi-inbound client, handler behavior and the response contract — `2xx {message}` /
`4xx {errMessage}` (ogen), status codes, the exact error text, etc.).

## Layout by package

```
apitest/api/        — the shared support package (NOT _test, under the apitest tag):
                      the SDK Client + server start + the Base suite + the 3x-ui «ground truth» check.
apitest/auth/       — POST /admin/api/login + GET /admin/login (the page), POST /admin/api/logout,
                      the session gate (401), static + the SPA shell.
apitest/users/      — /admin/api/users/{create,edit,delete,recreate} + GET /admin/api/users.
apitest/nodes/      — /admin/api/nodes/{save,delete} + GET /admin/api/nodes.
apitest/config/     — /admin/api/config/mihomo (read / schema / save / provider/check;
                      base vs per-user custom: customs / custom/create / custom/delete).
apitest/sub/        — /healthz, /sub/{kind}/{token}, /rules/{file}.
```

Each `apitest/<area>` is a separate `*_test` package importing `apitest/api`. Inside —
one file per endpoint/scenario; corner cases — pinpoint subtests
(`s.Run("dotted.case", …)`). At the start of each file — a **checklist** of all considered
corner cases, and a test is written for each one (happy path + all validation errors, authorization,
not-found, constraint violations, boundary input, idempotency).

## What docker is and is not needed for

Some areas **do not require panels** — they bring up subgen and work on their own; they can be
run in ordinary CI without docker:

| Area | Need panels? | What is inside |
|---|---|---|
| `auth` | no | login/logout/gate/shell — the panel is not needed |
| `config` | no | the routing config in its own store; provider/check — against a **local** httptest server in the test itself |
| `sub` (`SubSuite`) | no | `/healthz`, the `/sub` 404 paths, and the **mirror** `/rules/<file>` via a local upstream |
| `sub` (`SubPanelSuite`) | yes | a valid `/sub` for an actually provisioned user |
| `users` | yes | provisioning clients on the panel |
| `nodes` | yes | the basic fleet N1/N2 is set up on the panels in `SetupSuite` |

The gate — `api.SkipUnlessConfigured(t)` in the suite runner: without `SUBGEN_APITEST_PANEL1_URL`
the panel-dependent suites are **skipped**, while the ungated ones (auth/config/SubSuite) still
run.

## Running

**Without docker** (runs auth/config/SubSuite, skips the rest):

```
go test -tags apitest -count=1 ./apitest/...
```

**Turnkey** (with panels — also runs the panel-dependent suites):

```
make -C subgen/apitest test
```

The `test` target: `docker compose up -d` (two clean panels) → waits for readiness → grabs from
each the auto-generated API token (`x-ui setting -getApiToken`) → forwards it into env →
`go test -tags apitest ./apitest/...` → `docker compose down -v` (even on failure).
It requires docker + `docker compose` + free ports **13053/13054**. Building the subgen binary and
starting the process is done by the test itself.

Manual mode: `make -C subgen/apitest up`, then the tests against any ready panel via env
(`SUBGEN_APITEST_PANEL1_URL`, `_PANEL1_TOKEN`, `_PANEL2_URL`, `_PANEL2_TOKEN`);
`make -C subgen/apitest down` to stop. Without the `apitest` tag, an ordinary `go test ./...` does not touch these
packages at all.

## `apitest/api` — the shared support

- **`Client` (the SDK)** — a typed HTTP SDK to the running server. The core is
  `do(method, path, reqBody, &out)` + a cookie jar and **capturing the session cookie** from the
  login response (the cookie is `Secure`, so the jar will not return it over plain HTTP — the SDK substitutes it
  itself; production is not touched by this). Per endpoint — a typed method. **Request
  bodies are built NOT from generated structs** (`internal/oas`), but from `map[string]any` /
  hand-rolled structs / raw JSON — this is a black box: if the test sent with the same type
  the server decodes with, a bug in request mapping (a renamed field, etc.) would be
  invisible (encode+decode with one type give a consistent but wrong pair). Mutations
  are normalized into `Result{Status, OK, Msg, Err}` over the ogen contract (`2xx {message}` →
  `Msg`, `4xx {errMessage}` → `Err`), read endpoints are decoded into hand-rolled
  `User`/`Node`/`Config`/`Schema`. For corner cases there are «raw» forms (`PostRaw`,
  `LoginRaw`, `SaveConfigRaw`, `Get`/`GetURL`) returning the status/body/headers. **A gotcha
  (the ogen schema):** required arrays (`groups`/`rules`/`providers`/`inbounds`/…) are decoded
  strictly by the server — `null` (what a nil slice is JSON-encoded into) is rejected, so the SDK
  sends empty arrays `[]`, not `null`.

- **Server start** — `StartServer(t)` / `StartServerWith(t, Options)`: builds the binary
  (`go build` in `t.TempDir()`), starts it as a process with a temporary DB, test creds and
  a free port, waits for `/healthz`, hangs a cleanup. `Options{DBPath}` allows reusing the
  store between two starts — this is needed for the **mirror** of rule-providers: the set of served
  files is fixed at start, so the test starts the server, saves a mirror provider through the
  API, shuts it down, and starts a second time on the same DB.

- **The `Base` suite** — embedded by the areas (`api.Base`): `SetupSuite` brings up the whole stack once
  (seeds inbounds on the panels, builds+starts the server, logs in, registers N1/N2 through
  the API). It exposes `API()` (the SDK), `XC()` (a direct 3x-ui client for the «ground truth»), `Pan1()/Pan2()`,
  and tooling — `ClientUUID`/`RequireClient`/`RequireNoClient` (they read the panel directly),
  `InboundID(node, inbound)`, `PanelInboundID`, `UniqueName(prefix)`.

  The areas that do not need a panel (auth/config/SubSuite) **do not** embed `Base` — they themselves
  do `api.StartServer(t)` + `api.New(...)` without registering nodes.

## Test topology (for the panel-dependent suites)

- **N1** (panel 1): a smart inbound :4433, a force inbound :8443.
- **N2** (panel 2): a smart inbound :9443, a force inbound :9444.

The inbounds are created by the test itself (`Base.SetupSuite`) via `POST /panel/api/inbounds/add`; the nodes —
via `POST /admin/api/nodes/save`. A user = one 3x-ui client per panel (one uuid,
email = nickname, shared subId), bound to all of its inbounds on this panel.

## Error texts

The tests check the **exact** text the API returns (`Result.Err`). The production strings
live on the presentation layer and are not exported, so in each area there is a neighboring
`messages_test.go` with constants mirroring those strings — this is the seam between
the (unexported) production text and the assertions. The interpolated messages of the node validator
(`web.ValidateNode`) are checked by a stable substring.

## Isolation

The panel-dependent suites survive the whole run; each case takes a unique name and cleans up
its own users/nodes in `t.Cleanup`. The ungated suites bring up **their own** process with a clean
DB, so config records/garbage do not interfere with others. The suites are intentionally not marked `t.Parallel()`:
within a suite the cases share one server and one store, and the codegen of the process start must not run
concurrently (an exception to the general parallelism rule — as in the unit style).

## One nuance with plain HTTP

The admin session cookie is marked `Secure` (see `internal/handlers/web/auth.go`), so the
Go cookie jar **will not send** it back over `http://`. To stay on plain HTTP and **not
change production**, the SDK captures the cookie from the login response itself and substitutes it into every request —
exactly what a browser would do over HTTPS. Everything else (creating nodes, provisioning, the subscription,
the rules mirror) is driven strictly through the public HTTP API.
