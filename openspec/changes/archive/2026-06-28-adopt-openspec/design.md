# Design — adopt OpenSpec, retire ADRs

## Context

The project recorded design decisions as ADRs (`docs/decisions/0001–0009`) plus a per-PR
`CHANGELOG.md` (the convention codified in ADR-0001). ADRs capture *point-in-time decisions*, but
there was no maintained, machine-checkable description of the as-built behavior; it stayed
implicit across code, docs and tests. [OpenSpec](https://github.com/Fission-AI/OpenSpec) (the
`spec-driven` schema) provides a living behavior contract plus a structured change workflow
(`proposal` / `design` / `tasks` + spec deltas), with slash commands and a validating CLI.

## Goals / Non-Goals

**Goals:**
- One forward process for non-trivial changes, with a living spec baseline.
- Keep the per-PR `CHANGELOG`.
- Make the workflow reproducible for every contributor (tracked, not local-only).

**Non-Goals:**
- No Go behavior change.
- Not re-homing every historical ADR as a retroactive change (git history keeps them).

## Considered Options

- **Keep ADRs only.** Familiar, zero new tooling, but no living behavior spec; the as-built
  contract stays implicit and scattered.
- **Adopt OpenSpec alongside ADRs.** Both a decision catalog and a spec workflow — but two
  overlapping processes invite confusion and duplicate the rationale.
- **Adopt OpenSpec and retire ADRs (chosen).** One forward process; the rationale that ADRs held
  now lives in each change's `design.md`; the `CHANGELOG` stays. `docs/decisions/` is removed (the
  records remain in git history).

## Decision

Adopt OpenSpec as the planning and change-documentation process and **retire the ADR process**.
Seed a full spec baseline under `openspec/specs/`. Record this migration as this archived OpenSpec
change rather than a final ADR. Remove `docs/decisions/`; rely on git history for the old records.

## Consequences

- The `/opsx:*` commands and skills are generated under `.claude/`; the `.claude` gitignore is
  narrowed so they are tracked (reproducible for contributors, CI, and review). Contributors not
  using Claude run the same flow with the `openspec` CLI (`npx @fission-ai/openspec`).
- `openspec/specs/` is the living behavior contract, validated by
  `openspec validate --specs --strict`. Keeping it in sync becomes part of the per-change
  checklist.
- The English-only rule applies to all OpenSpec artifacts (the standing grep guard stays empty).
- Inline ADR citations in the docs and a few code comments are dropped or repointed; their
  decisions are preserved in git history and, where they describe behavior, in the specs.
