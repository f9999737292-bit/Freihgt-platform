CREATE TABLE transport.shipment_cargo_execution_evidence (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL REFERENCES transport.shipments (id),
    shipment_version integer NOT NULL,
    cargo_id uuid NOT NULL REFERENCES transport.cargoes (id),
    state text NOT NULL,
    state_version integer NOT NULL,
    source text NOT NULL,
    source_event_type text NOT NULL,
    actor_id uuid,
    driver_id uuid,
    occurred_at timestamptz NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT shipment_cargo_execution_evidence_state_chk CHECK (
        state IN ('PLANNED', 'PICKED_UP', 'CONFIRMED_ONBOARD', 'UNLOADED')
    ),
    CONSTRAINT shipment_cargo_execution_evidence_state_version_chk CHECK (state_version > 0),
    CONSTRAINT shipment_cargo_execution_evidence_shipment_version_chk CHECK (shipment_version > 0),
    CONSTRAINT shipment_cargo_execution_evidence_source_chk CHECK (btrim(source) <> ''),
    CONSTRAINT shipment_cargo_execution_evidence_event_chk CHECK (btrim(source_event_type) <> ''),
    CONSTRAINT shipment_cargo_execution_evidence_version_uidx UNIQUE (shipment_id, cargo_id, state_version)
);

CREATE INDEX shipment_cargo_execution_evidence_latest_idx
    ON transport.shipment_cargo_execution_evidence (tenant_id, shipment_id, state_version DESC, occurred_at DESC, id DESC);

COMMENT ON TABLE transport.shipment_cargo_execution_evidence IS
    'Append-only unit execution evidence. Shipment status is not onboard proof. No historical backfill.';

CREATE OR REPLACE FUNCTION transport.deny_shipment_cargo_execution_evidence_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'shipment_cargo_execution_evidence is append-only';
END;
$$;

CREATE TRIGGER trg_shipment_cargo_execution_evidence_no_update
    BEFORE UPDATE ON transport.shipment_cargo_execution_evidence
    FOR EACH ROW EXECUTE FUNCTION transport.deny_shipment_cargo_execution_evidence_mutation();

CREATE TRIGGER trg_shipment_cargo_execution_evidence_no_delete
    BEFORE DELETE ON transport.shipment_cargo_execution_evidence
    FOR EACH ROW EXECUTE FUNCTION transport.deny_shipment_cargo_execution_evidence_mutation();
