-- RFx v3.0E7 — per-carrier late submission permission requests.

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_participants_tenant_event_company
    ON rfx.rfx_participants (tenant_id, rfx_event_id, company_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_participants_tenant_event_id
    ON rfx.rfx_participants (tenant_id, rfx_event_id, id);

CREATE TABLE IF NOT EXISTS rfx.rfx_late_submission_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_event_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,
    participant_id UUID NULL,
    reason_code VARCHAR(50) NOT NULL,
    reason_text TEXT NOT NULL,
    requested_until TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'REQUESTED',
    approved_valid_from TIMESTAMPTZ NULL,
    approved_valid_until TIMESTAMPTZ NULL,
    decision_comment TEXT NULL,
    requested_by UUID NOT NULL,
    decided_by UUID NULL,
    decided_at TIMESTAMPTZ NULL,
    consumed_at TIMESTAMPTZ NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_rfx_late_submission_status CHECK (
        status IN ('REQUESTED', 'APPROVED', 'REJECTED', 'EXPIRED', 'CONSUMED')
    ),
    CONSTRAINT chk_rfx_late_submission_reason_code CHECK (
        reason_code IN (
            'TECHNICAL_FAILURE',
            'ORGANIZATIONAL_DELAY',
            'BUYER_REQUEST',
            'FORCE_MAJEURE',
            'OTHER'
        )
    ),
    CONSTRAINT chk_rfx_late_submission_reason_text_nonempty CHECK (length(trim(reason_text)) > 0),
    CONSTRAINT chk_rfx_late_submission_approved_window CHECK (
        (
            status IN ('APPROVED', 'CONSUMED')
            AND approved_valid_from IS NOT NULL
            AND approved_valid_until IS NOT NULL
            AND approved_valid_until > approved_valid_from
        )
        OR (
            status NOT IN ('APPROVED', 'CONSUMED')
            AND approved_valid_from IS NULL
            AND approved_valid_until IS NULL
        )
    ),
    CONSTRAINT chk_rfx_late_submission_consumed_at CHECK (
        (status = 'CONSUMED' AND consumed_at IS NOT NULL)
        OR (status <> 'CONSUMED' AND consumed_at IS NULL)
    )
);

ALTER TABLE rfx.rfx_late_submission_requests
    ADD CONSTRAINT fk_rfx_late_submission_event_composite
    FOREIGN KEY (tenant_id, rfx_event_id)
    REFERENCES rfx.rfx_events (tenant_id, id);

ALTER TABLE rfx.rfx_late_submission_requests
    ADD CONSTRAINT fk_rfx_late_submission_participant_composite
    FOREIGN KEY (tenant_id, rfx_event_id, carrier_company_id)
    REFERENCES rfx.rfx_participants (tenant_id, rfx_event_id, company_id);

ALTER TABLE rfx.rfx_late_submission_requests
    ADD CONSTRAINT fk_rfx_late_submission_participant_id_composite
    FOREIGN KEY (tenant_id, rfx_event_id, participant_id)
    REFERENCES rfx.rfx_participants (tenant_id, rfx_event_id, id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_late_submission_active_event_carrier
    ON rfx.rfx_late_submission_requests (tenant_id, rfx_event_id, carrier_company_id)
    WHERE status IN ('REQUESTED', 'APPROVED');

CREATE INDEX IF NOT EXISTS idx_rfx_late_submission_buyer_queue
    ON rfx.rfx_late_submission_requests (tenant_id, rfx_event_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_rfx_late_submission_carrier_lookup
    ON rfx.rfx_late_submission_requests (tenant_id, rfx_event_id, carrier_company_id, status);

CREATE TRIGGER trg_rfx_late_submission_requests_updated_at
    BEFORE UPDATE ON rfx.rfx_late_submission_requests
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();
