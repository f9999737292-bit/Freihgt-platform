-- BNO-0.1A foundation. Owned by network-optimizer.
-- Does not alter shipment or transport-order tables.
-- Pallet count, linear metres, and trailer capability are intentionally absent
-- so unknown physical attributes cannot be stored as zero.
--
-- Indexes:
--   tenant + status: owner lists and lifecycle scans
--   visibility + status: marketplace candidate reads
--   source reference: duplicate-publication guard
--   windows: availability and pickup/delivery filters
--   GIN on invited/audience arrays: explicit audience membership

CREATE SCHEMA IF NOT EXISTS network_optimizer;

CREATE TABLE network_optimizer.load_opportunities (
    id uuid PRIMARY KEY,
    owner_tenant_id uuid NOT NULL,
    source_type text NOT NULL,
    source_id uuid NOT NULL,
    pickup_location_id uuid,
    pickup_label text NOT NULL DEFAULT '',
    pickup_latitude double precision,
    pickup_longitude double precision,
    pickup_window_start timestamptz,
    pickup_window_end timestamptz,
    delivery_location_id uuid,
    delivery_label text NOT NULL DEFAULT '',
    delivery_latitude double precision,
    delivery_longitude double precision,
    delivery_window_start timestamptz,
    delivery_window_end timestamptz,
    weight_kg double precision,
    volume_m3 double precision,
    body_type text NOT NULL DEFAULT '',
    equipment text[] NOT NULL DEFAULT '{}',
    cargo jsonb NOT NULL DEFAULT '{}'::jsonb,
    commercial_mode text NOT NULL DEFAULT '',
    commercial_amount double precision,
    commercial_currency text NOT NULL DEFAULT '',
    visibility_scope text NOT NULL,
    invited_carrier_company_ids uuid[] NOT NULL DEFAULT '{}',
    status text NOT NULL,
    version integer NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT load_opportunities_source_type_chk CHECK (source_type IN ('TRANSPORT_ORDER', 'SHIPMENT')),
    CONSTRAINT load_opportunities_status_chk CHECK (status IN ('DRAFT', 'PUBLISHED', 'WITHDRAWN')),
    CONSTRAINT load_opportunities_visibility_chk CHECK (visibility_scope IN (
        'PRIVATE', 'INVITED_CARRIERS', 'MARKETPLACE', 'ANONYMIZED_MARKETPLACE', 'NETWORK_OPTIMIZATION_ONLY'
    )),
    CONSTRAINT load_opportunities_version_chk CHECK (version >= 1),
    CONSTRAINT load_opportunities_weight_chk CHECK (weight_kg IS NULL OR weight_kg > 0),
    CONSTRAINT load_opportunities_volume_chk CHECK (volume_m3 IS NULL OR volume_m3 > 0),
    CONSTRAINT load_opportunities_amount_chk CHECK (commercial_amount IS NULL OR commercial_amount > 0),
    CONSTRAINT load_opportunities_pickup_geo_chk CHECK (
        (pickup_latitude IS NULL AND pickup_longitude IS NULL)
        OR (pickup_latitude BETWEEN -90 AND 90 AND pickup_longitude BETWEEN -180 AND 180)
    ),
    CONSTRAINT load_opportunities_delivery_geo_chk CHECK (
        (delivery_latitude IS NULL AND delivery_longitude IS NULL)
        OR (delivery_latitude BETWEEN -90 AND 90 AND delivery_longitude BETWEEN -180 AND 180)
    ),
    CONSTRAINT load_opportunities_pickup_window_chk CHECK (
        (pickup_window_start IS NULL AND pickup_window_end IS NULL)
        OR (pickup_window_start IS NOT NULL AND pickup_window_end IS NOT NULL AND pickup_window_end > pickup_window_start)
    ),
    CONSTRAINT load_opportunities_delivery_window_chk CHECK (
        (delivery_window_start IS NULL AND delivery_window_end IS NULL)
        OR (delivery_window_start IS NOT NULL AND delivery_window_end IS NOT NULL AND delivery_window_end > delivery_window_start)
    )
);

CREATE UNIQUE INDEX load_opportunities_active_source_uidx
    ON network_optimizer.load_opportunities (owner_tenant_id, source_type, source_id)
    WHERE status IN ('DRAFT', 'PUBLISHED');

CREATE INDEX load_opportunities_owner_status_idx
    ON network_optimizer.load_opportunities (owner_tenant_id, status, created_at DESC);

CREATE INDEX load_opportunities_visibility_status_idx
    ON network_optimizer.load_opportunities (visibility_scope, status, created_at DESC);

CREATE INDEX load_opportunities_pickup_window_idx
    ON network_optimizer.load_opportunities (pickup_window_start, pickup_window_end);

CREATE INDEX load_opportunities_delivery_window_idx
    ON network_optimizer.load_opportunities (delivery_window_start, delivery_window_end);

CREATE INDEX load_opportunities_invited_gin
    ON network_optimizer.load_opportunities USING GIN (invited_carrier_company_ids);

CREATE TABLE network_optimizer.capacities (
    id uuid PRIMARY KEY,
    owner_tenant_id uuid NOT NULL,
    carrier_company_id uuid,
    vehicle_id uuid,
    location_label text NOT NULL DEFAULT '',
    latitude double precision,
    longitude double precision,
    available_from timestamptz NOT NULL,
    available_until timestamptz NOT NULL,
    source text NOT NULL,
    body_type text NOT NULL DEFAULT '',
    equipment text[] NOT NULL DEFAULT '{}',
    payload_remaining_kg double precision,
    volume_remaining_m3 double precision,
    visibility_scope text NOT NULL,
    audience_tenant_ids uuid[] NOT NULL DEFAULT '{}',
    status text NOT NULL,
    version integer NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT capacities_source_chk CHECK (source = 'MANUAL'),
    CONSTRAINT capacities_status_chk CHECK (status IN ('AVAILABLE', 'WITHDRAWN')),
    CONSTRAINT capacities_visibility_chk CHECK (visibility_scope IN (
        'PRIVATE', 'SHIPPER_NETWORK', 'MARKETPLACE', 'ANONYMIZED'
    )),
    CONSTRAINT capacities_version_chk CHECK (version >= 1),
    CONSTRAINT capacities_payload_chk CHECK (payload_remaining_kg IS NULL OR payload_remaining_kg > 0),
    CONSTRAINT capacities_volume_chk CHECK (volume_remaining_m3 IS NULL OR volume_remaining_m3 > 0),
    CONSTRAINT capacities_window_chk CHECK (available_until > available_from),
    CONSTRAINT capacities_geo_chk CHECK (
        (latitude IS NULL AND longitude IS NULL)
        OR (latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180)
    )
);

CREATE INDEX capacities_owner_status_idx
    ON network_optimizer.capacities (owner_tenant_id, status, created_at DESC);

CREATE INDEX capacities_visibility_status_idx
    ON network_optimizer.capacities (visibility_scope, status, available_from, available_until);

CREATE INDEX capacities_audience_gin
    ON network_optimizer.capacities USING GIN (audience_tenant_ids);

CREATE TABLE network_optimizer.audit_events (
    id uuid PRIMARY KEY,
    actor_user_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    action text NOT NULL,
    from_status text NOT NULL DEFAULT '',
    to_status text NOT NULL,
    visibility_scope text NOT NULL DEFAULT '',
    source_type text NOT NULL DEFAULT '',
    source_id uuid,
    occurred_at timestamptz NOT NULL
);

CREATE INDEX audit_events_aggregate_idx
    ON network_optimizer.audit_events (aggregate_type, aggregate_id, occurred_at);

CREATE TABLE network_optimizer.outbox (
    id uuid PRIMARY KEY,
    event_name text NOT NULL,
    schema_version integer NOT NULL,
    tenant_id uuid NOT NULL,
    aggregate_id uuid NOT NULL,
    aggregate_version integer NOT NULL,
    occurred_at timestamptz NOT NULL,
    payload jsonb NOT NULL,
    published_at timestamptz
);

CREATE INDEX outbox_unpublished_idx
    ON network_optimizer.outbox (occurred_at)
    WHERE published_at IS NULL;

CREATE TABLE network_optimizer.idempotency_keys (
    tenant_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    request_hash text NOT NULL,
    response_status integer NOT NULL,
    response_body jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, idempotency_key)
);
