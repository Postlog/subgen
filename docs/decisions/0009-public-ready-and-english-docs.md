# 0009 — Public-ready: English-only docs & UI, MIT license

- **Status:** Accepted
- **Date:** 2026-06-18
- **PR:** #TBD

## Context

subgen is being published as a public repository. Two project-level decisions had to be
made and recorded so future contributors share the same defaults:

1. **Language.** Human-facing text was inconsistent: `README.md` and `docs/subgen.md` were
   English, while `CHANGELOG.md`, the ADRs, `AGENTS.md`, the admin UI labels and the
   user-facing handler messages were Russian. A public repo needs one language.
2. **License.** There was no `LICENSE` file. Without one the code is, by default, "all
   rights reserved" — not open-source; nobody may legally fork or reuse it.

## Considered Options

- **Language**
  - **English everywhere** — docs, ADRs, `AGENTS.md`, `CHANGELOG`, admin UI, user-facing
    messages. Widest reach for an open-source audience; one language to maintain.
  - **Russian everywhere** — matches the original UI/market, but narrows the contributor
    and user base for a public project.
  - **Bilingual** (e.g. `README.md` + `README.ru.md`) — broadest, but double the
    maintenance and easy to let drift.
- **License**
  - **MIT** — short, maximally permissive, by far the most common; reuse with attribution.
  - **Apache-2.0** — permissive plus an explicit patent grant; more ceremony.
  - **GPL-3.0** — copyleft; forks must stay open.
  - **None** — remain proprietary.

## Decision

**English for all human-facing text** (docs, ADRs, `AGENTS.md`, `CHANGELOG`, admin UI
labels, and user-facing handler/error messages), and the **MIT** license.

English maximizes reach and keeps a single source of truth without the drift risk of a
bilingual setup. MIT is the lowest-friction choice for a small self-hosted tool we want
people to freely run and fork; the patent and copyleft concerns of Apache/GPL do not apply
here.

## Consequences

- A one-time sweep translated `AGENTS.md`, `CHANGELOG.md`, every ADR, `apitest/README.md`,
  `docs/subgen.md`, the admin SPA (`internal/handlers/web/static/`), and all user-facing
  message constants in `internal/handlers/*` to English. Unit tests and the black-box
  `apitest` assertions were updated in lockstep (they assert the exact text).
- New code and docs must stay English — including user-facing strings. This is the standing
  rule; `CONTRIBUTING.md` states it, and a `grep -I '[А-Яа-я]'` over the tree should stay
  empty (operator-entered data in the running store is exempt).
- A `LICENSE` (MIT) is added; GitHub now recognises the project as MIT-licensed, and the
  `README` carries the badge and a license section.
- The product can still serve Russian-market users; only the codebase, UI chrome and docs
  are English.
