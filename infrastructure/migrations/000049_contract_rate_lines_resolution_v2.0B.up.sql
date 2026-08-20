CREATE TABLE contract_rate.rate_line (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rate_card_version_id UUID NOT NULL,
    origin_location_id UUID NOT NULL,
    destination_location_id UUID NOT NULL,
    equipment_type VARCHAR(64) NOT NULL,
    transport_mode VARCHAR(32) NOT NULL DEFAULT 'ROAD',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    CONSTRAINT uq_rate_line_tenant_id UNIQUE (tenant_id, id),
    CONSTRAINT fk_rate_line_version FOREIGN KEY (tenant_id, rate_card_version_id)
        REFERENCES contract_rate.rate_card_version (tenant_id, id),
    CONSTRAINT uq_rate_line_logical_lane UNIQUE (
        rate_card_version_id, origin_location_id, destination_location_id, equipment_type, transport_mode
    ),
    CONSTRAINT chk_rate_line_transport_mode CHECK (transport_mode = 'ROAD'),
    CONSTRAINT chk_rate_line_equipment_type CHECK (char_length(trim(equipment_type)) > 0),
    CONSTRAINT chk_rate_line_distinct_locations CHECK (origin_location_id <> destination_location_id)
);

CREATE INDEX idx_rate_line_version ON contract_rate.rate_line (tenant_id, rate_card_version_id);
CREATE INDEX idx_rate_line_resolution ON contract_rate.rate_line (
    tenant_id, origin_location_id, destination_location_id, equipment_type, transport_mode
);

CREATE TABLE contract_rate.rate_component (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rate_line_id UUID NOT NULL,
    component_type VARCHAR(64) NOT NULL,
    calculation_method VARCHAR(32) NOT NULL,
    amount NUMERIC(18, 2),
    percent_value NUMERIC(9, 6),
    unit_code VARCHAR(32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    CONSTRAINT uq_rate_component_tenant_id UNIQUE (tenant_id, id),
    CONSTRAINT fk_rate_component_line FOREIGN KEY (tenant_id, rate_line_id)
        REFERENCES contract_rate.rate_line (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT uq_rate_component_type UNIQUE (rate_line_id, component_type),
    CONSTRAINT chk_rate_component_type CHECK (
        component_type IN ('BASE_FREIGHT', 'FUEL_SURCHARGE', 'WAITING', 'DETENTION')
    ),
    CONSTRAINT chk_rate_component_method CHECK (
        calculation_method IN ('FLAT', 'PERCENT', 'UNIT_RATE')
    ),
    CONSTRAINT chk_rate_component_flat CHECK (
        calculation_method <> 'FLAT'
        OR (amount IS NOT NULL AND percent_value IS NULL AND unit_code IS NULL)
    ),
    CONSTRAINT chk_rate_component_percent CHECK (
        calculation_method <> 'PERCENT'
        OR (amount IS NULL AND percent_value IS NOT NULL AND unit_code IS NULL)
    ),
    CONSTRAINT chk_rate_component_unit_rate CHECK (
        calculation_method <> 'UNIT_RATE'
        OR (amount IS NOT NULL AND percent_value IS NULL AND unit_code IS NOT NULL)
    )
);

CREATE INDEX idx_rate_component_line ON contract_rate.rate_component (tenant_id, rate_line_id);
