-- Autopilot queue schema. This is not a platform migration.
-- Do not copy it into infrastructure/migrations. Platform head stays 000105.
--
-- Tenant identity is the authenticated session_user, mapped by the migration
-- owner in tenant_principals. Policies do not read a custom GUC. The runtime
-- role is NOLOGIN and cannot write that mapping.

CREATE SCHEMA IF NOT EXISTS autopilot_queue;

REVOKE ALL ON SCHEMA autopilot_queue FROM PUBLIC;
REVOKE CREATE ON SCHEMA autopilot_queue FROM PUBLIC;

CREATE TABLE autopilot_queue.schema_migrations (
    version integer PRIMARY KEY,
    name text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE autopilot_queue.tenant_principals (
    tenant_id text PRIMARY KEY,
    role_name text NOT NULL UNIQUE,
    CONSTRAINT tenant_principals_tenant_check CHECK (tenant_id ~ '^[A-Za-z0-9._:-]{1,80}$'),
    CONSTRAINT tenant_principals_role_check CHECK (role_name ~ '^[A-Za-z_][A-Za-z0-9_]{0,62}$')
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

CREATE FUNCTION autopilot_queue.bound_tenant() RETURNS text
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $$
    SELECT principals.tenant_id
    FROM autopilot_queue.tenant_principals AS principals
    WHERE principals.role_name = session_user
$$;

REVOKE ALL ON FUNCTION autopilot_queue.bound_tenant() FROM PUBLIC;

CREATE FUNCTION autopilot_queue.reject_tenant_change() RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $$
BEGIN
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id THEN
        RAISE EXCEPTION 'tenant_immutable' USING ERRCODE = '42501';
    END IF;
    RETURN NEW;
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue.reject_tenant_change() FROM PUBLIC;

CREATE TRIGGER jobs_tenant_immutable
    BEFORE UPDATE OF tenant_id ON autopilot_queue.jobs
    FOR EACH ROW
    EXECUTE FUNCTION autopilot_queue.reject_tenant_change();

ALTER TABLE autopilot_queue.jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE autopilot_queue.jobs FORCE ROW LEVEL SECURITY;

CREATE POLICY jobs_select ON autopilot_queue.jobs
    FOR SELECT
    USING (tenant_id = autopilot_queue.bound_tenant());

CREATE POLICY jobs_insert ON autopilot_queue.jobs
    FOR INSERT
    WITH CHECK (tenant_id = autopilot_queue.bound_tenant());

CREATE POLICY jobs_update ON autopilot_queue.jobs
    FOR UPDATE
    USING (tenant_id = autopilot_queue.bound_tenant())
    WITH CHECK (tenant_id = autopilot_queue.bound_tenant());

GRANT USAGE ON SCHEMA autopilot_queue TO autopilot_queue_runtime;
GRANT SELECT, INSERT ON autopilot_queue.jobs TO autopilot_queue_runtime;
GRANT UPDATE (
    status,
    attempt_count,
    lease_owner,
    lease_token,
    lease_expires_at,
    last_error,
    available_at,
    updated_at
) ON autopilot_queue.jobs TO autopilot_queue_runtime;
GRANT EXECUTE ON FUNCTION autopilot_queue.bound_tenant() TO autopilot_queue_runtime;
GRANT EXECUTE ON FUNCTION autopilot_queue.reject_tenant_change() TO autopilot_queue_runtime;
REVOKE ALL ON autopilot_queue.schema_migrations FROM autopilot_queue_runtime;
REVOKE ALL ON autopilot_queue.tenant_principals FROM autopilot_queue_runtime;
REVOKE ALL ON autopilot_queue.jobs FROM PUBLIC;
REVOKE ALL ON autopilot_queue.schema_migrations FROM PUBLIC;
REVOKE ALL ON autopilot_queue.tenant_principals FROM PUBLIC;
