-- TMS-RETURN-0.1 delivery rejection, return, and redirection.
-- Quantity is split on the existing cargo. Handling-unit and pallet masters are not invented.
-- Historical onboard evidence and completed execution facts stay append-only.

CREATE TABLE transport.delivery_disposition_cases (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL REFERENCES transport.transport_executions (id),
    source_revision_id uuid NOT NULL,
    source_execution_stop_id uuid NOT NULL REFERENCES transport.transport_execution_stops (id),
    source_action_id uuid NOT NULL REFERENCES transport.transport_execution_actions (id),
    shipment_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    cargo_id uuid NOT NULL REFERENCES transport.cargoes (id),
    handling_unit_id uuid NULL,
    pallet_id uuid NULL,
    attempted_quantity numeric(18,3) NOT NULL,
    accepted_quantity numeric(18,3) NOT NULL,
    rejected_quantity numeric(18,3) NOT NULL,
    pending_quantity numeric(18,3) NOT NULL,
    return_reserved_quantity numeric(18,3) NOT NULL,
    redirect_reserved_quantity numeric(18,3) NOT NULL,
    hold_reserved_quantity numeric(18,3) NOT NULL,
    resolved_quantity numeric(18,3) NOT NULL,
    uom text NOT NULL,
    reason_code text NOT NULL,
    reason_comment text NULL,
    disposition_type text NULL,
    status text NOT NULL,
    return_target_location_id uuid NULL,
    redirect_target_location_id uuid NULL,
    successor_revision_id uuid NULL,
    created_by_actor_kind text NOT NULL,
    created_by_actor_id uuid NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    version integer NOT NULL DEFAULT 1,
    CONSTRAINT delivery_disposition_cases_uom_chk CHECK (uom = 'PALLET'),
    CONSTRAINT delivery_disposition_cases_qty_chk CHECK (
        attempted_quantity > 0
        AND accepted_quantity >= 0
        AND rejected_quantity > 0
        AND attempted_quantity = accepted_quantity + rejected_quantity
        AND rejected_quantity = pending_quantity + return_reserved_quantity + redirect_reserved_quantity + hold_reserved_quantity + resolved_quantity
    ),
    CONSTRAINT delivery_disposition_cases_reason_chk CHECK (reason_code IN (
        'DAMAGE', 'MIS_SORT', 'SHORTAGE', 'OVERAGE', 'PACKAGING_DAMAGE', 'TEMPERATURE_DEVIATION',
        'QUALITY_REJECTION', 'DOCUMENT_PROBLEM', 'WRONG_PRODUCT', 'EXPIRED_PRODUCT', 'CUSTOMER_REFUSAL', 'OTHER'
    )),
    CONSTRAINT delivery_disposition_cases_comment_chk CHECK (
        reason_code <> 'OTHER' OR (reason_comment IS NOT NULL AND btrim(reason_comment) <> '')
    ),
    CONSTRAINT delivery_disposition_cases_type_chk CHECK (
        disposition_type IS NULL OR disposition_type IN ('RETURN_TO_ORIGIN', 'REDIRECT', 'HOLD_PENDING_DISPOSITION')
    ),
    CONSTRAINT delivery_disposition_cases_status_chk CHECK (status IN (
        'OPEN', 'DISPOSITION_PENDING', 'RETURN_AUTHORIZED', 'REDIRECT_AUTHORIZED',
        'IN_RETURN_TRANSIT', 'IN_REDIRECT_TRANSIT', 'RESOLVED', 'CANCELLED'
    )),
    CONSTRAINT delivery_disposition_cases_actor_chk CHECK (created_by_actor_kind IN ('DRIVER', 'OPERATOR', 'SYSTEM')),
    CONSTRAINT delivery_disposition_cases_version_chk CHECK (version >= 1)
);

CREATE INDEX delivery_disposition_cases_execution_idx
    ON transport.delivery_disposition_cases (operating_tenant_id, execution_id);
CREATE INDEX delivery_disposition_cases_stop_idx
    ON transport.delivery_disposition_cases (source_execution_stop_id);
CREATE INDEX delivery_disposition_cases_shipment_idx
    ON transport.delivery_disposition_cases (shipment_id);
CREATE INDEX delivery_disposition_cases_cargo_idx
    ON transport.delivery_disposition_cases (cargo_id);
CREATE INDEX delivery_disposition_cases_status_idx
    ON transport.delivery_disposition_cases (operating_tenant_id, status);

CREATE TABLE transport.delivery_attempt_facts (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL REFERENCES transport.transport_executions (id),
    source_revision_id uuid NOT NULL,
    source_execution_stop_id uuid NOT NULL,
    source_action_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    cargo_id uuid NOT NULL,
    attempted_quantity numeric(18,3) NOT NULL,
    accepted_quantity numeric(18,3) NOT NULL,
    rejected_quantity numeric(18,3) NOT NULL,
    uom text NOT NULL,
    disposition_case_id uuid NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT delivery_attempt_facts_qty_chk CHECK (
        attempted_quantity > 0
        AND accepted_quantity >= 0
        AND rejected_quantity >= 0
        AND attempted_quantity = accepted_quantity + rejected_quantity
        AND uom = 'PALLET'
    )
);

CREATE INDEX delivery_attempt_facts_cargo_idx
    ON transport.delivery_attempt_facts (execution_id, cargo_id);

CREATE TABLE transport.delivery_disposition_commands (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
    command_name text NOT NULL CHECK (char_length(command_name) BETWEEN 1 AND 64),
    request_sha256 text NOT NULL CHECK (char_length(request_sha256) = 64),
    result_json jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT delivery_disposition_commands_key_uq UNIQUE (operating_tenant_id, idempotency_key)
);

CREATE INDEX delivery_disposition_commands_execution_idx
    ON transport.delivery_disposition_commands (execution_id, created_at);

CREATE TABLE transport.delivery_disposition_audit (
    id uuid PRIMARY KEY,
    case_id uuid NOT NULL REFERENCES transport.delivery_disposition_cases (id),
    command_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    actor_kind text NOT NULL,
    actor_id uuid NULL,
    reason_code text NULL,
    previous_status text NULL,
    new_status text NOT NULL,
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX delivery_disposition_audit_case_idx
    ON transport.delivery_disposition_audit (case_id, created_at);

CREATE TABLE transport.delivery_disposition_evidence (
    id uuid PRIMARY KEY,
    case_id uuid NOT NULL REFERENCES transport.delivery_disposition_cases (id),
    operating_tenant_id uuid NOT NULL,
    evidence_type text NOT NULL CHECK (evidence_type IN (
        'PHOTO', 'DOCUMENT', 'DISCREPANCY_ACT', 'TEMPERATURE', 'DRIVER_NOTE', 'CONSIGNEE_NOTE'
    )),
    source text NOT NULL CHECK (btrim(source) <> ''),
    reference_id text NOT NULL CHECK (char_length(reference_id) BETWEEN 1 AND 512),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX delivery_disposition_evidence_case_idx
    ON transport.delivery_disposition_evidence (case_id, created_at);

CREATE FUNCTION transport.deny_delivery_disposition_append_only()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'delivery disposition history is append-only';
END;
$$;

CREATE TRIGGER trg_delivery_attempt_facts_no_update
    BEFORE UPDATE ON transport.delivery_attempt_facts
    FOR EACH ROW EXECUTE FUNCTION transport.deny_delivery_disposition_append_only();
CREATE TRIGGER trg_delivery_attempt_facts_no_delete
    BEFORE DELETE ON transport.delivery_attempt_facts
    FOR EACH ROW EXECUTE FUNCTION transport.deny_delivery_disposition_append_only();
CREATE TRIGGER trg_delivery_disposition_audit_no_update
    BEFORE UPDATE ON transport.delivery_disposition_audit
    FOR EACH ROW EXECUTE FUNCTION transport.deny_delivery_disposition_append_only();
CREATE TRIGGER trg_delivery_disposition_audit_no_delete
    BEFORE DELETE ON transport.delivery_disposition_audit
    FOR EACH ROW EXECUTE FUNCTION transport.deny_delivery_disposition_append_only();
CREATE TRIGGER trg_delivery_disposition_evidence_no_update
    BEFORE UPDATE ON transport.delivery_disposition_evidence
    FOR EACH ROW EXECUTE FUNCTION transport.deny_delivery_disposition_append_only();
CREATE TRIGGER trg_delivery_disposition_evidence_no_delete
    BEFORE DELETE ON transport.delivery_disposition_evidence
    FOR EACH ROW EXECUTE FUNCTION transport.deny_delivery_disposition_append_only();

CREATE FUNCTION transport.reject_delivery_disposition_fact_rewrite()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.attempted_quantity IS DISTINCT FROM OLD.attempted_quantity
        OR NEW.accepted_quantity IS DISTINCT FROM OLD.accepted_quantity
        OR NEW.rejected_quantity IS DISTINCT FROM OLD.rejected_quantity
        OR NEW.cargo_id IS DISTINCT FROM OLD.cargo_id
        OR NEW.source_execution_stop_id IS DISTINCT FROM OLD.source_execution_stop_id
        OR NEW.shipment_id IS DISTINCT FROM OLD.shipment_id
        OR NEW.reason_code IS DISTINCT FROM OLD.reason_code
    THEN
        RAISE EXCEPTION 'DISPOSITION_FACT_IMMUTABLE';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_delivery_disposition_cases_fact_immutable
    BEFORE UPDATE ON transport.delivery_disposition_cases
    FOR EACH ROW EXECUTE FUNCTION transport.reject_delivery_disposition_fact_rewrite();

CREATE SCHEMA IF NOT EXISTS control_tower;

CREATE TABLE control_tower.delivery_disposition_inbox (
    event_id uuid PRIMARY KEY,
    disposition_case_id uuid NOT NULL,
    event_type text NOT NULL,
    payload_sha256 text NOT NULL,
    processing_outcome text NOT NULL,
    received_at timestamptz NOT NULL
);

CREATE TABLE control_tower.delivery_disposition_projection (
    disposition_case_id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    source_execution_stop_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    cargo_id uuid NOT NULL,
    accepted_quantity numeric(18,3) NOT NULL,
    rejected_quantity numeric(18,3) NOT NULL,
    uom text NOT NULL,
    reason_code text NOT NULL,
    disposition_type text NULL,
    status text NOT NULL,
    target_location_id uuid NULL,
    disposition_sequence bigint NOT NULL CHECK (disposition_sequence > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE INDEX delivery_disposition_projection_execution_idx
    ON control_tower.delivery_disposition_projection (operating_tenant_id, execution_id, status);
