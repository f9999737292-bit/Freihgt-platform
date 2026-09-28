-- NLO-0.4C accept and activate. Append-only activation rows.
-- Does not alter shipment tables and does not relax execution_supported = false.

CREATE TABLE network_optimizer.route_plan_activations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    route_plan_id uuid NOT NULL REFERENCES network_optimizer.route_plans (id),
    plan_version integer NOT NULL CHECK (plan_version >= 1),
    idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
    execution_shipment_id uuid NULL,
    -- Set only while this row is the shipment's effective activation.
    -- Cleared in the same transaction that supersedes the plan. NULL is not unique.
    effective_shipment_id uuid NULL,
    status text NOT NULL CHECK (status IN ('PENDING_EXECUTION', 'EXECUTION_LINKED', 'REJECTED')),
    created_at timestamptz NOT NULL,
    UNIQUE (tenant_id, route_plan_id),
    UNIQUE (tenant_id, idempotency_key)
);

CREATE UNIQUE INDEX route_plan_activations_one_effective_shipment_idx
    ON network_optimizer.route_plan_activations (tenant_id, effective_shipment_id)
    WHERE effective_shipment_id IS NOT NULL;

CREATE INDEX route_plan_activations_shipment_idx
    ON network_optimizer.route_plan_activations (tenant_id, execution_shipment_id, status);
