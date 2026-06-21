-- Additional indexes for high-load list/filter queries (idempotent).

CREATE INDEX IF NOT EXISTS idx_freight_requests_request_type ON rfx.freight_requests(request_type);
