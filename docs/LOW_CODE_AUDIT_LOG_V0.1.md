# Low-code Audit Log v0.1

## Summary

This pack adds read-only audit logging for **low-code custom field value updates** only.

When `PUT /v1/low-code/custom-field-values` succeeds, the low-code service writes an audit row to the existing `lowcode.configuration_audit_log` table (migration `000011`). A read-only API and web-admin page expose those events per tenant.

Core entity data changes are **not** audited here. No new migrations were created in this pack.

## Audit Write

On successful custom field value upsert:

- **Table:** `lowcode.configuration_audit_log`
- **DB action:** `UPDATE` (CHECK constraint on table)
- **API action:** `CUSTOM_FIELD_VALUES_UPDATED` (derived from `event_kind` in `new_value_json`)
- **Payload:**
  - `old_value_json`: `{ "values": { "<field_code>": ... } }`
  - `new_value_json`: `{ "event_kind", "form_template_id", "changed_fields", "values" }`
- **Actor:** `changed_by_user_id` from `X-User-ID` header (gateway JWT)
- **Correlation:** `request_id` from `X-Request-ID` when present
- **Transaction:** audit insert runs in the same DB transaction as value upsert; audit failure rolls back the PUT

## Audit API

```
GET /v1/low-code/audit-events
```

Gateway:

```
GET /api/v1/low-code/audit-events
```

Query parameters:

| Param | Required | Default | Max |
|-------|----------|---------|-----|
| `tenant_id` | via `X-Tenant-ID` header | — | — |
| `entity_type` | optional | — | — |
| `entity_id` | optional (UUID) | — | — |
| `action` | optional | — | — |
| `limit` | optional | 50 | 100 |

Response:

```json
{
  "items": [
    {
      "id": "...",
      "tenant_id": "...",
      "entity_type": "TRANSPORT_ORDER",
      "entity_id": "...",
      "action": "CUSTOM_FIELD_VALUES_UPDATED",
      "actor": "...",
      "changed_fields": ["internal_cost_center"],
      "old_values": {},
      "new_values": {},
      "created_at": "..."
    }
  ]
}
```

v0.1 returns only low-code custom field value audit events (filtered by `event_kind`).

## UI Page

Web-admin read-only pages:

- `/low-code/audit` — filterable audit table
- `/low-code/custom-field-values` — after Save, shows "View audit log" link and up to 5 recent events for the loaded entity

Navigation: Low-code hub dashboard link card.

No edit/delete for audit events.

## Security Guardrails

- Tenant filtering required on list API
- No cross-tenant event access
- `limit` capped at 100
- Invalid `entity_id` UUID rejected
- Secrets are not written into audit payloads (field values only)
- Read-only audit API; no mutation endpoints

## Tenant Isolation

All queries filter by `tenant_id` from the tenant header convention (`X-Tenant-ID`). Audit rows are inserted with the same tenant as the custom field value upsert.

## What Is Not Implemented Yet

- Audit for core entity CRUD (transport orders, shipments, billing registers)
- Audit delete/edit
- Form template create/edit audit UI
- Form Builder, BPMN, connectors, rule engine
- New migration (reused existing table)
- Playwright browser install in this pack

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

cd services\low-code-service
go test ./...

cd D:\Projects\freight-platform
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check
```

PUT then GET audit (replace token/entity as needed after seed):

```powershell
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb"
```

Frontend:

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run build
npm run dev
```

Regression:

```powershell
cd D:\Projects\freight-platform
make seed-dev-admin
make seed-demo-data
make integration-smoke-test
```

## Known Limitations

- DB stores `action = UPDATE`; API maps to `CUSTOM_FIELD_VALUES_UPDATED` via JSON payload
- Actor is user UUID from gateway header, not display name
- Only custom field value updates are audited in v0.1
- Audit list filters by low-code event kind; other configuration audit rows are excluded from this API

## Next Action

- Extend audit coverage to form template publish/archive when template editing UI lands
- Optional: resolve actor UUID to user display name in API or UI
- Optional: dedicated audit index page filters for date range
