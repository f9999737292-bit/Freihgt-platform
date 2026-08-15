-- Driver Mobile Platform v0.1: idempotency, driver-reported exceptions, user binding constraint

CREATE TABLE IF NOT EXISTS transport.driver_operation_idempotency (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    driver_id UUID NOT NULL REFERENCES transport.drivers(id),
    operation_type VARCHAR(64) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id UUID NOT NULL,
    response_status_code INT NOT NULL,
    response_body JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_driver_operation_idempotency UNIQUE (tenant_id, driver_id, operation_type, idempotency_key)
);

CREATE INDEX idx_driver_operation_idempotency_resource
    ON transport.driver_operation_idempotency (tenant_id, resource_type, resource_id);

CREATE TABLE IF NOT EXISTS transport.driver_reported_exception (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    shipment_id UUID NOT NULL REFERENCES transport.shipments(id),
    driver_id UUID NOT NULL REFERENCES transport.drivers(id),
    category VARCHAR(64) NOT NULL,
    comment TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    source VARCHAR(32) NOT NULL DEFAULT 'driver',
    idempotency_key VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_driver_reported_exception_idempotency UNIQUE (tenant_id, driver_id, idempotency_key),
    CONSTRAINT chk_driver_reported_exception_category CHECK (category IN (
        'TRAFFIC', 'VEHICLE_BREAKDOWN', 'ACCIDENT', 'LOADING_DELAY', 'UNLOADING_DELAY',
        'CARGO_ISSUE', 'DOCUMENT_ISSUE', 'CUSTOMER_UNAVAILABLE', 'ROUTE_BLOCKED', 'OTHER'
    ))
);

CREATE INDEX idx_driver_reported_exception_shipment
    ON transport.driver_reported_exception (tenant_id, shipment_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uq_drivers_tenant_user_active
    ON transport.drivers (tenant_id, user_id)
    WHERE user_id IS NOT NULL AND deleted_at IS NULL;
