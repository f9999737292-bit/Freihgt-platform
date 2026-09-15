# ADR-RFX-012: ERP Canonical JSON Contract

**Status:** FROZEN_PENDING_REVIEW
**Date:** 2026-09-16
**Deciders:** E7 Phase 2 ERP architecture stream
**Normative detail:** [RFX_V3_0E7_ERP_API.md](../implementation/RFX_V3_0E7_ERP_API.md) §6

---

## Context

Buyer ERP systems need a vendor-neutral JSON contract. Excel import (`BINTRANS_RFX_BUYER_XLSX_V1`) proves Preview/Commit but does not cover M2M integration or stable external identity.

---

## Decision

Adopt **`BINTRANS_RFX_ERP_JSON_V1`**.

### Schema identity

| Field | Value |
|---|---|
| `schema_version` | `BINTRANS_RFX_ERP_JSON_V1` |
| Content-Type | `application/json` |
| Max body size | 2 MiB |

### Top-level allowlist

| JSON path | Type | Required | Validation | Canonicalization | Domain mapping |
|---|---|---|---|---|---|
| `schema_version` | string | Yes | Must equal `BINTRANS_RFX_ERP_JSON_V1` | Literal | — |
| `requested_operation` | enum | Yes | `CREATE_DRAFT` \| `UPDATE_DRAFT` | Uppercase | Routes target type |
| `external.system` | string | Yes (CREATE) | `[A-Z0-9_]{1,64}` | Uppercase trim | Stable link |
| `external.object_id` | string | Yes (CREATE) | 1–256 NFC trim | Preserved case | Stable link |
| `external.revision` | string | No | 1–64; default `"1"` | Trim | Revision metadata only |
| `event.type` | enum | Yes | Platform RFx types | Enum normalize | `rfx_events.type` |
| `event.title` | string | Yes | 1–500 NFC | Trim | `rfx_events.title` |
| `event.description` | string | No | Max 8000 | NFC trim | `rfx_events.description` |
| `event.currency` | string | Yes | Mapped ISO 4217 | Uppercase | Event currency |
| `event.timezone` | string | Yes | Mapped IANA | Canonical TZ | Deadline display |
| `event.deadline` | datetime | No | RFC3339 UTC | UTC normalize | `response_deadline` |
| `lots[]` | array | No | Max 200 | Sort by `lot_number` | `rfx_lots` |
| `lots[].lot_number` | string | Yes per lot | Unique in payload | Trim | `lot_number` |
| `lots[].name` | string | Yes | 1–255 | NFC trim | `name` |
| `lots[].description` | string | No | Max 2000 | NFC trim | `description` |
| `lots[].category` | string | No | Mapped | Trim | `category` |
| `lots[].estimated_value` | number | No | ≥ 0 | 2dp round | `estimated_value` |
| `lots[].currency_code` | string | No | Mapped ISO 4217 | Uppercase | `currency_code` |
| `questionnaire.*` | object | No | Same stable-code rules as XLSX canonical | Deterministic sort | Graph reconcile |
| `template_reference.*` | object | No | Valid UUIDs if set | — | Provenance |
| `mapping_context` | object | Server-only in stored analysis | Set at Preview after mapping | In canonical hash | Pin mapping version |
| `mapping_context.mapping_set_id` | uuid | Set by server | — | — | ADR-015 |
| `mapping_context.mapping_set_version` | int | Set by server | — | — | ADR-015 |
| `extensions.*` | object | No | Namespaced keys only | Sorted keys | Non-authoritative |

### Freight domain disposition (H-05)

| Field group | v1 status |
|---|---|
| Lots + questionnaire | **SUPPORTED_V1** |
| Lanes, origin/destination | **DEFERRED_DOMAIN_GAP** — `rfx_lanes` not in commit reconcile |
| Standalone cargo/weight/volume/packaging | **DEFERRED_DOMAIN_GAP** — use questionnaire |
| Vehicle/body, temperature, hazardous | **DEFERRED_DOMAIN_GAP** |
| Incoterms | **DEFERRED_DOMAIN_GAP** |
| `lanes[]`, `cargo[]`, `routes[]` top-level keys | **Rejected** → 422 `unsupported_field_v1` |

ERP v1 aligns with Buyer XLSX commit surface (lots + questionnaire only).

### Canonicalization

1. Sorted JSON keys; deterministic array ordering.
2. `mapping_context` + resolved canonical codes included before hash.
3. SHA-256 via JSONB-stable bytes (same pattern as `StableStoredPayload`).
4. Schema downgrade rejected → 422.

### Mass assignment

Unknown top-level keys → 422 `unknown_field`. No `additionalProperties`.

---

## Consequences

- Migration 000074 required for schema CHECK extension.
- ERP parser separate from XLSX parser.
- Full freight lane parity deferred explicitly.

---

## References

- [ADR-RFX-014](./ADR-RFX-014-ERP-PREVIEW-COMMIT-IDEMPOTENCY.md)
- [ADR-RFX-015](./ADR-RFX-015-ERP-EXTERNAL-IDS-REFERENCE-MAPPING.md)
- `infrastructure/migrations/000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql`
