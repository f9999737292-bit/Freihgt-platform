-- Project, TMS_RELATIONSHIP_ACTIVATION_ENABLED off.
-- Entry: ExecutionProjectionHandler.Create -> AcceptRoutePlanActivation -> Project.
-- Transaction: transport_execution_repository.go:31 BEGIN, :126 COMMIT.
-- Modeled statements: verifyMaterializedRows :261 and :276 (plain SELECT),
-- insertExecution :295, insertRevision :305, UPDATE new execution :92,
-- insert active link :335.
-- Omitted: participants, stops, actions, driver-stop tasks, outbox, digest.
-- No FOR UPDATE of an existing execution, shipment, driver, or vehicle.
-- Target shipment already has an active slot, so the link INSERT conflicts.
BEGIN;
SELECT version
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222'
   AND tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND deleted_at IS NULL;
SELECT version
  FROM transport.cargoes
 WHERE id = '44444444-4444-4444-4444-444444444444'
   AND tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND deleted_at IS NULL;
INSERT INTO transport.transport_executions (
    id, operating_tenant_id, driver_id, vehicle_id, current_revision_id, event_seq
) VALUES (
    '15151515-1515-1515-1515-151515151515', '00000000-0000-0000-0000-00000000000b',
    '77777777-7777-7777-7777-777777777777', '33333333-3333-3333-3333-333333333333', NULL, 0
);
INSERT INTO transport.transport_execution_revisions (
    id, execution_id, operating_tenant_id, status, version
) VALUES (
    '55555555-5555-5555-5555-555555555551', '15151515-1515-1515-1515-151515151515',
    '00000000-0000-0000-0000-00000000000b', 'ACTIVE', 1
);
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555551', updated_at = now()
 WHERE id = '15151515-1515-1515-1515-151515151515'
   AND operating_tenant_id = '00000000-0000-0000-0000-00000000000b';
INSERT INTO transport.transport_execution_active_shipments (
    shipment_tenant_id, shipment_id, execution_id
) VALUES (
    '00000000-0000-0000-0000-00000000000a', '22222222-2222-2222-2222-222222222222',
    '15151515-1515-1515-1515-151515151515'
);
COMMIT;
