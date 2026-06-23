# Low-code migration edge-case curl payloads

Demo tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f`

Resolve entity IDs:

```powershell
curl.exe -s -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/transport-orders?limit=100" | jq -r '.items[] | select(.order_number=="DEMO-TO-001" or .order_number=="DEMO-TO-002") | "\(.order_number) \(.id)"'
```

| File | Purpose | Demo coverage |
|------|---------|---------------|
| `safe_transport_order.json` | SAFE preview for DEMO-TO-001 | Manual curl |
| `warning_transport_order_allow_false.json` | Execute without warnings | Returns 409 if WARNING |
| `warning_transport_order_allow_true.json` | Execute with warnings | Manual when WARNING |
| `blocked_transport_order.json` | Blocked execute example | Unit tests only unless incompatible data seeded |
| `empty_values_transport_order.json` | Empty entity preview | Replace entity id with DEMO-TO-002 |
| `invalid_entity_id.json` | Validation error | Always 400 |

Preview:

```powershell
curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" --data-binary "@scripts/dev/payloads/migration-edge-cases/safe_transport_order.json" http://localhost:8080/api/v1/low-code/admin/custom-field-values/migration-preview
```

Execute:

```powershell
curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" --data-binary "@scripts/dev/payloads/migration-edge-cases/warning_transport_order_allow_true.json" http://localhost:8080/api/v1/low-code/admin/custom-field-values/migrate-to-active
```

Legacy / incompatible / missing-required edge cases are covered by Go unit and service tests (`go test ./...` in `services/low-code-service`).
