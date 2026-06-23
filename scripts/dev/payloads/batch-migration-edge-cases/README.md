# Batch migration edge-case curl payloads

Demo tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f`

Demo entity (TRANSPORT_ORDER): `2db04b49-665c-469f-bcb1-ffeb1274fedb`

| File | Purpose | Demo curl | Notes |
|------|---------|-----------|-------|
| `safe_batch_transport_order.json` | SAFE batch preview/execute | Works after `make seed-lowcode-demo` | Single SAFE entity |
| `warning_batch_allow_false_transport_order.json` | Execute without warnings | May return 409 if entity has legacy fields | WARNING policy |
| `warning_batch_allow_true_transport_order.json` | Execute with warnings | Manual when WARNING data exists | |
| `blocked_batch_skip_false_transport_order.json` | Whole batch blocked | 409 if any BLOCKED in batch | `skip_blocked=false` |
| `blocked_batch_skip_true_transport_order.json` | Skip blocked entities | Partial success when mixed | Default wizard behavior |
| `mixed_batch_transport_order.json` | SAFE + WARNING + BLOCKED example | **Unit tests only** | Placeholder entity IDs; use Go tests for real mixed fixtures |
| `invalid_batch_too_many_entities.json` | Validation error | Always 400 | 101 entity IDs |
| `invalid_batch_bad_uuid.json` | Validation error | Always 400 | Invalid UUID |

## Preview

```powershell
curl.exe -X POST `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/batch-migration-edge-cases/safe_batch_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migration-preview
```

## Execute

```powershell
curl.exe -X POST `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/batch-migration-edge-cases/safe_batch_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migrate-to-active
```

Mixed SAFE/WARNING/BLOCKED, tenant isolation, idempotency, and audit batch metadata are covered by Go unit/service/handler tests:

```powershell
cd services\low-code-service
go test ./...
```

See `docs/LOW_CODE_BATCH_MIGRATION_EDGE_CASES_TEST_PACK_V0.1.md`.
