DROP TABLE IF EXISTS network_optimizer.pallet_equivalences;
DROP TABLE IF EXISTS network_optimizer.compatibility_rules;
DROP TABLE IF EXISTS network_optimizer.compatibility_rule_sets;
DROP TABLE IF EXISTS network_optimizer.catalog_aliases;
DROP TABLE IF EXISTS network_optimizer.packaging_type_catalog;
DROP TABLE IF EXISTS network_optimizer.pallet_type_catalog;
DROP TABLE IF EXISTS network_optimizer.equipment_type_catalog;
DROP TABLE IF EXISTS network_optimizer.cargo_type_catalog;
DROP TABLE IF EXISTS network_optimizer.reference_catalog_versions;

ALTER TABLE transport.vehicles
    DROP CONSTRAINT IF EXISTS vehicles_internal_height_chk,
    DROP CONSTRAINT IF EXISTS vehicles_internal_width_chk,
    DROP CONSTRAINT IF EXISTS vehicles_internal_length_chk,
    DROP CONSTRAINT IF EXISTS vehicles_usable_linear_meters_chk,
    DROP CONSTRAINT IF EXISTS vehicles_pallet_positions_chk,
    DROP CONSTRAINT IF EXISTS vehicles_equipment_unit_kind_chk,
    DROP COLUMN IF EXISTS adr_capability,
    DROP COLUMN IF EXISTS food_grade_capability,
    DROP COLUMN IF EXISTS internal_height_mm,
    DROP COLUMN IF EXISTS internal_width_mm,
    DROP COLUMN IF EXISTS internal_length_mm,
    DROP COLUMN IF EXISTS usable_linear_meters,
    DROP COLUMN IF EXISTS pallet_positions,
    DROP COLUMN IF EXISTS equipment_unit_kind;

ALTER TABLE transport.cargoes
    DROP CONSTRAINT IF EXISTS cargoes_planning_temperature_chk,
    DROP CONSTRAINT IF EXISTS cargoes_type_code_chk,
    DROP CONSTRAINT IF EXISTS cargoes_height_chk,
    DROP CONSTRAINT IF EXISTS cargoes_linear_meters_chk,
    DROP CONSTRAINT IF EXISTS cargoes_pallet_count_chk,
    DROP COLUMN IF EXISTS contamination_class,
    DROP COLUMN IF EXISTS odor_sensitive,
    DROP COLUMN IF EXISTS odor_emission_class,
    DROP COLUMN IF EXISTS preferred_temperature_setpoint_c,
    DROP COLUMN IF EXISTS temperature_required,
    DROP COLUMN IF EXISTS food_grade_required,
    DROP COLUMN IF EXISTS packaging_type_code,
    DROP COLUMN IF EXISTS fragile,
    DROP COLUMN IF EXISTS stackable,
    DROP COLUMN IF EXISTS max_loaded_height_mm,
    DROP COLUMN IF EXISTS linear_meters,
    DROP COLUMN IF EXISTS pallet_type_code,
    DROP COLUMN IF EXISTS pallet_count,
    DROP COLUMN IF EXISTS cargo_type_code;
