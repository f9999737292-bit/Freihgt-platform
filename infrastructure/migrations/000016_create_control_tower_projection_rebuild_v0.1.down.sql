DROP TABLE IF EXISTS control_tower.shipment_status_projection_rebuild_backup;
DROP TABLE IF EXISTS control_tower.shipment_status_projection_rebuild_stage;
DROP TABLE IF EXISTS control_tower.shipment_status_projection_rebuild_job;

ALTER TABLE control_tower.shipment_status_projection
    DROP CONSTRAINT IF EXISTS chk_shipment_status_projection_source;

ALTER TABLE control_tower.shipment_status_projection
    DROP COLUMN IF EXISTS rebuilt_at,
    DROP COLUMN IF EXISTS authoritative_as_of,
    DROP COLUMN IF EXISTS snapshot_id,
    DROP COLUMN IF EXISTS projection_source;
