DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM control_tower.shipment_status_projection_rebuild_backup
        WHERE last_event_type IS NULL
    ) THEN
        RAISE EXCEPTION
            'cannot restore NOT NULL: backup rows contain NULL last_event_type';
    END IF;

    ALTER TABLE control_tower.shipment_status_projection_rebuild_backup
        ALTER COLUMN last_event_type SET NOT NULL;
END
$$;
