ALTER TABLE control_tower.shipment_status_projection_rebuild_job
    DROP CONSTRAINT IF EXISTS chk_projection_rebuild_job_activated_rows;

ALTER TABLE control_tower.shipment_status_projection_rebuild_job
    DROP CONSTRAINT IF EXISTS chk_projection_rebuild_job_backup_rows;

ALTER TABLE control_tower.shipment_status_projection_rebuild_job
    DROP COLUMN IF EXISTS rollback_error_code,
    DROP COLUMN IF EXISTS activation_error_code,
    DROP COLUMN IF EXISTS rolled_back_at,
    DROP COLUMN IF EXISTS rollback_started_at,
    DROP COLUMN IF EXISTS rollback_eligible,
    DROP COLUMN IF EXISTS backup_created_at,
    DROP COLUMN IF EXISTS activated_rows,
    DROP COLUMN IF EXISTS backup_rows;

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
                'FAILED',
                'CANCELLED',
                'CLEANED',
                'ROLLED_BACK'
            )
        );
