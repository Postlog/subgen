# Database Migrations Spec

## Purpose

The SQLite store evolves through an ordered, forward-only migration runner invoked on startup.
Files are `NNNN-*.sql`, so a plain name sort is the apply order: `0001-init.sql` is the immutable
baseline (`CREATE … IF NOT EXISTS`), later files add changes. Each file is applied exactly once,
tracked in `schema_migrations`, in its own transaction; on error the service crashes rather than
running on a half-migrated schema. Migrations are pure DDL — connection PRAGMAs live in the DSN.
Constraint violations surface to the repositories as typed codes, never string matches.

## Requirements

### Requirement: Ordered, apply-once migrations

The runner SHALL apply each migration file exactly once, in filename order, tracking applied
files in `schema_migrations`, and MUST skip already-applied files on restart.

#### Scenario: Fresh database

- **WHEN** the runner starts against an empty database
- **THEN** every migration is applied in name order and recorded in `schema_migrations`

#### Scenario: Restart

- **WHEN** the runner starts against a database where some migrations are already recorded
- **THEN** only the not-yet-applied files run, in order

#### Scenario: New schema change

- **WHEN** a structural schema change is needed
- **THEN** it is added as a new `NNNN-*.sql` file rather than by editing the baseline

### Requirement: Transactional, forward-only, crash on error

The runner SHALL apply each migration atomically and MUST abort startup on failure; there are no
rollbacks.

#### Scenario: Migration fails

- **WHEN** a migration errors mid-apply
- **THEN** its transaction is rolled back, it is not recorded, and startup fails (the service crashes)

#### Scenario: Non-transactional rebuild

- **WHEN** a migration must rebuild a table outside a transaction (e.g. toggling a connection PRAGMA)
- **THEN** it uses the dedicated non-transactional mode and records itself

### Requirement: Constraint violations map to domain sentinels

Repositories SHALL detect uniqueness and foreign-key violations by the driver's extended result
code, never by error text, and translate them to domain sentinel errors.

#### Scenario: Unique or primary-key violation

- **WHEN** an insert/update violates a unique or primary-key constraint (code 2067 or 1555)
- **THEN** the repository returns the corresponding domain sentinel (e.g. `entity.ErrNameTaken`, `entity.ErrNodeNameTaken`, `entity.ErrRuleProviderNameTaken`)

#### Scenario: Foreign-key violation

- **WHEN** a delete violates a foreign-key constraint (code 787)
- **THEN** the repository returns `entity.ErrInboundReferenced`
