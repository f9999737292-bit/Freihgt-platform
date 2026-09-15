# ADR-RFX-015: External IDs and Reference Mapping

**Status:** Accepted (architecture freeze — pending controller review)  
**Date:** 2026-09-16  
**Deciders:** E7 Phase 2 ERP architecture stream  
**Normative detail:** [RFX_V3_0E7_ERP_API.md](../implementation/RFX_V3_0E7_ERP_API.md) §7

---

## Context

ERP systems identify business objects with external keys (SAP document number, 1C reference, TMS load ID). The platform must:

- Map external identity → internal `rfx_event_id`
- Support safe retries and duplicate detection
- Translate external reference codes (currency, units, locations) before domain validation

Repository evidence:

- Table `rfx.rfx_external_object_links` — migration `000073`
- Unique index `uq_rfx_external_object_identity` on `(tenant_id, integration_principal_id, external_system, external_object_type, external_object_id, external_version)`
- Repository: `external_object_link_repository.go`
- Payment precedent: `external_id` + source-scoped uniqueness — migration `000045`

No reference mapping tables exist today for RFx ERP.

---

## Decision

### 1. External ID uniqueness (frozen)

**Composite key:**

```
tenant_id
+ integration_principal_id
+ external_system
+ external_object_type   (= "RFX_EVENT" for buyer RFx)
+ external_object_id
+ external_version
```

### 2. Lifecycle scenarios

| Scenario | Behavior |
|---|---|
| First CREATE commit | Insert link; allocate `rfx_event_id` |
| Retry CREATE (same external ID, same payload) | Idempotency-Key replay → same event |
| Duplicate CREATE (same external ID, new Idempotency-Key) | 409 `external_id_conflict` if link exists |
| UPDATE commit | Link must exist; `payload_hash` updated |
| External version bump | New link row OR update per policy (default: same `object_id` + incremented `object_version`) |
| Disabled external system | Principal flag `external_system_enabled=false` → 403 |
| Cross-principal same external ID | Allowed (different `integration_principal_id`) |
| Cross-tenant | Impossible via credential binding |

### 3. Internal ID allocation

- Internal UUID assigned at CREATE commit (not at preview)
- Preview may reference provisional `target_type=NEW_EVENT` with `target_id=NULL`
- GET by external ID resolves via link table → tenant-scoped event fetch

### 4. Reference mapping architecture

New logical entities (migration 000074 — proposed, not created):

#### Mapping sets

| Field | Purpose |
|---|---|
| `mapping_type` | `CURRENCY`, `COUNTRY`, `LANGUAGE`, `UNIT`, `TIMEZONE`, `CARGO_TYPE`, `VEHICLE_BODY`, `LOCATION`, `CARRIER_CODE`, `INCOTERM` |
| `version` | Monotonic per tenant+type |
| `status` | `DRAFT`, `ACTIVE`, `RETIRED` |
| `tenant_id` | NULL = platform default; non-null = tenant override |

#### Mapping entries

| Field | Purpose |
|---|---|
| `bintrans_code` | Canonical platform code |
| `external_code` | ERP/vendor code |
| `effective_from` / `effective_to` | Optional date range |
| `severity_on_unknown` | Inherited from set default |

### 5. Unknown-code behavior

| Mapping type | Default behavior |
|---|---|
| `CURRENCY`, `COUNTRY`, `UNIT`, `TIMEZONE` | **Error** (fail closed) |
| `CARGO_TYPE`, `VEHICLE_BODY` | **Warning** if mapping set allows; else error |
| `LOCATION` | Error unless tenant enables fuzzy match (future) |
| `CARRIER_CODE` | Error (invitation requires valid carrier) |
| `INCOTERM` | Warning if optional field; error if required |

Mapping runs **before** canonical hash computation so stored analysis reflects resolved canonical codes.

### 6. Vendor adapter boundary

- SAP/1C/Oracle adapters transform to `BINTRANS_RFX_ERP_JSON_V1` **before** API call
- Core API never accepts SAP-specific field names
- Mapping failures return 422 with `external_source` in issue envelope

---

## Alternatives considered

### A. External ID only (no mapping tables)

**Rejected.** ERP codes diverge from platform enums; inline mapping in JSON pushes vendor logic into core API.

### B. Runtime lookup against master data APIs only

**Rejected.** No unified master data service exists; tenant-specific overrides required.

### C. Store external ID in event title/metadata JSON

**Rejected.** No uniqueness enforcement; breaks retry/idempotency semantics.

---

## Consequences

### Positive

- Reuses existing link table and unique index
- Clear adapter boundary
- Tenant-specific mapping without schema forks

### Negative / cost

- New mapping admin API/UI (future)
- Migration 000074 for mapping tables + seed platform defaults

---

## References

- `infrastructure/migrations/000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql`
- [ADR-RFX-012](./ADR-RFX-012-ERP-CANONICAL-JSON-CONTRACT.md)
- `services/rfx-service/internal/repository/external_object_link_repository.go`
