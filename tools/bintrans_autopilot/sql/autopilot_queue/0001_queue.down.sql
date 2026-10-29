-- Drops only the queue schema. The NOLOGIN role is cluster-wide and is left in place
-- so a down on one database does not remove a role still used by another database.
-- This statement does not touch the platform migration table.

DROP SCHEMA IF EXISTS autopilot_queue CASCADE;
