ALTER TABLE control_tower.shipment_status_projection
    ALTER COLUMN last_event_type DROP NOT NULL;

ALTER TABLE control_tower.shipment_status_projection_rebuild_stage
    ADD COLUMN IF NOT EXISTS last_event_type VARCHAR(128);
