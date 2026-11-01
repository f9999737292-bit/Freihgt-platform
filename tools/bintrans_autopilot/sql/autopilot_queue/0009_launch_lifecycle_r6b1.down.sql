-- Roll back R6B-1 launch lifecycle extensions.

REVOKE INSERT ON autopilot_queue.manual_reconciliation_audit FROM autopilot_launch_admin;

REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_mark_external_cancelled(text, text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_reopen_unknown(text, text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_manual_reject(text, text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_manual_confirm(text, text, text, text, text) FROM PUBLIC;

DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_mark_external_cancelled(text, text, text);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_reopen_unknown(text, text, text);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_manual_reject(text, text, text);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_manual_confirm(text, text, text, text, text);

DROP TRIGGER IF EXISTS launch_intents_immutable_ids ON autopilot_queue.launch_intents;
DROP FUNCTION IF EXISTS autopilot_queue._launch_intents_immutable_ids_trigger();

DROP INDEX IF EXISTS autopilot_queue.budget_reservations_tenant_launch_uidx;

GRANT UPDATE ON autopilot_queue.launch_intents
    TO autopilot_launch_adapter, autopilot_launch_recovery, autopilot_launch_admin;

-- Restore 0008 definitions for replaced functions (0008 down drops all; re-up from 8 file on full rollback).
