# Low-code MVP Scope v0.1

Date: 2026-06-22  
Project: `D:\Projects\freight-platform`  
Status: **Scope definition only — no code, no migrations, no UI changes**  
Related:

- `docs/LOW_CODE_LAYER_ROADMAP_V0.1.md`
- `docs/LOW_CODE_DATA_MODEL_DESIGN_V0.1.md`

Git baseline: `39690bb`

---

## Summary

The full 7Rights Low-code Layer (BPMN Process Builder, Form Builder, Rule Engine, No-code Connectors) is **too large for a single delivery phase**. Attempting everything at once increases security risk, delays value, and threatens Core TMS stability.

**MVP v0.1** is deliberately small, safe, and useful:

| In MVP v0.1 | Deferred |
| ----------- | -------- |
| Configuration Registry | Full BPMN visual designer |
| Custom Fields | Process runtime engine |
| Form Templates | No-code Connectors |
| Simple validation & visibility rules | Mapping studio / ERP integrations |
| Configuration Audit Log | Pricing / tariff engine |
| Tenant isolation | Carrier selection automation |

**Recommended first MVP:** **Custom Fields + Form Templates + Simple Rules + Audit Log.**

This gives enterprise clients configurable screens and basic declarative rules **without** a workflow engine or integration platform. Core services (`transport-order-service`, `rfx-service`, `shipment-service`, `document-service`, `billing-register-service`) remain the authority for business logic and state transitions.

---

## Why Not Full Low-code Immediately

| Risk area | Why defer |
| --------- | --------- |
| **BPMN Process Builder** | Requires process runtime, task assignment, timers, and domain API orchestration — high complexity for v0.1 |
| **No-code Connectors** | Needs secret management, retry queues, dead-letter handling, idempotency, and integration monitoring |
| **Full Rule Engine** | Pricing, carrier selection, and financial rules can conflict with protected domain validation if released too early |
| **Form Builder without guardrails** | Tenants could hide or weaken mandatory system fields without `system_field` protections |
| **Configuration chaos** | Without audit log and publish lifecycle, tenant-specific configs become unmaintainable |

**Strategy:** Build a **safe configuration foundation** first. Add process orchestration (v0.2), advanced rules (v0.3), and connectors (v0.4) only after MVP v0.1 is deployed and `make integration-smoke-test` stays green.

---

## Recommended MVP v0.1

MVP v0.1 delivers eight capabilities:

| # | Capability | Description |
| - | ---------- | ----------- |
| 1 | **Low-code Configuration Registry** | Central index (`low_code_configurations`) for all tenant configs |
| 2 | **Custom Fields** | Tenant-defined fields on core entities |
| 3 | **Form Templates** | Section/field layout per entity type |
| 4 | **Simple Validation Rules** | Declarative constraints (required-if, range, pattern) |
| 5 | **Simple Visibility Rules** | Show/hide/read-only by role or entity status |
| 6 | **Configuration Audit Log** | Append-only change history |
| 7 | **Tenant Isolation** | All configs scoped by `tenant_id` |
| 8 | **Read-only preview (later UI phase)** | Admin UI to browse/preview configs — **design only in this pack**; UI implementation follows API |

---

## MVP v0.1 Functional Scope

### 1. Custom Fields

**Supported entity types:**

| Entity | Core service | MVP custom fields |
| ------ | ------------ | ----------------- |
| Transport orders | `transport-order-service` | ✅ |
| RFx / freight requests | `rfx-service` | ✅ |
| Shipments | `shipment-service` | ✅ |
| Documents | `document-service` | ✅ |
| Billing registers | `billing-register-service` | ✅ |
| Company profile | `company-service` | ✅ |

**Not in MVP v0.1:** Bids (read-only extension only), driver mobile forms, dashboard widgets.

**Field types (MVP subset):**

| Type | MVP | Notes |
| ---- | --- | ----- |
| `TEXT` | ✅ | |
| `NUMBER` | ✅ | |
| `DATE` | ✅ | |
| `DATETIME` | ✅ | |
| `SELECT` | ✅ | Static options in `options_json` |
| `CHECKBOX` | ✅ | |
| `FILE` | ✅ | Reference to document/file storage — metadata only in v0.1 |
| `MONEY` | ✅ | Amount + currency envelope |
| `COMPANY_REFERENCE` | ✅ | UUID ref validated via `company-service` |
| `DOCUMENT_REFERENCE` | ✅ | UUID ref validated via `document-service` |
| `MULTI_SELECT`, `ROUTE`, `ADDRESS`, `VEHICLE`, `CURRENCY`, `VAT_TAX` | ❌ v0.2+ | Design reserved in data model |

**Functions:**

- Required / optional (custom fields only)
- Default value (`default_value_json`)
- Validation (linked simple rules)
- Visibility by role (`visibility_rule_id` or inline condition)
- Read-only by entity status
- Localization: RU / EN / ZH labels in `localization_json`

**Storage:** Values in `custom_field_values` — **separate from core entity tables** (see data model design).

---

### 2. Form Templates

**MVP form templates:**

| Template | Entity type | Primary UI page (future) |
| -------- | ----------- | ------------------------ |
| Transport Order Form | `TRANSPORT_ORDER` | `/transport-orders` |
| RFx Form | `RFX` / `FREIGHT_REQUEST` | `/rfx`, `/freight-requests` |
| Shipment Form | `SHIPMENT` | `/shipments` |
| Document Form | `DOCUMENT` | `/documents` |
| Billing Register Form | `BILLING_REGISTER` | `/billing-registers` |

**Functions:**

- Sections (`form_sections`) with sort order
- Field order within sections
- Tenant-specific templates (one published template per entity type per tenant in v0.1)
- Draft / published status
- Version number increment on publish
- Rollback — **design for v0.1, implement in v0.1.1** (restore prior published version from audit snapshot)

**System fields:** Core mandatory columns rendered as `system_field = true` — cannot be removed or marked optional.

---

### 3. Simple Rule Engine MVP

**Included rule types (safe DSL only):**

| Rule type | Example |
| --------- | ------- |
| Required if condition | IF `cargo_type = DANGEROUS` THEN `hazmat_cert_ref` required |
| Visible if role/status | IF role = `CARRIER_DISPATCHER` THEN show section "Carrier ops" |
| Read-only if status | IF `shipment.status >= IN_TRANSIT` THEN custom fields read-only |
| Validation by range | IF `custom.insured_value > 1000000` THEN validation error |
| Notification trigger (draft) | IF rule matches THEN queue `low_code_notifications` record — **send in v0.2** |

**Explicitly NOT in v0.1:**

- Pricing / tariff calculation engine
- Carrier selection automation
- Financial closing override (billing register → CLOSED bypass)
- Arbitrary code execution (`eval`, scripts, SQL)
- Direct database mutation
- Cross-tenant data access
- RBAC elevation

**Rule format:** Declarative JSON DSL validated against schema (see `docs/LOW_CODE_DATA_MODEL_DESIGN_V0.1.md` — `condition_json`, `action_json` whitelist).

**Tables (MVP):** `rule_sets`, `rule_versions`, `rules` — lite subset without `rule_evaluation_logs` in first slice (optional v0.1.1).

---

### 4. Audit Log

**Logged actions:**

| Action | Trigger |
| ------ | ------- |
| `CREATE` | New config / field / template / rule |
| `UPDATE` | Draft edit |
| `PUBLISH` | Draft → published |
| `ARCHIVE` | Retire config |
| `ROLLBACK` | Restore prior version |
| Custom field change | Via form template publish |
| Validation rule change | Via rule set publish |

**Audit record fields (`configuration_audit_log`):**

| Field | Required |
| ----- | -------- |
| `tenant_id` | ✅ |
| `configuration_id` | ✅ |
| `entity_type` | ✅ |
| `entity_id` | ✅ |
| `action` | ✅ |
| `old_value_json` | On UPDATE / ROLLBACK |
| `new_value_json` | On CREATE / UPDATE / PUBLISH |
| `changed_by_user_id` | ✅ |
| `changed_at` | ✅ |
| `request_id` | ✅ (correlation) |

Append-only — no UPDATE/DELETE on audit rows.

---

## Out of Scope for MVP v0.1

The following are **explicitly excluded** from MVP v0.1:

| Area | Reason |
| ---- | ------ |
| Full BPMN visual designer | v0.2 — needs process runtime |
| Process runtime engine | v0.2 |
| No-code Connectors | v0.4 |
| Mapping studio | v0.4 |
| Connector credentials / secret vault integration | v0.4 |
| ERP / 1C / SAP / EDI integration | v0.4 |
| Pricing / tariff rule engine | v0.3 |
| Carrier selection automation | v0.3 |
| Dashboard / widget builder | Future |
| Document signing workflow builder | v0.2 (process overlay) |
| Production-grade marketplace templates | v0.4 |
| Rule simulation UI | v0.3 |
| Publish approval workflow (`configuration_approvals`) | Optional v0.1.1; mandatory v0.4 |
| Dedicated `low-code-service` deployment | Design in next pack; implement after API design |
| OpenAPI routes under `/v1/low-code/*` | Next pack — API design only |
| web-admin Form Builder UI | After API — UI MVP phase |
| PostgreSQL migrations | After technical design pack |

---

## MVP v0.2 Scope

**Theme:** Process and workflow foundation.

| Deliverable | Description |
| ----------- | ------------- |
| BPMN Process Builder MVP | Linear + approval + notification steps |
| Process templates | TO approval, RFx tender, billing close checklist |
| Workflow versioning | `process_versions` publish lifecycle |
| Status transition configuration | Overlay on domain-allowed edges only |
| Approval workflows | Multi-step human tasks |
| Process instance tracking | `process_instances`, `process_tasks` |
| Notification delivery | Send queued notifications from v0.1 draft |

**Tables added:** `process_templates`, `process_versions`, `process_steps`, `process_transitions`, `process_instances`, `process_tasks`, `low_code_runtime_events`.

---

## MVP v0.3 Scope

**Theme:** Advanced rules and operational intelligence.

| Deliverable | Description |
| ----------- | ------------- |
| Rule Engine advanced | Priority chains, compound conditions |
| SLA / KPI rules | Time-based triggers → Control Tower alerts |
| Pricing / tariff rules | Lane tables, fuel surcharge (read-only calc → domain API) |
| Carrier selection rules | Rating thresholds, exclusion lists |
| Rule simulation mode | Fixture input → matched rules preview |
| `rule_evaluation_logs` | Full evaluation audit |

---

## MVP v0.4 Scope

**Theme:** Enterprise integrations and marketplace.

| Deliverable | Description |
| ----------- | ------------- |
| No-code Connectors | Runtime connector service |
| Connector catalog | Global `connector_definitions` |
| Mapping studio | Visual field mapper UI |
| Webhook / API connectors | Inbound/outbound HTTP |
| ERP / 1C / SAP templates | Pre-built mapping starters |
| Retry / error queue | `integration_jobs` DEAD_LETTER |
| Integration logs | Admin UI `/settings/integration-logs` |
| Template marketplace | `global_templates`, `tenant_template_copies` |
| Publish approval governance | `configuration_approvals` mandatory |

---

## Recommended First Implementation Target

After this scope document, the **next documentation pack** (not code):

### Low-code Custom Fields Technical Design Pack v0.1

**Scope:** Detailed technical design for MVP v0.1 tables and service boundaries.

**Entities (from data model design):**

| Table | MVP v0.1 |
| ----- | -------- |
| `low_code_configurations` | ✅ |
| `form_templates` | ✅ |
| `form_sections` | ✅ |
| `form_fields` | ✅ |
| `custom_field_values` | ✅ |
| `rule_sets` | ✅ (lite) |
| `rule_versions` | ✅ (lite) |
| `rules` | ✅ (lite) |
| `configuration_audit_log` | ✅ |

**Implementation order (after technical design):**

```
1. Design only     ← Custom Fields Technical Design Pack v0.1
2. Migrations      ← lowcode schema, MVP v0.1 tables only
3. API design      ← draft OpenAPI /v1/low-code/*
4. Service         ← low-code-service OR extension module (TBD in technical design)
5. UI              ← read-only config browser, then Form Builder MVP
```

**Regression gate:** `make integration-smoke-test` must pass after every increment. Core domain logic unchanged.

---

## Guardrails

Non-negotiable for MVP v0.1 and all future versions:

| # | Guardrail |
| - | --------- |
| 1 | Low-code **cannot bypass RBAC** — all config APIs require authenticated roles |
| 2 | Low-code **cannot change protected financial statuses** (billing close, UPD, payment) |
| 3 | Low-code **cannot create cross-tenant access** — `tenant_id` enforced on every query |
| 4 | Low-code **cannot sign documents** without `document-service` domain validation |
| 5 | Low-code **cannot execute arbitrary code** — declarative JSON DSL only |
| 6 | All rules are **declarative JSON DSL** — schema-validated, action whitelist |
| 7 | All configuration changes → **`configuration_audit_log`** |
| 8 | **Secrets never in Git** or plain DB columns |
| 9 | **Production templates require publish** (draft not visible to runtime) |
| 10 | **System fields immutable** — `system_field = true` rows protected |
| 11 | **Custom field values separate** from core tables — no ALTER on domain tables for MVP |
| 12 | Core services **retain final validation** on create/update API calls |

---

## Success Criteria

MVP v0.1 is **successful** when all criteria are met:

| # | Criterion | Verification |
| - | --------- | ------------ |
| 1 | Tenant admin can create a custom field template | API + UI test |
| 2 | Custom fields bound to tenant (`tenant_id`) | Integration test |
| 3 | Fields support draft → published versioning | Publish API test |
| 4 | Fields render in entity form (UI phase) | Manual UI check |
| 5 | Custom field values stored in `custom_field_values`, not core columns | DB + API test |
| 6 | Audit log records every config change | Audit query test |
| 7 | `make integration-smoke-test` passes unchanged | CI / local smoke |
| 8 | Backend business logic remains protected | No domain rule bypass in code review |
| 9 | No migrations until technical design approved | Process gate |
| 10 | Demo tenant can show 1+ custom field on transport order (demo seed extension — optional v0.1.1) | UI smoke |

---

## MVP v0.1 Delivery Phases (Suggested)

| Phase | Deliverable | Type |
| ----- | ----------- | ---- |
| **Phase 0** | This scope doc + Custom Fields Technical Design | Docs |
| **Phase 1** | Migrations (`lowcode` schema, 9 tables) | Infra |
| **Phase 2** | `low-code-service` skeleton + audit + registry API | Backend |
| **Phase 3** | Form template + custom field CRUD API | Backend |
| **Phase 4** | Custom field values read/write on existing entity APIs (extension) | Backend integration |
| **Phase 5** | web-admin read-only config pages | Frontend |
| **Phase 6** | Form Builder editor MVP | Frontend |

Phases 1–6 are **out of scope for this pack** — documented for planning only.

---

## Recommended Next Action

1. **Do not write code or migrations now.**
2. **Next pack:** `Low-code Custom Fields Technical Design Pack v0.1` — service boundary, API draft, validation flow, integration points with existing services.
3. **Then:** Migrations design review → implement `lowcode` schema (MVP tables only).
4. **Then:** API design (`/v1/low-code/*` draft OpenAPI).
5. **Then:** UI Form Builder MVP (read-only first, then editor).
6. **Always:** Keep smoke test green; treat `tests/integration/smoke-test.sh` as regression baseline.

---

## Related Documentation

| Document | Role |
| -------- | ---- |
| `docs/LOW_CODE_LAYER_ROADMAP_V0.1.md` | Strategic modules and version roadmap |
| `docs/LOW_CODE_DATA_MODEL_DESIGN_V0.1.md` | Entity fields, indexes, relationships |
| `docs/AUTH_RBAC.md` | Roles for config admin / publisher |
| `docs/DEMO_SEED.md` | Future demo custom fields for UI verification |
| `AGENTS.md` | AI safety rules — no domain rewrite |

---

## Document History

| Version | Date | Notes |
| ------- | ---- | ----- |
| v0.1 | 2026-06-22 | Initial MVP scope definition — docs only |
