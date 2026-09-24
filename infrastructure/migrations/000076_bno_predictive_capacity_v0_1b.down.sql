DROP TABLE IF EXISTS network_optimizer.predicted_capacities;

DELETE FROM network_optimizer.capacities
WHERE source = 'CURRENT_SHIPMENT_PREDICTION' OR status = 'PREDICTED';

ALTER TABLE network_optimizer.capacities DROP CONSTRAINT capacities_source_chk;
ALTER TABLE network_optimizer.capacities
    ADD CONSTRAINT capacities_source_chk CHECK (source = 'MANUAL');

ALTER TABLE network_optimizer.capacities DROP CONSTRAINT capacities_status_chk;
ALTER TABLE network_optimizer.capacities
    ADD CONSTRAINT capacities_status_chk CHECK (status IN ('AVAILABLE', 'WITHDRAWN'));

ALTER TABLE transport.vehicles
    DROP CONSTRAINT IF EXISTS vehicles_combination_type_chk,
    DROP CONSTRAINT IF EXISTS vehicles_body_type_chk,
    DROP CONSTRAINT IF EXISTS vehicles_loading_access_chk,
    DROP CONSTRAINT IF EXISTS vehicles_unloading_access_chk,
    DROP CONSTRAINT IF EXISTS vehicles_temperature_mode_chk,
    DROP CONSTRAINT IF EXISTS vehicles_temperature_range_chk,
    DROP CONSTRAINT IF EXISTS vehicles_temperature_zone_count_chk,
    DROP CONSTRAINT IF EXISTS vehicles_container_size_chk;

ALTER TABLE transport.vehicles
    DROP COLUMN IF EXISTS combination_type,
    DROP COLUMN IF EXISTS body_type,
    DROP COLUMN IF EXISTS loading_access,
    DROP COLUMN IF EXISTS unloading_access,
    DROP COLUMN IF EXISTS temperature_control_mode,
    DROP COLUMN IF EXISTS temperature_capability_min_c,
    DROP COLUMN IF EXISTS temperature_capability_max_c,
    DROP COLUMN IF EXISTS temperature_zone_count,
    DROP COLUMN IF EXISTS independent_temperature_control,
    DROP COLUMN IF EXISTS container_size;
