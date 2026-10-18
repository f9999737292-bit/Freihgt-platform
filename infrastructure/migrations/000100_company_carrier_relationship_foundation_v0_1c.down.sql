DROP TRIGGER IF EXISTS crr_heads_identity_guard ON company.carrier_relationship_request_heads;
DROP TRIGGER IF EXISTS cr_root_tenant_guard ON company.carrier_relationships;
DROP TRIGGER IF EXISTS crr_requests_tenant_guard ON company.carrier_relationship_requests;
DROP TRIGGER IF EXISTS cr_versions_append_only ON company.carrier_relationship_versions;
DROP TRIGGER IF EXISTS cr_root_append_only ON company.carrier_relationships;
DROP TRIGGER IF EXISTS crr_events_append_only ON company.carrier_relationship_request_events;
DROP TRIGGER IF EXISTS crr_requests_append_only ON company.carrier_relationship_requests;

DROP FUNCTION IF EXISTS company.reject_request_head_identity_change();
DROP FUNCTION IF EXISTS company.enforce_relationship_endpoint_tenants();
DROP FUNCTION IF EXISTS company.enforce_request_same_tenant();
DROP FUNCTION IF EXISTS company.reject_append_only_mutation();

DROP TABLE IF EXISTS company.carrier_relationship_outbox;
DROP TABLE IF EXISTS company.carrier_relationship_audit;
DROP TABLE IF EXISTS company.carrier_relationship_acceptance_idempotency;
DROP TABLE IF EXISTS company.carrier_relationship_heads;
DROP TABLE IF EXISTS company.carrier_relationship_versions;
DROP TABLE IF EXISTS company.carrier_relationships;
DROP TABLE IF EXISTS company.carrier_relationship_request_idempotency;
DROP TABLE IF EXISTS company.carrier_relationship_request_heads;
DROP TABLE IF EXISTS company.carrier_relationship_request_events;
DROP TABLE IF EXISTS company.carrier_relationship_requests;

DROP SCHEMA IF EXISTS company;
