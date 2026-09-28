-- Fail closed. Do not delete activation history to make rollback succeed.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM network_optimizer.route_plan_activations) THEN
        RAISE EXCEPTION 'nlo-0.4c down migration refused: route plan activation rows exist';
    END IF;
END $$;

DROP TABLE IF EXISTS network_optimizer.route_plan_activations;
