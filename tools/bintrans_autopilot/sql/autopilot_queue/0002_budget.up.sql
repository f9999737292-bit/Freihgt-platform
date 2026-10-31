-- Budget controller ledger for autopilot phase 0.3C2.
-- Platform migration head stays 000105. This history is separate.

CREATE TABLE autopilot_queue.budget_tenant_ledger (
    tenant_id text PRIMARY KEY,
    utc_day date NOT NULL,
    utc_month date NOT NULL,
    daily_limit_cents bigint NOT NULL CHECK (daily_limit_cents >= 0),
    monthly_limit_cents bigint NOT NULL CHECK (monthly_limit_cents >= 0),
    daily_reserved_cents bigint NOT NULL DEFAULT 0 CHECK (daily_reserved_cents >= 0),
    monthly_reserved_cents bigint NOT NULL DEFAULT 0 CHECK (monthly_reserved_cents >= 0),
    daily_committed_cents bigint NOT NULL DEFAULT 0 CHECK (daily_committed_cents >= 0),
    monthly_committed_cents bigint NOT NULL DEFAULT 0 CHECK (monthly_committed_cents >= 0),
    CONSTRAINT budget_tenant_ledger_tenant_check CHECK (tenant_id ~ '^[A-Za-z0-9._:-]{1,80}$')
);

CREATE TABLE autopilot_queue.budget_reservations (
    launch_id text PRIMARY KEY,
    tenant_id text NOT NULL,
    state text NOT NULL,
    reserve_cents bigint NOT NULL CHECK (reserve_cents >= 0),
    raw_cost_text text,
    charged_text text,
    charged_known boolean NOT NULL DEFAULT false,
    agent_id text,
    run_id text,
    block_reason text,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT budget_reservations_state_check CHECK (
        state IN ('REQUESTED', 'RESERVED', 'COMMITTED', 'RELEASED', 'BLOCKED')
    ),
    CONSTRAINT budget_reservations_tenant_check CHECK (tenant_id ~ '^[A-Za-z0-9._:-]{1,80}$'),
    CONSTRAINT budget_reservations_launch_check CHECK (launch_id ~ '^[A-Za-z0-9._:-]{1,120}$')
);

CREATE INDEX budget_reservations_tenant_state_idx
    ON autopilot_queue.budget_reservations (tenant_id, state, launch_id);

ALTER TABLE autopilot_queue.budget_tenant_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE autopilot_queue.budget_tenant_ledger FORCE ROW LEVEL SECURITY;

ALTER TABLE autopilot_queue.budget_reservations ENABLE ROW LEVEL SECURITY;
ALTER TABLE autopilot_queue.budget_reservations FORCE ROW LEVEL SECURITY;

CREATE POLICY budget_ledger_select ON autopilot_queue.budget_tenant_ledger
    FOR SELECT
    USING (tenant_id = autopilot_queue.bound_tenant());

CREATE POLICY budget_reservations_select ON autopilot_queue.budget_reservations
    FOR SELECT
    USING (tenant_id = autopilot_queue.bound_tenant());

GRANT SELECT ON autopilot_queue.budget_tenant_ledger TO autopilot_queue_runtime;
GRANT SELECT ON autopilot_queue.budget_reservations TO autopilot_queue_runtime;
