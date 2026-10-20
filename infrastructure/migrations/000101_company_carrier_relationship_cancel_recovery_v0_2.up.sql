-- Cancel idempotency, one terminal request event, and a one-way head transition.
-- Historical request roots, events, and relationship versions are not updated.

CREATE TABLE company.carrier_relationship_cancel_idempotency (
    caller_service VARCHAR(64) NOT NULL,
    source_tenant_id UUID NOT NULL,
    requesting_company_id UUID NOT NULL,
    idempotency_key VARCHAR(200) NOT NULL,
    relationship_request_id UUID NOT NULL REFERENCES company.carrier_relationship_requests(relationship_request_id),
    body_hash BYTEA NOT NULL,
    request_status VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (caller_service, source_tenant_id, requesting_company_id, idempotency_key),
    CONSTRAINT chk_crr_cancel_idempotency_key CHECK (length(btrim(idempotency_key)) > 0),
    CONSTRAINT chk_crr_cancel_body_hash_len CHECK (octet_length(body_hash) = 32),
    CONSTRAINT chk_crr_cancel_idempotency_status CHECK (request_status IN ('CANCELLED', 'EXPIRED'))
);

CREATE UNIQUE INDEX uq_crr_one_terminal_event
    ON company.carrier_relationship_request_events (relationship_request_id)
    WHERE event_type IN ('ACCEPTED', 'EXPIRED', 'CANCELLED');

CREATE OR REPLACE FUNCTION company.reject_request_head_illegal_transition()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.request_status = NEW.request_status THEN
        IF OLD.request_status <> 'PENDING'
           AND (
                NEW.current_event_id IS DISTINCT FROM OLD.current_event_id
                OR NEW.current_version IS DISTINCT FROM OLD.current_version
           ) THEN
            RAISE EXCEPTION 'request head terminal state is immutable'
                USING ERRCODE = '55000';
        END IF;
        RETURN NEW;
    END IF;
    IF OLD.request_status = 'PENDING'
       AND NEW.request_status IN ('ACCEPTED', 'EXPIRED', 'CANCELLED') THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'request head transition is illegal'
        USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER crr_heads_transition_guard
    BEFORE UPDATE ON company.carrier_relationship_request_heads
    FOR EACH ROW EXECUTE FUNCTION company.reject_request_head_illegal_transition();
