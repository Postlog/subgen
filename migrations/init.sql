PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- Logical split inside one SQLite file (SQLite has no in-file schemas, and FKs can't
-- cross attached databases — so a single file keeps inbound↔rule/member FKs intact):
--   * subgen admin tables (unprefixed): nodes, node_inbounds, users, user_connections.
--   * mihomo config tables (mihomo_ prefix): proxy-groups, members, routing rules,
--     rule-providers, base-YAML/settings. These reference node_inbounds (same file).

-- ============================ subgen admin ============================

CREATE TABLE IF NOT EXISTS nodes (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  name            TEXT NOT NULL UNIQUE,
  vpn_host        TEXT NOT NULL,
  panel_base_url  TEXT NOT NULL,
  panel_base_path TEXT NOT NULL,
  token           TEXT NOT NULL,
  created_at      INTEGER NOT NULL
);

-- A node exposes any number of uniform inbounds; each has a per-node-unique name
-- (a-z/0-9/-) and port. The inbound label "<node name>-<inbound name>" is the mihomo
-- proxy name. node_inbounds.id is stable across edits — user connections and mihomo
-- rules/group-members reference it — so the UI sends the id back for existing inbounds.
CREATE TABLE IF NOT EXISTS node_inbounds (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  node_id      INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  inbound_port INTEGER NOT NULL,
  UNIQUE(node_id, name),
  UNIQUE(node_id, inbound_port)
);

-- Service-owned subscribers. The service generates sub_id (the subscription
-- token source) and provisions one 3x-ui client per connection. A user's name
-- is an admin-chosen nickname; sub_id groups all their clients into one
-- subscription (token = HMAC(secret, sub_id)).
CREATE TABLE IF NOT EXISTS users (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL UNIQUE,   -- admin nickname, ^[a-z0-9_-]{1,32}$
  sub_id      TEXT NOT NULL UNIQUE,   -- subscription id (token = HMAC(secret, sub_id))
  created_at  INTEGER NOT NULL
);

-- One row per (user, inbound): which inbounds a user is provisioned on. The 3x-ui
-- client identity (uuid/email/subId) is per-user, not per-row — a user is one
-- 3x-ui client (email = name, subId = users.sub_id) bound to all its inbounds on
-- a panel. The inbound_id FK has NO cascade on purpose: a node_inbounds row
-- referenced here cannot be deleted (it would orphan the panel client) — detach
-- the users first.
CREATE TABLE IF NOT EXISTS user_connections (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  inbound_id  INTEGER NOT NULL REFERENCES node_inbounds(id),
  created_at  INTEGER NOT NULL,
  UNIQUE(user_id, inbound_id)
);
CREATE INDEX IF NOT EXISTS idx_uconn_user ON user_connections(user_id);
CREATE INDEX IF NOT EXISTS idx_uconn_inbound ON user_connections(inbound_id);

-- ============================ subscription configs ============================

-- A subscription config is the ownership/identity anchor for one engine's config:
-- either the base (user_id NULL — served to everyone without an override) or a
-- per-user custom (user_id set — a full snapshot, cloned from the base then edited
-- freely). kind is the engine discriminator (mihomo today; xray/sing-box later) so
-- base/custom are independent per engine. All engine content rows (mihomo_* below,
-- future xray_*) hang off a config via config_id; deleting a user (or a config)
-- cascades its content away. No seed: the base row is created lazily on first save.
CREATE TABLE IF NOT EXISTS subscription_configs (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER REFERENCES users(id) ON DELETE CASCADE,  -- NULL = base config
  kind       TEXT NOT NULL,                                   -- engine: 'mihomo' (later xray/singbox)
  created_at INTEGER NOT NULL
);
-- Exactly one base + at most one custom per (user, engine). COALESCE folds the base
-- (user_id NULL) to sentinel 0 so it collides with itself (a plain UNIQUE on a
-- nullable column would let duplicate bases through, since NULLs are distinct).
CREATE UNIQUE INDEX IF NOT EXISTS idx_subcfg_owner
  ON subscription_configs(COALESCE(user_id, 0), kind);

-- ============================ mihomo config ============================

-- Operator-defined mihomo proxy-groups, scoped to a subscription config. Members are
-- typed PolicyRefs resolved per-subscriber at render time.
CREATE TABLE IF NOT EXISTS mihomo_proxy_groups (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  config_id INTEGER NOT NULL REFERENCES subscription_configs(id) ON DELETE CASCADE,
  position  INTEGER NOT NULL,
  name      TEXT NOT NULL,               -- unique within a config (see index below)
  type      TEXT NOT NULL,               -- select|url-test|fallback|load-balance|relay
  url       TEXT NOT NULL DEFAULT '',
  interval  INTEGER NOT NULL DEFAULT 0,
  tolerance INTEGER NOT NULL DEFAULT 0,
  lazy      INTEGER NOT NULL DEFAULT 0,
  UNIQUE(config_id, name)
);

-- One row per group member, in order. kind is a PolicyKind; inbound_id/ref_group_id
-- are RESTRICTed (a referenced inbound/group can't be deleted) and required exactly
-- for their kind. Members cascade when their group is deleted.
CREATE TABLE IF NOT EXISTS mihomo_proxy_group_members (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id     INTEGER NOT NULL REFERENCES mihomo_proxy_groups(id) ON DELETE CASCADE,
  position     INTEGER NOT NULL,
  kind         TEXT NOT NULL,            -- direct|reject|reject-drop|reject-no-drop|pass|inbound|group
  inbound_id   INTEGER REFERENCES node_inbounds(id),
  ref_group_id INTEGER REFERENCES mihomo_proxy_groups(id),
  CHECK ((kind='inbound')=(inbound_id IS NOT NULL) AND (kind='group')=(ref_group_id IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_mihomo_pgmember_group ON mihomo_proxy_group_members(group_id);

-- Ordered routing rules with a typed target (PolicyRef): target_kind + an optional
-- inbound_id (inbound) or target_group_id (group). FKs RESTRICT, mirroring members.
CREATE TABLE IF NOT EXISTS mihomo_routing_rules (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  config_id       INTEGER NOT NULL REFERENCES subscription_configs(id) ON DELETE CASCADE,
  position        INTEGER NOT NULL,
  type            TEXT NOT NULL,         -- mihomo rule type (DOMAIN-SUFFIX, IP-CIDR, RULE-SET, MATCH, …)
  value           TEXT NOT NULL DEFAULT '',
  no_resolve      INTEGER NOT NULL DEFAULT 0,
  target_kind     TEXT NOT NULL,         -- PolicyKind
  inbound_id      INTEGER REFERENCES node_inbounds(id),
  target_group_id INTEGER REFERENCES mihomo_proxy_groups(id),
  CHECK ((target_kind='inbound')=(inbound_id IS NOT NULL) AND (target_kind='group')=(target_group_id IS NOT NULL))
);

-- interval: mihomo client ruleset auto-update TTL (seconds), always rendered.
-- mirror_interval: subgen mirror refresh period (seconds), used only when mirror=1.
CREATE TABLE IF NOT EXISTS mihomo_rule_providers (
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

-- mihomo base config + free-form settings (currently just base_yaml), per config.
CREATE TABLE IF NOT EXISTS mihomo_settings (
  config_id INTEGER NOT NULL REFERENCES subscription_configs(id) ON DELETE CASCADE,
  key       TEXT NOT NULL,
  value     TEXT NOT NULL,
  PRIMARY KEY(config_id, key)
);
