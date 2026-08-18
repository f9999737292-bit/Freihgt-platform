ALTER TABLE control_tower.shipment_status_projection_rebuild_backup
    DROP COLUMN IF EXISTS actual_delivery_at,
    DROP COLUMN IF EXISTS actual_pickup_at,
    DROP COLUMN IF EXISTS planned_delivery_at,
    DROP COLUMN IF EXISTS planned_pickup_at;

ALTER TABLE control_tower.shipment_status_projection_rebuild_stage
    DROP COLUMN IF EXISTS actual_delivery_at,
    DROP COLUMN IF EXISTS actual_pickup_at,
    DROP COLUMN IF EXISTS planned_delivery_at,
    DROP COLUMN IF EXISTS planned_pickup_at;

ALTER TABLE control_tower.shipment_status_projection
    DROP COLUMN IF EXISTS actual_delivery_at,
    DROP COLUMN IF EXISTS actual_pickup_at,
    DROP COLUMN IF EXISTS planned_delivery_at,
    DROP COLUMN IF EXISTS planned_pickup_at;
