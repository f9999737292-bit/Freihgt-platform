DROP TRIGGER IF EXISTS transport_executions_current_revision_guard ON transport.transport_executions;
DROP FUNCTION IF EXISTS transport.reject_transport_execution_revision_mismatch();

DROP TRIGGER IF EXISTS transport_execution_stops_parent_once ON transport.transport_execution_stops;
DROP FUNCTION IF EXISTS transport.reject_transport_execution_stop_reparent();

DROP TABLE IF EXISTS transport.transport_execution_revision_actions;
DROP TABLE IF EXISTS transport.transport_execution_actions;
DROP TABLE IF EXISTS transport.transport_execution_revision_stops;
DROP TABLE IF EXISTS transport.transport_execution_stops;
DROP TABLE IF EXISTS transport.transport_execution_active_shipments;
DROP TABLE IF EXISTS transport.transport_execution_participants;

ALTER TABLE IF EXISTS transport.transport_executions
    DROP CONSTRAINT IF EXISTS transport_executions_current_revision_fk;

DROP TABLE IF EXISTS transport.transport_execution_revisions;
DROP TABLE IF EXISTS transport.transport_executions;
