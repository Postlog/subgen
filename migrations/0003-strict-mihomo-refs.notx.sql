-- 0003 (no-transaction): strict mihomo refs. Gives mihomo_rule_providers a surrogate id
-- (RULE-SET rules reference it via mihomo_routing_rules.provider_id), and makes value /
-- interval / tolerance / lazy nullable. SQLite can't drop NOT NULL or change a PRIMARY KEY
-- in place, so the three tables are rebuilt — which needs foreign_keys OFF, impossible
-- inside a transaction. Hence `.notx.sql`: the runner execs this OUTSIDE a transaction; the
-- file owns its own atomicity (the BEGIN/COMMIT below) AND records itself in
-- schema_migrations inside that same transaction, so "applied" commits with the rebuild.
--
-- Order: providers first (nothing references it yet — provider_id is added below), then the
-- rules, then the groups. ids are PRESERVED on rebuild so existing FK values stay valid;
-- legacy_alter_table keeps the rename from rewriting FK clauses in the untouched tables.

PRAGMA foreign_keys=OFF;
PRAGMA legacy_alter_table=ON;

BEGIN;

-- 1. mihomo_rule_providers: composite PK(config_id,name) → surrogate id.
CREATE TABLE mihomo_rule_providers_new (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  config_id       INTEGER NOT NULL REFERENCES subscription_configs(id) ON DELETE CASCADE,
  name            TEXT NOT NULL,
  behavior        TEXT NOT NULL,
  format          TEXT NOT NULL,
  mirror          INTEGER NOT NULL,
  url             TEXT NOT NULL,
  interval        INTEGER NOT NULL,
  mirror_interval INTEGER NOT NULL DEFAULT 0,
  UNIQUE(config_id, name)
);
INSERT INTO mihomo_rule_providers_new (config_id, name, behavior, format, mirror, url, interval, mirror_interval)
SELECT config_id, name, behavior, format, mirror, url, interval, mirror_interval FROM mihomo_rule_providers;
DROP TABLE mihomo_rule_providers;
ALTER TABLE mihomo_rule_providers_new RENAME TO mihomo_rule_providers;

-- 2. mihomo_routing_rules: nullable value + provider_id. Backfill provider_id for RULE-SET
--    from the matching provider name, NULL out value for MATCH/RULE-SET (and any empty).
CREATE TABLE mihomo_routing_rules_new (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  config_id       INTEGER NOT NULL REFERENCES subscription_configs(id) ON DELETE CASCADE,
  position        INTEGER NOT NULL,
  type            TEXT NOT NULL,
  value           TEXT,
  provider_id     INTEGER REFERENCES mihomo_rule_providers(id),
  no_resolve      INTEGER NOT NULL DEFAULT 0,
  target_kind     TEXT NOT NULL,
  inbound_id      INTEGER REFERENCES node_inbounds(id),
  target_group_id INTEGER REFERENCES mihomo_proxy_groups(id),
  CHECK ((target_kind='inbound')=(inbound_id IS NOT NULL) AND (target_kind='group')=(target_group_id IS NOT NULL))
);
INSERT INTO mihomo_routing_rules_new
  (id, config_id, position, type, value, provider_id, no_resolve, target_kind, inbound_id, target_group_id)
SELECT
  r.id, r.config_id, r.position, r.type,
  CASE WHEN r.type IN ('RULE-SET','MATCH') OR r.value='' THEN NULL ELSE r.value END,
  CASE WHEN r.type='RULE-SET'
       THEN (SELECT p.id FROM mihomo_rule_providers p WHERE p.config_id=r.config_id AND p.name=r.value)
       ELSE NULL END,
  r.no_resolve, r.target_kind, r.inbound_id, r.target_group_id
FROM mihomo_routing_rules r;
DROP TABLE mihomo_routing_rules;
ALTER TABLE mihomo_routing_rules_new RENAME TO mihomo_routing_rules;

-- 3. mihomo_proxy_groups: nullable interval/tolerance/lazy (NULL where the type does not
--    use the field, or the value was the zero stand-in for "unset").
CREATE TABLE mihomo_proxy_groups_new (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  config_id INTEGER NOT NULL REFERENCES subscription_configs(id) ON DELETE CASCADE,
  position  INTEGER NOT NULL,
  name      TEXT NOT NULL,
  type      TEXT NOT NULL,
  url       TEXT NOT NULL DEFAULT '',
  interval  INTEGER,
  tolerance INTEGER,
  lazy      INTEGER,
  UNIQUE(config_id, name)
);
INSERT INTO mihomo_proxy_groups_new (id, config_id, position, name, type, url, interval, tolerance, lazy)
SELECT
  g.id, g.config_id, g.position, g.name, g.type, g.url,
  CASE WHEN g.type IN ('url-test','fallback','load-balance') AND g.interval<>0 THEN g.interval ELSE NULL END,
  CASE WHEN g.type='url-test' AND g.tolerance<>0 THEN g.tolerance ELSE NULL END,
  CASE WHEN g.type IN ('url-test','fallback','load-balance') AND g.lazy<>0 THEN g.lazy ELSE NULL END
FROM mihomo_proxy_groups g;
DROP TABLE mihomo_proxy_groups;
ALTER TABLE mihomo_proxy_groups_new RENAME TO mihomo_proxy_groups;

-- Record this migration as applied, atomically with the rebuild (this file is .notx, so
-- the runner does NOT record it — see migrations/run.go applyNoTx).
INSERT INTO schema_migrations(name, applied_at)
VALUES('0003-strict-mihomo-refs.notx.sql', CAST(strftime('%s','now') AS INTEGER));

COMMIT;

PRAGMA legacy_alter_table=OFF;
PRAGMA foreign_keys=ON;
