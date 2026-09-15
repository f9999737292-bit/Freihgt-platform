# ADR-RFX-012: ERP Canonical JSON Contract

**Status:** Accepted (architecture freeze — pending controller review)  
**Date:** 2026-09-16  
**Deciders:** E7 Phase 2 ERP architecture stream  
**Normative detail:** [RFX_V3_0E7_ERP_API.md](../implementation/RFX_V3_0E7_ERP_API.md) §6

---

## Context

Buyer ERP systems (SAP, 1C, Oracle, Dynamics, TMS) need a **vendor-neutral JSON contract** to create and update RFx DRAFT events. Excel import (`BINTRANS_RFX_BUYER_XLSX_V1`) proves the Preview/Commit pipeline but is not suitable for machine integration.

Requirements:

- Strict schema with no arbitrary mass assignment
- Deterministic canonicalization for SHA-256 hashing (reuse XLSX hash pipeline pattern)
- External identity for idempotent ERP retries
- Mapping of external reference codes before commit

Repository evidence:

- XLSX canonical struct: `services/rfx-service/internal/xlsxexchange/buyer_import_hash.go`
- Schema versioning pattern: `domain/excel_exchange.go` (`SchemaVersionBuyerXLSXV1`)
- DB constraint on schema versions: migration `000073` (requires 000074 extension)

---

## Decision

Adopt schema name **`BINTRANS_RFX_ERP_JSON_V1`** as the canonical ERP ingress contract.

### Schema identity

| Field | Value |
|---|---|
| `schema_name` | `BINTRANS_RFX_ERP_JSON_V1` |
| `schema_version` | `1` |
| Content-Type | `application/json` |
| Max body size | 2 MiB (proposed; align with gateway `MaxBodySize`) |

### Top-level structure (allowlist)

| JSON path | Type | Required | Validation | Canonicalization | Domain mapping |
|---|---|---|---|---|---|
| `schema_version` | string | Yes | Must equal `BINTRANS_RFX_ERP_JSON_V1` | Literal | — |
| `requested_operation` | enum | Yes | `CREATE_DRAFT` \| `UPDATE_DRAFT` | Uppercase | Routes preview target type |
| `external.system` | string | Yes (CREATE) | 1–64 chars, `[A-Z0-9_]+` | Trim, uppercase | `external_system` in links |
| `external.object_id` | string | Yes (CREATE) | 1–256 chars | Trim | `external_object_id` |
| `external.object_version` | string | No | 1–64 chars; default `"1"` | Trim | `external_version` |
| `event.type` | enum | Yes | Platform RFx type codes | Enum normalize | `rfx_events.type` |
| `event.title` | string | Yes | 1–500 chars | NFC trim | `rfx_events.title` |
| `event.description` | string | No | Max 8000 | NFC trim | `rfx_events.description` |
| `event.currency` | string | Yes | ISO 4217 via mapping | Uppercase | Lot/event currency |
| `event.timezone` | string | Yes | IANA via mapping | Canonical TZ | Deadline display |
| `event.deadline` | datetime | No | RFC3339 UTC | UTC normalize | `response_deadline` |
| `lots[]` | array | No | Max 200 lots | Sort by `lot_number` | `rfx_lots` |
| `lots[].lot_number` | string | Yes | Unique per payload | Trim | `lot_number` |
| `lots[].name` | string | Yes | 1–255 | NFC trim | `name` |
| `lots[].description` | string | No | Max 2000 | NFC trim | `description` |
| `lots[].category` | string | No | Mapping | Trim | `category` |
| `lots[].estimated_value` | number | No | ≥ 0 | 2 decimal round | `estimated_value` |
| `lots[].currency_code` | string | No | ISO 4217 via mapping | Uppercase | `currency_code` |
| `questionnaire.sections[]` | array | No | Stable `section_code` unique | Sort by `sort_order`, code | `rfx_sections` |
| `questionnaire.questions[]` | array | No | Stable codes unique | Sort by section, order | `rfx_questions` |
| `questionnaire.options[]` | array | No | Per-question unique codes | Sort | `rfx_question_options` |
| `questionnaire.rules[]` | array | No | DAG validated | Sort by `rule_code` | `rfx_question_rules` |
| `template_reference.template_id` | uuid | No | Must exist if set | — | Clone provenance optional |
| `template_reference.version_id` | uuid | No | Published version | — | `source_template_version_id` |
| `invited_carriers[]` | array | No | Policy-gated | Sort by external code | Participants (future gate) |
| `extensions` | object | No | Namespaced keys only | Sorted keys | Non-authoritative |

### Canonicalization rules

1. JSON marshaled with sorted object keys (same approach as `marshalCanonical` in XLSX package).
2. `validation_json` / `condition_json` normalized via JSON round-trip (`normalizeJSONBytes`).
3. Arrays sorted deterministically before hash (sections, questions, lots, rules).
4. Empty strings → omitted optional fields in stored canonical form.
5. SHA-256 over JSONB-stable bytes (`StableStoredPayload` pattern).

### Mass assignment prohibition

- Reject unknown top-level keys → 422 `unknown_field`.
- Reject unknown keys under `event`, `external`, `questionnaire` → 422.
- `extensions` only accepts keys matching `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`.

---

## Alternatives considered

### A. Reuse XLSX schema internally without new name

**Rejected.** ERP JSON includes metadata (external IDs, event fields) not present in XLSX workbook sheets; conflating schemas breaks DB constraints and audit clarity.

### B. OpenAPI `additionalProperties: true`

**Rejected.** Enables mass assignment and unpredictable canonical hashes.

### C. Per-vendor schema versions (SAP_JSON_V1, etc.)

**Rejected.** Violates vendor-neutral core API; adapters map to single canonical schema.

---

## Consequences

### Positive

- Single contract for all ERP adapters
- Reuses proven hash/immutability pipeline
- Clear migration path from XLSX canonical lots/questionnaire shapes

### Negative / cost

- Migration 000074 required to add `BINTRANS_RFX_ERP_JSON_V1` to DB check constraint
- New parser/validator module in rfx-service
- Reference mapping layer must run before analysis persistence

---

## Security

- Payload depth limit: 32 levels
- Array length caps enforced at validation
- No executable content fields
- Extensions cannot override authoritative fields

---

## References

- [ADR-RFX-014](./ADR-RFX-014-ERP-PREVIEW-COMMIT-IDEMPOTENCY.md)
- [ADR-RFX-015](./ADR-RFX-015-ERP-EXTERNAL-IDS-REFERENCE-MAPPING.md)
- `infrastructure/migrations/000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql`
