CREATE OR REPLACE FUNCTION core.set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_tenants_updated_at
BEFORE UPDATE ON core.tenants
FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_companies_updated_at
BEFORE UPDATE ON core.companies
FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_users_updated_at
BEFORE UPDATE ON core.users
FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_transport_orders_updated_at
BEFORE UPDATE ON transport.transport_orders
FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_shipments_updated_at
BEFORE UPDATE ON transport.shipments
FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_rfx_events_updated_at
BEFORE UPDATE ON rfx.rfx_events
FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_freight_requests_updated_at
BEFORE UPDATE ON rfx.freight_requests
FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_documents_updated_at
BEFORE UPDATE ON documents.documents
FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_billing_registers_updated_at
BEFORE UPDATE ON billing.billing_registers
FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();
