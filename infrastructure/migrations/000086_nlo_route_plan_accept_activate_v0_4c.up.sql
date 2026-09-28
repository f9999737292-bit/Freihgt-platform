-- NLO-0.4C accept and activate. Append-only activation rows.
-- Does not alter shipment tables and does not relax execution_supported = false.
-- execution_shipment_id is the planning context shipment. It is not TransportExecution.id.
-- execution_id and execution_revision_id belong to the execution projection.
-- They are not execution_shipment_id. Public activate leaves them null.

CREATE TABLE network_optimizer.route_plan_activations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    route_plan_id uuid NOT NULL REFERENCES network_optimizer.route_plans (id),
    plan_version integer NOT NULL CHECK (plan_version >= 1),
    version integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
    execution_shipment_id uuid NULL,
    -- Set only while this row is the shipment's effective EXECUTION_LINKED activation.
    -- Pending and rejected rows leave it null. NULL is not unique.
    effective_shipment_id uuid NULL,
    execution_id uuid NULL,
    execution_revision_id uuid NULL,
    status text NOT NULL CHECK (status IN ('PENDING_EXECUTION', 'EXECUTION_LINKED', 'REJECTED')),
    created_at timestamptz NOT NULL,
    UNIQUE (tenant_id, route_plan_id),
    UNIQUE (tenant_id, idempotency_key),
    CONSTRAINT route_plan_activations_linkage_all_or_nothing_chk CHECK (
        (execution_id IS NULL AND execution_revision_id IS NULL)
        OR (execution_id IS NOT NULL AND execution_revision_id IS NOT NULL)
    ),
    CONSTRAINT route_plan_activations_status_linkage_chk CHECK (
        (
            status = 'EXECUTION_LINKED'
            AND execution_id IS NOT NULL
            AND execution_revision_id IS NOT NULL
        )
        OR (
            status IN ('PENDING_EXECUTION', 'REJECTED')
            AND execution_id IS NULL
            AND execution_revision_id IS NULL
            AND effective_shipment_id IS NULL
        )
    )
);

CREATE UNIQUE INDEX route_plan_activations_one_effective_shipment_idx
    ON network_optimizer.route_plan_activations (tenant_id, effective_shipment_id)
    WHERE effective_shipment_id IS NOT NULL;

CREATE INDEX route_plan_activations_shipment_idx
    ON network_optimizer.route_plan_activations (tenant_id, execution_shipment_id, status);
