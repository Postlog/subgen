# 0008 — Subscription-link catalog on the backend

- **Status:** Accepted
- **Date:** 2026-06-17
- **PR:** #116

## Context

In the users list the "Subscription" column was a single "Mihomo" button that copied the one
subscription URL (`sub.url` in the `GET /admin/api/users` response). We need to copy **several**
things per user: the Mihomo subscription URL itself and the Clashmi app deeplink
(`clashmi://install-config?url=<enc>&name=<title>&overwrite=false`), and in the future others
(new engines, new client apps).

Hard requirement: **the frontend must not hardcode which links exist or what their titles are** —
otherwise every new client/engine drags a SPA edit. That also conflicts with the repo's general
style (no magic strings; signals expressed as types/catalogs on the backend).

## Considered Options

- **A. Hardcode on the frontend.** The SPA builds the clashmi deeplink from `sub.url` itself and
  knows the titles. Cons: a direct violation of the requirement; the deeplink format and the link
  list live in JS; every new client = a frontend edit; duplicated escaping logic.
- **B. `sub.url` + a parallel `deeplinks` field.** Keep the URL, add a list of deeplinks beside it.
  Cons: the frontend still knows the shape of each special field; not uniform; adding a link kind
  isn't catalog-driven — again contract and frontend edits.
- **C. A flat `sub.links: [{title, value}]` list, the catalog in a backend service (chosen).** The
  response carries a ready, ordered list of copyable "title → value" pairs; the frontend renders it
  as is. The catalog (which links, their titles, the deeplink format) lives in a new
  `internal/service/sublinks`.
- Source of the clashmi deeplink's `name`: **(i) the profile title of the effective config (chosen)**,
  (ii) a separate service env field, (iii) the user's nickname. (i) matches semantically what the
  subscription already returns in the `Profile-Title` header and adds no new configuration.

## Decision

Option **C** was chosen. A new `internal/service/sublinks` service owns an ordered catalog
`[]linkSpec{ title, kind, build(subURL, profileTitle) }`: for Mihomo `build` is the identity (the
URL itself), for Clashmi it is the deeplink format with `url.QueryEscape`. `Links(users)` builds
each user's URL (`base + /sub/<kind>/<token>`) and, for deeplinks, substitutes `name` = the profile
title of the user's **effective** config (custom, else base). Title resolution is efficient: the
base title is read once per engine, custom ones only for users who have them.

The `sub` contract in `GET /admin/api/users` changes from `{id, url}` to `{links: [{title, value}]}`
(`id`/`url` removed — `id` was unused by the frontend, `url` is replaced by the list). The
`users_get` handler delegates assembly to the service and no longer builds the URL itself.

## Consequences

- **Adding an engine/app = one line in the `catalog`** — no frontend, contract, or admin-API edits.
  The frontend renders `sub.links` verbatim (title + a "Copy" button).
- `sub.id`/`sub.url` are gone from the response. apitest (black box) selects the "raw" subscription
  URL by scheme (`http…`), not by title (`UserSub.SubURL()`), staying agnostic to the link set.
- Title resolution per page is several reads (base + customs), not one-per-user; for an admin page
  that is unnoticeable.
- Today the catalog is tied to mihomo (the Clashmi deeplink is a Clash client; `sublinks` imports
  `mihomo.Profile` for the title). This is deliberate: when xray/sing-box are added, the catalog and
  its dependencies expand explicitly, without hidden "magic".
