-- Removes only the cancel-recovery objects. Request rows, events, and relationship versions stay.

DROP TRIGGER IF EXISTS crr_heads_transition_guard ON company.carrier_relationship_request_heads;
DROP FUNCTION IF EXISTS company.reject_request_head_illegal_transition();
DROP INDEX IF EXISTS company.uq_crr_one_terminal_event;
DROP TABLE IF EXISTS company.carrier_relationship_cancel_idempotency;
