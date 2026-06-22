# Low-code Custom Field Values API v0.1

## Summary

Read/write API in **low-code-service** for custom field values stored in `lowcode.custom_field_values`. Values are scoped by tenant and linked to published form templates. No integration with core domain services in v0.1.

## Endpoints

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/v1/low-code/custom-field-values` | List values for an entity |
| PUT | `/v1/low-code/custom-field-values` | Upsert values (idempotent) |

## Tenant Header

```http
X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f
```

Fallback: query param `tenant_id`.

## GET Values

Query params (required):

- `entity_type`
- `entity_id` (UUID)

Example response:

```json
{
  "tenant_id": "74519f22-ff9b-4a8b-8fff-a958c689682f",
  "entity_type": "TRANSPORT_ORDER",
  "entity_id": "...",
  "items": [
    {
      "field_id": "...",
      "field_code": "cargo_class",
      "value_json": "GENERAL",
      "updated_at": "2026-06-22T12:00:00Z"
    }
  ]
}
```

## PUT Values

Request body:

```json
{
  "entity_type": "TRANSPORT_ORDER",
  "entity_id": "...",
  "form_template_id": "...",
  "validation_context": {
    "entity_status": "READY_FOR_SOURCING",
    "role": "PLATFORM_ADMIN"
  },
  "values": [
    { "field_code": "cargo_class", "value_json": "GENERAL" },
    { "field_code": "internal_cost_center", "value_json": "CC-1001" }
  ]
}
```

Optional `validation_context` enables conditional required rules that reference `context.entity_status` or `context.role`. Field-based conditional rules work without it.

Response:

```json
{
  "status": "ok",
  "tenant_id": "...",
  "entity_type": "TRANSPORT_ORDER",
  "entity_id": "...",
  "saved_count": 2
}
```

Uses `ON CONFLICT (tenant_id, entity_type, entity_id, field_id)` — repeated PUT is idempotent.

## Validation Rules

Field-type validation (v0.1):

| Type | Rule |
| ---- | ---- |
| TEXT | JSON string |
| NUMBER | JSON number |
| DATE | `YYYY-MM-DD` |
| DATETIME | RFC3339 string |
| SELECT | string in `options_json` (if options defined) |
| MULTI_SELECT | array of strings in options |
| CHECKBOX | boolean |
| MONEY | `{ "amount": number, "currency": string }` |
| CURRENCY | non-empty string |
| FILE | object or null |
| COMPANY_REFERENCE / DOCUMENT_REFERENCE | UUID string (no FK check) |
| ROUTE / ADDRESS / VEHICLE / VAT_TAX | object |

Simple `validation_rule_json`: `minLength`, `maxLength`, `min`, `max`.

Conditional required (v0.1): `{ "if": { ... }, "then": { "required": [...] } }` — evaluated against merged existing + incoming values. See `docs/LOW_CODE_CONDITIONAL_REQUIRED_VALIDATION_V0.1.md`.

Protected:

- `system_field=true` → `SYSTEM_FIELD_PROTECTED`
- Draft/archived templates → `FORM_TEMPLATE_NOT_PUBLISHED`
- Unknown field → `FIELD_NOT_FOUND`

## Demo Seed Values

`make seed-lowcode-demo` (after `make seed-demo-data`) inserts dev-only values via **psql**:

| Entity | Demo ref | Sample fields |
| ------ | -------- | ------------- |
| TRANSPORT_ORDER | DEMO-TO-001 | cargo_class=GENERAL, internal_cost_center=CC-1001 |
| SHIPMENT | DEMO-SH-PLANNED | temperature_mode=AMBIENT |
| BILLING_REGISTER | DEMO-BR-001 | payment_priority=NORMAL |
| FREIGHT_REQUEST | DEMO-FR-001 | lane_priority=HIGH |
| DOCUMENT | DEMO-DOC-001 | archive_reference=ARC-2026-001 |
| RFX | DEMO-RFX-001 | confidentiality_level=INTERNAL |

Idempotent — 3 runs safe.

## Direct Service URL

```powershell
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8088/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=<UUID>"

curl -X PUT -H "Content-Type: application/json" -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" --data-binary "@payload.json" http://localhost:8088/v1/low-code/custom-field-values
```

## Gateway URL

```powershell
curl -H "X-Tenant-ID: ..." "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=<UUID>"
```

## Security Guardrails

- All queries filter `tenant_id`
- Template must belong to tenant and be `PUBLISHED`
- `entity_type` must match template
- No writes to core entity tables
- No cross-tenant reads/writes

## What Is Not Implemented Yet

- Core entity existence validation
- Integration with transport/rfx/shipment/document/billing services
- File upload for FILE fields
- Delete endpoint (use `null` value_json to clear)
- Full Rule Engine (visibility on save, cross-field `then.visible`)
- RLS

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check
make seed-demo-data
make seed-lowcode-demo
go test ./...   # from services/low-code-service
make integration-smoke-test
```

## Next Action

1. Wire custom fields into entity detail UI (read-only display).
2. Optional: validate entity exists via gateway/core (future pack).
3. Custom field values batch read API.

See also: `docs/LOW_CODE_FORM_TEMPLATES_API_V0.1.md`.
