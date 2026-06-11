-- Manual migration — NOT applied automatically (embed.go ships only init.sql). Run by
-- hand, once, against an existing DB. TAKE A BACKUP FIRST and eyeball the data with the
-- read-only checks at the bottom before committing.
--
-- Brings an existing DB to the "strict mihomo refs" schema (PR #17):
--   1. mihomo_rule_providers gets a surrogate id; RULE-SET rules reference it by id.
--   2. mihomo_routing_rules: value becomes nullable (NULL for MATCH/RULE-SET); provider_id
--      is added and backfilled from the old provider-name-in-value.
--   3. mihomo_proxy_groups: interval/tolerance/lazy become nullable (NULL = not set / not
--      applicable to the type), replacing the old NOT NULL 0 defaults.
--
-- SQLite can't drop a NOT NULL / change a PRIMARY KEY in place, so the three tables are
-- rebuilt. Order matters: rebuild providers first (nothing references it yet), then the
-- rules (which reference providers), then the groups; ids are PRESERVED on rebuild so the
-- existing FK values (target_group_id, members.group_id, …) stay valid. legacy_alter_table
-- keeps the rename from rewriting FK clauses in the untouched tables; foreign_keys is off
-- for the rebuild (both PRAGMAs must sit outside the transaction).

PRAGMA foreign_keys=OFF;
PRAGMA legacy_alter_table=ON;

BEGIN;

-- 1. Rebuild mihomo_rule_providers with a surrogate id (was PRIMARY KEY(config_id,name)).
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

INSERT INTO mihomo_rule_providers_new
  (config_id, name, behavior, format, mirror, url, interval, mirror_interval)
SELECT config_id, name, behavior, format, mirror, url, interval, mirror_interval
  FROM mihomo_rule_providers;

DROP TABLE mihomo_rule_providers;
ALTER TABLE mihomo_rule_providers_new RENAME TO mihomo_rule_providers;

-- 2. Rebuild mihomo_routing_rules: nullable value + provider_id. Backfill provider_id for
--    RULE-SET from the matching provider name, and NULL out value for MATCH/RULE-SET (and
--    any empty value).
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

-- 3. Rebuild mihomo_proxy_groups: nullable interval/tolerance/lazy. Map the old NOT NULL
--    zeros to NULL where the type does not use the field (or the value was the zero stand-in
--    for "unset").
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

INSERT INTO mihomo_proxy_groups_new
  (id, config_id, position, name, type, url, interval, tolerance, lazy)
SELECT
  g.id, g.config_id, g.position, g.name, g.type, g.url,
  CASE WHEN g.type IN ('url-test','fallback','load-balance') AND g.interval<>0 THEN g.interval ELSE NULL END,
  CASE WHEN g.type='url-test' AND g.tolerance<>0 THEN g.tolerance ELSE NULL END,
  CASE WHEN g.type IN ('url-test','fallback','load-balance') AND g.lazy<>0 THEN g.lazy ELSE NULL END
FROM mihomo_proxy_groups g;

DROP TABLE mihomo_proxy_groups;
ALTER TABLE mihomo_proxy_groups_new RENAME TO mihomo_proxy_groups;
CREATE INDEX IF NOT EXISTS idx_mihomo_pgmember_group ON mihomo_proxy_group_members(group_id);

COMMIT;

PRAGMA legacy_alter_table=OFF;
PRAGMA foreign_keys=ON;

-- Read-only sanity (run separately, expect zero rows):
--   -- RULE-SET rules that failed to resolve to a provider:
--   SELECT id, config_id, value FROM mihomo_routing_rules
--    WHERE type='RULE-SET' AND provider_id IS NULL;
--   -- provider_id / target_group_id pointing at a missing parent (referential integrity):
--   SELECT r.id FROM mihomo_routing_rules r
--    LEFT JOIN mihomo_rule_providers p ON p.id = r.provider_id
--    WHERE r.provider_id IS NOT NULL AND p.id IS NULL;
--   PRAGMA foreign_key_check;
