-- Drops only the known queue objects. There is no CASCADE.
-- MigrationRunner refuses to execute this file unless the live catalogs match
-- version 1 exactly: tables, columns, defaults, constraints, indexes,
-- triggers, policies, functions, sequences, views, and types. A sequence
-- owned by a queue column, including one outside this schema, blocks the
-- file. A foreign static dependency blocks it as well. Dynamic SQL is not
-- visible here; the runner also refuses this file outside an isolated test
-- database. Job rows block it unless that isolated test rollback is explicit.
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
