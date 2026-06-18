# 0002 — An ordered migration runner instead of manual ones

- **Status:** Accepted
- **Date:** 2026-06-11
- **PR:** #18

## Context

The DB schema was applied with a single embedded `init.sql` (`migrations.Schema`,
`CREATE … IF NOT EXISTS`) on start, while structural changes to existing tables
(`ALTER TABLE …`) were done **by hand**: a one-off `*.manual.sql` file that no code
runs — the operator must remember to execute it on prod. This is fragile: easy to
forget, there is no trace of «what is already applied», ordering and atomicity are on the human's
conscience. We need a mechanism that itself, and in the right order, brings any base (fresh or
existing) up to the current schema, failing loudly on error.

## Considered Options

- **Keep manual migrations** (`init.sql` + `*.manual.sql`) — minimal code, but exactly the
  fragility we are fixing: a manual step, no record of what was applied, easy to get schema drift.
- **`CREATE/ALTER … IF NOT EXISTS`, run everything every start** — without a tracking table. But
  SQLite cannot do `ADD COLUMN IF NOT EXISTS`, so an `ALTER` on a repeat start would fail;
  one would have to «probe» `PRAGMA table_info` — a homemade half-migrator.
- **An external migration library** (goose / golang-migrate) — functional, but pulls in a
  dependency and its own model of files/CLI for the sake of a few files; for an embedded schema in
  a single binary it is overkill.
- **Our own runner of ordered files + a tracking table** *(chosen)* — `0001-init.sql` as the
  baseline, then `NNNN-*.sql`; applied by name order, each in a transaction, the fact
  of application written to `schema_migrations`. Idempotent, without dependencies.

## Decision

We introduce a runner `migrations.Apply(ctx, db)` (the package `migrations`, files `embed.go` +
`run.go`), which `repository.Open` calls instead of `ExecContext(Schema)`:

- `0001-init.sql` — the **baseline** (the full schema-as-of-now); then `0002-*.sql`, …. All files are
  `NNNN-`-prefixed, so the ordinary lexicographic name sort = the apply order
  (without special logic and without pinning the baseline).
- The table `schema_migrations(name PRIMARY KEY, applied_at)` stores what was applied; each
  file is applied **once**, a repeat start is a no-op.
- Each migration — in **its own transaction** together with the write to `schema_migrations`
  (atomically: a crash mid-file leaves no «half-applied» state and does not write the fact).
- On error `Apply` returns it → `main` crashes (`log.Fatal`); each apply is
  logged (`slog.Info`).
- **The connection PRAGMA have moved to the DSN** (`open.go`: `busy_timeout`, `foreign_keys`,
  `journal_mode=WAL`), not into `0001-init.sql` — `PRAGMA journal_mode=WAL` cannot be executed
  inside the transaction the runner wraps the file in, therefore migrations are pure DDL.

This **cancels** the prior «DB migrations — by hand only» (the section in `AGENTS.md` was rewritten).
Our own, not a library — because the volume (an embedded schema of a single binary) does not justify a
dependency, and the semantics are trivial.

## Consequences

- Any base is brought up to the current schema automatically and in order; `*.manual.sql`
  are no longer needed (the pattern was removed from the rules). A structural change = a new
  `NNNN-*.sql`, and that is all.
- An existing prod base is adopted safely: `schema_migrations` is created empty, the
  baseline (`CREATE … IF NOT EXISTS`) is a no-op on a repeat run and is marked as applied, then
  `NNNN-*.sql` land on top. A separate «tracking backfill» is not needed.
- `0001-init.sql` is now an **immutable baseline**: schema edits go only via new
  files, not by editing the baseline (otherwise it diverges from already-adopted bases).
- PRAGMA — in one place, in the DSN; migrations are obliged to be pure DDL (PRAGMA that change the
  journal mode must not be put into a migration file).
- Rolling back migrations is not implemented (forward-only) — deliberately: for a continuous deploy
  «forward + a new fix file» is simpler and safer than down scripts.
