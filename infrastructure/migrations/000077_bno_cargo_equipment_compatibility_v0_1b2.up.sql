-- BNO-0.1B2 cargo and equipment reference data plus compatibility rules.
-- Cargo facts remain on transport.cargoes. Vehicle facts remain on transport.vehicles.
-- New vehicle columns are effective combination capabilities. They are not a physical trailer asset.
-- NULL remains unknown. This migration does not invent ADR segregation rules or pallet conversion factors.

ALTER TABLE transport.cargoes
    ADD COLUMN cargo_type_code text,
    ADD COLUMN pallet_count integer,
    ADD COLUMN pallet_type_code text,
    ADD COLUMN linear_meters numeric(10,3),
    ADD COLUMN max_loaded_height_mm integer,
    ADD COLUMN stackable boolean,
    ADD COLUMN fragile boolean,
    ADD COLUMN packaging_type_code text,
    ADD COLUMN food_grade_required boolean,
    ADD COLUMN temperature_required boolean,
    ADD COLUMN preferred_temperature_setpoint_c numeric(6,2),
    ADD COLUMN odor_emission_class text,
    ADD COLUMN odor_sensitive boolean,
    ADD COLUMN contamination_class text,
    ADD CONSTRAINT cargoes_pallet_count_chk CHECK (pallet_count IS NULL OR pallet_count > 0),
    ADD CONSTRAINT cargoes_linear_meters_chk CHECK (linear_meters IS NULL OR linear_meters > 0),
    ADD CONSTRAINT cargoes_height_chk CHECK (max_loaded_height_mm IS NULL OR max_loaded_height_mm > 0),
    ADD CONSTRAINT cargoes_type_code_chk CHECK (cargo_type_code IS NULL OR btrim(cargo_type_code) <> ''),
    ADD CONSTRAINT cargoes_planning_temperature_chk CHECK (
        temperature_min IS NULL OR temperature_max IS NULL OR temperature_min <= temperature_max
    );

ALTER TABLE transport.vehicles
    ADD COLUMN equipment_unit_kind text,
    ADD COLUMN pallet_positions integer,
    ADD COLUMN usable_linear_meters numeric(10,3),
    ADD COLUMN internal_length_mm integer,
    ADD COLUMN internal_width_mm integer,
    ADD COLUMN internal_height_mm integer,
    ADD COLUMN food_grade_capability boolean,
    ADD COLUMN adr_capability boolean,
    ADD CONSTRAINT vehicles_equipment_unit_kind_chk CHECK (
        equipment_unit_kind IS NULL OR equipment_unit_kind IN (
            'TRUCK_BODY', 'TRAILER', 'SEMITRAILER', 'CONTAINER_CHASSIS', 'SWAP_BODY', 'OTHER'
        )
    ),
    ADD CONSTRAINT vehicles_pallet_positions_chk CHECK (pallet_positions IS NULL OR pallet_positions > 0),
    ADD CONSTRAINT vehicles_usable_linear_meters_chk CHECK (usable_linear_meters IS NULL OR usable_linear_meters > 0),
    ADD CONSTRAINT vehicles_internal_length_chk CHECK (internal_length_mm IS NULL OR internal_length_mm > 0),
    ADD CONSTRAINT vehicles_internal_width_chk CHECK (internal_width_mm IS NULL OR internal_width_mm > 0),
    ADD CONSTRAINT vehicles_internal_height_chk CHECK (internal_height_mm IS NULL OR internal_height_mm > 0);

CREATE TABLE network_optimizer.reference_catalog_versions (
    id uuid PRIMARY KEY,
    catalog_kind text NOT NULL,
    scope text NOT NULL,
    tenant_id uuid,
    version integer NOT NULL,
    status text NOT NULL,
    effective_from timestamptz,
    effective_to timestamptz,
    source_reference text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by text NOT NULL,
    activated_at timestamptz,
    activated_by text,
    CONSTRAINT reference_catalog_version_chk CHECK (version >= 1),
    CONSTRAINT reference_catalog_kind_chk CHECK (btrim(catalog_kind) <> ''),
    CONSTRAINT reference_catalog_scope_chk CHECK (scope IN ('SYSTEM', 'TENANT')),
    CONSTRAINT reference_catalog_status_chk CHECK (status IN ('DRAFT', 'ACTIVE', 'RETIRED')),
    CONSTRAINT reference_catalog_tenant_chk CHECK (
        (scope = 'SYSTEM' AND tenant_id IS NULL) OR (scope = 'TENANT' AND tenant_id IS NOT NULL)
    ),
    CONSTRAINT reference_catalog_effective_chk CHECK (effective_to IS NULL OR effective_from IS NULL OR effective_to > effective_from),
    CONSTRAINT reference_catalog_source_chk CHECK (btrim(source_reference) <> '')
);

CREATE UNIQUE INDEX reference_catalog_one_active_system_uidx
    ON network_optimizer.reference_catalog_versions (catalog_kind)
    WHERE status = 'ACTIVE' AND scope = 'SYSTEM';

CREATE UNIQUE INDEX reference_catalog_one_active_tenant_uidx
    ON network_optimizer.reference_catalog_versions (catalog_kind, tenant_id)
    WHERE status = 'ACTIVE' AND scope = 'TENANT';

CREATE TABLE network_optimizer.cargo_type_catalog (
    id uuid PRIMARY KEY,
    version_id uuid NOT NULL REFERENCES network_optimizer.reference_catalog_versions (id),
    code text NOT NULL,
    parent_code text,
    display_name text NOT NULL,
    description text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    tags text[] NOT NULL DEFAULT '{}',
    CONSTRAINT cargo_type_code_chk CHECK (btrim(code) <> ''),
    UNIQUE (version_id, code)
);

CREATE TABLE network_optimizer.equipment_type_catalog (
    id uuid PRIMARY KEY,
    version_id uuid NOT NULL REFERENCES network_optimizer.reference_catalog_versions (id),
    code text NOT NULL,
    unit_kind text NOT NULL,
    combination_type text,
    body_type text NOT NULL,
    description text NOT NULL DEFAULT '',
    nominal_payload_kg numeric(12,3),
    nominal_volume_m3 numeric(10,3),
    pallet_positions integer,
    usable_linear_meters numeric(10,3),
    internal_length_mm integer,
    internal_width_mm integer,
    internal_height_mm integer,
    loading_access text[],
    unloading_access text[],
    temperature_control_mode text,
    temperature_min_c numeric(6,2),
    temperature_max_c numeric(6,2),
    temperature_zone_count integer,
    independent_temperature_control boolean,
    food_grade_capability boolean,
    adr_capability boolean,
    container_size text,
    active boolean NOT NULL DEFAULT true,
    CONSTRAINT equipment_type_code_chk CHECK (btrim(code) <> ''),
    CONSTRAINT equipment_type_unit_kind_chk CHECK (unit_kind IN ('TRUCK_BODY', 'TRAILER', 'SEMITRAILER', 'CONTAINER_CHASSIS', 'SWAP_BODY', 'OTHER')),
    CONSTRAINT equipment_type_positive_chk CHECK (
        (nominal_payload_kg IS NULL OR nominal_payload_kg > 0)
        AND (nominal_volume_m3 IS NULL OR nominal_volume_m3 > 0)
        AND (pallet_positions IS NULL OR pallet_positions > 0)
        AND (usable_linear_meters IS NULL OR usable_linear_meters > 0)
        AND (internal_length_mm IS NULL OR internal_length_mm > 0)
        AND (internal_width_mm IS NULL OR internal_width_mm > 0)
        AND (internal_height_mm IS NULL OR internal_height_mm > 0)
    ),
    CONSTRAINT equipment_type_body_chk CHECK (body_type IN (
        'TENT', 'CONTAINER', 'ISOTHERMAL', 'REFRIGERATOR', 'BOX', 'PLATFORM', 'LOWBED', 'TANK', 'TIPPER', 'CAR_CARRIER', 'TIMBER', 'OTHER'
    )),
    CONSTRAINT equipment_type_combination_chk CHECK (
        combination_type IS NULL OR combination_type IN ('TRUCK', 'TRACTOR_SEMITRAILER', 'TRUCK_TRAILER', 'ROAD_TRAIN', 'OTHER')
    ),
    CONSTRAINT equipment_type_loading_access_chk CHECK (
        loading_access IS NULL OR loading_access <@ ARRAY['REAR', 'SIDE', 'TOP']::text[]
    ),
    CONSTRAINT equipment_type_unloading_access_chk CHECK (
        unloading_access IS NULL OR unloading_access <@ ARRAY['REAR', 'SIDE', 'TOP']::text[]
    ),
    CONSTRAINT equipment_type_temperature_mode_chk CHECK (
        temperature_control_mode IS NULL OR temperature_control_mode IN ('NONE', 'PASSIVE', 'ACTIVE')
    ),
    CONSTRAINT equipment_type_temperature_range_chk CHECK (
        temperature_min_c IS NULL OR temperature_max_c IS NULL OR temperature_min_c <= temperature_max_c
    ),
    CONSTRAINT equipment_type_temperature_zone_chk CHECK (
        temperature_zone_count IS NULL OR temperature_zone_count >= 1
    ),
    CONSTRAINT equipment_type_container_size_chk CHECK (
        container_size IS NULL OR container_size IN ('20FT', '40FT', '40HC', '45FT', 'REEFER_CONTAINER')
    ),
    UNIQUE (version_id, code)
);

CREATE TABLE network_optimizer.pallet_type_catalog (
    id uuid PRIMARY KEY,
    version_id uuid NOT NULL REFERENCES network_optimizer.reference_catalog_versions (id),
    code text NOT NULL,
    display_name text NOT NULL,
    length_mm integer NOT NULL,
    width_mm integer NOT NULL,
    standard_reference text,
    active boolean NOT NULL DEFAULT true,
    CONSTRAINT pallet_type_code_chk CHECK (btrim(code) <> ''),
    CONSTRAINT pallet_type_dimensions_chk CHECK (length_mm > 0 AND width_mm > 0),
    UNIQUE (version_id, code)
);

CREATE TABLE network_optimizer.packaging_type_catalog (
    id uuid PRIMARY KEY,
    version_id uuid NOT NULL REFERENCES network_optimizer.reference_catalog_versions (id),
    code text NOT NULL,
    display_name text NOT NULL,
    description text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    CONSTRAINT packaging_type_code_chk CHECK (btrim(code) <> ''),
    UNIQUE (version_id, code)
);

CREATE TABLE network_optimizer.catalog_aliases (
    id uuid PRIMARY KEY,
    version_id uuid NOT NULL REFERENCES network_optimizer.reference_catalog_versions (id),
    alias_code text NOT NULL,
    canonical_code text NOT NULL,
    CONSTRAINT catalog_alias_code_chk CHECK (btrim(alias_code) <> '' AND btrim(canonical_code) <> ''),
    UNIQUE (version_id, alias_code)
);

CREATE TABLE network_optimizer.compatibility_rule_sets (
    id uuid PRIMARY KEY,
    scope text NOT NULL,
    tenant_id uuid,
    version integer NOT NULL,
    status text NOT NULL,
    effective_from timestamptz,
    effective_to timestamptz,
    source_reference text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid,
    activated_at timestamptz,
    activated_by uuid,
    CONSTRAINT compatibility_rule_set_version_chk CHECK (version >= 1),
    CONSTRAINT compatibility_rule_set_scope_chk CHECK (scope IN ('SYSTEM', 'TENANT')),
    CONSTRAINT compatibility_rule_set_status_chk CHECK (status IN ('DRAFT', 'ACTIVE', 'RETIRED')),
    CONSTRAINT compatibility_rule_set_tenant_chk CHECK (
        (scope = 'SYSTEM' AND tenant_id IS NULL) OR (scope = 'TENANT' AND tenant_id IS NOT NULL)
    ),
    CONSTRAINT compatibility_rule_set_effective_chk CHECK (effective_to IS NULL OR effective_from IS NULL OR effective_to > effective_from),
    CONSTRAINT compatibility_rule_set_source_chk CHECK (btrim(source_reference) <> '')
);

CREATE UNIQUE INDEX compatibility_rule_sets_one_active_system_uidx
    ON network_optimizer.compatibility_rule_sets (scope)
    WHERE status = 'ACTIVE' AND scope = 'SYSTEM';

CREATE UNIQUE INDEX compatibility_rule_sets_one_active_tenant_uidx
    ON network_optimizer.compatibility_rule_sets (tenant_id)
    WHERE status = 'ACTIVE' AND scope = 'TENANT';

CREATE TABLE network_optimizer.compatibility_rules (
    id uuid PRIMARY KEY,
    rule_set_id uuid NOT NULL REFERENCES network_optimizer.compatibility_rule_sets (id),
    rule_code text NOT NULL,
    rule_kind text NOT NULL,
    layer text NOT NULL,
    left_selector_type text NOT NULL,
    left_selector_value text NOT NULL,
    right_selector_type text NOT NULL,
    right_selector_value text NOT NULL,
    decision text NOT NULL,
    severity text NOT NULL DEFAULT 'HARD',
    reason_code text NOT NULL,
    required_separation text,
    source_reference text,
    priority integer NOT NULL DEFAULT 0,
    CONSTRAINT compatibility_rule_code_chk CHECK (btrim(rule_code) <> ''),
    CONSTRAINT compatibility_rule_kind_chk CHECK (rule_kind IN ('CARGO_CARGO', 'CARGO_EQUIPMENT')),
    CONSTRAINT compatibility_rule_layer_chk CHECK (layer IN ('REGULATORY', 'PLATFORM', 'TENANT')),
    CONSTRAINT compatibility_rule_decision_chk CHECK (decision IN ('ALLOW', 'DENY', 'REQUIRE_SEPARATION', 'REQUIRE_CONDITION')),
    CONSTRAINT compatibility_rule_regulatory_source_chk CHECK (
        layer <> 'REGULATORY' OR (source_reference IS NOT NULL AND btrim(source_reference) <> '')
    ),
    CONSTRAINT compatibility_rule_severity_chk CHECK (severity IN ('HARD', 'SOFT')),
    CONSTRAINT compatibility_rule_separation_chk CHECK (
        (decision = 'REQUIRE_SEPARATION' AND required_separation IS NOT NULL AND btrim(required_separation) <> '')
        OR (decision <> 'REQUIRE_SEPARATION' AND required_separation IS NULL)
    ),
    UNIQUE (rule_set_id, rule_code)
);

CREATE TABLE network_optimizer.pallet_equivalences (
    id uuid PRIMARY KEY,
    version_id uuid NOT NULL REFERENCES network_optimizer.reference_catalog_versions (id),
    from_code text NOT NULL,
    basis_code text NOT NULL,
    positions_each numeric(8,3) NOT NULL,
    source_reference text NOT NULL,
    CONSTRAINT pallet_equivalence_positive_chk CHECK (positions_each > 0),
    CONSTRAINT pallet_equivalence_source_chk CHECK (btrim(source_reference) <> ''),
    UNIQUE (version_id, from_code, basis_code)
);

CREATE INDEX cargo_type_catalog_version_idx ON network_optimizer.cargo_type_catalog (version_id, code);
CREATE INDEX equipment_type_catalog_version_idx ON network_optimizer.equipment_type_catalog (version_id, code);
CREATE INDEX pallet_type_catalog_version_idx ON network_optimizer.pallet_type_catalog (version_id, code);
CREATE INDEX packaging_type_catalog_version_idx ON network_optimizer.packaging_type_catalog (version_id, code);
CREATE INDEX compatibility_rules_set_idx ON network_optimizer.compatibility_rules (rule_set_id, rule_code);

INSERT INTO network_optimizer.reference_catalog_versions (
    id, catalog_kind, scope, version, status, source_reference, created_by, activated_at, activated_by
) VALUES
    ('00000000-0000-4000-8000-000000000083', 'CARGO_TYPE', 'SYSTEM', 1, 'ACTIVE', 'SYSTEM_SEED', 'SYSTEM_SEED', now(), 'SYSTEM_SEED'),
    ('00000000-0000-4000-8000-000000000086', 'EQUIPMENT_TYPE', 'SYSTEM', 1, 'ACTIVE', 'SYSTEM_SEED', 'SYSTEM_SEED', now(), 'SYSTEM_SEED'),
    ('00000000-0000-4000-8000-000000000087', 'PALLET_TYPE', 'SYSTEM', 1, 'ACTIVE', 'SYSTEM_SEED', 'SYSTEM_SEED', now(), 'SYSTEM_SEED'),
    ('00000000-0000-4000-8000-000000000088', 'PACKAGING_TYPE', 'SYSTEM', 1, 'ACTIVE', 'SYSTEM_SEED', 'SYSTEM_SEED', now(), 'SYSTEM_SEED');

INSERT INTO network_optimizer.cargo_type_catalog (id, version_id, code, parent_code, display_name) VALUES
    ('00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000083', 'GENERAL_CARGO', NULL, 'General cargo'),
    ('00000000-0000-4000-8000-000000000102', '00000000-0000-4000-8000-000000000083', 'FOOD', NULL, 'Food'),
    ('00000000-0000-4000-8000-000000000103', '00000000-0000-4000-8000-000000000083', 'BEVERAGES', 'FOOD', 'Beverages'),
    ('00000000-0000-4000-8000-000000000104', '00000000-0000-4000-8000-000000000083', 'DAIRY', 'FOOD', 'Dairy'),
    ('00000000-0000-4000-8000-000000000105', '00000000-0000-4000-8000-000000000083', 'MEAT', 'FOOD', 'Meat'),
    ('00000000-0000-4000-8000-000000000106', '00000000-0000-4000-8000-000000000083', 'FISH', 'FOOD', 'Fish'),
    ('00000000-0000-4000-8000-000000000107', '00000000-0000-4000-8000-000000000083', 'FROZEN_FOOD', 'FOOD', 'Frozen food'),
    ('00000000-0000-4000-8000-000000000108', '00000000-0000-4000-8000-000000000083', 'FRESH_PRODUCE', 'FOOD', 'Fresh produce'),
    ('00000000-0000-4000-8000-000000000109', '00000000-0000-4000-8000-000000000083', 'PHARMA', NULL, 'Pharma'),
    ('00000000-0000-4000-8000-000000000110', '00000000-0000-4000-8000-000000000083', 'CHEMICAL', NULL, 'Chemical'),
    ('00000000-0000-4000-8000-000000000111', '00000000-0000-4000-8000-000000000083', 'HOUSEHOLD_CHEMICAL', 'CHEMICAL', 'Household chemical'),
    ('00000000-0000-4000-8000-000000000112', '00000000-0000-4000-8000-000000000083', 'PAINT_COATINGS', 'CHEMICAL', 'Paint and coatings'),
    ('00000000-0000-4000-8000-000000000113', '00000000-0000-4000-8000-000000000083', 'ELECTRONICS', NULL, 'Electronics'),
    ('00000000-0000-4000-8000-000000000114', '00000000-0000-4000-8000-000000000083', 'TEXTILE', NULL, 'Textile'),
    ('00000000-0000-4000-8000-000000000115', '00000000-0000-4000-8000-000000000083', 'PAPER', NULL, 'Paper'),
    ('00000000-0000-4000-8000-000000000116', '00000000-0000-4000-8000-000000000083', 'FURNITURE', NULL, 'Furniture'),
    ('00000000-0000-4000-8000-000000000117', '00000000-0000-4000-8000-000000000083', 'CONSTRUCTION_MATERIALS', NULL, 'Construction materials'),
    ('00000000-0000-4000-8000-000000000118', '00000000-0000-4000-8000-000000000083', 'METAL_PRODUCTS', NULL, 'Metal products'),
    ('00000000-0000-4000-8000-000000000119', '00000000-0000-4000-8000-000000000083', 'AUTO_PARTS', NULL, 'Auto parts'),
    ('00000000-0000-4000-8000-000000000120', '00000000-0000-4000-8000-000000000083', 'FRAGILE_GOODS', NULL, 'Fragile goods'),
    ('00000000-0000-4000-8000-000000000121', '00000000-0000-4000-8000-000000000083', 'OTHER', NULL, 'Other');

INSERT INTO network_optimizer.packaging_type_catalog (id, version_id, code, display_name) VALUES
    ('00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000088', 'PALLETIZED', 'Palletized'),
    ('00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000000088', 'BOX', 'Box'),
    ('00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000000088', 'CRATE', 'Crate'),
    ('00000000-0000-4000-8000-000000000204', '00000000-0000-4000-8000-000000000088', 'BAG', 'Bag'),
    ('00000000-0000-4000-8000-000000000205', '00000000-0000-4000-8000-000000000088', 'DRUM', 'Drum'),
    ('00000000-0000-4000-8000-000000000206', '00000000-0000-4000-8000-000000000088', 'IBC', 'IBC'),
    ('00000000-0000-4000-8000-000000000207', '00000000-0000-4000-8000-000000000088', 'ROLL', 'Roll'),
    ('00000000-0000-4000-8000-000000000208', '00000000-0000-4000-8000-000000000088', 'BULK', 'Bulk'),
    ('00000000-0000-4000-8000-000000000209', '00000000-0000-4000-8000-000000000088', 'OTHER', 'Other');

INSERT INTO network_optimizer.pallet_type_catalog (id, version_id, code, display_name, length_mm, width_mm, standard_reference) VALUES
    ('00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000087', 'EUR', 'EUR pallet', 1200, 800, 'EN 13698-1'),
    ('00000000-0000-4000-8000-000000000302', '00000000-0000-4000-8000-000000000087', 'EUR2', 'EUR2 pallet', 1200, 1000, NULL);

INSERT INTO network_optimizer.equipment_type_catalog (id, version_id, code, unit_kind, body_type, description) VALUES
    ('00000000-0000-4000-8000-000000000401', '00000000-0000-4000-8000-000000000086', 'SEMITRAILER_REEFER', 'SEMITRAILER', 'REFRIGERATOR', 'Reference semitrailer refrigerator profile'),
    ('00000000-0000-4000-8000-000000000402', '00000000-0000-4000-8000-000000000086', 'SEMITRAILER_TENT', 'SEMITRAILER', 'TENT', 'Reference semitrailer tent profile'),
    ('00000000-0000-4000-8000-000000000403', '00000000-0000-4000-8000-000000000086', 'TRAILER_TENT', 'TRAILER', 'TENT', 'Reference trailer tent profile'),
    ('00000000-0000-4000-8000-000000000404', '00000000-0000-4000-8000-000000000086', 'TRUCK_BODY_BOX', 'TRUCK_BODY', 'BOX', 'Reference truck body profile'),
    ('00000000-0000-4000-8000-000000000405', '00000000-0000-4000-8000-000000000086', 'CONTAINER_CHASSIS', 'CONTAINER_CHASSIS', 'CONTAINER', 'Reference container chassis profile'),
    ('00000000-0000-4000-8000-000000000406', '00000000-0000-4000-8000-000000000086', 'SWAP_BODY', 'SWAP_BODY', 'BOX', 'Reference swap body profile'),
    ('00000000-0000-4000-8000-000000000407', '00000000-0000-4000-8000-000000000086', 'OTHER', 'OTHER', 'OTHER', 'Other reference equipment profile');

INSERT INTO network_optimizer.compatibility_rule_sets (
    id, scope, version, status, source_reference, activated_at
) VALUES (
    '00000000-0000-4000-8000-000000000091', 'SYSTEM', 1, 'ACTIVE', 'SYSTEM_SEED', now()
);
