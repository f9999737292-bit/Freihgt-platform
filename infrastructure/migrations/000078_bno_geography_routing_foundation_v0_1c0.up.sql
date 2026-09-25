-- BNO-0.1C0 geography and routing foundation.
-- Location master remains transport.locations. These columns are planning snapshots
-- and references. They do not create a second address master.

ALTER TABLE network_optimizer.capacities
    ADD COLUMN location_id uuid,
    ADD COLUMN country_code char(2),
    ADD COLUMN region text,
    ADD COLUMN city text,
    ADD CONSTRAINT capacities_country_code_chk CHECK (country_code IS NULL OR char_length(country_code) = 2);

CREATE INDEX capacities_location_id_idx
    ON network_optimizer.capacities (location_id)
    WHERE location_id IS NOT NULL;

ALTER TABLE network_optimizer.load_opportunities
    ADD COLUMN pickup_country_code char(2),
    ADD COLUMN pickup_region text,
    ADD COLUMN pickup_city text,
    ADD COLUMN delivery_country_code char(2),
    ADD COLUMN delivery_region text,
    ADD COLUMN delivery_city text,
    ADD CONSTRAINT load_opportunities_pickup_country_chk CHECK (
        pickup_country_code IS NULL OR char_length(pickup_country_code) = 2
    ),
    ADD CONSTRAINT load_opportunities_delivery_country_chk CHECK (
        delivery_country_code IS NULL OR char_length(delivery_country_code) = 2
    );

CREATE TABLE network_optimizer.carrier_search_policies (
    owner_tenant_id uuid PRIMARY KEY,
    search_mode text NOT NULL,
    target_location_id uuid,
    forward_search_km double precision,
    corridor_deviation_km double precision,
    max_deadhead_km double precision,
    preferred_deadhead_km double precision,
    max_deadhead_minutes double precision,
    min_loaded_distance_km double precision,
    max_route_increase_km double precision,
    objective_profile text NOT NULL,
    allow_unknown_road_distance boolean NOT NULL DEFAULT false,
    version integer NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT carrier_search_mode_chk CHECK (search_mode IN ('RADIUS', 'DIRECTIONAL_CORRIDOR', 'ROUTE_ELLIPSE')),
    CONSTRAINT carrier_search_limits_chk CHECK (
        (forward_search_km IS NULL OR forward_search_km >= 0)
        AND (corridor_deviation_km IS NULL OR corridor_deviation_km >= 0)
        AND (max_deadhead_km IS NULL OR max_deadhead_km >= 0)
        AND (preferred_deadhead_km IS NULL OR preferred_deadhead_km >= 0)
        AND (max_deadhead_minutes IS NULL OR max_deadhead_minutes >= 0)
        AND (min_loaded_distance_km IS NULL OR min_loaded_distance_km >= 0)
        AND (max_route_increase_km IS NULL OR max_route_increase_km >= 0)
    )
);

CREATE TABLE network_optimizer.capacity_search_policies (
    capacity_id uuid PRIMARY KEY,
    owner_tenant_id uuid NOT NULL,
    search_mode text,
    target_location_id uuid,
    forward_search_km double precision,
    corridor_deviation_km double precision,
    max_deadhead_km double precision,
    preferred_deadhead_km double precision,
    max_deadhead_minutes double precision,
    min_loaded_distance_km double precision,
    max_route_increase_km double precision,
    objective_profile text,
    allow_unknown_road_distance boolean NOT NULL DEFAULT false,
    version integer NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT capacity_search_mode_chk CHECK (
        search_mode IS NULL OR search_mode IN ('RADIUS', 'DIRECTIONAL_CORRIDOR', 'ROUTE_ELLIPSE')
    ),
    CONSTRAINT capacity_search_limits_chk CHECK (
        (forward_search_km IS NULL OR forward_search_km >= 0)
        AND (corridor_deviation_km IS NULL OR corridor_deviation_km >= 0)
        AND (max_deadhead_km IS NULL OR max_deadhead_km >= 0)
        AND (preferred_deadhead_km IS NULL OR preferred_deadhead_km >= 0)
        AND (max_deadhead_minutes IS NULL OR max_deadhead_minutes >= 0)
        AND (min_loaded_distance_km IS NULL OR min_loaded_distance_km >= 0)
        AND (max_route_increase_km IS NULL OR max_route_increase_km >= 0)
    )
);

CREATE UNIQUE INDEX capacities_id_owner_uidx
    ON network_optimizer.capacities (id, owner_tenant_id);

ALTER TABLE network_optimizer.capacity_search_policies
    ADD CONSTRAINT capacity_search_policies_capacity_owner_fk
    FOREIGN KEY (capacity_id, owner_tenant_id)
    REFERENCES network_optimizer.capacities (id, owner_tenant_id);
