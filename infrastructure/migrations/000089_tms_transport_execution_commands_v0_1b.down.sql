-- Removes only TMS-MSTOP-0.1B command objects. 0.1A execution tables stay.

DROP TRIGGER IF EXISTS transport_execution_actions_terminal_guard ON transport.transport_execution_actions;
DROP TRIGGER IF EXISTS transport_execution_stops_terminal_guard ON transport.transport_execution_stops;
DROP FUNCTION IF EXISTS transport.reject_completed_transport_execution_action_mutation();
DROP FUNCTION IF EXISTS transport.reject_completed_transport_execution_stop_mutation();
DROP TABLE IF EXISTS transport.transport_execution_command_audit;
DROP TABLE IF EXISTS transport.transport_execution_commands;
ALTER TABLE transport.transport_executions DROP COLUMN IF EXISTS event_seq;
