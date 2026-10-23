-- Platform PostgreSQL role foundation 0.2 I1.
-- Grants the company and shipment runtime gaps recorded by the role-coverage
-- contract. company_runtime and shipment_runtime stay NOLOGIN.
-- Does not create a LOGIN role or a host authentication file.
-- Does not alter the shared application login.
-- Does not install trg_new_activation_authorization.
-- Does not enable TMS_RELATIONSHIP_ACTIVATION_ENABLED.
-- Does not create an award-link lock function.
-- That function stays out until a server-owned caller identity exists.
-- See the role foundation security contract for the blocker.

-- company-service administration. Relationship Create, Accept, Cancel, and
-- Revoke already have their grants from 000103. These statements add only the
-- company, membership, and user-role writes in the company repository.
-- core.user_roles is not updated by that repository, so UPDATE stays absent.
-- Activation evidence and pins stay revoked.
-- UPDATE lists are the columns assigned by company_repository.go and
-- membership_repository.go. Tenant, company, and user identity columns are
-- not in those assignments.

GRANT INSERT ON core.companies TO company_runtime;
GRANT UPDATE (legal_name, short_name, legal_name_en, legal_name_zh, company_type, tax_id, registration_number, country_code, preferred_locale, status, updated_at, version, deleted_at) ON core.companies TO company_runtime;
GRANT INSERT ON core.company_memberships TO company_runtime;
GRANT UPDATE (position, status, deleted_at, updated_at, version) ON core.company_memberships TO company_runtime;
GRANT INSERT, DELETE ON core.user_roles TO company_runtime;

REVOKE INSERT, UPDATE, DELETE ON company.carrier_relationship_activation_authorizations FROM company_runtime;
REVOKE INSERT, UPDATE, DELETE ON company.carrier_relationship_activation_subjects FROM company_runtime;
REVOKE INSERT, UPDATE, DELETE ON transport.transport_execution_carrier_relationship_pins FROM company_runtime;

-- shipment-service. Project and CreateSuccessorRevision already have their
-- grants from 000103 and 000104. These statements add shipment create,
-- driver and vehicle insert, delivery disposition, source-authority reads,
-- and the driver-platform writes the shipment binary actually runs.
-- transport.transport_orders is a column update of status, version, and
-- updated_at. rfx.rfx_award_transport_orders is SELECT only.
-- core.users is limited to the columns read by the shipment creator query.
-- driver_task.id is assigned by the task insert conflict target. It is the
-- task key, not a tenant, company, or user identity column.

GRANT USAGE ON SCHEMA documents, rfx TO shipment_runtime;

GRANT INSERT ON transport.shipments TO shipment_runtime;
GRANT INSERT ON transport.drivers TO shipment_runtime;
GRANT INSERT ON transport.vehicles TO shipment_runtime;

GRANT SELECT (id, tenant_id, status, deleted_at) ON core.users TO shipment_runtime;
GRANT SELECT ON core.company_memberships TO shipment_runtime;
GRANT SELECT ON core.user_roles TO shipment_runtime;
GRANT SELECT ON core.roles TO shipment_runtime;

GRANT SELECT ON documents.documents TO shipment_runtime;
GRANT SELECT ON rfx.bids TO shipment_runtime;
GRANT SELECT ON rfx.rfx_award_transport_orders TO shipment_runtime;

GRANT SELECT, UPDATE (status, version, updated_at) ON transport.transport_orders TO shipment_runtime;

GRANT SELECT ON transport.transport_execution_pod_requirement_decisions TO shipment_runtime;
GRANT SELECT, INSERT ON transport.shipment_status_history TO shipment_runtime;
GRANT SELECT, INSERT ON transport.shipment_cargo_execution_evidence TO shipment_runtime;

GRANT SELECT, INSERT ON transport.delivery_attempt_facts TO shipment_runtime;
GRANT SELECT, INSERT ON transport.delivery_disposition_cases TO shipment_runtime;
GRANT UPDATE (disposition_type, status, pending_quantity, return_reserved_quantity, redirect_reserved_quantity, hold_reserved_quantity, return_target_location_id, redirect_target_location_id, successor_revision_id, resolved_quantity, version, updated_at) ON transport.delivery_disposition_cases TO shipment_runtime;
GRANT SELECT, INSERT ON transport.delivery_disposition_commands TO shipment_runtime;
GRANT UPDATE (result_json) ON transport.delivery_disposition_commands TO shipment_runtime;
GRANT INSERT ON transport.delivery_disposition_audit TO shipment_runtime;
GRANT INSERT ON transport.delivery_disposition_evidence TO shipment_runtime;

GRANT SELECT, INSERT ON transport.driver_device TO shipment_runtime;
GRANT UPDATE (push_token_hash, push_token_ciphertext, platform, app_version, locale, last_seen_at, updated_at, revoked_at) ON transport.driver_device TO shipment_runtime;
GRANT SELECT, INSERT ON transport.driver_notification_delivery TO shipment_runtime;
GRANT UPDATE (status, claimed_by, claimed_until, updated_at, attempt_count, provider_message_id, sent_at, error_code, next_attempt_at) ON transport.driver_notification_delivery TO shipment_runtime;
GRANT SELECT, INSERT ON transport.driver_operation_idempotency TO shipment_runtime;
GRANT SELECT, INSERT ON transport.driver_reported_delay TO shipment_runtime;
GRANT SELECT, INSERT ON transport.driver_reported_exception TO shipment_runtime;
GRANT SELECT, INSERT ON transport.driver_task TO shipment_runtime;
GRANT UPDATE (id, status, version, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at) ON transport.driver_task TO shipment_runtime;
GRANT SELECT, INSERT ON transport.driver_task_response TO shipment_runtime;

REVOKE INSERT, UPDATE, DELETE ON transport.transport_execution_carrier_relationship_pins FROM shipment_runtime;
REVOKE INSERT, UPDATE, DELETE ON company.carrier_relationship_activation_authorizations FROM shipment_runtime;
REVOKE INSERT, UPDATE, DELETE ON company.carrier_relationship_activation_subjects FROM shipment_runtime;
REVOKE DELETE ON transport.transport_execution_revisions FROM shipment_runtime;
REVOKE INSERT, UPDATE, DELETE ON rfx.rfx_award_transport_orders FROM shipment_runtime;
