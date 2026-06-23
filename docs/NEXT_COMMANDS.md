# Daily Commands

## Project root

```powershell
cd D:\Projects\freight-platform
```

## Check current state

```powershell
git status --short
git log --oneline -5
```

## Start backend

```powershell
make platform-up-no-build
make health-check
```

## Check bash (Windows)

```powershell
make bash-check
```

On Windows, Makefile uses Git Bash for `.sh` scripts (not WSL `bash` from PATH). See `docs/WINDOWS_MAKE_BASH.md`.

## Seed dev data

```powershell
make seed-dev-admin
make seed-demo-data
make seed-lowcode-demo
```

Custom field values API (after seed-demo-data + seed-lowcode-demo):

```powershell
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8088/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=<ENTITY_ID>"
```

See `docs/LOW_CODE_CUSTOM_FIELD_VALUES_API_V0.1.md`.

Low-code admin UI (read-only preview):

```text
http://localhost:3000/low-code
http://localhost:3000/low-code/form-templates
http://localhost:3000/low-code/custom-field-values
```

See `docs/LOW_CODE_ADMIN_UI_PREVIEW_V0.1.md`.

Low-code custom field values edit UI:

```text
http://localhost:3000/low-code/custom-field-values
```

See `docs/LOW_CODE_CUSTOM_FIELD_VALUES_EDIT_UI_V0.1.md`.

Low-code audit log:

```text
http://localhost:3000/low-code/audit
```

Audit API:

```powershell
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&entity_id=<ENTITY_ID>"
```

See `docs/LOW_CODE_AUDIT_LOG_V0.1.md`.

Low-code form template draft API:

```powershell
make create-lowcode-draft-template
curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/admin/form-templates?status=DRAFT"
```

See `docs/LOW_CODE_FORM_TEMPLATE_DRAFT_API_V0.1.md`.

Low-code form template admin UI:

```text
http://localhost:3000/low-code/admin/form-templates
http://localhost:3000/low-code/admin/form-templates/new
```

See `docs/LOW_CODE_FORM_TEMPLATE_ADMIN_UI_V0.1.md`.

Form template preview:

```text
http://localhost:3000/low-code/form-templates/{id}  (Preview tab)
http://localhost:3000/low-code/custom-field-values   (values preview)
```

See `docs/LOW_CODE_FORM_TEMPLATE_PREVIEW_RENDERER_V0.1.md`.

Entity detail custom fields + preview:

```text
http://localhost:3000/transport-orders/{id}
http://localhost:3000/shipments/{id}
http://localhost:3000/billing-registers/{id}
```

See `docs/LOW_CODE_ENTITY_DETAIL_PREVIEW_V0.1.md`.

RFx / document / freight request detail custom fields:

```text
http://localhost:3000/freight-requests/{id}
http://localhost:3000/documents/{id}
http://localhost:3000/rfx/{id}
```

See `docs/LOW_CODE_ENTITY_DETAIL_RFX_DOCUMENT_V0.1.md`.

Preview visibility rules:

```text
http://localhost:3000/transport-orders/{id}   (loading_window_note when cargo_class=GENERAL)
http://localhost:3000/shipments/{id}         (driver_comment when cold chain)
```

See `docs/LOW_CODE_PREVIEW_VISIBILITY_RULES_V0.1.md`.

Preview context (entity status + role):

```text
http://localhost:3000/transport-orders/{id}
http://localhost:3000/billing-registers/{id}
```

See `docs/LOW_CODE_PREVIEW_CONTEXT_V0.1.md`.

Conditional required in preview:

```text
http://localhost:3000/low-code/custom-field-values  (change cargo_class to A)
```

See `docs/LOW_CODE_PREVIEW_CONDITIONAL_REQUIRED_V0.1.md`.

Custom field values preview status:

```text
http://localhost:3000/low-code/custom-field-values
```

See `docs/LOW_CODE_CUSTOM_FIELD_VALUES_PREVIEW_STATUS_V0.1.md`.

Entity detail inline edit:

```text
http://localhost:3000/transport-orders/{id}
http://localhost:3000/shipments/{id}
```

See `docs/LOW_CODE_ENTITY_DETAIL_INLINE_EDIT_V0.1.md`.

Conditional required validation (server):

See `docs/LOW_CODE_CONDITIONAL_REQUIRED_VALIDATION_V0.1.md`.

Create-first-value edit (empty demo DEMO-TO-002):

```text
http://localhost:3000/transport-orders/{id}
http://localhost:3000/low-code/custom-field-values
```

See `docs/LOW_CODE_CREATE_FIRST_VALUE_EDIT_V0.1.md`.

Rich field editors (DATE, MONEY, MULTI_SELECT):

```text
http://localhost:3000/shipments/{id}   (DEMO-SH-PLANNED)
```

See `docs/LOW_CODE_RICH_FIELD_EDITORS_V0.1.md`.

Clone published template to draft:

```text
http://localhost:3000/low-code/admin/form-templates
```

See `docs/LOW_CODE_CLONE_PUBLISHED_TEMPLATE_TO_DRAFT_V0.1.md`.

Form builder UX (palette, presets, validation, live preview):

```text
http://localhost:3000/low-code/admin/form-templates/new
http://localhost:3000/low-code/admin/form-templates/{id}
```

See `docs/LOW_CODE_FORM_BUILDER_UX_V0.1.md`.

Form template version compare (draft vs published):

```text
http://localhost:3000/low-code/admin/form-templates/{draft-id}
```

See `docs/LOW_CODE_FORM_TEMPLATE_VERSION_COMPARE_V0.1.md`.

Form template version activation (active published selection):

```powershell
curl -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER"
```

```text
http://localhost:3000/low-code/form-templates
http://localhost:3000/low-code/admin/form-templates
```

See `docs/LOW_CODE_FORM_TEMPLATE_VERSION_ACTIVATION_POLICY_V0.1.md`.

## Low-code Runtime Integration

* Runtime policy: `docs/LOW_CODE_RUNTIME_INTEGRATION_POLICY_V0.1.md`
* Prerequisite (implemented): Low-code Form Template Version Activation Policy v0.1 — active template endpoint + UI badges
* Inline edit guardrails: `docs/LOW_CODE_RUNTIME_INLINE_EDIT_GUARDRAILS_V0.1.md`
* Runtime next steps (compliance test, validation headers, migrate): `docs/LOW_CODE_RUNTIME_NEXT_STEPS_V0.1.md`

## Low-code Runtime Headers

* Contract: `docs/LOW_CODE_RUNTIME_HEADERS_CONTRACT_V0.1.md`
* Verify tenant-required behavior
* Verify audit request_id behavior

## Low-code Migrate-to-Active

* Design: `docs/LOW_CODE_MIGRATE_TO_ACTIVE_DESIGN_V0.1.md`
* Preview API: `docs/LOW_CODE_MIGRATE_TO_ACTIVE_PREVIEW_API_V0.1.md`
* Execute API: `docs/LOW_CODE_MIGRATE_TO_ACTIVE_EXECUTE_V0.1.md`

Preview migration (read-only):

```powershell
curl.exe -X POST `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_migration_preview_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/migration-preview
```

Execute migration (entity-level):

```powershell
curl.exe -X POST `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_migrate_to_active_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/migrate-to-active
```

Admin UI migration preview modal:

```text
http://localhost:3000/low-code/custom-field-values
```

See `docs/LOW_CODE_ADMIN_MIGRATION_PREVIEW_MODAL_V0.1.md`.

Admin UI migration history & audit:

```text
http://localhost:3000/low-code/audit?category=migrations
http://localhost:3000/low-code/custom-field-values
```

See `docs/LOW_CODE_MIGRATION_HISTORY_AUDIT_UI_V0.1.md`.

Migration edge-case tests:

```powershell
cd services\low-code-service
go test ./...
```

Payloads: `scripts/dev/payloads/migration-edge-cases/`

See `docs/LOW_CODE_MIGRATION_EDGE_CASES_TEST_PACK_V0.1.md`.

Low-code batch migration:

* Design: `docs/LOW_CODE_BATCH_MIGRATION_DESIGN_V0.1.md`
* Batch preview API: `docs/LOW_CODE_BATCH_MIGRATION_PREVIEW_API_V0.1.md`
* Batch execute API: `docs/LOW_CODE_BATCH_MIGRATION_EXECUTE_API_V0.1.md`

Batch migration preview (read-only):

```powershell
curl.exe -X POST `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migration-preview
```

Batch migration execute:

```powershell
curl.exe -X POST `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_batch_migrate_to_active_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migrate-to-active
```

See `docs/LOW_CODE_BATCH_MIGRATION_EXECUTE_API_V0.1.md`.

Admin UI batch migration wizard:

```text
http://localhost:3000/low-code/custom-field-values
```

See `docs/LOW_CODE_ADMIN_BATCH_MIGRATION_WIZARD_V0.1.md`.

Batch migration audit & metrics:

See `docs/LOW_CODE_BATCH_MIGRATION_AUDIT_METRICS_V0.1.md`.

Batch migration edge-case tests:

See `docs/LOW_CODE_BATCH_MIGRATION_EDGE_CASES_TEST_PACK_V0.1.md`.

Batch migration hardening (dedup, guardrails, runbook):

See `docs/LOW_CODE_BATCH_MIGRATION_HARDENING_V0.1.md`.

Batch migration admin UI polish:

See `docs/LOW_CODE_ADMIN_BATCH_MIGRATION_POLISH_V0.1.md`.

Next implementation:

1. Low-code Runtime Readiness Review Pack v0.1

Drag-and-drop form builder (section/field reorder):

```text
http://localhost:3000/low-code/admin/form-templates/new
http://localhost:3000/low-code/admin/form-templates/{draft-id}
```

See `docs/LOW_CODE_DRAG_AND_DROP_FORM_BUILDER_V0.1.md`.

If a target fails with WSL/bash errors, override:

```powershell
make BASH="C:/Program Files/Git/bin/bash.exe" seed-dev-admin
```

## Run smoke test

```powershell
make integration-smoke-test
```

## Start frontend

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run dev
```

Open:

```text
http://localhost:3000/login
```

Login:

```text
Tenant ID: 74519f22-ff9b-4a8b-8fff-a958c689682f
Email: admin@7rights.local
Password: Admin123456!
```

## Commit

```powershell
cd D:\Projects\freight-platform
git status --short
git add .
git commit -m "..."
git push origin main
```

## Last commits

```powershell
git log --oneline -5
```
