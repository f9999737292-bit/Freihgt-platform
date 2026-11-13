-- Isolated stand-in. Not a platform migration and not freight_platform.
-- Columns are only those the 0.1H function reads or locks.
-- There are no foreign keys, enums, or row-level security policies.

CREATE SCHEMA transport;
CREATE SCHEMA tracking;

CREATE TABLE transport.shipments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    driver_id uuid,
    vehicle_id uuid,
    version integer NOT NULL,
    deleted_at timestamp with time zone,
    updated_at timestamp with time zone NOT NULL
);

CREATE TABLE transport.drivers (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    status text NOT NULL,
    deleted_at timestamp with time zone,
    updated_at timestamp with time zone NOT NULL
);

CREATE TABLE transport.vehicles (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    status text NOT NULL,
    updated_at timestamp with time zone NOT NULL
);

CREATE TABLE transport.transport_executions (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    driver_id uuid,
    vehicle_id uuid,
    current_revision_id uuid,
    updated_at timestamp with time zone NOT NULL
);

CREATE TABLE transport.transport_execution_revisions (
    id uuid PRIMARY KEY,
    version integer NOT NULL
);

CREATE TABLE transport.transport_execution_active_shipments (
    shipment_tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    execution_id uuid NOT NULL
);

CREATE TABLE transport.transport_execution_carrier_relationship_pins (
    execution_revision_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL
);

CREATE TABLE transport.binding_authority_decisions (
    decision_id uuid PRIMARY KEY,
    shipment_tenant_id uuid NOT NULL,
    operating_tenant_id uuid,
    shipment_id uuid NOT NULL,
    driver_id uuid,
    vehicle_id uuid,
    execution_id uuid,
    current_revision_id uuid,
    shipment_version integer,
    revision_version integer,
    owner_epoch text NOT NULL,
    use_status text NOT NULL,
    consumed_at timestamp with time zone,
    binding_id uuid
);

CREATE TABLE tracking.shipment_tracking_binding (
    binding_id uuid PRIMARY KEY,
    shipment_tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    driver_id uuid,
    vehicle_id uuid,
    execution_id uuid,
    decision_id uuid NOT NULL
);

INSERT INTO transport.drivers (id, tenant_id, status, deleted_at, updated_at) VALUES
    ('33333333-3333-3333-3333-333333333333', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'ACTIVE', NULL, '2000-01-01+00'),
    ('77777777-7777-7777-7777-777777777777', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'ACTIVE', NULL, '2000-01-01+00');

INSERT INTO transport.vehicles (id, tenant_id, status, updated_at) VALUES
    ('44444444-4444-4444-4444-444444444444', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'ACTIVE', '2000-01-01+00'),
    ('88888888-8888-8888-8888-888888888888', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'ACTIVE', '2000-01-01+00');

INSERT INTO transport.transport_execution_revisions (id, version) VALUES
    ('55555555-5555-5555-5555-555555555555', 7),
    ('12121212-1212-1212-1212-121212121212', 8);

INSERT INTO transport.transport_executions (
    id, operating_tenant_id, driver_id, vehicle_id, current_revision_id, updated_at
) VALUES
    ('11111111-1111-1111-1111-111111111111', 'cccccccc-cccc-cccc-cccc-cccccccccccc',
     '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444',
     '55555555-5555-5555-5555-555555555555', '2000-01-01+00'),
    ('99999999-9999-9999-9999-999999999999', 'cccccccc-cccc-cccc-cccc-cccccccccccc',
     '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444',
     '55555555-5555-5555-5555-555555555555', '2000-01-01+00');

INSERT INTO transport.shipments (id, tenant_id, driver_id, vehicle_id, version, deleted_at, updated_at) VALUES
    ('22222222-2222-2222-2222-222222222222', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444',
     3, NULL, '2000-01-01+00'),
    ('00000000-0000-0000-0000-000000000002', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     '33333333-3333-3333-3333-333333333333', NULL,
     1, '2000-01-02+00', '2000-01-01+00'),
    ('00000000-0000-0000-0000-000000000004', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     '00000000-0000-0000-0000-000000000099', NULL,
     1, NULL, '2000-01-01+00'),
    ('00000000-0000-0000-0000-000000000005', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     '33333333-3333-3333-3333-333333333333', NULL,
     1, NULL, '2000-01-01+00'),
    ('00000000-0000-0000-0000-000000000006', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     '33333333-3333-3333-3333-333333333333', NULL,
     1, NULL, '2000-01-01+00'),
    ('00000000-0000-0000-0000-000000000007', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     '33333333-3333-3333-3333-333333333333', NULL,
     1, NULL, '2000-01-01+00'),
    ('00000000-0000-0000-0000-000000000008', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     '33333333-3333-3333-3333-333333333333', NULL,
     1, NULL, '2000-01-01+00');

INSERT INTO transport.transport_execution_active_shipments (shipment_tenant_id, shipment_id, execution_id) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '00000000-0000-0000-0000-000000000005', '11111111-1111-1111-1111-111111111111'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '00000000-0000-0000-0000-000000000005', '99999999-9999-9999-9999-999999999999'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '00000000-0000-0000-0000-000000000008', '11111111-1111-1111-1111-111111111111');

INSERT INTO transport.transport_execution_carrier_relationship_pins (
    execution_revision_id, shipment_tenant_id, shipment_id
) VALUES (
    '55555555-5555-5555-5555-555555555555',
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    '22222222-2222-2222-2222-222222222222'
);

INSERT INTO transport.binding_authority_decisions (
    decision_id, shipment_tenant_id, operating_tenant_id, shipment_id, driver_id, vehicle_id,
    execution_id, current_revision_id, shipment_version, revision_version, owner_epoch,
    use_status, consumed_at, binding_id
) VALUES
    ('66666666-6666-6666-6666-666666666666', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', '22222222-2222-2222-2222-222222222222',
     '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444',
     '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555',
     3, 7, 'CURRENT', 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666671', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', '22222222-2222-2222-2222-222222222222',
     '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444',
     '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555',
     3, 7, 'CURRENT', 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666672', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', '22222222-2222-2222-2222-222222222222',
     '77777777-7777-7777-7777-777777777777', '44444444-4444-4444-4444-444444444444',
     '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555',
     3, 7, 'CURRENT', 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666673', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', '22222222-2222-2222-2222-222222222222',
     '33333333-3333-3333-3333-333333333333', '88888888-8888-8888-8888-888888888888',
     '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555',
     3, 7, 'CURRENT', 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666674', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', '22222222-2222-2222-2222-222222222222',
     '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444',
     '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555',
     3, 7, 'ABSENT', 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666675', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', '22222222-2222-2222-2222-222222222222',
     '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444',
     '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555',
     3, 7, 'STALE', 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666676', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     NULL, '00000000-0000-0000-0000-000000000006',
     '33333333-3333-3333-3333-333333333333', NULL,
     NULL, NULL, 1, NULL, 'ABSENT', 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666677', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     NULL, '00000000-0000-0000-0000-000000000007',
     '33333333-3333-3333-3333-333333333333', NULL,
     NULL, NULL, 1, NULL, 'ABSENT', 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666670', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', '22222222-2222-2222-2222-222222222222',
     '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444',
     '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555',
     3, 7, 'CURRENT', 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666678', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', '22222222-2222-2222-2222-222222222222',
     '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444',
     '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555',
     3, 7, 'CURRENT', 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666679', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     NULL, '00000000-0000-0000-0000-000000000008',
     '33333333-3333-3333-3333-333333333333', NULL,
     NULL, NULL, 1, NULL, 'ABSENT', 'ISSUED', NULL, NULL);

CREATE ROLE column_select_only NOLOGIN;
CREATE ROLE table_select_only NOLOGIN;
CREATE ROLE assignment_lock_owner NOLOGIN;
CREATE ROLE binding_authority_fence NOLOGIN;
CREATE ROLE tracking_runtime NOLOGIN;
CREATE ROLE column_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE table_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE fence_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE runtime_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE writer_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE command_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE inherit_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE set_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;

GRANT column_select_only TO column_probe;
GRANT table_select_only TO table_probe;
GRANT binding_authority_fence TO fence_probe;
GRANT tracking_runtime TO runtime_probe;

\ir grants.sql
