# ADR-RFX-012: ERP Canonical JSON Contract

**Status:** Accepted
**Date:** 2026-09-16
**Deciders:** E7 Phase 2 ERP architecture stream
**Normative detail:** [RFX_V3_0E7_ERP_API.md](../implementation/RFX_V3_0E7_ERP_API.md) §6

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
| Max JSON nesting depth | **32** (`ERP_JSON_MAX_DEPTH`) |
| Duplicate JSON keys | **REJECT_FAIL_CLOSED** → 422 `duplicate_field` |

Depth is enforced during streaming ingest **before** full materialization of deep structures. Duplicate keys in any object are rejected; **no last-wins** semantics.

### Top-level allowlist

| JSON path | Type | Required | Validation | Canonicalization | Domain mapping |
|---|---|---|---|---|---|
| `schema_version` | string | Yes | Must equal `BINTRANS_RFX_ERP_JSON_V1` | Literal | — |
| `requested_operation` | enum | Yes | `CREATE_DRAFT` \| `UPDATE_DRAFT` | Uppercase | Routes target type |
| `external.system` | string | Yes (CREATE); Yes on UPDATE if the target event already has an external link | `[A-Z0-9_]{1,64}` | Uppercase trim | Stable link |
| `external.object_id` | string | Yes (CREATE); Yes on UPDATE if the target event already has an external link | 1–256 NFC trim | Preserved case | Stable link |
| `external.revision` | string | No for CREATE and for UPDATE of an unbound UI event (default `"1"`). **Required and must be new** on UPDATE of an event that already has an external link | 1–64; default `"1"` only when no existing link | Trim | Revision metadata only. JSON Schema cannot express the existing-link condition. |
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

### UPDATE compatibility (existing external link)

If the target DRAFT already has an `rfx_external_object_links` row, UPDATE Preview **must** send `external` with the same normalized `system`/`object_id` and a new `revision` that is not the current link revision and is not already stored in `rfx_external_object_link_revisions` for that link. Omitted `external` / omitted `revision` / replayed revision → **422 `VALIDATION_ERROR`** with `details.field=external.revision` and no new `machine_code`. Changing stable identity is rejected before analysis persist.

**Compatibility risk:** E3 UPDATE payloads that omit `external` remain valid only for events **without** a link (UI-created drafts). After E4 CREATE (default revision `"1"`), a later UPDATE without a new revision is rejected.

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
