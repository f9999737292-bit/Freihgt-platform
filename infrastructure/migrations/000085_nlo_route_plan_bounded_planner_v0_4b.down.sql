-- Fail closed. Do not delete evaluated route plans to make rollback succeed.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM network_optimizer.route_plans) THEN
        RAISE EXCEPTION 'nlo-0.4b down migration refused: route plan rows exist';
    END IF;
END $$;

DROP TABLE IF EXISTS network_optimizer.route_plan_dependencies;
DROP TABLE IF EXISTS network_optimizer.route_capacity_snapshots;
DROP TABLE IF EXISTS network_optimizer.route_plan_legs;
DROP TABLE IF EXISTS network_optimizer.route_stop_actions;
DROP TABLE IF EXISTS network_optimizer.route_plan_stops;
DROP TABLE IF EXISTS network_optimizer.route_plans;
