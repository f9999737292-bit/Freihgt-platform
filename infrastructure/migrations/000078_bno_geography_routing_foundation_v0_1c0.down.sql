DROP TABLE IF EXISTS network_optimizer.capacity_search_policies;
DROP TABLE IF EXISTS network_optimizer.carrier_search_policies;

ALTER TABLE network_optimizer.load_opportunities
    DROP CONSTRAINT IF EXISTS load_opportunities_delivery_country_chk,
    DROP CONSTRAINT IF EXISTS load_opportunities_pickup_country_chk,
    DROP COLUMN IF EXISTS delivery_city,
    DROP COLUMN IF EXISTS delivery_region,
    DROP COLUMN IF EXISTS delivery_country_code,
    DROP COLUMN IF EXISTS pickup_city,
    DROP COLUMN IF EXISTS pickup_region,
    DROP COLUMN IF EXISTS pickup_country_code;

DROP INDEX IF EXISTS network_optimizer.capacities_location_id_idx;

ALTER TABLE network_optimizer.capacities
    DROP CONSTRAINT IF EXISTS capacities_country_code_chk,
    DROP COLUMN IF EXISTS city,
    DROP COLUMN IF EXISTS region,
    DROP COLUMN IF EXISTS country_code,
    DROP COLUMN IF EXISTS location_id;
