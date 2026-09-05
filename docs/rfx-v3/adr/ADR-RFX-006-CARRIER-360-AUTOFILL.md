# ADR-RFX-006: Carrier 360 Autofill

**Status:** Accepted (architecture draft)  
**Date:** 2026-09-03

## Context

Carriers re-enter the same fleet, insurance, and certificate data across tenders. Platform already holds company, document, and shipment data in separate services.

## Decision

Introduce **Carrier 360** aggregation layer feeding RFx autofill with provenance, freshness, expiry, and carrier confirmation before authoritative persist.

Autofill promotes to `answer_source=CARRIER_PROFILE` only after carrier confirms; edits use `CARRIER_DECLARED`.

## Consequences

- Cross-service read APIs with tenant isolation.
- Stale/expired fields trigger warnings or re-verification.
- Operational KPI available for `SYSTEM_DERIVED` scoring.

## References

- [RFX_V3_CARRIER_360.md](../RFX_V3_CARRIER_360.md)
