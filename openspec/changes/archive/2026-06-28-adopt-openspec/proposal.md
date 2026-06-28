# Adopt OpenSpec; retire the ADR process

## Why

Design decisions were recorded as ADRs (`docs/decisions/0001–0009`) plus a per-PR `CHANGELOG.md`.
ADRs capture point-in-time decisions well, but the project grew subsystems with real behavioral
contracts — routing, config ownership, cross-panel provisioning, subscription rendering — and
there was no maintained, machine-checkable description of *what the system currently does*. We
want a structured, AI-native, spec-driven workflow that keeps a living behavior contract
alongside the decision trail, with first-class tooling.

## What Changes

- Adopt [OpenSpec](https://github.com/Fission-AI/OpenSpec) (the `spec-driven` schema) as the
  planning and change-documentation process.
- Seed a full spec baseline under `openspec/specs/<capability>/spec.md` (12 capabilities) from the
  current code, in the requirement + WHEN/THEN scenario format.
- Add the `/opsx:*` workflow under `.claude/` and narrow the blanket `.claude` gitignore to
  `.claude/settings.local.json` so the commands/skills are tracked.
- Rewrite the process in `AGENTS.md`, `CONTRIBUTING.md`, `README.md`, and the `CHANGELOG.md`
  header to point at OpenSpec.
- **Remove the `docs/decisions/` ADR catalog** (the records remain in git history). The
  `CHANGELOG` stays; non-trivial changes now link an archived OpenSpec change.

## Impact

- `openspec/` — new spec baseline, config, and change workflow.
- `.claude/` — `/opsx:*` commands and skills (now tracked).
- `.gitignore`, `AGENTS.md`, `CONTRIBUTING.md`, `README.md`, `CHANGELOG.md` — process pointers.
- `docs/decisions/` — removed. Inline ADR citations across the docs and a few code comments are
  dropped or repointed.
- No Go behavior changes.
