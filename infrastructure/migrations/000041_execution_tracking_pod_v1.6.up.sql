-- Execution tracking actuals on Control Tower shipment projection.
ALTER TABLE control_tower.shipment_status_projection
    ADD COLUMN IF NOT EXISTS planned_pickup_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS planned_delivery_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS actual_pickup_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS actual_delivery_at TIMESTAMPTZ;

ALTER TABLE control_tower.shipment_status_projection_rebuild_stage
    ADD COLUMN IF NOT EXISTS planned_pickup_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS planned_delivery_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS actual_pickup_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS actual_delivery_at TIMESTAMPTZ;

ALTER TABLE control_tower.shipment_status_projection_rebuild_backup
    ADD COLUMN IF NOT EXISTS planned_pickup_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS planned_delivery_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS actual_pickup_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS actual_delivery_at TIMESTAMPTZ;
