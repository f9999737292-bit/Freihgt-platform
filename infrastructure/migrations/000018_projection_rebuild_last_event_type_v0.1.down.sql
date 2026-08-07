ALTER TABLE control_tower.shipment_status_projection_rebuild_stage
    DROP COLUMN IF EXISTS last_event_type;

ALTER TABLE control_tower.shipment_status_projection
    ALTER COLUMN last_event_type SET NOT NULL;
