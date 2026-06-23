# Low-code Runtime Readiness Review v0.1

## Summary

Readiness review of the low-code runtime layer at commit `37a4a8d` (`feat: polish low-code batch migration admin ui`). All automated verification passed; runtime API smoke checks returned HTTP 200. The platform is **ready for controlled pilot integration** with transport orders, shipments, and billing registers on the **frontend sidecar pattern** (independent low-code API calls from entity detail pages). Gaps remain in **admin authorization**, **batch scale**, **frontend automated tests**, and **core-service validation_context wiring**.

**Recommended next pack:** **Low-code Permissions & Admin Guardrails Pack v0.1** — admin template/migration endpoints are tenant-scoped but not role-gated inside `low-code-service`.

## Current Commit

| Field | Value |
|-------|-------|
| Commit | `37a4a8d` |
| Message | `feat: polish low-code batch migration admin ui` |
| Branch | `main` (synced with `origin/main`) |
| Working tree | clean |

## Runtime Inventory

### Backend (`services/low-code-service`)

| Capability | Endpoint(s) | Notes |
|------------|-------------|-------|
| Form templates (public) | `GET /v1/low-code/form-templates`, `GET .../{id}` | Published templates |
| Active template resolution | `GET /v1/low-code/form-templates/active` | Latest `PUBLISHED` by `entity_type` + `code` |
| Custom field values | `GET/PUT /v1/low-code/custom-field-values` | Tenant-scoped upsert + audit |
| `validation_context` | PUT body + headers `X-Low-Code-Entity-Status`, `X-Low-Code-Role` | Soft validation only |
| Conditional required | Server-side in `ValidateConditionalRequiredFields` | Domain + service tests |
| Migration preview (admin) | `POST .../admin/custom-field-values/migration-preview` | Read-only |
| Migrate-to-active (admin) | `POST .../admin/custom-field-values/migrate-to-active` | Per-entity execute |
| Batch migration preview | `POST .../admin/custom-field-values/batch-migration-preview` | Max 100, dedupe |
| Batch migration execute | `POST .../admin/custom-field-values/batch-migrate-to-active` | Preview gate, 409 guards |
| Admin form templates | `POST/GET/PUT .../admin/form-templates`, publish, clone-to-draft | Draft lifecycle |
| Audit events | `GET /v1/low-code/audit-events` | Filters by entity/action |
| Metrics | `/metrics` | Service + batch migration counters |
| Structured logs | Access log + `batchmigration` package | No `value_json` in batch logs |

Router reference: `services/low-code-service/internal/http/router.go`.

### Frontend (`apps/web-admin`)

| Area | Path / component |
|------|------------------|
| Low-code hub | `/low-code` |
| Form templates (read) | `/low-code/form-templates` |
| Admin template editor | `/low-code/admin/form-templates/*` |
| Custom field values editor | `/low-code/custom-field-values` |
| Audit log | `/low-code/audit` |
| Custom values panel | `LowCodeCustomFieldsPanel.vue` |
| Preview renderer | `LowCodeFormTemplatePreview.vue` |
| Diff / publish review | `LowCodeFormTemplateDiff.vue` |
| Migration preview modal | `LowCodeMigrationPreviewModal.vue` |
| Batch migration wizard | `LowCodeBatchMigrationWizard.vue` |
| Audit cards | `LowCodeAuditEventCard.vue`, `LowCodeMigrationAuditCard.vue` |

**Entity detail integration (inline edit enabled):**

| Entity | Page |
|--------|------|
| TRANSPORT_ORDER | `/transport-orders/[id]` |
| SHIPMENT | `/shipments/[id]` |
| BILLING_REGISTER | `/billing-registers/[id]` |
| FREIGHT_REQUEST | `/freight-requests/[id]` |
| DOCUMENT | `/documents/[id]` |
| RFX | `/rfx/[id]` |

### Key documentation (`docs/LOW_CODE_*.md`)

49 documents. Primary references:

| Topic | Doc |
|-------|-----|
| Integration policy | `LOW_CODE_RUNTIME_INTEGRATION_POLICY_V0.1.md` |
| Active template policy | `LOW_CODE_FORM_TEMPLATE_VERSION_ACTIVATION_POLICY_V0.1.md` |
| Custom field values API | `LOW_CODE_CUSTOM_FIELD_VALUES_API_V0.1.md` |
| Entity detail integration | `LOW_CODE_ENTITY_DETAIL_INTEGRATION_V0.1.md` |
| Inline edit | `LOW_CODE_ENTITY_DETAIL_INLINE_EDIT_V0.1.md` |
| Conditional required | `LOW_CODE_CONDITIONAL_REQUIRED_VALIDATION_V0.1.md` |
| Migration design | `LOW_CODE_MIGRATE_TO_ACTIVE_DESIGN_V0.1.md` |
| Batch migration hardening runbook | `LOW_CODE_BATCH_MIGRATION_HARDENING_V0.1.md` |
| Batch admin UI polish | `LOW_CODE_ADMIN_BATCH_MIGRATION_POLISH_V0.1.md` |
| Runtime compliance test | `LOW_CODE_RUNTIME_NEXT_STEPS_V0.1.md` |
| Roadmap | `LOW_CODE_LAYER_ROADMAP_V0.1.md` |

## Verification Results

| Check | Result | When |
|-------|--------|------|
| `make health-check` | **OK** | 2026-06-24 |
| `make seed-dev-admin` | **OK** | 2026-06-24 |
| `make seed-demo-data` | **OK** | 2026-06-24 |
| `make seed-lowcode-demo` | **OK** | 2026-06-24 |
| `make integration-smoke-test` | **PASSED** | TEST-20260624010712 |
| `make lowcode-runtime-compliance-test` | **PASSED** | core status unchanged after low-code PUT |
| `go test ./...` (low-code-service) | **OK** | 27 test files, all packages pass |
| `npm run build` (web-admin) | **OK** | exit 0 |

## API Smoke Checks

Demo tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f`  
Demo entity: `2db04b49-665c-469f-bcb1-ffeb1274fedb` (DEMO-TO-001)

| Check | HTTP | Outcome |
|-------|------|---------|
| Active template `TRANSPORT_ORDER` / `transport_order_default` | 200 | 1 active PUBLISHED template, `is_active: true` |
| Custom field values GET | 200 | 3 fields (`cargo_class`, `internal_cost_center`, `loading_window_note`) |
| Migration preview POST | 200 | 1 entity, status `SAFE` |
| Batch migration preview POST | 200 | summary `total:1`, `safe:1` |
| Audit events GET | 200 | migration + value update events with `batch_id` metadata |

No destructive migration execute was run (preview-only, per review scope).

## Readiness Matrix

| # | Dimension | Rating | Evidence |
|---|-----------|--------|----------|
| 1 | Runtime API stability | **READY** | Smoke checks 200; go tests pass; smoke + compliance tests pass |
| 2 | Tenant isolation | **PARTIAL** | `X-Tenant-ID` enforced; tenant mismatch tests on admin/migration; no cross-tenant SQL in service design. Gateway auth assumed; **no service-level RBAC** on admin routes |
| 3 | Active template behavior | **READY** | `/form-templates/active` works; activation policy documented and tested |
| 4 | Draft/published lifecycle | **READY** | Admin CRUD, publish, clone-to-draft; DRAFT never on public runtime |
| 5 | Custom value editing | **READY** | GET/PUT API; inline edit on 6 entity detail pages + standalone editor |
| 6 | Validation safety | **PARTIAL** | Type + conditional required enforced; `validation_context` is **soft only** — must not gate financial/document workflows (documented policy) |
| 7 | Migration safety | **READY** | Preview-first; 409 BLOCKED/WARNINGS; edge-case tests; no auto-migrate on publish |
| 8 | Batch migration safety | **READY** | Dedupe, max 100, no writes before block decision, hardening runbook. **Limit:** sync only, no background job |
| 9 | Auditability | **PARTIAL** | Per-entity audit for values + migrations with `batch_id`; **batch-level completion event deferred** |
| 10 | Observability | **READY** | Prometheus metrics; batch logs without high-cardinality labels; bounded label tests |
| 11 | Admin UX | **READY** | Migration modal, batch wizard (polished), audit UI, template admin |
| 12 | i18n RU/EN/ZH | **READY** | Batch + migration + audit keys in `en-US`, `ru-RU`, `zh-CN` |
| 13 | Docs completeness | **READY** | 49 LOW_CODE docs; policy, API, runbook, edge cases covered |
| 14 | Test coverage | **PARTIAL** | Strong Go unit/handler coverage (27 files); integration smoke + compliance script; **no frontend unit/E2E tests** |
| 15 | Operational runbook | **PARTIAL** | Batch hardening runbook exists; broader runtime ops spread across multiple docs — no single ops playbook |

### Summary counts

- **READY:** 10
- **PARTIAL:** 5
- **NOT_READY:** 0

## Gaps

### Must fix before core runtime integration (multi-role pilot)

| Gap | Status | Notes |
|-----|--------|-------|
| No low-code admin RBAC in service | Open | Any gateway-authenticated caller with tenant header can hit `/admin/*` migration and template APIs if routed |
| `validation_context` not trusted for financial gates | Policy only | Documented; core services must not use low-code validation for UPD/billing approval |

### Should fix before pilot

| Gap | Status | Notes |
|-----|--------|-------|
| Batch >100 entities | Open | No async/background batch job; admin must chunk manually |
| Batch-level audit event | Deferred | Per-entity `CUSTOM_FIELD_VALUES_MIGRATED_TO_ACTIVE` only |
| Core services do not pass `validation_context` | Open | Header helper in `shared-go/lowcode` exists; core BFF not wired |
| No frontend automated tests | Open | Manual + build only |
| No migration rollback UI | Open | Re-migrate or manual ops; documented in hardening runbook |

### Can defer

| Gap | Notes |
|-----|-------|
| Template JSON import/export | No API/UI found |
| Auto-migrate on template publish | By design — explicit migration packs only |
| RFx lot / bid entity types | Not in demo seed scope |
| FILE / reference field editors | Complex types use JSON textarea |
| Live Docker E2E suite beyond smoke | Smoke + compliance sufficient for v0.1 |

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Admin migration API without role check | Medium | Next: Permissions & Admin Guardrails pack |
| Operator runs batch migrate without preview | Low | UI wizard enforces preview; API allows direct execute — document ops discipline |
| Stale `form_template_id` on values after new publish | Low | Migration preview/execute; documented in activation policy |
| Trusting client `validation_context.role` | Medium | Policy forbids financial use; enforce in core services |
| Duplicate batch audit rows on re-execute | Low | Idempotent values; audit duplicates acceptable v0.1 |

## Recommended Next Pack

**Low-code Permissions & Admin Guardrails Pack v0.1** (Option B)

### Why not the others?

| Option | Assessment |
|--------|------------|
| **A. Runtime Entity Integration** | **Largely delivered** — `LowCodeCustomFieldsPanel` with `editable` on TO/shipment/billing and 3 other entity types. Remaining work (create-flow seeding, core BFF `validation_context`) fits a follow-up **Entity Integration v0.2** |
| **B. Permissions & Admin Guardrails** | **Recommended** — verified gap: no RBAC in `low-code-service`; required before multi-role pilot |
| **C. Template Import/Export** | Not implemented; useful for ops but not blocking runtime integration |
| **D. Runtime Readiness Fix Pack** | **Not needed** — no blocking defects found in verification |

### Permissions pack scope (suggested)

- Role gate admin template CRUD / publish / clone
- Role gate migration preview + execute + batch endpoints
- UI hide/disable admin actions for non-admin roles
- Tests for forbidden responses (403) without changing public runtime APIs

## What Is Ready

- Public runtime APIs: active template, custom values GET/PUT, audit read
- Entity detail display + inline edit on transport order, shipment, billing register (+ FR/document/RFX)
- Admin template draft/publish/clone workflow
- Single-entity and batch migration preview/execute with guardrails
- Batch audit metadata, metrics, structured logs
- Tenant header enforcement and tenant-mismatch tests on sensitive admin paths
- Runtime compliance test (core entity status unchanged after low-code save)
- RU/EN/ZH admin strings for migration/batch flows

## What Is Not Ready

- Service-level authorization matrix for admin low-code operations
- Async batch migration for >100 entities
- Batch-level single audit completion event
- Automated frontend test suite
- Template import/export between environments
- Core backend services automatically forwarding `validation_context` on low-code saves

## Operational Notes

- **Seeds required:** `make seed-demo-data` then `make seed-lowcode-demo` for demo custom fields
- **Demo login:** tenant `74519f22-ff9b-4a8b-8fff-a958c689682f`, `admin@7rights.local`
- **Batch migration:** preview first; see `docs/LOW_CODE_BATCH_MIGRATION_HARDENING_V0.1.md`
- **Do not** use `validation_context` for billing approval or UPD gates
- **Do not** manually edit DB for migration rollback
- **Compliance regression:** `make lowcode-runtime-compliance-test`

## Verification Commands

```powershell
cd D:\Projects\freight-platform
git status --short
make health-check
make seed-dev-admin
make seed-demo-data
make seed-lowcode-demo
make integration-smoke-test
make lowcode-runtime-compliance-test

cd services\low-code-service
go test ./...

cd D:\Projects\freight-platform\apps\web-admin
npm run build

# API smoke (preview only)
curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"

curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb"

curl.exe -X POST -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_migration_preview_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/migration-preview

curl.exe -X POST -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migration-preview

curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&limit=10"
```

## Next Action

**Low-code Permissions & Admin Guardrails Pack v0.1**
