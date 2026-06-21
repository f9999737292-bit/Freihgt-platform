CREATE TABLE transport.locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    company_id UUID,
    location_type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    country_code CHAR(2) NOT NULL,
    region VARCHAR(255),
    city VARCHAR(255),
    address_line TEXT,
    postal_code VARCHAR(50),
    lat NUMERIC(10,7),
    lon NUMERIC(10,7),
    timezone VARCHAR(100) NOT NULL DEFAULT 'Europe/Moscow',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT chk_location_type CHECK (
        location_type IN (
            'WAREHOUSE','FACTORY','DISTRIBUTION_CENTER','TERMINAL','PORT',
            'AIRPORT','RAIL_STATION','BORDER_CHECKPOINT','CUSTOMER_SITE'
        )
    )
);

CREATE INDEX idx_locations_tenant_id ON transport.locations(tenant_id);
CREATE INDEX idx_locations_company_id ON transport.locations(company_id);
CREATE INDEX idx_locations_country_city ON transport.locations(country_code, city);
CREATE INDEX idx_locations_type ON transport.locations(location_type);

CREATE TABLE transport.cargoes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    cargo_type VARCHAR(100) NOT NULL,
    description TEXT,
    gross_weight NUMERIC(18,3),
    net_weight NUMERIC(18,3),
    volume NUMERIC(18,3),
    temperature_min NUMERIC(6,2),
    temperature_max NUMERIC(6,2),
    dangerous_goods_flag BOOLEAN NOT NULL DEFAULT false,
    customs_required BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_cargoes_tenant_id ON transport.cargoes(tenant_id);
CREATE INDEX idx_cargoes_type ON transport.cargoes(cargo_type);

CREATE TABLE transport.cargo_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cargo_id UUID NOT NULL REFERENCES transport.cargoes(id) ON DELETE CASCADE,
    sku VARCHAR(100),
    name VARCHAR(255) NOT NULL,
    quantity NUMERIC(18,3) NOT NULL,
    unit VARCHAR(50) NOT NULL,
    weight NUMERIC(18,3),
    volume NUMERIC(18,3),
    package_type VARCHAR(100),
    hazard_class VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cargo_items_cargo_id ON transport.cargo_items(cargo_id);
CREATE INDEX idx_cargo_items_sku ON transport.cargo_items(sku);

CREATE TABLE transport.vehicles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,
    plate_number VARCHAR(50) NOT NULL,
    vehicle_type VARCHAR(100) NOT NULL,
    equipment_type VARCHAR(100),
    capacity_weight NUMERIC(18,3),
    capacity_volume NUMERIC(18,3),
    registration_country CHAR(2) NOT NULL DEFAULT 'RU',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_vehicle_plate_tenant UNIQUE (tenant_id, plate_number)
);

CREATE INDEX idx_vehicles_tenant_id ON transport.vehicles(tenant_id);
CREATE INDEX idx_vehicles_carrier_company_id ON transport.vehicles(carrier_company_id);
CREATE INDEX idx_vehicles_plate_number ON transport.vehicles(plate_number);

CREATE TABLE transport.drivers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,
    user_id UUID,
    full_name VARCHAR(255) NOT NULL,
    phone VARCHAR(50),
    license_number VARCHAR(100),
    license_country CHAR(2) DEFAULT 'RU',
    preferred_locale VARCHAR(10) NOT NULL DEFAULT 'ru-RU',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_drivers_tenant_id ON transport.drivers(tenant_id);
CREATE INDEX idx_drivers_carrier_company_id ON transport.drivers(carrier_company_id);
CREATE INDEX idx_drivers_user_id ON transport.drivers(user_id);

CREATE TABLE transport.transport_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    order_number VARCHAR(100) NOT NULL,
    shipper_company_id UUID NOT NULL,
    consignee_company_id UUID NOT NULL,
    origin_location_id UUID NOT NULL REFERENCES transport.locations(id),
    destination_location_id UUID NOT NULL REFERENCES transport.locations(id),
    cargo_id UUID REFERENCES transport.cargoes(id),
    requested_pickup_date DATE,
    requested_delivery_date DATE,
    transport_mode VARCHAR(50) NOT NULL DEFAULT 'ROAD',
    equipment_type VARCHAR(100),
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    source_system VARCHAR(100),
    external_reference VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_transport_order_number UNIQUE (tenant_id, order_number),
    CONSTRAINT chk_transport_order_status CHECK (
        status IN (
            'DRAFT','READY_FOR_SOURCING','SOURCING_IN_PROGRESS',
            'ASSIGNED','CANCELLED','CONVERTED_TO_SHIPMENT'
        )
    )
);

CREATE INDEX idx_transport_orders_tenant_id ON transport.transport_orders(tenant_id);
CREATE INDEX idx_transport_orders_shipper ON transport.transport_orders(shipper_company_id);
CREATE INDEX idx_transport_orders_consignee ON transport.transport_orders(consignee_company_id);
CREATE INDEX idx_transport_orders_status ON transport.transport_orders(status);
CREATE INDEX idx_transport_orders_pickup_date ON transport.transport_orders(requested_pickup_date);

CREATE TABLE transport.shipments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    shipment_number VARCHAR(100) NOT NULL,
    transport_order_id UUID REFERENCES transport.transport_orders(id),
    shipper_company_id UUID NOT NULL,
    consignee_company_id UUID NOT NULL,
    carrier_company_id UUID,
    forwarder_company_id UUID,
    driver_id UUID REFERENCES transport.drivers(id),
    vehicle_id UUID REFERENCES transport.vehicles(id),
    origin_location_id UUID NOT NULL REFERENCES transport.locations(id),
    destination_location_id UUID NOT NULL REFERENCES transport.locations(id),
    cargo_id UUID REFERENCES transport.cargoes(id),
    transport_mode VARCHAR(50) NOT NULL DEFAULT 'ROAD',
    status VARCHAR(80) NOT NULL DEFAULT 'CREATED',
    planned_pickup_at TIMESTAMPTZ,
    planned_delivery_at TIMESTAMPTZ,
    actual_pickup_at TIMESTAMPTZ,
    actual_delivery_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_shipment_number UNIQUE (tenant_id, shipment_number),
    CONSTRAINT chk_shipment_status CHECK (
        status IN (
            'CREATED','CARRIER_ASSIGNED','ACCEPTED_BY_CARRIER','VEHICLE_ASSIGNED',
            'DRIVER_ASSIGNED','PICKUP_SLOT_BOOKED','DELIVERY_SLOT_BOOKED',
            'IN_PICKUP','LOADED','IN_TRANSIT','ARRIVED_AT_CONSIGNEE',
            'UNLOADING','DELIVERED','DELIVERY_CONFIRMED','DOCUMENTS_COMPLETED',
            'READY_FOR_BILLING','INCLUDED_IN_BILLING_REGISTER',
            'FINANCIALLY_CLOSED','CANCELLED'
        )
    )
);

CREATE INDEX idx_shipments_tenant_id ON transport.shipments(tenant_id);
CREATE INDEX idx_shipments_transport_order_id ON transport.shipments(transport_order_id);
CREATE INDEX idx_shipments_shipper ON transport.shipments(shipper_company_id);
CREATE INDEX idx_shipments_consignee ON transport.shipments(consignee_company_id);
CREATE INDEX idx_shipments_carrier ON transport.shipments(carrier_company_id);
CREATE INDEX idx_shipments_status ON transport.shipments(status);
CREATE INDEX idx_shipments_planned_pickup ON transport.shipments(planned_pickup_at);
CREATE INDEX idx_shipments_planned_delivery ON transport.shipments(planned_delivery_at);
