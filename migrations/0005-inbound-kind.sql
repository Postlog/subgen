-- 0005: give node_inbounds a `kind` + a kind-specific `settings` blob, so an inbound can
-- be a static **hysteria2** node (its creds live in `settings` JSON) instead of a
-- panel-managed VLESS one. Existing rows default to 'vless' (panel-managed) with empty
-- settings — no behaviour change. A hysteria2 inbound is NOT provisioned on 3x-ui (Xray has
-- no hysteria2): the fleet renders it from `settings`, and provisioning skips it.
ALTER TABLE node_inbounds ADD COLUMN kind     TEXT NOT NULL DEFAULT 'vless';
ALTER TABLE node_inbounds ADD COLUMN settings TEXT NOT NULL DEFAULT '';
