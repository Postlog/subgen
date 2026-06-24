-- 0005: live providers. Nodes and authored rule lists are now delivered as mihomo
-- proxy-providers / rule-providers (the core refreshes them on their interval while the
-- tunnel is up) instead of inline. Three additive changes — no table rebuild, so this is
-- a plain transactional migration (the runner wraps it and records it; see migrations/run.go).
--
--  1. mihomo_rule_providers.source: 'external' (upstream URL, the legacy behavior, default
--     for existing rows) or 'authored' (an in-subgen matcher list subgen serves itself).
--  2. mihomo_authored_matchers: the recursive matcher tree of an authored provider — leaf
--     and logical (AND/OR/NOT) rules with NO target (the action comes from the referencing
--     RULE-SET rule in the skeleton). Mirrors mihomo_routing_rules' parent_id self-ref;
--     scoped by provider_id (surrogate id added in 0003) with ON DELETE CASCADE so a save's
--     provider delete drops its matchers.
--  3. mihomo_profile.proxies_interval: the auto-generated proxy-provider's refresh TTL in
--     seconds (core-level). Defaults to 3600 for existing rows.

ALTER TABLE mihomo_rule_providers ADD COLUMN source TEXT NOT NULL DEFAULT 'external';

CREATE TABLE IF NOT EXISTS mihomo_authored_matchers (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  provider_id INTEGER NOT NULL REFERENCES mihomo_rule_providers(id) ON DELETE CASCADE,
  parent_id   INTEGER REFERENCES mihomo_authored_matchers(id) ON DELETE CASCADE,
  position    INTEGER NOT NULL,
  type        TEXT NOT NULL,         -- mihomo matcher type (DOMAIN-SUFFIX, IP-CIDR, AND, …; never MATCH/RULE-SET/SUB-RULE)
  value       TEXT                   -- leaf payload; NULL for a logical (AND/OR/NOT) node
);

CREATE INDEX IF NOT EXISTS idx_mihomo_authored_provider ON mihomo_authored_matchers(provider_id);
CREATE INDEX IF NOT EXISTS idx_mihomo_authored_parent ON mihomo_authored_matchers(parent_id);

ALTER TABLE mihomo_profile ADD COLUMN proxies_interval INTEGER NOT NULL DEFAULT 3600;
