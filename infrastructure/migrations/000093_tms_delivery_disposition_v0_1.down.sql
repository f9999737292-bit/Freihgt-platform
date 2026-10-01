DROP TABLE IF EXISTS control_tower.delivery_disposition_projection;
DROP TABLE IF EXISTS control_tower.delivery_disposition_inbox;

DROP TRIGGER IF EXISTS trg_delivery_disposition_cases_fact_immutable ON transport.delivery_disposition_cases;
DROP FUNCTION IF EXISTS transport.reject_delivery_disposition_fact_rewrite();

DROP TRIGGER IF EXISTS trg_delivery_disposition_evidence_no_delete ON transport.delivery_disposition_evidence;
DROP TRIGGER IF EXISTS trg_delivery_disposition_evidence_no_update ON transport.delivery_disposition_evidence;
DROP TRIGGER IF EXISTS trg_delivery_disposition_audit_no_delete ON transport.delivery_disposition_audit;
DROP TRIGGER IF EXISTS trg_delivery_disposition_audit_no_update ON transport.delivery_disposition_audit;
DROP TRIGGER IF EXISTS trg_delivery_attempt_facts_no_delete ON transport.delivery_attempt_facts;
DROP TRIGGER IF EXISTS trg_delivery_attempt_facts_no_update ON transport.delivery_attempt_facts;
DROP FUNCTION IF EXISTS transport.deny_delivery_disposition_append_only();

DROP TABLE IF EXISTS transport.delivery_disposition_evidence;
DROP TABLE IF EXISTS transport.delivery_disposition_audit;
DROP TABLE IF EXISTS transport.delivery_disposition_commands;
DROP TABLE IF EXISTS transport.delivery_attempt_facts;
DROP TABLE IF EXISTS transport.delivery_disposition_cases;
