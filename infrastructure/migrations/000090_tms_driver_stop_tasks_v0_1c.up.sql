-- TMS-MSTOP-0.1C driver stop tasks.
-- Separate from notice transport.driver_tasks.
-- One task per execution stop. Status mirrors the authoritative stop.

CREATE TABLE transport.driver_stop_tasks (
    id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL REFERENCES transport.transport_executions (id),
    execution_stop_id uuid NOT NULL REFERENCES transport.transport_execution_stops (id),
    driver_id uuid NULL,
    vehicle_id uuid NULL,
    shipment_id uuid NULL,
    ordinal integer NOT NULL,
    location_id uuid NOT NULL,
    planned_arrival timestamptz NULL,
    action_summary jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('PLANNED', 'ARRIVED', 'SERVICE_STARTED', 'COMPLETED', 'CANCELLED', 'SKIPPED')),
    version integer NOT NULL CHECK (version >= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT driver_stop_tasks_stop_uq UNIQUE (execution_stop_id)
);

CREATE INDEX driver_stop_tasks_driver_idx
    ON transport.driver_stop_tasks (operating_tenant_id, driver_id);

CREATE INDEX driver_stop_tasks_execution_idx
    ON transport.driver_stop_tasks (execution_id, ordinal);
