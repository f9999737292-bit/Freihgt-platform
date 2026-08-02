ALTER TABLE control_tower.shipment_status_projection_rebuild_job
    DROP CONSTRAINT IF EXISTS chk_projection_rebuild_job_state;

ALTER TABLE control_tower.shipment_status_projection_rebuild_job
    ADD CONSTRAINT chk_projection_rebuild_job_state
        CHECK (
            state IN (
                'IMPORTING',
                'VALIDATED',
                'ACTIVATING',
                'ACTIVE',
                'ROLLING_BACK',
                'ROLLED_BACK',
                'FAILED',
                'CANCELLED',
                'CLEANED'
            )
        );

ALTER TABLE control_tower.shipment_status_projection_rebuild_job
    ADD COLUMN IF NOT EXISTS backup_rows BIGINT,
    ADD COLUMN IF NOT EXISTS activated_rows BIGINT,
    ADD COLUMN IF NOT EXISTS backup_created_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS rollback_eligible BOOLEAN,
    ADD COLUMN IF NOT EXISTS rollback_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS rolled_back_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS activation_error_code VARCHAR(64),
    ADD COLUMN IF NOT EXISTS rollback_error_code VARCHAR(64);

ALTER TABLE control_tower.shipment_status_projection_rebuild_job
    DROP CONSTRAINT IF EXISTS chk_projection_rebuild_job_backup_rows;

ALTER TABLE control_tower.shipment_status_projection_rebuild_job
    ADD CONSTRAINT chk_projection_rebuild_job_backup_rows
        CHECK (backup_rows IS NULL OR backup_rows >= 0);

ALTER TABLE control_tower.shipment_status_projection_rebuild_job
    DROP CONSTRAINT IF EXISTS chk_projection_rebuild_job_activated_rows;

ALTER TABLE control_tower.shipment_status_projection_rebuild_job
    ADD CONSTRAINT chk_projection_rebuild_job_activated_rows
        CHECK (activated_rows IS NULL OR activated_rows >= 0);
