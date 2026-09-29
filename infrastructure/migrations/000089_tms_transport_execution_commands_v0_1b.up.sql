-- TMS-MSTOP-0.1B stop and action commands.
-- Adds command idempotency, command audit, an execution event sequence,
-- and database guards for completed stop and action facts.
-- Does not add a shipment status, EN_ROUTE, or a second cargo evidence ledger.

ALTER TABLE transport.transport_executions
    ADD COLUMN event_seq bigint NOT NULL DEFAULT 0;

CREATE TABLE transport.transport_execution_commands (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL REFERENCES transport.transport_executions (id),
    idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
    command_name text NOT NULL CHECK (char_length(command_name) BETWEEN 1 AND 64),
    request_sha256 text NOT NULL CHECK (char_length(request_sha256) = 64),
    result_json jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT transport_execution_commands_key_uq UNIQUE (operating_tenant_id, idempotency_key)
);

CREATE INDEX transport_execution_commands_execution_idx
    ON transport.transport_execution_commands (execution_id, created_at);

CREATE TABLE transport.transport_execution_command_audit (
    id uuid PRIMARY KEY,
    command_id uuid NOT NULL REFERENCES transport.transport_execution_commands (id),
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    revision_id uuid NOT NULL,
    stop_id uuid NULL,
    action_id uuid NULL,
    actor_kind text NOT NULL CHECK (actor_kind IN ('DRIVER', 'OPERATOR', 'SYSTEM')),
    actor_id uuid NULL,
    reason_code text NULL,
    skipped_ordinals integer[] NOT NULL DEFAULT '{}',
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX transport_execution_command_audit_execution_idx
    ON transport.transport_execution_command_audit (execution_id, created_at);

CREATE FUNCTION transport.reject_completed_transport_execution_stop_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status IN ('COMPLETED', 'SKIPPED', 'CANCELLED') THEN
        IF NEW.execution_id IS DISTINCT FROM OLD.execution_id
            OR NEW.ordinal IS DISTINCT FROM OLD.ordinal
            OR NEW.stop_role IS DISTINCT FROM OLD.stop_role
            OR NEW.point_kind IS DISTINCT FROM OLD.point_kind
            OR NEW.location_id IS DISTINCT FROM OLD.location_id
            OR NEW.latitude IS DISTINCT FROM OLD.latitude
            OR NEW.longitude IS DISTINCT FROM OLD.longitude
            OR NEW.planned_arrival IS DISTINCT FROM OLD.planned_arrival
            OR NEW.planned_departure IS DISTINCT FROM OLD.planned_departure
            OR NEW.service_duration_seconds IS DISTINCT FROM OLD.service_duration_seconds
            OR NEW.arrived_at IS DISTINCT FROM OLD.arrived_at
            OR NEW.service_started_at IS DISTINCT FROM OLD.service_started_at
            OR NEW.completed_at IS DISTINCT FROM OLD.completed_at
            OR NEW.status IS DISTINCT FROM OLD.status
            OR NEW.status_reason IS DISTINCT FROM OLD.status_reason
        THEN
            RAISE EXCEPTION 'TERMINAL_STOP_IMMUTABLE'
                USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER transport_execution_stops_terminal_guard
    BEFORE UPDATE ON transport.transport_execution_stops
    FOR EACH ROW
    EXECUTE FUNCTION transport.reject_completed_transport_execution_stop_mutation();

CREATE FUNCTION transport.reject_completed_transport_execution_action_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status = 'COMPLETED' THEN
        IF NEW.execution_stop_id IS DISTINCT FROM OLD.execution_stop_id
            OR NEW.shipment_id IS DISTINCT FROM OLD.shipment_id
            OR NEW.shipment_tenant_id IS DISTINCT FROM OLD.shipment_tenant_id
            OR NEW.cargo_id IS DISTINCT FROM OLD.cargo_id
            OR NEW.cargo_version IS DISTINCT FROM OLD.cargo_version
            OR NEW.action_type IS DISTINCT FROM OLD.action_type
            OR NEW.ordinal IS DISTINCT FROM OLD.ordinal
            OR NEW.evidence_id IS DISTINCT FROM OLD.evidence_id
            OR NEW.completed_at IS DISTINCT FROM OLD.completed_at
            OR NEW.status IS DISTINCT FROM OLD.status
        THEN
            RAISE EXCEPTION 'COMPLETED_ACTION_IMMUTABLE'
                USING ERRCODE = '23514';
        END IF;
    END IF;
    IF OLD.status IN ('COMPLETED', 'FAILED', 'CANCELLED') AND NEW.status IS DISTINCT FROM OLD.status THEN
        RAISE EXCEPTION 'TERMINAL_ACTION_IMMUTABLE'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER transport_execution_actions_terminal_guard
    BEFORE UPDATE ON transport.transport_execution_actions
    FOR EACH ROW
    EXECUTE FUNCTION transport.reject_completed_transport_execution_action_mutation();
