-- TMS-MSTOP-0.1A transport execution foundation.
-- 000086 is NLO-0.4C on main. 000087 is reserved by EDO-0.3 I1.
-- The route root is transport.transport_executions. It has no shipment_id.
-- Stable stops and actions do not store revision ids or RoutePlan source ids.
-- Those ids live on the revision link tables.
-- Shipment ownership is not moved. Participants keep shipment_tenant_id.
-- One shipment may occupy at most one active execution slot.
-- Historical participant rows are not globally unique.

CREATE TABLE transport.transport_executions (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    carrier_company_id uuid NOT NULL,
    vehicle_id uuid NULL,
    driver_id uuid NULL,
    current_revision_id uuid NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT transport_executions_id_tenant_uq UNIQUE (id, operating_tenant_id)
);

CREATE TABLE transport.transport_execution_revisions (
    id uuid PRIMARY KEY,
    execution_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    source_route_plan_id uuid NOT NULL,
    source_route_plan_version integer NOT NULL CHECK (source_route_plan_version >= 1),
    source_activation_id uuid NOT NULL,
    source_activation_version integer NOT NULL CHECK (source_activation_version >= 1),
    evaluation_fingerprint text NOT NULL CHECK (char_length(evaluation_fingerprint) BETWEEN 1 AND 512),
    planning_mode text NOT NULL CHECK (planning_mode IN ('DEPOT_START', 'CURRENT_TRIP')),
    supersedes_revision_id uuid NULL,
    status text NOT NULL CHECK (status IN ('ACTIVE', 'SUPERSEDED')),
    version integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    contract_sha256 text NOT NULL CHECK (char_length(contract_sha256) = 64),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT transport_execution_revisions_execution_fk
        FOREIGN KEY (execution_id, operating_tenant_id)
        REFERENCES transport.transport_executions (id, operating_tenant_id),
    CONSTRAINT transport_execution_revisions_supersedes_fk
        FOREIGN KEY (supersedes_revision_id)
        REFERENCES transport.transport_execution_revisions (id),
    CONSTRAINT transport_execution_revisions_activation_uq
        UNIQUE (operating_tenant_id, source_activation_id)
);

CREATE UNIQUE INDEX transport_execution_revisions_one_active_idx
    ON transport.transport_execution_revisions (execution_id)
    WHERE status = 'ACTIVE';

ALTER TABLE transport.transport_executions
    ADD CONSTRAINT transport_executions_current_revision_fk
    FOREIGN KEY (current_revision_id)
    REFERENCES transport.transport_execution_revisions (id)
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE transport.transport_execution_participants (
    execution_id uuid NOT NULL REFERENCES transport.transport_executions (id),
    shipment_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    shipment_version integer NOT NULL CHECK (shipment_version >= 1),
    cargo_id uuid NOT NULL,
    cargo_version integer NOT NULL CHECK (cargo_version >= 1),
    route_subject_type text NOT NULL CHECK (route_subject_type IN ('LOAD_OPPORTUNITY', 'SHIPMENT_CARGO')),
    route_subject_id uuid NOT NULL,
    provenance text NOT NULL CHECK (char_length(provenance) BETWEEN 1 AND 128),
    PRIMARY KEY (execution_id, shipment_tenant_id, shipment_id, cargo_id)
);

CREATE INDEX transport_execution_participants_shipment_idx
    ON transport.transport_execution_participants (shipment_tenant_id, shipment_id);

-- Active slot only. Removed by a later wave when the shipment leaves the route.
-- Do not put this uniqueness on the historical participant table.
CREATE TABLE transport.transport_execution_active_shipments (
    shipment_tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    execution_id uuid NOT NULL REFERENCES transport.transport_executions (id),
    PRIMARY KEY (shipment_tenant_id, shipment_id)
);

CREATE TABLE transport.transport_execution_stops (
    id uuid PRIMARY KEY,
    execution_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    ordinal integer NOT NULL CHECK (ordinal >= 0),
    stop_role text NOT NULL CHECK (stop_role IN ('START', 'CARGO', 'END')),
    point_kind text NOT NULL CHECK (point_kind IN ('CANONICAL_LOCATION', 'POSITION_ANCHOR')),
    location_id uuid NULL,
    latitude double precision NOT NULL,
    longitude double precision NOT NULL,
    planned_arrival timestamptz NULL,
    planned_departure timestamptz NULL,
    service_duration_seconds integer NULL CHECK (service_duration_seconds IS NULL OR service_duration_seconds >= 0),
    status text NOT NULL CHECK (status IN (
        'PLANNED', 'ARRIVED', 'SERVICE_STARTED', 'COMPLETED', 'CANCELLED', 'SKIPPED'
    )),
    status_reason text NULL,
    version integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    arrived_at timestamptz NULL,
    service_started_at timestamptz NULL,
    completed_at timestamptz NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT transport_execution_stops_execution_fk
        FOREIGN KEY (execution_id, operating_tenant_id)
        REFERENCES transport.transport_executions (id, operating_tenant_id),
    CONSTRAINT transport_execution_stops_ordinal_uq UNIQUE (execution_id, ordinal),
    CONSTRAINT transport_execution_stops_point_chk CHECK (
        (point_kind = 'CANONICAL_LOCATION' AND location_id IS NOT NULL)
        OR point_kind = 'POSITION_ANCHOR'
    )
);

CREATE INDEX transport_execution_stops_execution_idx
    ON transport.transport_execution_stops (execution_id, ordinal);

CREATE TABLE transport.transport_execution_revision_stops (
    revision_id uuid NOT NULL REFERENCES transport.transport_execution_revisions (id),
    stop_id uuid NOT NULL REFERENCES transport.transport_execution_stops (id),
    source_route_plan_stop_id uuid NOT NULL,
    source_ordinal integer NOT NULL CHECK (source_ordinal >= 0),
    membership text NOT NULL CHECK (membership IN (
        'INTRODUCED', 'INHERITED_COMPLETED', 'INHERITED_IN_SERVICE', 'SUPERSEDED'
    )),
    PRIMARY KEY (revision_id, stop_id)
);

CREATE INDEX transport_execution_revision_stops_stop_idx
    ON transport.transport_execution_revision_stops (stop_id);

CREATE TABLE transport.transport_execution_actions (
    id uuid PRIMARY KEY,
    execution_stop_id uuid NOT NULL REFERENCES transport.transport_execution_stops (id),
    shipment_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    cargo_id uuid NOT NULL,
    cargo_version integer NOT NULL CHECK (cargo_version >= 1),
    route_subject_type text NOT NULL CHECK (route_subject_type IN ('LOAD_OPPORTUNITY', 'SHIPMENT_CARGO')),
    route_subject_id uuid NOT NULL,
    action_type text NOT NULL CHECK (action_type IN ('PICKUP', 'DELIVERY')),
    ordinal integer NOT NULL CHECK (ordinal >= 0),
    status text NOT NULL CHECK (status IN ('PENDING', 'COMPLETED', 'FAILED', 'CANCELLED')),
    evidence_id uuid NULL,
    completed_at timestamptz NULL,
    version integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    CONSTRAINT transport_execution_actions_ordinal_uq UNIQUE (execution_stop_id, ordinal)
);

CREATE INDEX transport_execution_actions_shipment_idx
    ON transport.transport_execution_actions (shipment_tenant_id, shipment_id);

CREATE TABLE transport.transport_execution_revision_actions (
    revision_id uuid NOT NULL REFERENCES transport.transport_execution_revisions (id),
    action_id uuid NOT NULL REFERENCES transport.transport_execution_actions (id),
    source_route_plan_action_id uuid NOT NULL,
    source_action_ordinal integer NOT NULL CHECK (source_action_ordinal >= 0),
    membership text NOT NULL CHECK (membership IN (
        'INTRODUCED', 'INHERITED_COMPLETED', 'INHERITED_IN_SERVICE', 'SUPERSEDED'
    )),
    PRIMARY KEY (revision_id, action_id)
);

CREATE INDEX transport_execution_revision_actions_action_idx
    ON transport.transport_execution_revision_actions (action_id);

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
