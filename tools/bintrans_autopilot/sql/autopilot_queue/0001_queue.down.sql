-- Drops only the known queue objects. There is no CASCADE.
-- MigrationRunner refuses to execute this file when the schema contains
-- anything else, when a foreign dependency exists, or when job rows exist
-- outside an explicit isolated test rollback.
-- The NOLOGIN role is cluster-wide and is left in place.
-- This statement does not touch the platform migration table.

DROP POLICY IF EXISTS jobs_select ON autopilot_queue.jobs;
DROP POLICY IF EXISTS jobs_insert ON autopilot_queue.jobs;
DROP POLICY IF EXISTS jobs_update ON autopilot_queue.jobs;
DROP TRIGGER IF EXISTS jobs_tenant_immutable ON autopilot_queue.jobs;
DROP TABLE IF EXISTS autopilot_queue.jobs;
DROP FUNCTION IF EXISTS autopilot_queue.bound_tenant();
DROP FUNCTION IF EXISTS autopilot_queue.reject_tenant_change();
DROP TABLE IF EXISTS autopilot_queue.tenant_principals;
DROP TABLE IF EXISTS autopilot_queue.schema_migrations;
DROP SCHEMA autopilot_queue;
