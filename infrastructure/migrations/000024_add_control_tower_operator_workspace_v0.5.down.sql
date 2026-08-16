DROP TABLE IF EXISTS control_tower.shift_handoff_item;
DROP TABLE IF EXISTS control_tower.shift_handoff;
DROP TABLE IF EXISTS control_tower.user_workspace_preference;
DROP TABLE IF EXISTS control_tower.saved_view;

ALTER TABLE control_tower.shipment_risk
    DROP COLUMN IF EXISTS owner_user_id,
    DROP COLUMN IF EXISTS owned_at,
    DROP COLUMN IF EXISTS owned_by_user_id;

DROP INDEX IF EXISTS control_tower.idx_shipment_risk_tenant_owner_active;
