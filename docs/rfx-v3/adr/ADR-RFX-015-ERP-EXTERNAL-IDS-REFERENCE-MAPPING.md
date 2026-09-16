# ADR-RFX-015: External IDs and Reference Mapping

**Status:** Accepted
**Date:** 2026-09-16
**Deciders:** E7 Phase 2 ERP architecture stream
**Normative detail:** [RFX_V3_0E7_ERP_API.md](../implementation/RFX_V3_0E7_ERP_API.md) §5, §7

### Controller acceptance evidence

| Field | Value |
|---|---|
| Reviewed head | `c538bcf073f901f2e80a26b77fb14a6f4a9491c6` |
| Controller verdict | `ACCEPT_ERP_API_ARCHITECTURE` |
| HIGH findings open | 0 |
| MEDIUM findings open | 0 |
| Residual LOW findings | Fixed in acceptance-alignment commit |
| Implementation authorization | NO |

---

## Context

ERP objects need stable identity mapping. Migration 000073 unique index incorrectly includes `external_version` in uniqueness — allowing multiple RFx per same business object.

---

## Decision

### 1. Four separate concepts (H-03)

| Concept | Mechanism |
|---|---|
| Stable external object identity | One RFx per ERP business object |
| External payload revision | `external.revision` metadata on link |
| BINTRANS save version | `event_row_version` / `draft_row_version` |
| HTTP retry identity | `Idempotency-Key` |

### 2. Stable uniqueness key (frozen)

```
tenant_id
+ integration_principal_id
+ external_system              -- normalized [A-Z0-9_]
+ external_object_type         -- "RFX_EVENT"
+ normalized_external_object_id -- NFC trim, max 256, case-sensitive
```

**`external.revision` is NOT in the unique key.**

Migration 000074 replaces `uq_rfx_external_object_identity` to drop `external_version` from uniqueness.

### 3. Revision policy

| Event | Behavior |
|---|---|
| CREATE commit | Insert link with `external_revision` (default `"1"`) |
| UPDATE commit | Update `payload_hash` + append revision history row |
| Duplicate CREATE (same stable key) | 409 `external_id_conflict` |
| Rebind / second RFx for same stable key | Forbidden |
| GET without revision | Single RFx for stable key — test **E7P2-INT-192** |
| GET with `external_revision` query | Same RFx + revision metadata if recorded; not a different RFx |

### 4. Normalization

| Field | Rule |
|---|---|
| `external_system` | Uppercase, `[A-Z0-9_]{1,64}` |
| `external_object_id` | NFC trim, 1–256 chars, case preserved |
| Disabled principal | 403 |
| Cross-tenant | Impossible via credential |

### 5. Reference mapping

New tables in 000074: `rfx_reference_mapping_sets`, `rfx_reference_mapping_entries`.

### 6. Mapping version pin (H-04)

Preview stores in canonical analysis payload:

- `mapping_context.mapping_set_id`
- `mapping_context.mapping_set_version`
- Resolved canonical code values

Included in SHA-256 hash.

**Commit policy:** Apply pinned resolved values only — **no re-mapping**. If mapping set status is `RETIRED` after Preview → **409 `stale_mapping_context`**.

Test: **E7P2-INT-193**.

Mapping runs once at Preview; Commit trusts immutable analysis payload (DB trigger prevents payload mutation).

### 7. Unknown codes

| Type | Default |
|---|---|
| CURRENCY, COUNTRY, UNIT, TIMEZONE | Error (fail closed) |
| CARGO_TYPE, VEHICLE_BODY | Warning if set allows |
| CARRIER_CODE | Error |

---

## Consequences

- Requires migration 000074 unique index change on external links.
- Existing 000073 index is superseded — not compatible with stable identity policy without migration.

---

## References

- `infrastructure/migrations/000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql`
- [ADR-RFX-012](./ADR-RFX-012-ERP-CANONICAL-JSON-CONTRACT.md)
