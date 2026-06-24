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

## Accelerated AI Team Workflow

Virtual team role playbooks for faster Cursor-driven development:

```text
docs/ai-team/
```

| Resource | Purpose |
|----------|---------|
| [README.md](./ai-team/README.md) | How to use roles |
| [ACCELERATED_WORKFLOW.md](./ai-team/ACCELERATED_WORKFLOW.md) | Fast / Safe / Sprint tracks |
| [CURSOR_TASK_TEMPLATE.md](./ai-team/CURSOR_TASK_TEMPLATE.md) | Paste into Cursor chat |
| [DAILY_CHECKLIST.md](./ai-team/DAILY_CHECKLIST.md) | Morning / pack / commit routine |

**Working format:**

1. **PM** — plan scope, acceptance criteria, final report
2. **Backend / Frontend** — implement (owner role)
3. **QA** — health-check, tests, smoke, curl
4. **Security** — auth, tenant, import/export (when relevant)
5. **DevOps** — Docker, env flags, safe restart (when relevant)
6. **Docs** — pack doc + update this file

**Next pack (use AI team):** Low-code Pilot Week-3 Operator Feedback Scheduling Follow-up Pack v0.1.

Week-3 first real operator feedback capture:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_REAL_OPERATOR_FEEDBACK_CAPTURE_V0.1.md`.

Week-3 first real operator feedback summary:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_REAL_OPERATOR_FEEDBACK_SUMMARY_V0.1.md`.

Week-3 first real operator feedback PM action note:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_REAL_OPERATOR_FEEDBACK_PM_ACTION_NOTE_V0.1.md`.

Week-3 operator feedback PM escalation:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_PM_ESCALATION_V0.1.md`.

Week-3 operator feedback PM decision note:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_PM_DECISION_NOTE_V0.1.md`.

Week-3 operator feedback session schedule template:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_SESSION_SCHEDULE_TEMPLATE_V0.1.md`.

Week-3 first operator feedback session retry:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_RETRY_V0.1.md`.

Week-3 operator feedback scheduling note:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_SCHEDULING_NOTE_V0.1.md`.

Week-3 first operator feedback session:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_V0.1.md`.

Week-3 first operator feedback session notes template:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_NOTES_TEMPLATE_V0.1.md`.

Week-3 operator feedback evidence:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_EVIDENCE_V0.1.md`.

Week-3 first operator feedback action plan:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_ACTION_PLAN_V0.1.md`.

Week-3 operator feedback no-submissions report:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_NO_SUBMISSIONS_REPORT_V0.1.md`.

Week-3 feedback triage and backlog:

See `docs/LOW_CODE_PILOT_WEEK3_FEEDBACK_TRIAGE_AND_BACKLOG_V0.1.md`.

Week-3 improvements backlog:

See `docs/LOW_CODE_PILOT_WEEK3_IMPROVEMENTS_BACKLOG_V0.1.md`.

Week-3 feedback triage daily report template:

See `docs/LOW_CODE_PILOT_WEEK3_FEEDBACK_TRIAGE_DAILY_REPORT_TEMPLATE_V0.1.md`.

Week-3 operator feedback collection:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_COLLECTION_V0.1.md`.

Week-3 operator feedback form template:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md`.

Week-3 operator feedback log:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_LOG_V0.1.md`.

Week-3 operator feedback triage runbook:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_TRIAGE_RUNBOOK_V0.1.md`.

Week-3 auth-on staging verification:

See `docs/LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_VERIFICATION_V0.1.md`.

Week-3 auth-on staging runbook:

See `docs/LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_RUNBOOK_V0.1.md`.

Week-3 monitoring evidence:

See `docs/LOW_CODE_PILOT_WEEK3_MONITORING_EVIDENCE_V0.1.md`.

Week-3 monitoring baseline report:

See `docs/LOW_CODE_PILOT_WEEK3_MONITORING_BASELINE_REPORT_V0.1.md`.

Week-3 monitoring runbook:

See `docs/LOW_CODE_PILOT_WEEK3_MONITORING_RUNBOOK_V0.1.md`.

Week-2 closure:

See `docs/LOW_CODE_PILOT_WEEK2_CLOSURE_V0.1.md`.

Week-2 closure PM decision note:

See `docs/LOW_CODE_PILOT_WEEK2_CLOSURE_PM_DECISION_NOTE_V0.1.md`.

Week-3 execution plan:

See `docs/LOW_CODE_PILOT_WEEK3_EXECUTION_PLAN_V0.1.md`.

Week-2 cross-entity readiness review:

See `docs/LOW_CODE_PILOT_WEEK2_CROSS_ENTITY_READINESS_REVIEW_V0.1.md`.

Week-2 cross-entity decision note:

See `docs/LOW_CODE_PILOT_WEEK2_CROSS_ENTITY_DECISION_NOTE_V0.1.md`.

Week-3 candidate plan:

See `docs/LOW_CODE_PILOT_WEEK3_CANDIDATE_PLAN_V0.1.md`.

BILLING_REGISTER write monitoring:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_V0.1.md`.

BILLING_REGISTER write monitoring daily report template:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_REPORT_TEMPLATE_V0.1.md`.

BILLING_REGISTER limited write enablement:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_ENABLEMENT_V0.1.md`.

BILLING_REGISTER limited write approval note:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_APPROVAL_NOTE_V0.1.md`.

BILLING_REGISTER operator flow review:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_FLOW_REVIEW_V0.1.md`.

BILLING_REGISTER operator quick guide:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_QUICK_GUIDE_V0.1.md`.

BILLING_REGISTER controlled write validation:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_CONTROLLED_WRITE_VALIDATION_V0.1.md`.

BILLING_REGISTER write operator note:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_OPERATOR_NOTE_V0.1.md`.

BILLING_REGISTER write validation design:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_DESIGN_V0.1.md`.

BILLING_REGISTER write validation commands:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_COMMANDS_V0.1.md`.

BILLING_REGISTER write validation checklist:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_CHECKLIST_V0.1.md`.

BILLING_REGISTER read-only validation:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_READONLY_VALIDATION_V0.1.md`.

BILLING_REGISTER scope expansion note:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_SCOPE_EXPANSION_NOTE_V0.1.md`.

SHIPMENT write monitoring:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_V0.1.md`.

SHIPMENT write monitoring daily report template:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_REPORT_TEMPLATE_V0.1.md`.

SHIPMENT limited write enablement:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_LIMITED_WRITE_ENABLEMENT_V0.1.md`.

SHIPMENT limited write approval note:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_LIMITED_WRITE_APPROVAL_NOTE_V0.1.md`.

SHIPMENT operator flow review:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_OPERATOR_FLOW_REVIEW_V0.1.md`.

SHIPMENT operator quick guide:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_OPERATOR_QUICK_GUIDE_V0.1.md`.

SHIPMENT write validation execute:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_EXECUTE_V0.1.md`.

SHIPMENT write operator note:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_OPERATOR_NOTE_V0.1.md`.

SHIPMENT write validation design:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_DESIGN_V0.1.md`.

SHIPMENT write validation checklist:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_CHECKLIST_V0.1.md`.

SHIPMENT write validation commands:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_COMMANDS_V0.1.md`.

SHIPMENT read-only validation:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_READONLY_VALIDATION_V0.1.md`.

Week-2 scope expansion note:

See `docs/LOW_CODE_PILOT_WEEK2_SCOPE_EXPANSION_NOTE_V0.1.md`.

Week-1 review:

See `docs/LOW_CODE_PILOT_WEEK1_REVIEW_V0.1.md`.

Week-2 plan:

See `docs/LOW_CODE_PILOT_WEEK2_PLAN_V0.1.md`.

Week-1 feedback & fix plan:

See `docs/LOW_CODE_PILOT_WEEK1_FEEDBACK_FIX_PLAN_V0.1.md`.

Operator feedback form:

See `docs/LOW_CODE_PILOT_OPERATOR_FEEDBACK_FORM_V0.1.md`.

Week-1 review template:

See `docs/LOW_CODE_PILOT_WEEK1_REVIEW_TEMPLATE_V0.1.md`.

Day-1 monitoring:

See `docs/LOW_CODE_PILOT_DAY1_MONITORING_V0.1.md`.

Daily report template:

See `docs/LOW_CODE_PILOT_DAILY_REPORT_TEMPLATE_V0.1.md`.

Final smoke & handoff:

See `docs/LOW_CODE_PILOT_FINAL_SMOKE_HANDOFF_V0.1.md`.

Short handoff note (operators):

See `docs/LOW_CODE_PILOT_HANDOFF_NOTE_V0.1.md`.

Pilot fix & polish sprint:

See `docs/LOW_CODE_PILOT_FIX_POLISH_SPRINT_V0.1.md`.

Pilot manual UI verification:

See `docs/LOW_CODE_PILOT_MANUAL_UI_VERIFICATION_V0.1.md`.

Pilot release package:

See `docs/LOW_CODE_PILOT_RELEASE_PACKAGE_V0.1.md`.

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

Runtime readiness review:

See `docs/LOW_CODE_RUNTIME_READINESS_REVIEW_V0.1.md`.

Next implementation:

1. Low-code Pilot Week-3 Operator Feedback Scheduling Follow-up Pack v0.1

Week-3 first real operator feedback capture:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_REAL_OPERATOR_FEEDBACK_CAPTURE_V0.1.md`.

Week-3 first real operator feedback summary:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_REAL_OPERATOR_FEEDBACK_SUMMARY_V0.1.md`.

Week-3 first real operator feedback PM action note:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_REAL_OPERATOR_FEEDBACK_PM_ACTION_NOTE_V0.1.md`.

Week-3 operator feedback PM escalation:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_PM_ESCALATION_V0.1.md`.

Week-3 operator feedback PM decision note:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_PM_DECISION_NOTE_V0.1.md`.

Week-3 operator feedback session schedule template:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_SESSION_SCHEDULE_TEMPLATE_V0.1.md`.

Week-3 first operator feedback session retry:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_RETRY_V0.1.md`.

Week-3 operator feedback scheduling note:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_SCHEDULING_NOTE_V0.1.md`.

Week-3 first operator feedback session:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_V0.1.md`.

Week-3 first operator feedback session notes template:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_NOTES_TEMPLATE_V0.1.md`.

Week-3 operator feedback evidence:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_EVIDENCE_V0.1.md`.

Week-3 first operator feedback action plan:

See `docs/LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_ACTION_PLAN_V0.1.md`.

Week-3 operator feedback no-submissions report:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_NO_SUBMISSIONS_REPORT_V0.1.md`.

Week-3 feedback triage and backlog:

See `docs/LOW_CODE_PILOT_WEEK3_FEEDBACK_TRIAGE_AND_BACKLOG_V0.1.md`.

Week-3 improvements backlog:

See `docs/LOW_CODE_PILOT_WEEK3_IMPROVEMENTS_BACKLOG_V0.1.md`.

Week-3 feedback triage daily report template:

See `docs/LOW_CODE_PILOT_WEEK3_FEEDBACK_TRIAGE_DAILY_REPORT_TEMPLATE_V0.1.md`.

Week-3 operator feedback collection:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_COLLECTION_V0.1.md`.

Week-3 operator feedback form template:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md`.

Week-3 operator feedback log:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_LOG_V0.1.md`.

Week-3 operator feedback triage runbook:

See `docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_TRIAGE_RUNBOOK_V0.1.md`.

Week-3 auth-on staging verification:

See `docs/LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_VERIFICATION_V0.1.md`.

Week-3 auth-on staging runbook:

See `docs/LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_RUNBOOK_V0.1.md`.

Week-3 monitoring evidence:

See `docs/LOW_CODE_PILOT_WEEK3_MONITORING_EVIDENCE_V0.1.md`.

Week-3 monitoring baseline report:

See `docs/LOW_CODE_PILOT_WEEK3_MONITORING_BASELINE_REPORT_V0.1.md`.

Week-3 monitoring runbook:

See `docs/LOW_CODE_PILOT_WEEK3_MONITORING_RUNBOOK_V0.1.md`.

Week-2 closure:

See `docs/LOW_CODE_PILOT_WEEK2_CLOSURE_V0.1.md`.

Week-2 closure PM decision note:

See `docs/LOW_CODE_PILOT_WEEK2_CLOSURE_PM_DECISION_NOTE_V0.1.md`.

Week-3 execution plan:

See `docs/LOW_CODE_PILOT_WEEK3_EXECUTION_PLAN_V0.1.md`.

Week-2 cross-entity readiness review:

See `docs/LOW_CODE_PILOT_WEEK2_CROSS_ENTITY_READINESS_REVIEW_V0.1.md`.

Week-2 cross-entity decision note:

See `docs/LOW_CODE_PILOT_WEEK2_CROSS_ENTITY_DECISION_NOTE_V0.1.md`.

Week-3 candidate plan:

See `docs/LOW_CODE_PILOT_WEEK3_CANDIDATE_PLAN_V0.1.md`.

BILLING_REGISTER write monitoring:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_V0.1.md`.

BILLING_REGISTER write monitoring daily report template:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_REPORT_TEMPLATE_V0.1.md`.

BILLING_REGISTER limited write enablement:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_ENABLEMENT_V0.1.md`.

BILLING_REGISTER limited write approval note:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_APPROVAL_NOTE_V0.1.md`.

BILLING_REGISTER operator flow review:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_FLOW_REVIEW_V0.1.md`.

BILLING_REGISTER operator quick guide:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_QUICK_GUIDE_V0.1.md`.

BILLING_REGISTER controlled write validation:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_CONTROLLED_WRITE_VALIDATION_V0.1.md`.

BILLING_REGISTER write operator note:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_OPERATOR_NOTE_V0.1.md`.

BILLING_REGISTER write validation design:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_DESIGN_V0.1.md`.

BILLING_REGISTER write validation commands:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_COMMANDS_V0.1.md`.

BILLING_REGISTER write validation checklist:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_VALIDATION_CHECKLIST_V0.1.md`.

BILLING_REGISTER read-only validation:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_READONLY_VALIDATION_V0.1.md`.

BILLING_REGISTER scope expansion note:

See `docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_SCOPE_EXPANSION_NOTE_V0.1.md`.

SHIPMENT write monitoring:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_V0.1.md`.

SHIPMENT write monitoring daily report template:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_REPORT_TEMPLATE_V0.1.md`.

SHIPMENT limited write enablement:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_LIMITED_WRITE_ENABLEMENT_V0.1.md`.

SHIPMENT limited write approval note:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_LIMITED_WRITE_APPROVAL_NOTE_V0.1.md`.

SHIPMENT operator flow review:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_OPERATOR_FLOW_REVIEW_V0.1.md`.

SHIPMENT operator quick guide:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_OPERATOR_QUICK_GUIDE_V0.1.md`.

SHIPMENT write validation execute:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_EXECUTE_V0.1.md`.

SHIPMENT write operator note:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_OPERATOR_NOTE_V0.1.md`.

SHIPMENT write validation design:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_DESIGN_V0.1.md`.

SHIPMENT write validation checklist:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_CHECKLIST_V0.1.md`.

SHIPMENT write validation commands:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_VALIDATION_COMMANDS_V0.1.md`.

SHIPMENT read-only validation:

See `docs/LOW_CODE_PILOT_WEEK2_SHIPMENT_READONLY_VALIDATION_V0.1.md`.

Week-2 scope expansion note:

See `docs/LOW_CODE_PILOT_WEEK2_SCOPE_EXPANSION_NOTE_V0.1.md`.

Week-1 review:

See `docs/LOW_CODE_PILOT_WEEK1_REVIEW_V0.1.md`.

Week-2 plan:

See `docs/LOW_CODE_PILOT_WEEK2_PLAN_V0.1.md`.

Week-1 feedback & fix plan:

See `docs/LOW_CODE_PILOT_WEEK1_FEEDBACK_FIX_PLAN_V0.1.md`.

Operator feedback form:

See `docs/LOW_CODE_PILOT_OPERATOR_FEEDBACK_FORM_V0.1.md`.

Week-1 review template:

See `docs/LOW_CODE_PILOT_WEEK1_REVIEW_TEMPLATE_V0.1.md`.

Day-1 monitoring:

See `docs/LOW_CODE_PILOT_DAY1_MONITORING_V0.1.md`.

Daily report template:

See `docs/LOW_CODE_PILOT_DAILY_REPORT_TEMPLATE_V0.1.md`.

Final smoke & handoff:

See `docs/LOW_CODE_PILOT_FINAL_SMOKE_HANDOFF_V0.1.md`.

Short handoff note:

See `docs/LOW_CODE_PILOT_HANDOFF_NOTE_V0.1.md`.

Pilot fix & polish sprint:

See `docs/LOW_CODE_PILOT_FIX_POLISH_SPRINT_V0.1.md`.

Pilot manual UI verification:

See `docs/LOW_CODE_PILOT_MANUAL_UI_VERIFICATION_V0.1.md`.

Pilot release package:

See `docs/LOW_CODE_PILOT_RELEASE_PACKAGE_V0.1.md`.

Pilot operator checklist:

See `docs/LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md`.

Pilot release notes:

See `docs/LOW_CODE_PILOT_RELEASE_NOTES_V0.1.md`.

Pilot launch rehearsal:

See `docs/LOW_CODE_PILOT_LAUNCH_REHEARSAL_V0.1.md`.

Pilot launch runbook:

See `docs/LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md`.

Pilot go/no-go review:

See `docs/LOW_CODE_PILOT_GO_NO_GO_REVIEW_V0.1.md`.

Staging auth-on verification:

See `docs/LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md`.

Runtime pilot staging checklist:

See `docs/LOW_CODE_RUNTIME_PILOT_STAGING_CHECKLIST_V0.1.md`.

Template import execute API:

See `docs/LOW_CODE_TEMPLATE_IMPORT_EXECUTE_API_V0.1.md`.

Template import preview API:

See `docs/LOW_CODE_TEMPLATE_IMPORT_PREVIEW_API_V0.1.md`.

Template export API:

See `docs/LOW_CODE_TEMPLATE_EXPORT_API_V0.1.md`.

See `docs/LOW_CODE_TEMPLATE_IMPORT_EXPORT_DESIGN_V0.1.md`.

Low-code permissions matrix:

See `docs/LOW_CODE_PERMISSIONS_MATRIX_V0.1.md`.

Runtime pilot readiness review:

See `docs/LOW_CODE_RUNTIME_PILOT_READINESS_V0.1.md`.

Low-code entity integration (validation_context v0.2):

```text
http://localhost:3000/transport-orders/{id}
http://localhost:3000/shipments/{id}
http://localhost:3000/billing-registers/{id}
```

Verify helper:

```powershell
node scripts/dev/verify_lowcode_validation_context.mjs
```

See `docs/LOW_CODE_ENTITY_INTEGRATION_V0.2.md`.

Low-code admin permissions (v0.1):

```text
http://localhost:3000/low-code/admin/form-templates
```

Set `LOW_CODE_ADMIN_AUTH_ENABLED=true` on `low-code-service` for pilot RBAC. See `docs/LOW_CODE_PERMISSIONS_ADMIN_GUARDRAILS_V0.1.md`.

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
