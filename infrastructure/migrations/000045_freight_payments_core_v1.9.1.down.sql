DROP TRIGGER IF EXISTS trg_payments_updated_at ON billing.payments;
DROP TRIGGER IF EXISTS trg_payment_obligations_updated_at ON billing.payment_obligations;

DROP TABLE IF EXISTS billing.payment_audit_events;
DROP TABLE IF EXISTS billing.payment_allocations;
DROP TABLE IF EXISTS billing.payments;
DROP TABLE IF EXISTS billing.payment_obligations;
