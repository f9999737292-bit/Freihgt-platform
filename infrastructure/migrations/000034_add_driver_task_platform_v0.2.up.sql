-- Driver Mobile Platform v0.2 — Tasks / Inbox / Push

CREATE TABLE IF NOT EXISTS transport.driver_task (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    driver_id           UUID NOT NULL,
    shipment_id         UUID,
    task_type           VARCHAR(64) NOT NULL,
    status              VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    priority            VARCHAR(16) NOT NULL DEFAULT 'NORMAL',
    title               VARCHAR(255) NOT NULL,
    payload             JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    available_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at          TIMESTAMPTZ,
    delivered_at        TIMESTAMPTZ,
    read_at             TIMESTAMPTZ,
    acknowledged_at     TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    created_by_type     VARCHAR(32) NOT NULL,
    created_by_id       UUID,
    source              VARCHAR(32) NOT NULL,
    correlation_id      VARCHAR(128),
    source_event_id     VARCHAR(128),
    idempotency_key     VARCHAR(128),
    version             INT NOT NULL DEFAULT 1,
    CONSTRAINT chk_driver_task_status CHECK (status IN (
        'PENDING', 'DELIVERED', 'READ', 'ACKNOWLEDGED', 'COMPLETED', 'EXPIRED', 'CANCELLED'
    )),
    CONSTRAINT chk_driver_task_priority CHECK (priority IN ('NORMAL', 'HIGH', 'CRITICAL')),
    CONSTRAINT chk_driver_task_type CHECK (task_type IN (
        'REQUEST_DELAY_REASON',
        'REQUEST_STATUS_CONFIRMATION',
        'REQUEST_ARRIVAL_CONFIRMATION',
        'REQUEST_DOCUMENT_ACTION',
        'GENERAL_OPERATIONAL_NOTICE'
    )),
    CONSTRAINT chk_driver_task_source CHECK (source IN ('SYSTEM', 'CONTROL_TOWER', 'OPERATOR'))
);

CREATE UNIQUE INDEX uq_driver_task_idempotency
    ON transport.driver_task (tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX uq_driver_task_source_event
    ON transport.driver_task (tenant_id, source, source_event_id, task_type, driver_id, shipment_id)
    WHERE source_event_id IS NOT NULL AND shipment_id IS NOT NULL;

CREATE INDEX idx_driver_task_inbox
    ON transport.driver_task (tenant_id, driver_id, status, created_at DESC);

CREATE INDEX idx_driver_task_expires
    ON transport.driver_task (tenant_id, status, expires_at)
    WHERE status IN ('PENDING', 'DELIVERED', 'READ', 'ACKNOWLEDGED') AND expires_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS transport.driver_task_response (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    task_id             UUID NOT NULL REFERENCES transport.driver_task(id),
    driver_id           UUID NOT NULL,
    response_type       VARCHAR(64) NOT NULL,
    response_body       JSONB NOT NULL,
    occurred_at         TIMESTAMPTZ,
    received_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    idempotency_key     VARCHAR(128) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_driver_task_response_idempotency UNIQUE (tenant_id, task_id, idempotency_key)
);

CREATE INDEX idx_driver_task_response_task
    ON transport.driver_task_response (tenant_id, task_id);

CREATE TABLE IF NOT EXISTS transport.driver_device (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    driver_id           UUID NOT NULL,
    platform            VARCHAR(16) NOT NULL,
    push_provider       VARCHAR(16) NOT NULL DEFAULT 'FCM',
    push_token_hash     VARCHAR(64) NOT NULL,
    push_token_ciphertext TEXT NOT NULL,
    device_instance_id  VARCHAR(128) NOT NULL,
    app_version         VARCHAR(32),
    locale              VARCHAR(16),
    last_seen_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at          TIMESTAMPTZ,
    CONSTRAINT chk_driver_device_platform CHECK (platform IN ('ANDROID', 'IOS', 'WEB')),
    CONSTRAINT chk_driver_device_provider CHECK (push_provider IN ('FCM'))
);

CREATE UNIQUE INDEX uq_driver_device_active_instance
    ON transport.driver_device (tenant_id, driver_id, device_instance_id)
    WHERE revoked_at IS NULL;

CREATE UNIQUE INDEX uq_driver_device_active_token
    ON transport.driver_device (tenant_id, push_token_hash)
    WHERE revoked_at IS NULL;

CREATE INDEX idx_driver_device_active
    ON transport.driver_device (tenant_id, driver_id)
    WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS transport.driver_notification_delivery (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    driver_id           UUID NOT NULL,
    task_id             UUID NOT NULL REFERENCES transport.driver_task(id),
    channel             VARCHAR(16) NOT NULL DEFAULT 'PUSH',
    status              VARCHAR(32) NOT NULL DEFAULT 'pending',
    provider            VARCHAR(32),
    attempt_count       INT NOT NULL DEFAULT 0,
    max_attempts        INT NOT NULL DEFAULT 3,
    next_attempt_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    claimed_by          VARCHAR(64),
    claimed_until       TIMESTAMPTZ,
    provider_message_id VARCHAR(256),
    error_code          VARCHAR(64),
    sent_at             TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_driver_notification_channel CHECK (channel IN ('PUSH')),
    CONSTRAINT chk_driver_notification_status CHECK (status IN (
        'pending', 'processing', 'sent', 'skipped', 'failed', 'no_device'
    )),
    CONSTRAINT uq_driver_notification_task_channel UNIQUE (tenant_id, task_id, channel)
);

CREATE INDEX idx_driver_notification_pending
    ON transport.driver_notification_delivery (status, next_attempt_at)
    WHERE status IN ('pending', 'processing');
