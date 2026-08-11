-- 0005: give node_inbounds a `kind` + a kind-specific `settings` blob, so an inbound can
-- be a **hysteria2** node (its creds live in `settings` JSON) instead of a panel-managed
-- VLESS one. `kind NOT NULL DEFAULT 'vless'` backfills every existing row to vless (no
-- behaviour change) and is the only place a default lives — new writes always pass an
-- explicit kind. `settings` is NULLABLE: NULL for vless (no settings), JSON for hysteria2.
-- A hysteria2 inbound is NOT provisioned on 3x-ui (Xray has no hysteria2): the fleet renders
-- it from `settings`, and provisioning skips it.
ALTER TABLE node_inbounds ADD COLUMN kind     TEXT NOT NULL DEFAULT 'vless';
ALTER TABLE node_inbounds ADD COLUMN settings TEXT;
