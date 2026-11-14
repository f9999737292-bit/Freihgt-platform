-- Isolated stand-in for the assignment-integrity proof.
-- This is not a platform migration and it is not applied to freight_platform.
-- Active-slot uniqueness and the one-ACTIVE-revision index follow 000088.
-- owner_epoch bigint does not exist in migration 000105.

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
    owner_epoch bigint,
    updated_at timestamp with time zone NOT NULL
);

CREATE TABLE transport.transport_execution_revisions (
    id uuid PRIMARY KEY,
    execution_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    status text NOT NULL CHECK (status IN ('ACTIVE', 'SUPERSEDED')),
    version integer NOT NULL CHECK (version >= 1),
    supersedes_revision_id uuid
);

CREATE UNIQUE INDEX transport_execution_revisions_one_active_idx
    ON transport.transport_execution_revisions (execution_id)
    WHERE status = 'ACTIVE';

CREATE TABLE transport.transport_execution_active_shipments (
    shipment_tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    PRIMARY KEY (shipment_tenant_id, shipment_id)
);

CREATE TABLE transport.transport_execution_participants (
    execution_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    PRIMARY KEY (execution_id, shipment_tenant_id, shipment_id)
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
    owner_epoch bigint,
    use_status text NOT NULL,
    consumed_at timestamp with time zone,
    binding_id uuid
);

\ir revision_guard.sql
\ir epoch_guard.sql

INSERT INTO transport.drivers (id, tenant_id, status, deleted_at, updated_at) VALUES
    ('33333333-3333-3333-3333-333333333333', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'ACTIVE', NULL, '2000-01-01+00');

INSERT INTO transport.vehicles (id, tenant_id, status, updated_at) VALUES
    ('44444444-4444-4444-4444-444444444444', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'ACTIVE', '2000-01-01+00');

INSERT INTO transport.shipments (id, tenant_id, driver_id, vehicle_id, version, deleted_at, updated_at) VALUES
    ('22222222-2222-2222-2222-222222222222', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444', 3, NULL, '2000-01-01+00'),
    ('00000000-0000-0000-0000-000000000009', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '33333333-3333-3333-3333-333333333333', NULL, 1, NULL, '2000-01-01+00'),
    ('00000000-0000-0000-0000-00000000000a', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', '33333333-3333-3333-3333-333333333333', NULL, 1, NULL, '2000-01-01+00'),
    ('0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e02', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '33333333-3333-3333-3333-333333333333', NULL, 1, NULL, '2000-01-01+00'),
    ('0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e05', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '33333333-3333-3333-3333-333333333333', NULL, 1, NULL, '2000-01-01+00'),
    ('0c0c0c0c-0c0c-0c0c-0c0c-0c0c0c0c0c02', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '33333333-3333-3333-3333-333333333333', NULL, 1, NULL, '2000-01-01+00'),
    ('00000000-0000-0000-0000-000000000006', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '33333333-3333-3333-3333-333333333333', NULL, 1, NULL, '2000-01-01+00');

INSERT INTO transport.transport_executions (
    id, operating_tenant_id, driver_id, vehicle_id, current_revision_id, owner_epoch, updated_at
) VALUES
    ('11111111-1111-1111-1111-111111111111', 'cccccccc-cccc-cccc-cccc-cccccccccccc', '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444', NULL, 4, '2000-01-01+00'),
    ('99999999-9999-9999-9999-999999999999', 'cccccccc-cccc-cccc-cccc-cccccccccccc', '33333333-3333-3333-3333-333333333333', NULL, NULL, 1, '2000-01-01+00'),
    ('abababab-abab-abab-abab-abababababab', 'cccccccc-cccc-cccc-cccc-cccccccccccc', '33333333-3333-3333-3333-333333333333', NULL, NULL, 2, '2000-01-01+00'),
    ('bcbcbcbc-bcbc-bcbc-bcbc-bcbcbcbcbcbc', 'dddddddd-dddd-dddd-dddd-dddddddddddd', '33333333-3333-3333-3333-333333333333', NULL, NULL, 2, '2000-01-01+00'),
    ('0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e01', 'cccccccc-cccc-cccc-cccc-cccccccccccc', '33333333-3333-3333-3333-333333333333', NULL, NULL, NULL, '2000-01-01+00'),
    ('0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e04', 'cccccccc-cccc-cccc-cccc-cccccccccccc', '33333333-3333-3333-3333-333333333333', NULL, NULL, 1, '2000-01-01+00'),
    ('0c0c0c0c-0c0c-0c0c-0c0c-0c0c0c0c0c01', 'cccccccc-cccc-cccc-cccc-cccccccccccc', '33333333-3333-3333-3333-333333333333', NULL, NULL, 2, '2000-01-01+00');

INSERT INTO transport.transport_execution_revisions (
    id, execution_id, operating_tenant_id, status, version, supersedes_revision_id
) VALUES
    ('55555555-5555-5555-5555-555555555555', '11111111-1111-1111-1111-111111111111', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'ACTIVE', 7, NULL),
    ('12121212-1212-1212-1212-121212121212', '11111111-1111-1111-1111-111111111111', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'SUPERSEDED', 8, '55555555-5555-5555-5555-555555555555'),
    ('56565656-5656-5656-5656-565656565656', '99999999-9999-9999-9999-999999999999', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'ACTIVE', 1, NULL),
    ('0a0a0a0a-0a0a-0a0a-0a0a-0a0a0a0a0a0a', 'abababab-abab-abab-abab-abababababab', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'ACTIVE', 1, NULL),
    ('0b0b0b0b-0b0b-0b0b-0b0b-0b0b0b0b0b0b', 'bcbcbcbc-bcbc-bcbc-bcbc-bcbcbcbcbcbc', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'ACTIVE', 1, NULL),
    ('0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e03', '0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e01', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'ACTIVE', 1, NULL),
    ('0c0c0c0c-0c0c-0c0c-0c0c-0c0c0c0c0c03', '0c0c0c0c-0c0c-0c0c-0c0c-0c0c0c0c0c01', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'SUPERSEDED', 1, NULL);

UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555555'
 WHERE id = '11111111-1111-1111-1111-111111111111';
UPDATE transport.transport_executions
   SET current_revision_id = '56565656-5656-5656-5656-565656565656'
 WHERE id = '99999999-9999-9999-9999-999999999999';
UPDATE transport.transport_executions
   SET current_revision_id = '0a0a0a0a-0a0a-0a0a-0a0a-0a0a0a0a0a0a'
 WHERE id = 'abababab-abab-abab-abab-abababababab';
UPDATE transport.transport_executions
   SET current_revision_id = '0b0b0b0b-0b0b-0b0b-0b0b-0b0b0b0b0b0b'
 WHERE id = 'bcbcbcbc-bcbc-bcbc-bcbc-bcbcbcbcbcbc';
UPDATE transport.transport_executions
   SET current_revision_id = '0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e03'
 WHERE id = '0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e01';
UPDATE transport.transport_executions
   SET current_revision_id = '0c0c0c0c-0c0c-0c0c-0c0c-0c0c0c0c0c03'
 WHERE id = '0c0c0c0c-0c0c-0c0c-0c0c-0c0c0c0c0c01';

INSERT INTO transport.transport_execution_active_shipments (shipment_tenant_id, shipment_id, execution_id) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '00000000-0000-0000-0000-000000000009', 'abababab-abab-abab-abab-abababababab'),
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', '00000000-0000-0000-0000-00000000000a', 'bcbcbcbc-bcbc-bcbc-bcbc-bcbcbcbcbcbc'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e02', '0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e01'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e05', '0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e04'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '0c0c0c0c-0c0c-0c0c-0c0c-0c0c0c0c0c02', '0c0c0c0c-0c0c-0c0c-0c0c-0c0c0c0c0c01');

INSERT INTO transport.transport_execution_participants (execution_id, shipment_tenant_id, shipment_id) VALUES
    ('99999999-9999-9999-9999-999999999999', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '22222222-2222-2222-2222-222222222222'),
    ('11111111-1111-1111-1111-111111111111', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '22222222-2222-2222-2222-222222222222');

INSERT INTO transport.binding_authority_decisions (
    decision_id, shipment_tenant_id, operating_tenant_id, shipment_id, driver_id, vehicle_id,
    execution_id, current_revision_id, shipment_version, revision_version, owner_epoch,
    use_status, consumed_at, binding_id
) VALUES
    ('66666666-6666-6666-6666-666666666666', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'cccccccc-cccc-cccc-cccc-cccccccccccc', '22222222-2222-2222-2222-222222222222', '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444', '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555', 3, 7, 4, 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666675', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'cccccccc-cccc-cccc-cccc-cccccccccccc', '22222222-2222-2222-2222-222222222222', '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444444', '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555', 3, 7, 3, 'ISSUED', NULL, NULL),
    ('66666666-6666-6666-6666-666666666676', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'cccccccc-cccc-cccc-cccc-cccccccccccc', '00000000-0000-0000-0000-000000000006', '33333333-3333-3333-3333-333333333333', NULL, NULL, NULL, 1, NULL, NULL, 'ISSUED', NULL, NULL);

CREATE ROLE assignment_lock_owner NOLOGIN;
CREATE ROLE binding_authority_fence NOLOGIN;
CREATE ROLE tracking_runtime NOLOGIN;
CREATE ROLE column_select_only NOLOGIN;
CREATE ROLE column_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE fence_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE runtime_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE command_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE writer_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE inherit_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE set_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;

GRANT column_select_only TO column_probe;
GRANT binding_authority_fence TO fence_probe;
GRANT tracking_runtime TO runtime_probe;

\ir grants.sql
