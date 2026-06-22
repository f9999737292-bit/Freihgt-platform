# Low-code Runtime Inline Edit Guardrails v0.1

## Summary

Implemented runtime guardrails from `LOW_CODE_RUNTIME_INTEGRATION_POLICY_V0.1.md` for entity detail inline edit. Backend now rejects writes to **read-only** template fields (previously UI-only). **System fields** remain protected. No core service or API contract changes.

## Goal

Ensure low-code custom field saves on entity detail pages cannot bypass field-level protections, even if a client skips the UI filters.

## Guardrails Implemented

| Guardrail | Layer | Status |
|-----------|-------|--------|
| `system_field` write blocked | Backend + UI | Already enforced |
| `read_only` write blocked | Backend + UI | **Added in v0.1** |
| Tenant filtering on values API | Backend | Already enforced |
| DRAFT template blocked for values PUT | Backend | Already enforced |
| Active template for new values | Frontend | Already enforced |
| Audit on value writes | Backend | Already enforced |
| No core entity mutation from panel | Frontend architecture | By design |

## Backend Change

### Error code

```
READ_ONLY_FIELD_PROTECTED
HTTP 400
```

Message: `read-only field cannot be modified`

Details: `{ "field_code": "..." }`

### Validation

`domain.ValidateFieldValue()` rejects any write attempt to fields with `read_only = true`, including `null` clears.

`loadFieldDefinitions()` now loads `read_only` from `lowcode.form_fields` for published template context.

### Tests

- `TestValidateFieldValueReadOnlyFieldProtected` — domain
- `TestCustomFieldValuePutReadOnlyFieldProtected` — HTTP handler

## Frontend Change

`useLowCodeApi.getSaveErrorMessage()` maps `READ_ONLY_FIELD_PROTECTED` to i18n key `lowCode.errorReadOnlyFieldProtected`.

UI already excludes `read_only` fields from editable controls in `LowCodeCustomFieldsPanel`.

## Entity Detail Pages (unchanged integration)

| Page | Route | Low-code scope |
|------|-------|----------------|
| Transport order | `/transport-orders/[id]` | Custom field values only |
| Shipment | `/shipments/[id]` | Custom field values only |
| Billing register | `/billing-registers/[id]` | Custom field values only |
| Freight request | `/freight-requests/[id]` | Custom field values only |
| Document | `/documents/[id]` | Custom field values only |
| RFX | `/rfx/[id]` | Custom field values only |

Core status transitions and financial workflows remain on core APIs only.

## Verification Commands

```powershell
cd D:\Projects\freight-platform\services\low-code-service
go test ./internal/domain/... ./internal/http/handlers/...

cd D:\Projects\freight-platform
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check

cd apps/web-admin
npm run build
```

Manual check:

1. Open `/transport-orders/{id}` with demo entity
2. Edit custom fields — save succeeds for editable fields
3. Attempting to PUT a `read_only` field via API returns `READ_ONLY_FIELD_PROTECTED`

## What Is Not Implemented Yet

- Automatic migration on template publish (use explicit migrate API)
- Core BFF wiring for validation context headers (helper ready in shared-go)

## Next Action

See `docs/LOW_CODE_RUNTIME_NEXT_STEPS_V0.1.md`.

See also:

- `docs/LOW_CODE_RUNTIME_INTEGRATION_POLICY_V0.1.md`
- `docs/LOW_CODE_ENTITY_DETAIL_INLINE_EDIT_V0.1.md`
