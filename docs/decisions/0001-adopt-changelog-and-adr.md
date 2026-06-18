# 0001 — Documentation convention: CHANGELOG + ADR

- **Status:** Accepted
- **Date:** 2026-06-11
- **PR:** #16

## Context

«Why we decided this way» was getting lost in subgen: the context of a change lived only in the PR
discussion and in the author's head, while only diffs and one-line commit messages remained in the
repository. A couple of months later it is already hard to reconstruct the motivation of an
architectural choice. We need a lightweight but mandatory mechanism that records both *what*
changed and *why*.

## Considered Options

- **CHANGELOG only (Keep a Changelog)** — the keepachangelog.com standard: `[Unreleased]` +
  `Added/Changed/Fixed/Removed` sections, tied to versions. Recognizable, but
  designed for versioned releases, which subgen does not have (continuous deploy, without
  tags), and it is not the place for an elaborate «why» with an analysis of options.
- **ADR only** — detailed decisions in `docs/decisions/`, but without a consolidated list
  of changes: there is no quick «what even happened recently».
- **CHANGELOG (per-PR, without versions) + ADR for the non-trivial** *(chosen)* — a short
  entry per PR as an index of changes plus a separate ADR with the problem/options/
  rationale wherever there is a design decision. The CHANGELOG links to the ADR.
- **Leave it as is** — rely on the PR history. This is exactly the loss of context we are fixing.

## Decision

We introduce **both** artifacts: `CHANGELOG.md` (one entry per PR, reverse-chronologically,
without version sections) and an ADR catalog `docs/decisions/NNNN-slug.md` (Context / Considered
Options / Decision / Consequences) for non-trivial changes. The rule is in `AGENTS.md`.
The «per-PR without versions» format was chosen because the deploy is continuous and there is nothing
to tie the version sections of Keep a Changelog to.

## Consequences

- Every PR is obliged to add a CHANGELOG entry; a non-trivial one — also an ADR. A small
  constant overhead, paid back by the preserved context.
- A stable referenceable unit of a decision appears (`ADR-NNNN`), referenced by the
  CHANGELOG, code reviews and subsequent ADRs.
- ADRs are immutable: a revision is a new ADR with `Supersedes`, not an edit of the old one.
- This PR is the first one under the convention and follows it itself (a CHANGELOG entry + this ADR).
