-- 0004 (no-transaction): recursive routing rules. A rule is now recursive — a logical
-- rule (type AND/OR/NOT) carries its sub-rules in the SAME table via parent_id (self-ref);
-- a sub-rule is a matcher with NO target. There is no separate "condition" entity.
--
-- This needs target_kind to become nullable (a sub-rule has no target) and a new parent_id
-- column. SQLite can't drop a NOT NULL in place, so the table is rebuilt — which needs
-- foreign_keys OFF, impossible inside a transaction. Hence `.notx.sql`: the runner execs
-- this OUTSIDE a transaction; the file owns its own atomicity (the BEGIN/COMMIT below) AND
-- records itself in schema_migrations inside that same transaction.
--
-- ids are PRESERVED on rebuild so existing FK values stay valid; legacy_alter_table keeps
-- the rename from rewriting FK clauses in untouched tables. Existing rows are all
-- top-level (parent_id NULL). No other table references mihomo_routing_rules.

PRAGMA foreign_keys=OFF;
PRAGMA legacy_alter_table=ON;

BEGIN;

-- mihomo_routing_rules: add parent_id (self-ref, NULL = top-level rule) and make
-- target_kind nullable (a sub-rule has no target). The CHECK pins the invariant
-- "a top-level rule has a target, a sub-rule has none" (parent_id NULL iff target set).
CREATE TABLE mihomo_routing_rules_new (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  config_id       INTEGER NOT NULL REFERENCES subscription_configs(id) ON DELETE CASCADE,
  parent_id       INTEGER REFERENCES mihomo_routing_rules(id) ON DELETE CASCADE,
  position        INTEGER NOT NULL,
  type            TEXT NOT NULL,
  value           TEXT,
  provider_id     INTEGER REFERENCES mihomo_rule_providers(id),
  no_resolve      INTEGER NOT NULL DEFAULT 0,
  target_kind     TEXT,
  inbound_id      INTEGER REFERENCES node_inbounds(id),
  target_group_id INTEGER REFERENCES mihomo_proxy_groups(id),
  CHECK ((parent_id IS NULL) = (target_kind IS NOT NULL)),
  CHECK ((target_kind='inbound')=(inbound_id IS NOT NULL) AND (target_kind='group')=(target_group_id IS NOT NULL))
);
INSERT INTO mihomo_routing_rules_new
  (id, config_id, parent_id, position, type, value, provider_id, no_resolve, target_kind, inbound_id, target_group_id)
SELECT
  id, config_id, NULL, position, type, value, provider_id, no_resolve, target_kind, inbound_id, target_group_id
FROM mihomo_routing_rules;
DROP TABLE mihomo_routing_rules;
ALTER TABLE mihomo_routing_rules_new RENAME TO mihomo_routing_rules;

CREATE INDEX IF NOT EXISTS idx_mihomo_rules_config ON mihomo_routing_rules(config_id);
CREATE INDEX IF NOT EXISTS idx_mihomo_rules_parent ON mihomo_routing_rules(parent_id);

-- Record this migration as applied, atomically with the rebuild (this file is .notx, so
-- the runner does NOT record it — see migrations/run.go applyNoTx).
INSERT INTO schema_migrations(name, applied_at)
VALUES('0004-mihomo-rule-children.notx.sql', CAST(strftime('%s','now') AS INTEGER));

COMMIT;

PRAGMA legacy_alter_table=OFF;
PRAGMA foreign_keys=ON;
