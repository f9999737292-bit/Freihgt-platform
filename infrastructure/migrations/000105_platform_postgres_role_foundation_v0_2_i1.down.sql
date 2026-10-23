-- Reverses only the privileges added by 000105.
-- Does not remove roles, does not restore evidence or pin DML, and does not
-- alter the shared application login.
-- Column revokes match the column grants. SELECT and UPDATE privileges that
-- 000103 and 000104 already granted are left in place.

REVOKE INSERT, DELETE ON core.user_roles FROM company_runtime;
REVOKE UPDATE (position, status, deleted_at, updated_at, version) ON core.company_memberships FROM company_runtime;
REVOKE INSERT ON core.company_memberships FROM company_runtime;
REVOKE UPDATE (legal_name, short_name, legal_name_en, legal_name_zh, company_type, tax_id, registration_number, country_code, preferred_locale, status, updated_at, version, deleted_at) ON core.companies FROM company_runtime;
REVOKE INSERT ON core.companies FROM company_runtime;

REVOKE SELECT, INSERT ON transport.driver_task_response FROM shipment_runtime;
REVOKE UPDATE (id, status, version, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at) ON transport.driver_task FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.driver_task FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.driver_reported_exception FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.driver_reported_delay FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.driver_operation_idempotency FROM shipment_runtime;
REVOKE UPDATE (status, claimed_by, claimed_until, updated_at, attempt_count, provider_message_id, sent_at, error_code, next_attempt_at) ON transport.driver_notification_delivery FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.driver_notification_delivery FROM shipment_runtime;
REVOKE UPDATE (push_token_hash, push_token_ciphertext, platform, app_version, locale, last_seen_at, updated_at, revoked_at) ON transport.driver_device FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.driver_device FROM shipment_runtime;

REVOKE INSERT ON transport.delivery_disposition_evidence FROM shipment_runtime;
REVOKE INSERT ON transport.delivery_disposition_audit FROM shipment_runtime;
REVOKE UPDATE (result_json) ON transport.delivery_disposition_commands FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.delivery_disposition_commands FROM shipment_runtime;
REVOKE UPDATE (disposition_type, status, pending_quantity, return_reserved_quantity, redirect_reserved_quantity, hold_reserved_quantity, return_target_location_id, redirect_target_location_id, successor_revision_id, resolved_quantity, version, updated_at) ON transport.delivery_disposition_cases FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.delivery_disposition_cases FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.delivery_attempt_facts FROM shipment_runtime;

REVOKE SELECT, INSERT ON transport.shipment_cargo_execution_evidence FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.shipment_status_history FROM shipment_runtime;
REVOKE SELECT ON transport.transport_execution_pod_requirement_decisions FROM shipment_runtime;

REVOKE UPDATE (status, version, updated_at) ON transport.transport_orders FROM shipment_runtime;
REVOKE SELECT ON transport.transport_orders FROM shipment_runtime;

REVOKE SELECT ON rfx.rfx_award_transport_orders FROM shipment_runtime;
REVOKE SELECT ON rfx.bids FROM shipment_runtime;
REVOKE SELECT ON documents.documents FROM shipment_runtime;

REVOKE SELECT ON core.roles FROM shipment_runtime;
REVOKE SELECT ON core.user_roles FROM shipment_runtime;
REVOKE SELECT ON core.company_memberships FROM shipment_runtime;
REVOKE SELECT (id, tenant_id, status, deleted_at) ON core.users FROM shipment_runtime;

REVOKE INSERT ON transport.vehicles FROM shipment_runtime;
REVOKE INSERT ON transport.drivers FROM shipment_runtime;
REVOKE INSERT ON transport.shipments FROM shipment_runtime;

REVOKE USAGE ON SCHEMA rfx, documents FROM shipment_runtime;
