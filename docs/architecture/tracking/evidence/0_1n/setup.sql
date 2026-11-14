-- Disposable foreign-key model. Not a platform migration.
-- Omitted columns and unmodeled constraints are listed in FK_INVENTORY.md.
-- Every foreign key below is NOT DEFERRABLE and NO ACTION / NO ACTION
-- except transport_executions_current_revision_fk.

DROP SCHEMA IF EXISTS transport CASCADE;

-- Probe-only. Production DDL has no pgrowlocks extension.
-- Granted row locks are stored on the tuple and are absent from pg_locks
-- until another session waits. pgrowlocks reads the tuple lock mode.
CREATE EXTENSION IF NOT EXISTS pgrowlocks;
CREATE EXTENSION IF NOT EXISTS pageinspect;

CREATE SCHEMA transport;

CREATE TABLE transport.drivers (
    id uuid PRIMARY KEY
);

CREATE TABLE transport.vehicles (
    id uuid PRIMARY KEY
);

CREATE TABLE transport.cargoes (
    id uuid PRIMARY KEY
);

CREATE TABLE transport.locations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    location_type text NOT NULL,
    deleted_at timestamptz,
    lat double precision,
    lon double precision
);

CREATE TABLE transport.shipments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    driver_id uuid,
    vehicle_id uuid,
    origin_location_id uuid NOT NULL,
    cargo_id uuid,
    version integer NOT NULL,
    deleted_at timestamptz,
    CONSTRAINT shipments_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES transport.drivers (id),
    CONSTRAINT shipments_vehicle_id_fkey FOREIGN KEY (vehicle_id) REFERENCES transport.vehicles (id),
    CONSTRAINT shipments_origin_location_id_fkey FOREIGN KEY (origin_location_id) REFERENCES transport.locations (id),
    CONSTRAINT shipments_cargo_id_fkey FOREIGN KEY (cargo_id) REFERENCES transport.cargoes (id)
);

CREATE TABLE transport.transport_executions (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    driver_id uuid,
    vehicle_id uuid,
    current_revision_id uuid,
    event_seq bigint NOT NULL DEFAULT 0,
    CONSTRAINT transport_executions_id_tenant_uq UNIQUE (id, operating_tenant_id)
);

CREATE TABLE transport.transport_execution_revisions (
    id uuid PRIMARY KEY,
    execution_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    status text NOT NULL,
    supersedes_revision_id uuid,
    CONSTRAINT transport_execution_revisions_execution_fk
        FOREIGN KEY (execution_id, operating_tenant_id)
        REFERENCES transport.transport_executions (id, operating_tenant_id),
    CONSTRAINT transport_execution_revisions_supersedes_fk
        FOREIGN KEY (supersedes_revision_id)
        REFERENCES transport.transport_execution_revisions (id)
);

CREATE UNIQUE INDEX transport_execution_revisions_one_active_idx
    ON transport.transport_execution_revisions (execution_id)
    WHERE status = 'ACTIVE';

ALTER TABLE transport.transport_executions
    ADD CONSTRAINT transport_executions_current_revision_fk
    FOREIGN KEY (current_revision_id)
    REFERENCES transport.transport_execution_revisions (id)
    DEFERRABLE INITIALLY DEFERRED;

CREATE FUNCTION transport.reject_transport_execution_revision_mismatch()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.current_revision_id IS NULL THEN
        RETURN NEW;
    END IF;
    IF NOT EXISTS (
        SELECT 1
        FROM transport.transport_execution_revisions AS revision
        WHERE revision.id = NEW.current_revision_id
          AND revision.execution_id = NEW.id
          AND revision.operating_tenant_id = NEW.operating_tenant_id
    ) THEN
        RAISE EXCEPTION 'CURRENT_REVISION_MISMATCH'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER transport_executions_current_revision_guard
    BEFORE INSERT OR UPDATE OF current_revision_id, operating_tenant_id
    ON transport.transport_executions
    FOR EACH ROW
    EXECUTE FUNCTION transport.reject_transport_execution_revision_mismatch();

CREATE TABLE transport.transport_execution_participants (
    execution_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    cargo_id uuid NOT NULL,
    PRIMARY KEY (execution_id, shipment_tenant_id, shipment_id, cargo_id),
    CONSTRAINT transport_execution_participants_execution_id_fkey
        FOREIGN KEY (execution_id) REFERENCES transport.transport_executions (id)
);

CREATE TABLE transport.transport_execution_active_shipments (
    shipment_tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    PRIMARY KEY (shipment_tenant_id, shipment_id),
    CONSTRAINT transport_execution_active_shipments_execution_id_fkey
        FOREIGN KEY (execution_id) REFERENCES transport.transport_executions (id)
);

CREATE TABLE transport.transport_execution_stops (
    id uuid PRIMARY KEY,
    execution_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    ordinal integer NOT NULL,
    stop_role text NOT NULL,
    point_kind text NOT NULL,
    location_id uuid,
    latitude double precision NOT NULL,
    longitude double precision NOT NULL,
    status text NOT NULL,
    CONSTRAINT transport_execution_stops_execution_fk
        FOREIGN KEY (execution_id, operating_tenant_id)
        REFERENCES transport.transport_executions (id, operating_tenant_id),
    CONSTRAINT transport_execution_stops_point_chk CHECK (
        (point_kind = 'CANONICAL_LOCATION' AND location_id IS NOT NULL)
        OR point_kind = 'POSITION_ANCHOR'
    )
);

CREATE FUNCTION transport.reject_transport_execution_stop_reparent()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.execution_id IS DISTINCT FROM OLD.execution_id THEN
        RAISE EXCEPTION 'STOP_PARENT_SET_ONCE'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER transport_execution_stops_parent_once
    BEFORE UPDATE OF execution_id ON transport.transport_execution_stops
    FOR EACH ROW
    EXECUTE FUNCTION transport.reject_transport_execution_stop_reparent();

CREATE TABLE transport.transport_execution_actions (
    id uuid PRIMARY KEY,
    execution_stop_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    cargo_id uuid NOT NULL,
    action_type text NOT NULL,
    ordinal integer NOT NULL,
    status text NOT NULL,
    CONSTRAINT transport_execution_actions_execution_stop_id_fkey
        FOREIGN KEY (execution_stop_id) REFERENCES transport.transport_execution_stops (id)
);

CREATE TABLE transport.transport_execution_revision_stops (
    revision_id uuid NOT NULL,
    stop_id uuid NOT NULL,
    source_ordinal integer NOT NULL,
    membership text NOT NULL,
    PRIMARY KEY (revision_id, stop_id),
    CONSTRAINT transport_execution_revision_stops_revision_id_fkey
        FOREIGN KEY (revision_id) REFERENCES transport.transport_execution_revisions (id),
    CONSTRAINT transport_execution_revision_stops_stop_id_fkey
        FOREIGN KEY (stop_id) REFERENCES transport.transport_execution_stops (id)
);

CREATE TABLE transport.transport_execution_revision_actions (
    revision_id uuid NOT NULL,
    action_id uuid NOT NULL,
    source_action_ordinal integer NOT NULL,
    membership text NOT NULL,
    PRIMARY KEY (revision_id, action_id),
    CONSTRAINT transport_execution_revision_actions_revision_id_fkey
        FOREIGN KEY (revision_id) REFERENCES transport.transport_execution_revisions (id),
    CONSTRAINT transport_execution_revision_actions_action_id_fkey
        FOREIGN KEY (action_id) REFERENCES transport.transport_execution_actions (id)
);

CREATE TABLE transport.transport_execution_commands (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    command_name text NOT NULL,
    request_sha256 text NOT NULL,
    result_json jsonb NOT NULL,
    CONSTRAINT transport_execution_commands_execution_id_fkey
        FOREIGN KEY (execution_id) REFERENCES transport.transport_executions (id),
    CONSTRAINT transport_execution_commands_key_uq UNIQUE (operating_tenant_id, idempotency_key)
);

CREATE TABLE transport.delivery_disposition_cases (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    source_revision_id uuid NOT NULL,
    source_execution_stop_id uuid NOT NULL,
    source_action_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    cargo_id uuid NOT NULL,
    attempted_quantity numeric(18,3) NOT NULL,
    accepted_quantity numeric(18,3) NOT NULL,
    rejected_quantity numeric(18,3) NOT NULL,
    pending_quantity numeric(18,3) NOT NULL,
    return_reserved_quantity numeric(18,3) NOT NULL,
    redirect_reserved_quantity numeric(18,3) NOT NULL,
    hold_reserved_quantity numeric(18,3) NOT NULL,
    resolved_quantity numeric(18,3) NOT NULL,
    disposition_type text,
    status text NOT NULL,
    return_target_location_id uuid,
    redirect_target_location_id uuid,
    successor_revision_id uuid,
    version integer NOT NULL,
    CONSTRAINT delivery_disposition_cases_execution_id_fkey
        FOREIGN KEY (execution_id) REFERENCES transport.transport_executions (id),
    CONSTRAINT delivery_disposition_cases_source_execution_stop_id_fkey
        FOREIGN KEY (source_execution_stop_id) REFERENCES transport.transport_execution_stops (id),
    CONSTRAINT delivery_disposition_cases_source_action_id_fkey
        FOREIGN KEY (source_action_id) REFERENCES transport.transport_execution_actions (id),
    CONSTRAINT delivery_disposition_cases_cargo_id_fkey
        FOREIGN KEY (cargo_id) REFERENCES transport.cargoes (id),
    CONSTRAINT delivery_disposition_cases_qty_chk CHECK (
        attempted_quantity > 0
        AND accepted_quantity >= 0
        AND rejected_quantity > 0
        AND attempted_quantity = accepted_quantity + rejected_quantity
        AND rejected_quantity = pending_quantity + return_reserved_quantity
            + redirect_reserved_quantity + hold_reserved_quantity + resolved_quantity
    )
);

CREATE TABLE transport.delivery_disposition_commands (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    command_name text NOT NULL,
    request_sha256 text NOT NULL,
    result_json jsonb NOT NULL,
    CONSTRAINT delivery_disposition_commands_key_uq UNIQUE (operating_tenant_id, idempotency_key)
);

CREATE TABLE transport.delivery_disposition_audit (
    id uuid PRIMARY KEY,
    case_id uuid NOT NULL,
    command_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    previous_status text,
    new_status text NOT NULL,
    CONSTRAINT delivery_disposition_audit_case_id_fkey
        FOREIGN KEY (case_id) REFERENCES transport.delivery_disposition_cases (id)
);

CREATE FUNCTION transport.read_binding_line(p_execution_id uuid, p_shipment_id uuid)
    RETURNS text
    LANGUAGE plpgsql
AS $fn$
DECLARE
    v_tenant uuid;
    v_driver uuid;
    v_vehicle uuid;
    v_link uuid;
BEGIN
    PERFORM 1 FROM transport.transport_executions WHERE id = p_execution_id FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'missing execution' USING ERRCODE = 'P0002';
    END IF;
    PERFORM 1 FROM transport.shipments WHERE id = p_shipment_id AND deleted_at IS NULL FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'missing shipment' USING ERRCODE = 'P0002';
    END IF;
    SELECT tenant_id, driver_id, vehicle_id INTO v_tenant, v_driver, v_vehicle
      FROM transport.shipments WHERE id = p_shipment_id AND deleted_at IS NULL;
    SELECT execution_id INTO v_link
      FROM transport.transport_execution_active_shipments
     WHERE shipment_id = p_shipment_id AND shipment_tenant_id = v_tenant
       FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'no active link' USING ERRCODE = 'P0002';
    END IF;
    IF v_link IS DISTINCT FROM p_execution_id THEN
        RAISE EXCEPTION 'input execution is not the active link' USING ERRCODE = '23514';
    END IF;
    IF v_driver IS NOT NULL THEN
        PERFORM 1 FROM transport.drivers WHERE id = v_driver FOR SHARE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'missing driver' USING ERRCODE = 'P0002';
        END IF;
    END IF;
    IF v_vehicle IS NOT NULL THEN
        PERFORM 1 FROM transport.vehicles WHERE id = v_vehicle FOR SHARE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'missing vehicle' USING ERRCODE = 'P0002';
        END IF;
    END IF;
    RETURN 'HELD';
END;
$fn$;

DO $roles$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'reader_probe') THEN
        CREATE ROLE reader_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'writer_probe') THEN
        CREATE ROLE writer_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
    END IF;
END
$roles$;

GRANT USAGE ON SCHEMA transport TO reader_probe, writer_probe;
GRANT SELECT ON ALL TABLES IN SCHEMA transport TO reader_probe;
GRANT UPDATE ON
    transport.transport_executions,
    transport.shipments,
    transport.transport_execution_active_shipments,
    transport.drivers,
    transport.vehicles
    TO reader_probe;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA transport TO writer_probe;
GRANT EXECUTE ON FUNCTION transport.read_binding_line(uuid, uuid) TO reader_probe;
