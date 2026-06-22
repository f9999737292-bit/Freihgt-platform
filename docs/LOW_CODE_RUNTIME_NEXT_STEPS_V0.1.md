# Low-code Runtime Next Steps v0.1

## Summary

Implements the three next actions from `LOW_CODE_RUNTIME_INTEGRATION_POLICY_V0.1.md`:

1. Automated runtime compliance test
2. Server-side `validation_context` pass-through via headers
3. Admin API to migrate custom field values to the active published template

No core service business logic changes. No database migrations. Public API contracts unchanged.

---

## 1. Runtime Compliance Test

**Script:** `tests/integration/lowcode-runtime-compliance-test.sh`  
**Make target:** `make lowcode-runtime-compliance-test`

### Checks

| Check | Description |
|-------|-------------|
| Static source | `LowCodeCustomFieldsPanel.vue` must not call core entity write APIs |
| Static source | Panel must use `saveCustomFieldValues` |
| Runtime | PUT custom field values for `DEMO-TO-001` |
| Runtime | Core transport order `status` unchanged after low-code save |
| Runtime | Custom field value persisted |

### Prerequisites

```powershell
make platform-up-no-build
make health-check
make seed-demo-data
make seed-lowcode-demo
```

### Run

```powershell
make lowcode-runtime-compliance-test
```

---

## 2. Validation Context Pass-Through

Core services and BFF callers can pass soft validation context via **HTTP headers** (additive; body still supported).

| Header | Purpose |
|--------|---------|
| `X-Low-Code-Entity-Status` | Entity status for conditional required / preview rules |
| `X-Low-Code-Role` | Role hint for conditional rules |

Body `validation_context` takes precedence when both are set for the same field.

### Shared helper (for core / BFF)

Package: `github.com/freight-platform/shared-go/lowcode`

```go
import sharedlowcode "github.com/freight-platform/shared-go/lowcode"

sharedlowcode.ApplyValidationContextHeaders(req.Header, sharedlowcode.ValidationContext{
    EntityStatus: order.Status,
    Role:         "SHIPPER_ADMIN",
})
```

### Example

```powershell
curl.exe -X PUT -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  -H "X-Low-Code-Entity-Status: DRAFT" `
  -H "X-Low-Code-Role: PLATFORM_ADMIN" `
  --data "{\"entity_type\":\"TRANSPORT_ORDER\",\"entity_id\":\"...\",\"form_template_id\":\"...\",\"values\":[...]}" `
  http://localhost:8080/api/v1/low-code/custom-field-values
```

Core services are **not required** to pass context in v0.1 — headers are optional for future server-side integration.

---

## 3. Migrate Custom Field Values to Active Template

When a newer published template version becomes active, existing values may reference **older field IDs**. Admin migration remaps values by stable `field_code` to the active template.

### Endpoint

```http
POST /v1/low-code/admin/custom-field-values/migrate-to-active
POST /api/v1/low-code/admin/custom-field-values/migrate-to-active
X-Tenant-ID: {tenant_id}
```

Request:

```json
{
  "entity_type": "TRANSPORT_ORDER",
  "entity_id": "2db04b49-665c-469f-bcb1-ffeb1274fedb",
  "code": "transport_order_default",
  "validation_context": {
    "entity_status": "READY_FOR_SOURCING",
    "role": "PLATFORM_ADMIN"
  }
}
```

Response:

```json
{
  "status": "ok",
  "active_template_id": "b1111111-1111-4111-8111-111111111102",
  "migrated_count": 3,
  "skipped_count": 0,
  "skipped_fields": []
}
```

### Behavior

1. Resolve **active** published template for `tenant_id + entity_type + code`
2. Load existing custom field values for entity
3. For each value:
   - **Migrate** if `field_code` exists on active template and field is not `system_field` / `read_only`
   - **Skip** if field removed from active template or protected
4. Replace rows by `field_code` (delete old `field_id` row, insert with active template `field_id`)
5. Write `CUSTOM_FIELD_VALUES_UPDATED` audit event

### Skipped fields

Fields not present on the active template, or marked `system_field` / `read_only`, remain unchanged and are listed in `skipped_fields`.

### No automatic migration

Publishing a new template version does **not** auto-migrate values. Call migrate explicitly when needed.

---

## Verification

```powershell
cd D:\Projects\freight-platform\services\low-code-service
go test ./...

cd D:\Projects\freight-platform\packages\shared-go
go test ./lowcode/...

make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check
make seed-lowcode-demo
make lowcode-runtime-compliance-test
```

---

## Next Action

1. Wire core BFF/services to use `shared-go/lowcode` headers when calling low-code from server-side
2. Optional admin UI button for migrate-to-active on entity detail pages
3. Batch migration script for all entities of an entity_type after template publish
