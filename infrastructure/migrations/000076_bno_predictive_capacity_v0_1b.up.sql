-- BNO-0.1B predictive capacity and normalized vehicle capability.
-- Vehicle master remains transport.vehicles. NULL means unknown.
-- An empty access array is a known empty set, not unknown.
-- This migration does not invent trailer rows or temperature ranges.

ALTER TABLE transport.vehicles
    ADD COLUMN combination_type text,
    ADD COLUMN body_type text,
    ADD COLUMN loading_access text[],
    ADD COLUMN unloading_access text[],
    ADD COLUMN temperature_control_mode text,
    ADD COLUMN temperature_capability_min_c numeric(6,2),
    ADD COLUMN temperature_capability_max_c numeric(6,2),
    ADD COLUMN temperature_zone_count integer,
    ADD COLUMN independent_temperature_control boolean,
    ADD COLUMN container_size text,
    ADD CONSTRAINT vehicles_combination_type_chk CHECK (
        combination_type IS NULL OR combination_type IN ('TRUCK', 'TRACTOR_SEMITRAILER', 'TRUCK_TRAILER', 'ROAD_TRAIN', 'OTHER')
    ),
    ADD CONSTRAINT vehicles_body_type_chk CHECK (
        body_type IS NULL OR body_type IN (
            'TENT', 'CONTAINER', 'ISOTHERMAL', 'REFRIGERATOR', 'BOX', 'PLATFORM',
            'LOWBED', 'TANK', 'TIPPER', 'CAR_CARRIER', 'TIMBER', 'OTHER'
        )
    ),
    ADD CONSTRAINT vehicles_loading_access_chk CHECK (
        loading_access IS NULL OR loading_access <@ ARRAY['REAR', 'SIDE', 'TOP']::text[]
    ),
    ADD CONSTRAINT vehicles_unloading_access_chk CHECK (
        unloading_access IS NULL OR unloading_access <@ ARRAY['REAR', 'SIDE', 'TOP']::text[]
    ),
    ADD CONSTRAINT vehicles_temperature_mode_chk CHECK (
        temperature_control_mode IS NULL OR temperature_control_mode IN ('NONE', 'PASSIVE', 'ACTIVE')
    ),
    ADD CONSTRAINT vehicles_temperature_range_chk CHECK (
        temperature_capability_min_c IS NULL
        OR temperature_capability_max_c IS NULL
        OR temperature_capability_min_c <= temperature_capability_max_c
    ),
    ADD CONSTRAINT vehicles_temperature_zone_count_chk CHECK (
        temperature_zone_count IS NULL OR temperature_zone_count >= 1
    ),
    ADD CONSTRAINT vehicles_container_size_chk CHECK (
        container_size IS NULL OR container_size IN ('20FT', '40FT', '40HC', '45FT', 'REEFER_CONTAINER')
    );

ALTER TABLE network_optimizer.capacities DROP CONSTRAINT capacities_source_chk;
ALTER TABLE network_optimizer.capacities
    ADD CONSTRAINT capacities_source_chk CHECK (source IN ('MANUAL', 'CURRENT_SHIPMENT_PREDICTION'));

ALTER TABLE network_optimizer.capacities DROP CONSTRAINT capacities_status_chk;
ALTER TABLE network_optimizer.capacities
    ADD CONSTRAINT capacities_status_chk CHECK (status IN ('AVAILABLE', 'WITHDRAWN', 'PREDICTED'));

CREATE TABLE network_optimizer.predicted_capacities (
    id uuid PRIMARY KEY,
    capacity_id uuid NOT NULL REFERENCES network_optimizer.capacities (id),
    owner_tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    shipment_version integer NOT NULL,
    shipment_status text NOT NULL,
    vehicle_id uuid NOT NULL,
    vehicle_version integer,
    destination_location_id uuid NOT NULL,
    destination_latitude double precision,
    destination_longitude double precision,
    eta timestamptz NOT NULL,
    eta_observed_at timestamptz NOT NULL,
    delivery_window_start timestamptz,
    delivery_window_end timestamptz,
    predicted_available_at timestamptz NOT NULL,
    availability_window_start timestamptz NOT NULL,
    availability_window_end timestamptz NOT NULL,
    unload_duration_seconds integer NOT NULL,
    unload_policy_source text NOT NULL,
    uncertainty_seconds integer NOT NULL,
    confidence double precision NOT NULL,
    prediction_method text NOT NULL,
    input_fingerprint text NOT NULL,
    rule_version text NOT NULL,
    generated_at timestamptz NOT NULL,
    supersedes_prediction_id uuid,
    is_current boolean NOT NULL,
    combination_type text,
    body_type text,
    loading_access text[],
    unloading_access text[],
    capacity_weight_kg double precision,
    capacity_volume_m3 double precision,
    temperature_control_mode text,
    temperature_capability_min_c double precision,
    temperature_capability_max_c double precision,
    temperature_zone_count integer,
    independent_temperature_control boolean,
    current_temperature_setpoint_c double precision,
    legacy_equipment_type text,
    container_size text,
    version integer NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT predicted_capacities_version_chk CHECK (version >= 1),
    CONSTRAINT predicted_capacities_shipment_version_chk CHECK (shipment_version >= 1),
    CONSTRAINT predicted_capacities_method_chk CHECK (prediction_method = 'RULE_BASED'),
    CONSTRAINT predicted_capacities_confidence_chk CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT predicted_capacities_unload_chk CHECK (unload_duration_seconds > 0),
    CONSTRAINT predicted_capacities_uncertainty_chk CHECK (uncertainty_seconds > 0),
    CONSTRAINT predicted_capacities_window_chk CHECK (availability_window_end >= availability_window_start),
    CONSTRAINT predicted_capacities_point_chk CHECK (
        predicted_available_at >= availability_window_start
        AND predicted_available_at <= availability_window_end
    ),
    CONSTRAINT predicted_capacities_delivery_window_chk CHECK (
        delivery_window_end IS NULL
        OR (delivery_window_start IS NOT NULL AND delivery_window_end > delivery_window_start)
    ),
    CONSTRAINT predicted_capacities_weight_chk CHECK (capacity_weight_kg IS NULL OR capacity_weight_kg > 0),
    CONSTRAINT predicted_capacities_volume_chk CHECK (capacity_volume_m3 IS NULL OR capacity_volume_m3 > 0),
    CONSTRAINT predicted_capacities_geo_chk CHECK (
        (destination_latitude IS NULL AND destination_longitude IS NULL)
        OR (destination_latitude BETWEEN -90 AND 90 AND destination_longitude BETWEEN -180 AND 180)
    ),
    CONSTRAINT predicted_capacities_temp_chk CHECK (
        temperature_capability_min_c IS NULL
        OR temperature_capability_max_c IS NULL
        OR temperature_capability_min_c <= temperature_capability_max_c
    ),
    CONSTRAINT predicted_capacities_body_chk CHECK (
        body_type IS NULL OR body_type IN (
            'TENT', 'CONTAINER', 'ISOTHERMAL', 'REFRIGERATOR', 'BOX', 'PLATFORM',
            'LOWBED', 'TANK', 'TIPPER', 'CAR_CARRIER', 'TIMBER', 'OTHER'
        )
    ),
    CONSTRAINT predicted_capacities_loading_chk CHECK (
        loading_access IS NULL OR loading_access <@ ARRAY['REAR', 'SIDE', 'TOP']::text[]
    ),
    CONSTRAINT predicted_capacities_unloading_chk CHECK (
        unloading_access IS NULL OR unloading_access <@ ARRAY['REAR', 'SIDE', 'TOP']::text[]
    )
);

CREATE UNIQUE INDEX predicted_capacities_one_current_uidx
    ON network_optimizer.predicted_capacities (owner_tenant_id, shipment_id)
    WHERE is_current;

CREATE INDEX predicted_capacities_owner_idx
    ON network_optimizer.predicted_capacities (owner_tenant_id, generated_at DESC);

CREATE INDEX predicted_capacities_capacity_idx
    ON network_optimizer.predicted_capacities (capacity_id);

CREATE INDEX predicted_capacities_shipment_idx
    ON network_optimizer.predicted_capacities (owner_tenant_id, shipment_id, generated_at DESC);
