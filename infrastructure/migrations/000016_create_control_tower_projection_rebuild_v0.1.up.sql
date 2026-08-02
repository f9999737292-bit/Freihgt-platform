ALTER TABLE control_tower.shipment_status_projection
    ADD COLUMN IF NOT EXISTS projection_source VARCHAR(32) NOT NULL DEFAULT 'LIVE_EVENT',
    ADD COLUMN IF NOT EXISTS snapshot_id UUID,
    ADD COLUMN IF NOT EXISTS authoritative_as_of TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS rebuilt_at TIMESTAMPTZ;

ALTER TABLE control_tower.shipment_status_projection
    DROP CONSTRAINT IF EXISTS chk_shipment_status_projection_source;

ALTER TABLE control_tower.shipment_status_projection
    ADD CONSTRAINT chk_shipment_status_projection_source
        CHECK (projection_source IN ('LIVE_EVENT', 'AUTHORITATIVE_SNAPSHOT'));

CREATE TABLE control_tower.shipment_status_projection_rebuild_job (
    snapshot_id UUID PRIMARY KEY,
    schema_version INTEGER NOT NULL,
    scope VARCHAR(16) NOT NULL,
    tenant_id UUID,
    state VARCHAR(32) NOT NULL,

    started_at TIMESTAMPTZ NOT NULL,
    import_started_at TIMESTAMPTZ,
    validated_at TIMESTAMPTZ,
    activation_started_at TIMESTAMPTZ,
    activated_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,

    expected_rows BIGINT,
    imported_rows BIGINT NOT NULL DEFAULT 0,
    tenant_count BIGINT,
    expected_sha256 CHAR(64),
    actual_sha256 CHAR(64),
    error_code VARCHAR(64),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_projection_rebuild_job_scope
        CHECK (scope IN ('ALL', 'TENANT')),

    CONSTRAINT chk_projection_rebuild_job_state
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
        ),

    CONSTRAINT chk_projection_rebuild_job_schema_version
        CHECK (schema_version > 0),

    CONSTRAINT chk_projection_rebuild_job_tenant_scope
        CHECK (
            (scope = 'ALL' AND tenant_id IS NULL)
            OR (scope = 'TENANT' AND tenant_id IS NOT NULL)
        )
);

CREATE TABLE control_tower.shipment_status_projection_rebuild_stage (
    snapshot_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    shipment_id UUID NOT NULL,

    current_status VARCHAR(64) NOT NULL,
    previous_status VARCHAR(64),
    aggregate_version BIGINT NOT NULL,
    last_event_id UUID,
    last_source_event_id UUID,
    source_updated_at TIMESTAMPTZ NOT NULL,
    record_sequence BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (snapshot_id, tenant_id, shipment_id),

    CONSTRAINT fk_projection_rebuild_stage_job
        FOREIGN KEY (snapshot_id)
        REFERENCES control_tower.shipment_status_projection_rebuild_job (snapshot_id)
        ON DELETE CASCADE,

    CONSTRAINT chk_projection_rebuild_stage_version
        CHECK (aggregate_version >= 1)
);

CREATE INDEX idx_projection_rebuild_stage_snapshot_sequence
    ON control_tower.shipment_status_projection_rebuild_stage (snapshot_id, record_sequence);

CREATE INDEX idx_projection_rebuild_stage_snapshot_tenant
    ON control_tower.shipment_status_projection_rebuild_stage (snapshot_id, tenant_id);

CREATE TABLE control_tower.shipment_status_projection_rebuild_backup (
    snapshot_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    shipment_id UUID NOT NULL,

    shipment_version INTEGER NOT NULL,
    current_status VARCHAR(64) NOT NULL,
    previous_status VARCHAR(64),
    last_event_id UUID NOT NULL,
    last_source_event_id UUID NOT NULL,
    last_event_type VARCHAR(128) NOT NULL,
    last_occurred_at TIMESTAMPTZ NOT NULL,
    last_consumed_at TIMESTAMPTZ NOT NULL,
    complete BOOLEAN NOT NULL,
    gap_detected BOOLEAN NOT NULL,
    gap_from_version INTEGER,
    gap_to_version INTEGER,
    projection_source VARCHAR(32) NOT NULL DEFAULT 'LIVE_EVENT',
    snapshot_id_prev UUID,
    authoritative_as_of TIMESTAMPTZ,
    rebuilt_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    backed_up_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (snapshot_id, tenant_id, shipment_id),

    CONSTRAINT fk_projection_rebuild_backup_job
        FOREIGN KEY (snapshot_id)
        REFERENCES control_tower.shipment_status_projection_rebuild_job (snapshot_id)
        ON DELETE CASCADE
);

CREATE INDEX idx_projection_rebuild_backup_snapshot
    ON control_tower.shipment_status_projection_rebuild_backup (snapshot_id);
