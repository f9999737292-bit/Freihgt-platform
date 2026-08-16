CREATE TABLE rfx.audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    entity_type VARCHAR(80) NOT NULL,
    entity_id UUID NOT NULL,
    action VARCHAR(80) NOT NULL,
    actor_user_id UUID,
    actor_company_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_rfx_audit_events_tenant_id ON rfx.audit_events(tenant_id);
CREATE INDEX idx_rfx_audit_events_entity ON rfx.audit_events(tenant_id, entity_type, entity_id);
CREATE INDEX idx_rfx_audit_events_occurred_at ON rfx.audit_events(tenant_id, occurred_at DESC);
