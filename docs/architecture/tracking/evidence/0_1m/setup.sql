-- Disposable lock-order model. This is not a platform migration.
-- Tables contain only the columns the modeled statements touch.
-- No foreign keys: 0.1L left FK key-share waits UNKNOWN, and this proof does not invent them.
-- Drop is limited to the schemas this file creates. It does not touch other databases.

DROP SCHEMA IF EXISTS transport CASCADE;
DROP SCHEMA IF EXISTS company CASCADE;

CREATE SCHEMA IF NOT EXISTS transport;
CREATE SCHEMA IF NOT EXISTS company;

CREATE TABLE transport.transport_executions (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    driver_id uuid,
    vehicle_id uuid,
    current_revision_id uuid,
    event_seq bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE transport.transport_execution_revisions (
    id uuid PRIMARY KEY,
    execution_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    status text NOT NULL,
    version integer NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE transport.shipments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    driver_id uuid,
    vehicle_id uuid,
    cargo_id uuid,
    carrier_company_id uuid,
    status text NOT NULL,
    version integer NOT NULL,
    deleted_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE transport.cargoes (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    version integer NOT NULL,
    deleted_at timestamptz
);

CREATE TABLE transport.drivers (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    carrier_company_id uuid NOT NULL,
    status text NOT NULL,
    deleted_at timestamptz
);

CREATE TABLE transport.vehicles (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    carrier_company_id uuid NOT NULL,
    status text NOT NULL,
    deleted_at timestamptz
);

CREATE TABLE transport.transport_execution_stops (
    id uuid PRIMARY KEY,
    execution_id uuid NOT NULL,
    status text NOT NULL
);

CREATE TABLE transport.transport_execution_active_shipments (
    shipment_tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    PRIMARY KEY (shipment_tenant_id, shipment_id)
);

CREATE TABLE company.carrier_relationship_heads (
    relationship_id uuid PRIMARY KEY,
    status text NOT NULL,
    current_version_id uuid
);

-- Reader contract for this probe only. Not installed as a production function.
-- Lock order: execution, shipment, active link, driver, vehicle.
-- 0.1K also rejects a null owner_epoch. Product SQL on the audited SHA has no
-- owner_epoch column, so this probe does not apply that rejection.
CREATE FUNCTION transport.read_binding_line(p_execution_id uuid, p_shipment_id uuid)
    RETURNS text
    LANGUAGE plpgsql
AS $fn$
DECLARE
    v_tenant uuid;
    v_driver uuid;
    v_vehicle uuid;
    v_link uuid;
    v_version integer;
    v_status text;
    v_revision uuid;
    v_seq bigint;
BEGIN
    PERFORM 1
      FROM transport.transport_executions
     WHERE id = p_execution_id
       FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'missing execution' USING ERRCODE = 'P0002';
    END IF;

    PERFORM 1
      FROM transport.shipments
     WHERE id = p_shipment_id
       AND deleted_at IS NULL
       FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'missing shipment' USING ERRCODE = 'P0002';
    END IF;

    SELECT tenant_id, driver_id, vehicle_id, version, status
      INTO v_tenant, v_driver, v_vehicle, v_version, v_status
      FROM transport.shipments
     WHERE id = p_shipment_id
       AND deleted_at IS NULL;

    SELECT execution_id
      INTO v_link
      FROM transport.transport_execution_active_shipments
     WHERE shipment_id = p_shipment_id
       AND shipment_tenant_id = v_tenant
       FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'no active link for execution target' USING ERRCODE = 'P0002';
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

    SELECT current_revision_id, event_seq
      INTO v_revision, v_seq
      FROM transport.transport_executions
     WHERE id = v_link;

    RETURN p_execution_id::text || '|' || p_shipment_id::text || '|' || v_version::text
        || '|' || coalesce(v_driver::text, '') || '|' || coalesce(v_vehicle::text, '')
        || '|' || v_status || '|' || v_link::text || '|' || coalesce(v_revision::text, '')
        || '|' || coalesce(v_seq::text, '');
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

GRANT USAGE ON SCHEMA transport, company TO reader_probe, writer_probe;
GRANT SELECT ON ALL TABLES IN SCHEMA transport TO reader_probe;
-- PostgreSQL 16 rejects SELECT FOR SHARE unless the role also has UPDATE.
-- reader_probe scripts never UPDATE. The privilege exists only so the five
-- FOR SHARE statements in read_binding_line are allowed.
GRANT UPDATE ON
    transport.transport_executions,
    transport.shipments,
    transport.transport_execution_active_shipments,
    transport.drivers,
    transport.vehicles
    TO reader_probe;
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA transport TO writer_probe;
GRANT SELECT, UPDATE ON company.carrier_relationship_heads TO writer_probe;
GRANT EXECUTE ON FUNCTION transport.read_binding_line(uuid, uuid) TO reader_probe;
