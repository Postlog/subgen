-- Manual migration — NOT applied automatically (embed.go ships only init.sql). Run by
-- hand, once, against an existing DB. TAKE A BACKUP FIRST and eyeball the data with the
-- read-only checks at the bottom before committing.
--
-- What it does: gives mihomo_rule_providers a surrogate id and makes RULE-SET rules
-- reference a provider by that id (mihomo_routing_rules.provider_id) instead of by name
-- stuffed into `value`. Brings an existing DB to the same schema fresh DBs get from
-- init.sql.
--
-- SQLite can't ALTER a PRIMARY KEY, so the providers table is rebuilt. The rebuild runs
-- BEFORE the rules.provider_id FK is added, so at rebuild time nothing references the
-- providers table — the legacy_alter_table rename gotcha can't bite. foreign_keys is
-- toggled off for the rebuild regardless (PRAGMAs must sit outside the transaction).

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

-- 2. Add the rule -> provider id column (set only for RULE-SET).
ALTER TABLE mihomo_routing_rules ADD COLUMN provider_id INTEGER REFERENCES mihomo_rule_providers(id);

-- 3. Backfill: point each RULE-SET rule at the provider with the matching name in the
--    same config. value is cleared only where a match was found, so an orphaned RULE-SET
--    (no such provider — shouldn't exist, old validation forbade it) keeps its name for
--    inspection instead of silently losing it.
UPDATE mihomo_routing_rules
   SET provider_id = (
         SELECT p.id FROM mihomo_rule_providers p
          WHERE p.config_id = mihomo_routing_rules.config_id
            AND p.name = mihomo_routing_rules.value
       )
 WHERE type = 'RULE-SET';

UPDATE mihomo_routing_rules
   SET value = ''
 WHERE type = 'RULE-SET' AND provider_id IS NOT NULL;

COMMIT;

PRAGMA legacy_alter_table=OFF;
PRAGMA foreign_keys=ON;

-- Read-only sanity (run separately, expect zero rows):
--   -- RULE-SET rules that failed to resolve to a provider:
--   SELECT id, config_id, value FROM mihomo_routing_rules
--    WHERE type='RULE-SET' AND provider_id IS NULL;
--   -- provider_id pointing at a missing provider (referential integrity):
--   SELECT r.id FROM mihomo_routing_rules r
--    LEFT JOIN mihomo_rule_providers p ON p.id = r.provider_id
--    WHERE r.provider_id IS NOT NULL AND p.id IS NULL;
