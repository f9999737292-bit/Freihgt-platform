-- TMS-POD-REQUIREMENT-FOUNDATION-0.4D.
-- Append-only decision versions for one delivery action.
-- This migration does not write known decisions and does not backfill history.

CREATE TABLE transport.transport_execution_pod_requirement_decisions (
    id uuid PRIMARY KEY,
    decision_version integer NOT NULL,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    execution_action_id uuid NOT NULL,
    execution_revision_id uuid NULL,
    execution_stop_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    cargo_id uuid NOT NULL,
    decision_state text NOT NULL,
    pod_required boolean NULL,
    required_document_type text NULL,
    acceptance_policy_id uuid NULL,
    acceptance_policy_version integer NULL,
    requirement_decided_at timestamptz NULL,
    business_effective_at timestamptz NULL,
    source_policy_id uuid NULL,
    source_policy_version integer NULL,
    source_rule_id text NULL,
    decision_actor_class text NULL,
    created_at timestamptz NOT NULL,
    supersedes_decision_id uuid NULL,
    CONSTRAINT transport_execution_pod_requirement_decisions_execution_fk
        FOREIGN KEY (execution_id, operating_tenant_id)
        REFERENCES transport.transport_executions (id, operating_tenant_id),
    CONSTRAINT transport_execution_pod_requirement_decisions_action_fk
        FOREIGN KEY (execution_action_id)
        REFERENCES transport.transport_execution_actions (id),
    CONSTRAINT transport_execution_pod_requirement_decisions_stop_fk
        FOREIGN KEY (execution_stop_id)
        REFERENCES transport.transport_execution_stops (id),
    CONSTRAINT transport_execution_pod_requirement_decisions_revision_fk
        FOREIGN KEY (execution_revision_id)
        REFERENCES transport.transport_execution_revisions (id),
    CONSTRAINT transport_execution_pod_requirement_decisions_shipment_fk
        FOREIGN KEY (shipment_id)
        REFERENCES transport.shipments (id),
    CONSTRAINT transport_execution_pod_requirement_decisions_cargo_fk
        FOREIGN KEY (cargo_id)
        REFERENCES transport.cargoes (id),
    CONSTRAINT transport_execution_pod_requirement_decisions_supersedes_fk
        FOREIGN KEY (supersedes_decision_id)
        REFERENCES transport.transport_execution_pod_requirement_decisions (id),
    CONSTRAINT transport_execution_pod_requirement_decisions_version_chk
        CHECK (decision_version >= 1),
    CONSTRAINT transport_execution_pod_requirement_decisions_version_uq
        UNIQUE (operating_tenant_id, execution_action_id, decision_version),
    CONSTRAINT transport_execution_pod_requirement_decisions_self_supersedes_chk
        CHECK (supersedes_decision_id IS NULL OR supersedes_decision_id <> id),
    CONSTRAINT transport_execution_pod_requirement_decisions_acceptance_version_chk
        CHECK (acceptance_policy_version IS NULL OR acceptance_policy_version >= 1),
    CONSTRAINT transport_execution_pod_requirement_decisions_source_version_chk
        CHECK (source_policy_version IS NULL OR source_policy_version >= 1),
    CONSTRAINT transport_execution_pod_requirement_decisions_state_chk
        CHECK (
            (
                decision_state = 'UNKNOWN'
                AND pod_required IS NULL
                AND required_document_type IS NULL
                AND acceptance_policy_id IS NULL
                AND acceptance_policy_version IS NULL
                AND requirement_decided_at IS NULL
            )
            OR (
                decision_state = 'KNOWN'
                AND pod_required IS NOT DISTINCT FROM TRUE
                AND required_document_type IS NOT DISTINCT FROM 'POD'
                AND acceptance_policy_id IS NOT NULL
                AND acceptance_policy_version IS NOT NULL
                AND requirement_decided_at IS NOT NULL
            )
            OR (
                decision_state = 'KNOWN'
                AND pod_required IS NOT DISTINCT FROM FALSE
                AND required_document_type IS NULL
                AND acceptance_policy_id IS NULL
                AND acceptance_policy_version IS NULL
                AND requirement_decided_at IS NOT NULL
            )
        )
);

CREATE INDEX transport_execution_pod_requirement_decisions_action_idx
    ON transport.transport_execution_pod_requirement_decisions (operating_tenant_id, execution_action_id, decision_version DESC);

CREATE FUNCTION transport.reject_pod_requirement_decision_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'POD_REQUIREMENT_DECISION_IMMUTABLE'
        USING ERRCODE = '23514';
END;
$$;

CREATE TRIGGER trg_pod_requirement_decisions_no_update
    BEFORE UPDATE ON transport.transport_execution_pod_requirement_decisions
    FOR EACH ROW
    EXECUTE FUNCTION transport.reject_pod_requirement_decision_mutation();

CREATE TRIGGER trg_pod_requirement_decisions_no_delete
    BEFORE DELETE ON transport.transport_execution_pod_requirement_decisions
    FOR EACH ROW
    EXECUTE FUNCTION transport.reject_pod_requirement_decision_mutation();

CREATE FUNCTION transport.validate_pod_requirement_decision_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    action_type text;
    action_stop uuid;
    action_shipment uuid;
    action_cargo uuid;
    stop_execution uuid;
    stop_tenant uuid;
    prior_tenant uuid;
    prior_action uuid;
    prior_version integer;
BEGIN
    SELECT a.action_type, a.execution_stop_id, a.shipment_id, a.cargo_id
      INTO action_type, action_stop, action_shipment, action_cargo
      FROM transport.transport_execution_actions AS a
     WHERE a.id = NEW.execution_action_id;
    IF action_type IS DISTINCT FROM 'DELIVERY'
       OR action_stop IS DISTINCT FROM NEW.execution_stop_id
       OR action_shipment IS DISTINCT FROM NEW.shipment_id
       OR action_cargo IS DISTINCT FROM NEW.cargo_id THEN
        RAISE EXCEPTION 'POD_REQUIREMENT_SUBJECT_MISMATCH'
            USING ERRCODE = '23514';
    END IF;

    SELECT s.execution_id, s.operating_tenant_id
      INTO stop_execution, stop_tenant
      FROM transport.transport_execution_stops AS s
     WHERE s.id = NEW.execution_stop_id;
    IF stop_execution IS DISTINCT FROM NEW.execution_id
       OR stop_tenant IS DISTINCT FROM NEW.operating_tenant_id THEN
        RAISE EXCEPTION 'POD_REQUIREMENT_SUBJECT_MISMATCH'
            USING ERRCODE = '23514';
    END IF;

    IF NEW.supersedes_decision_id IS NOT NULL THEN
        SELECT d.operating_tenant_id, d.execution_action_id, d.decision_version
          INTO prior_tenant, prior_action, prior_version
          FROM transport.transport_execution_pod_requirement_decisions AS d
         WHERE d.id = NEW.supersedes_decision_id;
        IF prior_tenant IS DISTINCT FROM NEW.operating_tenant_id
           OR prior_action IS DISTINCT FROM NEW.execution_action_id
           OR prior_version IS NULL
           OR prior_version >= NEW.decision_version THEN
            RAISE EXCEPTION 'POD_REQUIREMENT_SUPERSEDES_INVALID'
                USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_pod_requirement_decisions_insert
    BEFORE INSERT ON transport.transport_execution_pod_requirement_decisions
    FOR EACH ROW
    EXECUTE FUNCTION transport.validate_pod_requirement_decision_insert();
