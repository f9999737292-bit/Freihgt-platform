-- Fail closed while policy rows exist. Empty rollback restores the 0.4B dependency list.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM network_optimizer.service_duration_policies) THEN
        RAISE EXCEPTION 'nlo-0.4d down migration refused: service duration policy rows exist';
    END IF;
END $$;

DROP TRIGGER IF EXISTS service_duration_policy_entries_immutable ON network_optimizer.service_duration_policy_entries;
DROP TRIGGER IF EXISTS service_duration_policies_immutable ON network_optimizer.service_duration_policies;
DROP FUNCTION IF EXISTS network_optimizer.reject_service_duration_entry_mutation();
DROP FUNCTION IF EXISTS network_optimizer.reject_service_duration_policy_mutation();
DROP TABLE IF EXISTS network_optimizer.service_duration_policy_entries;
DROP TABLE IF EXISTS network_optimizer.service_duration_policies;

ALTER TABLE network_optimizer.route_plan_dependencies
    DROP CONSTRAINT IF EXISTS route_plan_dependencies_dependency_kind_check;

ALTER TABLE network_optimizer.route_plan_dependencies
    ADD CONSTRAINT route_plan_dependencies_dependency_kind_check
    CHECK (dependency_kind IN (
        'SHIPMENT', 'CAPACITY', 'VEHICLE', 'LOAD_OPPORTUNITY', 'SHIPMENT_CARGO',
        'CURRENT_TRIP_CONTEXT', 'ROUTING_POLICY', 'CATALOG', 'RULE_SET', 'ALGORITHM_POLICY'
    ));
