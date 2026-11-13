-- Isolated proof schema for TRACKING-BINDING-POSTGRES-PRIVILEGE-PROOF-0.1I-R2.
-- This file is not a BINTRANS platform migration.

CREATE SCHEMA IF NOT EXISTS transport;
REVOKE ALL ON SCHEMA transport FROM PUBLIC;

CREATE TABLE transport.shipments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    driver_id uuid,
    vehicle_id uuid,
    version integer NOT NULL,
    deleted_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE transport.binding_authority_decisions (
    decision_id uuid PRIMARY KEY,
    use_status text NOT NULL,
    consumed_at timestamptz
);

INSERT INTO transport.shipments (
    id, tenant_id, driver_id, vehicle_id, version
) VALUES (
    '22222222-2222-2222-2222-222222222222',
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    '33333333-3333-3333-3333-333333333333',
    '44444444-4444-4444-4444-444444444444',
    1
);

INSERT INTO transport.binding_authority_decisions (
    decision_id, use_status, consumed_at
) VALUES (
    '66666666-6666-6666-6666-666666666666',
    'ISSUED',
    NULL
);

CREATE ROLE column_select_only NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE table_select_only NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE assignment_lock_owner NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE binding_authority_fence NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE tracking_runtime NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE no_usage_role NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;

CREATE ROLE column_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE table_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE lock_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE fence_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE runtime_login LOGIN NOSUPERUSER INHERIT NOBYPASSRLS;
CREATE ROLE nopriv_login LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE no_usage_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE inherit_probe LOGIN NOSUPERUSER INHERIT NOBYPASSRLS;
CREATE ROLE set_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE writer_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;

GRANT column_select_only TO column_probe WITH INHERIT FALSE, SET TRUE;
GRANT table_select_only TO table_probe WITH INHERIT FALSE, SET TRUE;
GRANT assignment_lock_owner TO lock_probe WITH INHERIT FALSE, SET TRUE;
GRANT binding_authority_fence TO fence_probe WITH INHERIT FALSE, SET TRUE;
GRANT tracking_runtime TO runtime_login WITH INHERIT TRUE, SET FALSE;
GRANT no_usage_role TO no_usage_probe WITH INHERIT FALSE, SET TRUE;

GRANT USAGE ON SCHEMA transport TO
    column_select_only,
    table_select_only,
    assignment_lock_owner,
    binding_authority_fence,
    tracking_runtime,
    nopriv_login,
    writer_probe;

GRANT SELECT (id, tenant_id, driver_id, vehicle_id, version, deleted_at)
    ON transport.shipments TO column_select_only;
GRANT SELECT (id, tenant_id, driver_id, vehicle_id, version, deleted_at)
    ON transport.shipments TO assignment_lock_owner;
GRANT SELECT ON transport.shipments TO table_select_only;

GRANT SELECT (id, driver_id) ON transport.shipments TO writer_probe;
GRANT UPDATE (driver_id) ON transport.shipments TO writer_probe;
