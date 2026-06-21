CREATE TABLE rfx.rfx_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_number VARCHAR(100) NOT NULL,
    rfx_type VARCHAR(50) NOT NULL,
    category VARCHAR(80) NOT NULL,
    title VARCHAR(500) NOT NULL,
    description TEXT,
    owner_company_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    currency_code CHAR(3) DEFAULT 'RUB',
    valid_from DATE,
    valid_to DATE,
    response_deadline TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_rfx_number UNIQUE (tenant_id, rfx_number),
    CONSTRAINT chk_rfx_type CHECK (
        rfx_type IN (
            'RFI','RFQ','RFP','RFG','RFT','SPOT_RFQ','MINI_TENDER',
            'LANE_TENDER','CONTRACT_TENDER','SEASONAL_TENDER',
            'PROJECT_TENDER','REVERSE_AUCTION'
        )
    ),
    CONSTRAINT chk_rfx_status CHECK (
        status IN (
            'DRAFT','PUBLISHED','INVITATION_SENT','QUESTIONS_OPEN',
            'RESPONSES_OPEN','RESPONSES_CLOSED','EVALUATION_IN_PROGRESS',
            'SHORTLISTED','AWARDED','PARTIALLY_AWARDED','CANCELLED','ARCHIVED'
        )
    )
);

CREATE INDEX idx_rfx_events_tenant_id ON rfx.rfx_events(tenant_id);
CREATE INDEX idx_rfx_events_owner_company_id ON rfx.rfx_events(owner_company_id);
CREATE INDEX idx_rfx_events_type ON rfx.rfx_events(rfx_type);
CREATE INDEX idx_rfx_events_status ON rfx.rfx_events(status);
CREATE INDEX idx_rfx_events_response_deadline ON rfx.rfx_events(response_deadline);

CREATE TABLE rfx.rfx_lots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_event_id UUID NOT NULL REFERENCES rfx.rfx_events(id) ON DELETE CASCADE,
    lot_number VARCHAR(100) NOT NULL,
    name VARCHAR(500) NOT NULL,
    description TEXT,
    category VARCHAR(80),
    estimated_value NUMERIC(18,2),
    currency_code CHAR(3) DEFAULT 'RUB',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_rfx_lot_number UNIQUE (rfx_event_id, lot_number)
);

CREATE INDEX idx_rfx_lots_tenant_id ON rfx.rfx_lots(tenant_id);
CREATE INDEX idx_rfx_lots_event_id ON rfx.rfx_lots(rfx_event_id);

CREATE TABLE rfx.rfx_lanes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_lot_id UUID NOT NULL REFERENCES rfx.rfx_lots(id) ON DELETE CASCADE,
    origin_location_id UUID,
    destination_location_id UUID,
    transport_mode VARCHAR(50) NOT NULL DEFAULT 'ROAD',
    equipment_type VARCHAR(100),
    estimated_volume NUMERIC(18,3),
    volume_unit VARCHAR(50),
    required_service_level VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID
);

CREATE INDEX idx_rfx_lanes_tenant_id ON rfx.rfx_lanes(tenant_id);
CREATE INDEX idx_rfx_lanes_lot_id ON rfx.rfx_lanes(rfx_lot_id);
CREATE INDEX idx_rfx_lanes_origin_destination ON rfx.rfx_lanes(origin_location_id, destination_location_id);

CREATE TABLE rfx.rfx_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_event_id UUID NOT NULL REFERENCES rfx.rfx_events(id) ON DELETE CASCADE,
    company_id UUID NOT NULL,
    participant_type VARCHAR(50) NOT NULL DEFAULT 'SUPPLIER',
    status VARCHAR(50) NOT NULL DEFAULT 'INVITED',
    invited_at TIMESTAMPTZ,
    viewed_at TIMESTAMPTZ,
    responded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    CONSTRAINT uq_rfx_participant UNIQUE (rfx_event_id, company_id),
    CONSTRAINT chk_rfx_participant_status CHECK (
        status IN (
            'INVITED','VIEWED','DECLINED','RESPONSE_DRAFT','RESPONSE_SUBMITTED',
            'SHORTLISTED','AWARDED','NOT_AWARDED'
        )
    )
);

CREATE INDEX idx_rfx_participants_tenant_id ON rfx.rfx_participants(tenant_id);
CREATE INDEX idx_rfx_participants_event_id ON rfx.rfx_participants(rfx_event_id);
CREATE INDEX idx_rfx_participants_company_id ON rfx.rfx_participants(company_id);

CREATE TABLE rfx.rfx_responses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_event_id UUID NOT NULL REFERENCES rfx.rfx_events(id) ON DELETE CASCADE,
    participant_company_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    submitted_at TIMESTAMPTZ,
    submitted_by UUID,
    total_score NUMERIC(10,2),
    commercial_score NUMERIC(10,2),
    technical_score NUMERIC(10,2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_rfx_response UNIQUE (rfx_event_id, participant_company_id),
    CONSTRAINT chk_rfx_response_status CHECK (
        status IN ('DRAFT','SUBMITTED','WITHDRAWN','REJECTED','ACCEPTED')
    )
);

CREATE INDEX idx_rfx_responses_tenant_id ON rfx.rfx_responses(tenant_id);
CREATE INDEX idx_rfx_responses_event_id ON rfx.rfx_responses(rfx_event_id);
CREATE INDEX idx_rfx_responses_participant ON rfx.rfx_responses(participant_company_id);

CREATE TABLE rfx.freight_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    freight_request_number VARCHAR(100) NOT NULL,
    transport_order_id UUID,
    shipment_id UUID,
    request_type VARCHAR(50) NOT NULL,
    shipper_company_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    response_deadline TIMESTAMPTZ,
    currency_code CHAR(3) DEFAULT 'RUB',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_freight_request_number UNIQUE (tenant_id, freight_request_number),
    CONSTRAINT chk_freight_request_type CHECK (
        request_type IN (
            'SPOT','MINI_TENDER','LANE_TENDER','CONTRACT_TENDER',
            'SEASONAL_TENDER','PROJECT_TENDER'
        )
    )
);

CREATE INDEX idx_freight_requests_tenant_id ON rfx.freight_requests(tenant_id);
CREATE INDEX idx_freight_requests_shipper ON rfx.freight_requests(shipper_company_id);
CREATE INDEX idx_freight_requests_status ON rfx.freight_requests(status);
CREATE INDEX idx_freight_requests_transport_order_id ON rfx.freight_requests(transport_order_id);
CREATE INDEX idx_freight_requests_shipment_id ON rfx.freight_requests(shipment_id);

CREATE TABLE rfx.bids (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    freight_request_id UUID NOT NULL REFERENCES rfx.freight_requests(id) ON DELETE CASCADE,
    carrier_company_id UUID NOT NULL,
    bid_number VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    total_amount NUMERIC(18,2),
    currency_code CHAR(3) DEFAULT 'RUB',
    vat_rate NUMERIC(5,2),
    vat_amount NUMERIC(18,2),
    total_amount_with_vat NUMERIC(18,2),
    valid_until TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,
    submitted_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_bid_number UNIQUE (tenant_id, bid_number),
    CONSTRAINT uq_bid_carrier_request UNIQUE (freight_request_id, carrier_company_id),
    CONSTRAINT chk_bid_status CHECK (
        status IN ('DRAFT','SUBMITTED','WITHDRAWN','ACCEPTED','REJECTED','EXPIRED')
    )
);

CREATE INDEX idx_bids_tenant_id ON rfx.bids(tenant_id);
CREATE INDEX idx_bids_freight_request_id ON rfx.bids(freight_request_id);
CREATE INDEX idx_bids_carrier_company_id ON rfx.bids(carrier_company_id);
CREATE INDEX idx_bids_status ON rfx.bids(status);

CREATE TABLE rfx.bid_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    bid_id UUID NOT NULL REFERENCES rfx.bids(id) ON DELETE CASCADE,
    lane_id UUID,
    description TEXT,
    base_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    fuel_surcharge NUMERIC(18,2) NOT NULL DEFAULT 0,
    toll_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    extra_charges NUMERIC(18,2) NOT NULL DEFAULT 0,
    amount_without_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    vat_rate NUMERIC(5,2),
    vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    amount_with_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_bid_items_tenant_id ON rfx.bid_items(tenant_id);
CREATE INDEX idx_bid_items_bid_id ON rfx.bid_items(bid_id);
