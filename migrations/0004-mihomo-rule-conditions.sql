-- 0004 — sub-conditions of logical routing rules (AND/OR/NOT). A logical rule
-- (mihomo_routing_rules.type IN ('AND','OR','NOT')) carries no value/provider; its
-- matcher is a tree of sub-conditions stored here, one row per node. The tree is a real
-- typed graph (not a JSON blob): a RULE-SET sub-condition's provider_id is a genuine FK,
-- exactly like a top-level RULE-SET rule.
--
-- A node belongs to one rule (rule_id, CASCADE — deleting the rule drops its whole tree,
-- which is how SaveMihomoConfig's replace works); parent_id is NULL for a root condition
-- (a direct child of the rule) or the owning condition for a nested one (self-reference,
-- CASCADE). position orders siblings. Sub-conditions carry no target and no no-resolve
-- (mihomo parses logical sub-conditions without params), so neither is stored.
--
-- Pure additive DDL (CREATE only) — runs inside the runner's transaction, no rebuild, so
-- this is a plain .sql (not .notx). The baseline (0001) is left untouched.
CREATE TABLE IF NOT EXISTS mihomo_rule_conditions (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  rule_id     INTEGER NOT NULL REFERENCES mihomo_routing_rules(id) ON DELETE CASCADE,
  parent_id   INTEGER REFERENCES mihomo_rule_conditions(id) ON DELETE CASCADE,
  position    INTEGER NOT NULL,
  type        TEXT NOT NULL,                                   -- RuleType (matcher, RULE-SET, or nested AND/OR/NOT)
  value       TEXT,                                            -- plain matcher payload; NULL for RULE-SET and logical types
  provider_id INTEGER REFERENCES mihomo_rule_providers(id)     -- RULE-SET sub-condition only
);
CREATE INDEX IF NOT EXISTS idx_mihomo_rcond_rule   ON mihomo_rule_conditions(rule_id);
CREATE INDEX IF NOT EXISTS idx_mihomo_rcond_parent ON mihomo_rule_conditions(parent_id);
