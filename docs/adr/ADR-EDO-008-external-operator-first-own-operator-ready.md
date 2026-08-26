# ADR-EDO-008: External Operator First / Own Operator Ready

## Status

Accepted — architecture freeze (EDO-0.2)

## Context

BINTRANS is a technology platform, not a licensed EPD/EDO operator today. Discovery noted `core.companies.company_type` includes `EDO_OPERATOR`, `EPD_OPERATOR` as metadata only. Billing exposes mock `mark-sent-to-edo` without real operator connectivity.

## Decision

### Operating modes (frozen flags)

```text
EXTERNAL_OPERATOR_MODE=YES          # Initial and near-term production path
FUTURE_OWN_OPERATOR_READY=YES       # Architecture must support own-operator adapter
OWN_IS_EPD_OPERATOR_MODE=NO         # Cannot enable until external licensing verified
GIS_EPD_CONNECTED=NO                # No Mintrans GIS integration in any current phase
```

### Architecture requirements

1. **Core EDO workflows** (sign, validate, archive, relate documents) must not branch on operator identity — only adapter layer varies.
2. **EPDOperator port** (ADR-EDO-004) accepts pluggable adapters:
   - `ExternalOperatorA`, `ExternalOperatorB` — initial production
   - `FutureBintransIS_EPD` — reserved adapter name; not implemented
3. **Operator routing policy** — configuration selects adapter by tenant, document type, or jurisdiction (design only).
4. **Transaction correlation** — operator-agnostic `epd_transaction_id` in transport-edo-service; maps to external operator reference in adapter.
5. **Credential isolation** — per-operator secrets in CRITICAL vault segment; never in shared-go or document-service config.
6. **Multi-operator** — architecture allows multiple concurrent adapters; no singleton operator assumption in domain code.

### Documented gaps (no implementation)

| Gap | Owner phase |
|-----|-------------|
| Real operator API contracts | TEDO-0.4+ |
| Own-operator licensing | External / business |
| GIS EPD certification | External |
| Operator SLA monitoring | INFRA + TEDO |
| Failover between operators | TEDO (future) |

### Compliance

**Do not claim regulatory certification or operator accreditation** from repository evidence.

## Consequences

- Staging can demo EDO flows with stub adapter before licensing
- Own-operator path is additive adapter swap, not domain rewrite

## References

- ADR-EDO-004
- Discovery finding F-003, F-005
- Future operator gaps list (discovery v0.1)
