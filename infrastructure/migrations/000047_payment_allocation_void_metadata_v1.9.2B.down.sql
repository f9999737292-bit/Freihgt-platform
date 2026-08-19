ALTER TABLE billing.payment_allocations
    DROP COLUMN IF EXISTS voided_by,
    DROP COLUMN IF EXISTS void_reason;
