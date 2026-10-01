-- NLO-0.4D service-duration policy. Network-optimizer owns the rows.
-- Scope is the operating tenant. One ACTIVE policy per tenant. ACTIVE rows are immutable.
-- Does not write transport execution tables and does not set execution_supported.

CREATE TABLE network_optimizer.service_duration_policies (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    version integer NOT NULL CHECK (version >= 1),
    status text NOT NULL CHECK (status IN ('DRAFT', 'ACTIVE', 'RETIRED')),
    created_at timestamptz NOT NULL,
    published_at timestamptz NULL,
    retired_at timestamptz NULL,
    UNIQUE (tenant_id, version),
    CONSTRAINT service_duration_policies_publish_chk CHECK (
        (status = 'DRAFT' AND published_at IS NULL AND retired_at IS NULL)
        OR (status = 'ACTIVE' AND published_at IS NOT NULL AND retired_at IS NULL)
        OR (status = 'RETIRED' AND published_at IS NOT NULL AND retired_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX service_duration_policies_one_active_idx
    ON network_optimizer.service_duration_policies (tenant_id)
    WHERE status = 'ACTIVE';

CREATE TABLE network_optimizer.service_duration_policy_entries (
    policy_id uuid NOT NULL REFERENCES network_optimizer.service_duration_policies (id),
    action_type text NOT NULL CHECK (action_type IN ('PICKUP', 'DELIVERY')),
    duration_seconds integer NOT NULL CHECK (duration_seconds >= 0),
    PRIMARY KEY (policy_id, action_type)
);

CREATE FUNCTION network_optimizer.reject_service_duration_policy_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status = 'ACTIVE' AND NEW.status = 'RETIRED'
       AND NEW.id = OLD.id
       AND NEW.tenant_id = OLD.tenant_id
       AND NEW.version = OLD.version
       AND NEW.created_at = OLD.created_at
       AND NEW.published_at = OLD.published_at
       AND NEW.retired_at IS NOT NULL THEN
        RETURN NEW;
    END IF;
    IF OLD.status = 'DRAFT' AND NEW.status = 'ACTIVE'
       AND NEW.id = OLD.id
       AND NEW.tenant_id = OLD.tenant_id
       AND NEW.version = OLD.version
       AND NEW.created_at = OLD.created_at
       AND NEW.published_at IS NOT NULL
       AND NEW.retired_at IS NULL THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'SERVICE_DURATION_POLICY_IMMUTABLE'
        USING ERRCODE = '23514';
END;
$$;

CREATE TRIGGER service_duration_policies_immutable
    BEFORE UPDATE ON network_optimizer.service_duration_policies
    FOR EACH ROW
    EXECUTE FUNCTION network_optimizer.reject_service_duration_policy_mutation();

CREATE FUNCTION network_optimizer.reject_service_duration_entry_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    policy_status text;
BEGIN
    SELECT status INTO policy_status
    FROM network_optimizer.service_duration_policies
    WHERE id = COALESCE(NEW.policy_id, OLD.policy_id);
    IF policy_status IS DISTINCT FROM 'DRAFT' THEN
        RAISE EXCEPTION 'SERVICE_DURATION_POLICY_IMMUTABLE'
            USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER service_duration_policy_entries_immutable
    BEFORE UPDATE OR DELETE ON network_optimizer.service_duration_policy_entries
    FOR EACH ROW
    EXECUTE FUNCTION network_optimizer.reject_service_duration_entry_mutation();

ALTER TABLE network_optimizer.route_plan_dependencies
    DROP CONSTRAINT IF EXISTS route_plan_dependencies_dependency_kind_check;

ALTER TABLE network_optimizer.route_plan_dependencies
    ADD CONSTRAINT route_plan_dependencies_dependency_kind_check
    CHECK (dependency_kind IN (
        'SHIPMENT', 'CAPACITY', 'VEHICLE', 'LOAD_OPPORTUNITY', 'SHIPMENT_CARGO',
        'CURRENT_TRIP_CONTEXT', 'ROUTING_POLICY', 'CATALOG', 'RULE_SET', 'ALGORITHM_POLICY',
        'SERVICE_DURATION_POLICY'
    ));
