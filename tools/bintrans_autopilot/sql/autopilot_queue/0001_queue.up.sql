-- Autopilot queue schema. This is not a platform migration.
-- Do not copy it into infrastructure/migrations. Platform head stays 000105.

CREATE SCHEMA IF NOT EXISTS autopilot_queue;

REVOKE ALL ON SCHEMA autopilot_queue FROM PUBLIC;

CREATE TABLE autopilot_queue.schema_migrations (
    version integer PRIMARY KEY,
    name text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE autopilot_queue.jobs (
    job_id text PRIMARY KEY,
    tenant_id text NOT NULL,
    job_type text NOT NULL,
    payload_version integer NOT NULL,
    payload jsonb NOT NULL,
    payload_digest text NOT NULL,
    status text NOT NULL,
    priority integer NOT NULL,
    created_at timestamptz NOT NULL,
    available_at timestamptz NOT NULL,
    attempt_count integer NOT NULL,
    max_attempts integer NOT NULL,
    backoff_seconds integer NOT NULL,
    lease_owner text,
    lease_token text,
    lease_expires_at timestamptz,
    last_error text,
    idempotency_key text NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT jobs_status_check CHECK (status IN ('PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED', 'DEAD')),
    CONSTRAINT jobs_attempts_check CHECK (attempt_count >= 0 AND max_attempts >= 1 AND attempt_count <= max_attempts),
    CONSTRAINT jobs_payload_version_check CHECK (payload_version >= 1),
    CONSTRAINT jobs_tenant_idem UNIQUE (tenant_id, idempotency_key),
    CONSTRAINT jobs_lease_token_unique UNIQUE (lease_token)
);

CREATE INDEX jobs_claim_idx
    ON autopilot_queue.jobs (tenant_id, priority, created_at, job_id)
    WHERE status = 'PENDING';

GRANT USAGE ON SCHEMA autopilot_queue TO autopilot_queue_runtime;
GRANT SELECT, INSERT, UPDATE ON autopilot_queue.jobs TO autopilot_queue_runtime;
REVOKE ALL ON autopilot_queue.schema_migrations FROM autopilot_queue_runtime;
REVOKE ALL ON autopilot_queue.jobs FROM PUBLIC;
REVOKE ALL ON autopilot_queue.schema_migrations FROM PUBLIC;
