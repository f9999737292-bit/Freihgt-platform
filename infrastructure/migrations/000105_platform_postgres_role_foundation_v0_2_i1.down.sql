-- Reverses only the privileges added by 000105.
-- Does not remove roles, does not restore evidence or pin DML, and does not
-- alter the shared application login.

REVOKE INSERT, DELETE ON core.user_roles FROM company_runtime;
REVOKE INSERT, UPDATE ON core.company_memberships FROM company_runtime;
REVOKE INSERT, UPDATE ON core.companies FROM company_runtime;

REVOKE SELECT, INSERT ON transport.driver_task_response FROM shipment_runtime;
REVOKE SELECT, INSERT, UPDATE ON transport.driver_task FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.driver_reported_exception FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.driver_reported_delay FROM shipment_runtime;
REVOKE SELECT, INSERT ON transport.driver_operation_idempotency FROM shipment_runtime;
REVOKE SELECT, INSERT, UPDATE ON transport.driver_notification_delivery FROM shipment_runtime;
REVOKE SELECT, INSERT, UPDATE ON transport.driver_device FROM shipment_runtime;

REVOKE INSERT ON transport.delivery_disposition_evidence FROM shipment_runtime;
REVOKE INSERT ON transport.delivery_disposition_audit FROM shipment_runtime;
REVOKE SELECT, INSERT, UPDATE ON transport.delivery_disposition_commands FROM shipment_runtime;
REVOKE SELECT, INSERT, UPDATE ON transport.delivery_disposition_cases FROM shipment_runtime;
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
REVOKE SELECT ON core.users FROM shipment_runtime;

REVOKE INSERT ON transport.vehicles FROM shipment_runtime;
REVOKE INSERT ON transport.drivers FROM shipment_runtime;
REVOKE INSERT ON transport.shipments FROM shipment_runtime;

REVOKE USAGE ON SCHEMA rfx, documents FROM shipment_runtime;
