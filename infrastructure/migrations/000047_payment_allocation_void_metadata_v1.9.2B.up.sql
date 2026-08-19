ALTER TABLE billing.payment_allocations
    ADD COLUMN voided_by UUID NULL,
    ADD COLUMN void_reason VARCHAR(255) NULL;
