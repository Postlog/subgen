-- MANUAL one-off migration — per-user custom configs.
--
-- Adds subscription_configs (the ownership anchor) and scopes the mihomo_* content
-- tables by config_id. SQLite can't ALTER a UNIQUE/PRIMARY KEY in place, so the
-- scoped tables are rebuilt (rename → create → copy → drop).
--
-- Run this ONCE, by hand, on a DB created BEFORE per-user configs. It is NOT applied
-- on startup (embed.go ships only init.sql). Fresh installs get the new schema from
-- init.sql and must NOT run this.
--
-- BEFORE running: stop the service and back up the db file (cp x-ui-style copy).
-- All existing global config is migrated to the base config (id=1, kind='mihomo').
--
-- legacy_alter_table=ON is REQUIRED: with the modern default (OFF), `ALTER TABLE
-- RENAME` rewrites FK references in OTHER tables to follow the rename — so renaming
-- mihomo_proxy_groups → _old would repoint mihomo_proxy_group_members / routing_rules
-- at the doomed _old table, leaving dangling FKs once it is dropped. ON keeps the FKs
-- bound by name, so they resolve to the freshly recreated mihomo_proxy_groups.

PRAGMA foreign_keys=OFF;
PRAGMA legacy_alter_table=ON;
BEGIN;

-- 1) Ownership anchor + the base config row for the existing global mihomo config.
CREATE TABLE IF NOT EXISTS subscription_configs (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER REFERENCES users(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subcfg_owner
  ON subscription_configs(COALESCE(user_id, 0), kind);

INSERT INTO subscription_configs(id, user_id, kind, created_at)
  VALUES (1, NULL, 'mihomo', CAST(strftime('%s','now') AS INTEGER));

-- 2) proxy_groups: + config_id, UNIQUE(name) → UNIQUE(config_id,name). Group ids are
--    preserved, so mihomo_proxy_group_members (FK on group id) needs no rebuild.
ALTER TABLE mihomo_proxy_groups RENAME TO _old_mihomo_proxy_groups;
CREATE TABLE mihomo_proxy_groups (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  config_id INTEGER NOT NULL REFERENCES subscription_configs(id) ON DELETE CASCADE,
  position  INTEGER NOT NULL,
  name      TEXT NOT NULL,
  type      TEXT NOT NULL,
  url       TEXT NOT NULL DEFAULT '',
  interval  INTEGER NOT NULL DEFAULT 0,
  tolerance INTEGER NOT NULL DEFAULT 0,
  lazy      INTEGER NOT NULL DEFAULT 0,
  UNIQUE(config_id, name)
);
INSERT INTO mihomo_proxy_groups(id,config_id,position,name,type,url,interval,tolerance,lazy)
  SELECT id,1,position,name,type,url,interval,tolerance,lazy FROM _old_mihomo_proxy_groups;
DROP TABLE _old_mihomo_proxy_groups;

-- 3) routing_rules: + config_id.
ALTER TABLE mihomo_routing_rules RENAME TO _old_mihomo_routing_rules;
CREATE TABLE mihomo_routing_rules (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  config_id       INTEGER NOT NULL REFERENCES subscription_configs(id) ON DELETE CASCADE,
  position        INTEGER NOT NULL,
  type            TEXT NOT NULL,
  value           TEXT NOT NULL DEFAULT '',
  no_resolve      INTEGER NOT NULL DEFAULT 0,
  target_kind     TEXT NOT NULL,
  inbound_id      INTEGER REFERENCES node_inbounds(id),
  target_group_id INTEGER REFERENCES mihomo_proxy_groups(id),
  CHECK ((target_kind='inbound')=(inbound_id IS NOT NULL) AND (target_kind='group')=(target_group_id IS NOT NULL))
);
INSERT INTO mihomo_routing_rules(id,config_id,position,type,value,no_resolve,target_kind,inbound_id,target_group_id)
  SELECT id,1,position,type,value,no_resolve,target_kind,inbound_id,target_group_id FROM _old_mihomo_routing_rules;
DROP TABLE _old_mihomo_routing_rules;

-- 4) rule_providers: PK(name) → PK(config_id,name).
ALTER TABLE mihomo_rule_providers RENAME TO _old_mihomo_rule_providers;
CREATE TABLE mihomo_rule_providers (
  config_id       INTEGER NOT NULL REFERENCES subscription_configs(id) ON DELETE CASCADE,
  name            TEXT NOT NULL,
  behavior        TEXT NOT NULL,
  format          TEXT NOT NULL,
  mirror          INTEGER NOT NULL,
  url             TEXT NOT NULL,
  interval        INTEGER NOT NULL,
  mirror_interval INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(config_id, name)
);
INSERT INTO mihomo_rule_providers(config_id,name,behavior,format,mirror,url,interval,mirror_interval)
  SELECT 1,name,behavior,format,mirror,url,interval,mirror_interval FROM _old_mihomo_rule_providers;
DROP TABLE _old_mihomo_rule_providers;

-- 5) settings: PK(key) → PK(config_id,key).
ALTER TABLE mihomo_settings RENAME TO _old_mihomo_settings;
CREATE TABLE mihomo_settings (
  config_id INTEGER NOT NULL REFERENCES subscription_configs(id) ON DELETE CASCADE,
  key       TEXT NOT NULL,
  value     TEXT NOT NULL,
  PRIMARY KEY(config_id, key)
);
INSERT INTO mihomo_settings(config_id,key,value)
  SELECT 1,key,value FROM _old_mihomo_settings;
DROP TABLE _old_mihomo_settings;

-- Re-assert the members index (table itself was untouched, index survives, but keep
-- it idempotent in case of partial prior runs).
CREATE INDEX IF NOT EXISTS idx_mihomo_pgmember_group ON mihomo_proxy_group_members(group_id);

COMMIT;
PRAGMA legacy_alter_table=OFF;
PRAGMA foreign_keys=ON;

-- Sanity (run after): every content row should carry config_id=1.
--   SELECT (SELECT COUNT(*) FROM mihomo_proxy_groups   WHERE config_id<>1)
--        + (SELECT COUNT(*) FROM mihomo_routing_rules  WHERE config_id<>1)
--        + (SELECT COUNT(*) FROM mihomo_rule_providers WHERE config_id<>1)
--        + (SELECT COUNT(*) FROM mihomo_settings       WHERE config_id<>1);  -- expect 0
