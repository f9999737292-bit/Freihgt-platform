# Low-code Layer Roadmap v0.1

Date: 2026-06-22  
Project: `D:\Projects\freight-platform`  
Status: **Draft roadmap — documentation only**  
Git baseline: `4d80251`

---

## Summary

The **Low-code Layer** is a strategic configuration plane above the Core TMS domain. It enables large enterprise clients (shippers, carriers, forwarders, consignees, platform operators) to adapt workflows, screens, business rules, and integrations **without custom development** for every tenant-specific change.

Key principles:

- Low-code **does not replace** backend business logic — it configures **allowed** behavior within domain guardrails.
- Core services (`transport-order-service`, `rfx-service`, `shipment-service`, `document-service`, `billing-register-service`, etc.) remain the **source of truth** for state transitions, financial integrity, and compliance.
- The layer covers four primary zones: **interfaces (forms)**, **processes (BPMN)**, **rules (Rule Engine)**, **integrations (No-code Connectors)**.
- All configuration is **tenant-scoped**, **versioned**, **audited**, and **publish-controlled**.

This document is a roadmap only. No API contracts, migrations, services, or UI are changed in v0.1.

---

## Why 7Rights Needs a Low-code Layer

7Rights / freight-platform targets an enterprise TMS / freight marketplace comparable to Transporeon, with **deep customization** for large customers. Without a Low-code Layer, every variation becomes a development ticket:

| Business need | Without low-code | With low-code |
| ------------- | ---------------- | ------------- |
| Different transport order forms per shipper | Hard-coded UI + backend changes | Form Builder + custom fields |
| Multi-step approval before RFx publish | Custom service logic per tenant | BPMN Process Builder template |
| Carrier selection by rating + lane rules | Developer-maintained code | Rule Engine |
| ERP / 1C / SAP / EDI sync | One-off integration projects | No-code Connectors + mapping studio |
| Document signing routes | Fixed workflow | Process + notification templates |
| Billing close prerequisites | Domain-only checks | Rules + process gates aligned with domain |

### Stakeholder examples

| Role | Typical customization |
| ---- | --------------------- |
| **Large shipper** | Custom TO fields, approval chains, RFx templates, SLA alerts |
| **Carrier** | Bid forms, capacity rules, telematics connector, mobile driver forms |
| **Forwarder** | Multi-leg processes, tariff rules, subcontractor connectors |
| **Consignee** | Delivery confirmation forms, POD requirements, slot rules |
| **Platform admin** | Global templates, marketplace catalog, governance policies |

Enterprise buyers expect **self-service configuration** after go-live. A Low-code Layer reduces time-to-value, lowers TCO, and prevents core codebase fragmentation.

---

## Current State

The platform already has a **solid Core TMS foundation**. The following capabilities exist today (partially or fully):

### Platform & tenancy

| Capability | Status | Location / notes |
| ---------- | ------ | ---------------- |
| Multi-tenant model | ✅ | `core.tenants`, tenant_id on entities |
| RBAC / roles | ✅ | `identity-service`, migrations `000009_seed_roles` |
| Dev admin seed | ✅ | `scripts/dev/seed_dev_admin.sh`, `make seed-dev-admin` |
| Demo seed | ✅ | `scripts/dev/seed_demo_data.sh`, `docs/DEMO_SEED.md` |

### Admin UI (web-admin)

| Page / area | Status |
| ----------- | ------ |
| Login + tenant context | ✅ |
| Dashboard | ✅ |
| Control Tower | ✅ (`docs/CONTROL_TOWER.md`) |
| Companies | ✅ |
| Users | ✅ |
| Transport orders | ✅ |
| RFx events | ✅ |
| Freight requests | ✅ |
| Bids | ✅ |
| Shipments | ✅ |
| Documents | ✅ |
| Billing registers | ✅ |
| Health / observability | ✅ |

### Core domain services (backend)

| Domain | Service | Key flows |
| ------ | ------- | --------- |
| Transport orders | `transport-order-service` | Create, submit, locations, cargo |
| RFx | `rfx-service` | RFx events, freight requests, bids, accept |
| Shipments | `shipment-service` | From bid, driver/vehicle, status machine |
| Documents | `document-service` | POD, signing sessions, signatures |
| Billing | `billing-register-service` | Register, items, calculate, approve, UPD |
| Companies / members | `company-service` | CRUD, memberships |
| Identity | `identity-service` | Users, roles, auth |
| API Gateway | `api-gateway` | Routing, auth, OpenAPI |

### Operations & quality

| Capability | Status |
| ---------- | ------ |
| Health check | ✅ `make health-check` |
| Integration smoke test | ✅ `tests/integration/smoke-test.sh` |
| CI v0.1 | ✅ `.github/workflows/ci.yml`, `docs/CI.md` |
| UI runtime verification | ✅ `docs/UI_RUNTIME_VERIFICATION_V0.1.md` |
| Demo UI verification | ✅ `docs/DEMO_UI_VERIFICATION_V0.1.md` |
| UPD flow (smoke-tested) | ✅ Full billing → UPD path verified |

### Important limitation today

**Processes, validations, status machines, and integrations are implemented as backend/domain logic**, not as tenant-configurable templates. Examples:

- Shipment status transitions: hard-coded in `shipment-service/internal/domain/shipment.go`
- Bid acceptance rules: service-layer validation
- Billing register close: domain state machine + smoke-tested UPD prerequisites
- Forms: static Nuxt pages, not schema-driven
- Integrations: none as configurable connectors

The Low-code Layer will **orchestrate and configure** within these boundaries — not bypass them.

---

## What Is Missing

Explicit gaps that motivate this roadmap:

### Process & workflow

- Visual **BPMN Process Builder**
- Process **versioning** and publish/rollback
- **Workflow templates** (TO approval, RFx, billing close, etc.)
- **Dynamic status transition configuration** (within domain-allowed edges)
- Approval simulation / dry-run

### Forms & UI configuration

- **Form Builder** for entity screens
- **Custom fields** runtime (tenant-specific attributes)
- **Dynamic validation rules** on forms
- Schema versioning and localization of custom labels

### Rules

- **Rule Engine** (declarative DSL)
- Visual **rule editor**
- **Pricing / tariff rule builder**
- Rule simulation before publish

### Integrations

- **No-code Connectors** framework
- **Mapping studio** (source ↔ platform fields)
- **Integration marketplace / catalog**
- Connector **logs**, retries, dead-letter queue
- Credential management (secure, not in Git)

### Governance

- **Configuration audit log** (who changed what, when)
- Configuration **approval** before production publish
- **Rollback** to previous configuration version
- Policy compliance reports

---

## Low-code Layer Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     Admin UI (web-admin) — future settings              │
│  /settings/forms  /settings/processes  /settings/rules  /settings/...   │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼─────────────────────────────────────┐
│              Low-code Configuration Layer (NEW — roadmap)               │
│  form schemas │ process templates │ rule sets │ connector definitions   │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼─────────────────────────────────────┐
│              Runtime Execution Layer (NEW — roadmap)                      │
│  process runner │ rule evaluator │ form renderer │ integration executor   │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │ respects guardrails
┌───────────────────────────────────▼─────────────────────────────────────┐
│                    Core TMS Domain Layer (EXISTS)                         │
│  transport orders │ RFx │ freight requests │ bids │ shipments │         │
│  documents │ billing registers │ companies │ users │ RBAC               │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼─────────────────────────────────────┐
│              Integration Layer (NEW — roadmap)                            │
│  ERP │ 1C │ SAP │ EDI │ WMS │ telematics │ e-sign │ webhooks │ ЭТрН     │
└─────────────────────────────────────────────────────────────────────────┘

        ┌──────────────── Governance & Audit Layer ────────────────┐
        │ versioning │ publish approval │ audit log │ rollback      │
        └──────────────────────────────────────────────────────────┘

        ┌──────────────── Tenant Isolation Layer ──────────────────┐
        │ tenant_id on all configs │ no cross-tenant access        │
        └──────────────────────────────────────────────────────────┘
```

### 1. Core TMS Domain Layer (existing)

Immutable business invariants enforced in Go services:

- Entity lifecycles (transport order → freight request → bid → shipment → document → billing)
- Protected financial statuses
- RBAC enforcement at API layer
- Multi-tenant data isolation in PostgreSQL schemas (`core`, `transport`, `rfx`, `documents`, `billing`)

### 2. Low-code Configuration Layer (planned)

Stores **declarative definitions** only:

- Process templates (BPMN-like JSON, not arbitrary code)
- Form schemas + custom field definitions
- Rule sets (safe DSL)
- Connector definitions + field mappings
- Document / notification templates
- Dashboard widget settings

### 3. Runtime Execution Layer (planned)

Interpretation engines that **read configuration** and invoke Core APIs:

- Process engine: advance workflow steps, assign tasks, call domain transitions
- Rule evaluator: fire on events (status change, amount threshold, SLA breach)
- Form renderer: merge base entity + custom fields for web-admin / mobile
- Integration executor: run connector jobs, apply mappings, handle retries

### 4. Integration Layer (planned)

External system connectivity:

- Scheduled and event-driven jobs
- Mapping transformations
- Retry policies and error queues
- Observability (integration logs in admin UI)

### 5. Governance and Audit Layer (planned)

- Draft → review → publish lifecycle for all config types
- Immutable audit trail per change
- Rollback to prior published version
- Platform admin approval for global templates

### 6. Tenant Isolation Layer (planned)

- Every configuration row scoped by `tenant_id`
- Global templates readable by platform admin only; tenants **copy** and customize
- Connector credentials never shared across tenants

---

## Module 1: BPMN Process Builder

### Purpose

Visual workflow designer for tenant-specific **process templates** that orchestrate human and system steps across the TMS lifecycle.

### Capabilities

| Feature | Description |
| ------- | ----------- |
| Process templates | Named workflows per entity type (TO, RFx, shipment, billing, etc.) |
| Steps & stages | User tasks, system tasks, gateways (AND/OR), timers |
| Transitions | Allowed paths between steps with conditions (from Rule Engine) |
| Approval flows | Multi-level approval with role assignments |
| Role bindings | Which role can complete each user task |
| Versioning | Draft versions; publish creates immutable version |
| Publish / rollback | Tenant admin publishes; rollback restores prior version |
| Audit log | Every template change recorded |

### Example processes

| Process | Core entities | Low-code configures |
| ------- | ------------- | ------------------- |
| Transport order approval | `transport-order-service` | Approval steps before `submit` |
| RFx tender process | `rfx-service` | Publish gates, participant invitation flow |
| Bid evaluation | `rfx-service` | Scoring steps, manual review tasks |
| Shipment lifecycle overlay | `shipment-service` | Optional checkpoints (not illegal transitions) |
| Document signing process | `document-service` | Signer order, reminders, escalations |
| Billing register closing | `billing-register-service` | Pre-close checklist tasks |
| Claims / dispute | cross-service | Case workflow, evidence collection |

### Guardrail (critical)

**BPMN must not bypass protected domain rules.** Examples:

- Cannot low-code a transition to `CLOSED` billing register if domain requires `APPROVED` + UPD + documents
- Cannot skip RBAC — process tasks still check roles via `identity-service`
- Cannot mutate shipment to `READY_FOR_BILLING` without domain-valid predecessor statuses
- Process engine **calls** domain APIs; it does not write arbitrary SQL or skip validation

---

## Module 2: Form Builder

### Purpose

Schema-driven UI constructor for entity forms and **custom fields**, enabling per-tenant / per-role screen customization without Nuxt code changes.

### Application areas

| Entity | Base form (today) | Low-code adds |
| ------ | ----------------- | ------------- |
| Transport orders | Static page | Custom fields, sections, validations |
| RFx events | Static page | Tender-specific fields |
| Freight requests | Static page | Lane attributes, incoterms extensions |
| Bids | Static page | Cost breakdown custom lines |
| Shipments | Static page | Operational notes, customs fields |
| Documents | Static page | Metadata extensions |
| Billing registers | Static page | Cost center, GL mapping fields |
| Company profile | Static page | Industry-specific attributes |
| Driver / mobile | Future | Mobile-optimized custom forms |

### Field types

`text` · `number` · `date` · `datetime` · `select` · `multi-select` · `checkbox` · `file` · `company reference` · `route` · `address` · `vehicle` · `money` · `currency` · `VAT/tax` · `document reference`

### Features

- Required / optional flags
- Default values (static or rule-driven)
- Validation rules (linked to Rule Engine)
- Visibility by role
- Read-only by entity status
- Field grouping / sections / tabs
- Localization: RU / EN / ZH labels and help text
- Schema versioning
- Audit log on schema changes

### Guardrail

Form Builder **cannot remove or weaken system mandatory fields** (e.g. `tenant_id`, `shipper_company_id`, core status fields). Custom fields stored separately (EAV or JSONB extension table — future data model pack).

---

## Module 3: Rule Engine

### Purpose

Declarative business rule evaluation for approvals, SLAs, pricing, validations, and notifications — **no arbitrary code execution**.

### Rule categories

| Category | Example |
| -------- | ------- |
| Approval rules | Amount > X → CFO approval required |
| SLA / KPI rules | Pickup delay > N hours → alert |
| Carrier selection | Rating below threshold → exclude from tender |
| RFx winner evaluation | Weighted score formula |
| Pricing / tariff | Lane rate tables, fuel surcharge |
| Billing validation | Documents missing → block close |
| Document validation | Required attachments by doc type |
| Notification rules | Status change → email/webhook |
| Risk / antifraud | Unusual bid pattern → manual review |
| Slot booking | Warehouse capacity constraints |

### Format requirements

- **Safe JSON-based DSL** (conditions + actions)
- **No arbitrary code execution** (no `eval`, no embedded scripts)
- Versioned rule sets
- **Test / simulation mode** before publish (input fixture → expected outcome)
- Audit log + rollback

### Example rules

```
IF transport_order.estimated_value > 500000 RUB
  THEN require_approval(role=FINANCE_MANAGER)

IF carrier.rating < 3.5
  THEN exclude_from_rfx(freight_request_id)

IF shipment.delay_hours > 24
  THEN create_alert(severity=HIGH, notify=CONTROL_TOWER)

IF billing_register.missing_documents
  THEN block_transition(to=CLOSED)

IF bid.amount > target_rate * 1.15
  THEN require_manual_acceptance
```

### Forbidden actions (guardrails)

Rule Engine actions **must not**:

- Bypass RBAC or elevate privileges
- Access another tenant's data
- Force protected financial state transitions
- Create or sign documents without domain validation
- Execute raw SQL or shell commands
- Call external URLs not registered in connector definitions

---

## Module 4: No-code Connectors

### Purpose

Integration framework for connecting external systems without per-customer coding projects.

### Integration types

| Type | Examples |
| ---- | -------- |
| ERP / accounting | 1C, SAP, Oracle |
| EDI | EDIFACT, X12 freight messages |
| WMS | Warehouse events |
| External TMS | Carrier capacity feeds |
| Carrier APIs | Rate quotes, tracking |
| Telematics / GPS | Position, ETA |
| E-signature | Diadoc, SBIS, CryptoPro flows |
| Email / webhook | Generic HTTP |
| Government | Минтранс, ЭТрН / ЭПД providers |

### Features

| Feature | Description |
| ------- | ----------- |
| Connector catalog | Pre-built templates (1C export, SAP IDoc, etc.) |
| Mapping studio | Visual field map: external ↔ platform entity |
| Transformation rules | Format conversion (date, currency, code tables) |
| Schedule triggers | Cron-based sync |
| Event triggers | On entity status change |
| Webhook receiver | Inbound HTTP endpoints per tenant |
| Retry policy | Exponential backoff, max attempts |
| Dead-letter queue | Failed jobs for manual replay |
| Integration logs | Payload hash, status, error detail in admin UI |
| Credential management | Secure vault reference, not plain DB |
| Tenant settings | Per-tenant endpoint URLs, mappings |

### Guardrails

- **Secrets never in Git** or plain configuration JSON
- Credentials in secure secret storage (Vault / cloud KMS — TBD in infra pack)
- Integration errors visible in admin UI (`/settings/integration-logs`)
- All mappings **versioned**; publish required before production use
- Idempotency keys to prevent duplicate entity creation

---

## Low-code Layer Scope by Version

### v0.1 — Configuration Foundation

**Goal:** Flexible configuration foundation without full BPMN complexity.

| Deliverable | Description |
| ----------- | ----------- |
| Custom fields registry | Tenant-scoped field definitions |
| Form schemas | JSON schema storage + read API |
| RFx templates | Reusable RFx / freight request field sets |
| Simple approval rules | Threshold-based approval (Rule Engine lite) |
| Notification rules | Event → channel mapping |
| Configuration audit log | Append-only change history |
| Tenant configuration registry | Central index of active configs |
| Admin UI (read-only) | Browse configs at `/settings/low-code` |

### v0.2 — Process and Rule Studio

**Goal:** Configure processes and rules through UI.

| Deliverable | Description |
| ----------- | ----------- |
| BPMN Process Builder MVP | Linear + gateway workflows |
| Workflow templates | Library for TO, RFx, billing |
| Status transition configuration | Allowed edges overlay (domain-validated) |
| Rule Engine MVP | Full DSL evaluator + simulation |
| SLA / KPI rules | Time-based triggers |
| Document templates | Merge fields for PDF/XML generation |
| Approval simulation | Dry-run before publish |

### v0.3 — Integration Studio

**Goal:** Enterprise integrations without constant development.

| Deliverable | Description |
| ----------- | ----------- |
| No-code Connectors framework | Connector runtime service |
| Connector catalog | 1C, SAP, EDI, webhook starters |
| Mapping studio | Visual mapper UI |
| Webhook / REST connectors | Generic HTTP in/out |
| ERP / EDI templates | Pre-built mappings |
| Retry / error queue | Job infrastructure |
| Integration monitoring | Dashboards + alerts |

### v0.4 — Marketplace and Advanced Governance

**Goal:** Scale customization across many tenants with control.

| Deliverable | Description |
| ----------- | ----------- |
| Template marketplace | Share / subscribe to templates |
| Reusable tenant templates | Clone from global catalog |
| Publish approval workflow | Reviewer sign-off before prod |
| Advanced rollback | Point-in-time config restore |
| Advanced audit | Compliance export |
| Policy compliance reports | SOC-style config change reports |

---

## Data Model Draft

> **Draft entities only — no migrations in this pack.**  
> All tenant-scoped entities include `tenant_id UUID NOT NULL`. Platform-global templates use `tenant_id IS NULL` + `is_global BOOLEAN`.

| Entity | Purpose |
| ------ | ------- |
| `low_code_configurations` | Registry of all config artifacts (type, key, version, status) |
| `form_templates` | Form layout + binding to entity type |
| `form_fields` | Custom field definitions (type, validation, i18n labels) |
| `process_templates` | Process definition metadata (name, entity_type, status) |
| `process_versions` | Immutable published process snapshots |
| `process_steps` | Steps within a process version (task type, role, SLA) |
| `process_transitions` | Edges between steps (condition expression ref) |
| `rule_sets` | Named collection of rules (versioned) |
| `rules` | Individual rule (condition DSL + action DSL) |
| `connector_definitions` | Catalog entry (type, schema, capabilities) |
| `connector_instances` | Tenant-deployed connector (definition + settings ref) |
| `connector_mappings` | Field maps per instance + entity direction |
| `integration_jobs` | Job queue entries (pending, running, failed) |
| `integration_logs` | Execution log lines (job_id, payload ref, error) |
| `configuration_audit_log` | Who/when/what changed (all config types) |

Suggested schema namespace: `lowcode` (future migration pack).

---

## API Draft

> **Future API routes — NOT implemented. Not part of current OpenAPI.**

| Method | Route | Purpose |
| ------ | ----- | ------- |
| GET | `/v1/low-code/configurations` | List config registry for tenant |
| POST | `/v1/low-code/form-templates` | Create form template (draft) |
| GET | `/v1/low-code/form-templates/{id}` | Get form template |
| PATCH | `/v1/low-code/form-templates/{id}` | Update draft |
| POST | `/v1/low-code/process-templates` | Create process template |
| POST | `/v1/low-code/process-templates/{id}/publish` | Publish version |
| POST | `/v1/low-code/process-templates/{id}/rollback` | Rollback to prior version |
| POST | `/v1/low-code/rule-sets` | Create rule set |
| POST | `/v1/low-code/rule-sets/{id}/simulate` | Run simulation with fixture |
| POST | `/v1/low-code/connectors` | Deploy connector instance |
| POST | `/v1/low-code/connectors/{id}/test` | Test connection |
| GET | `/v1/low-code/integration-jobs` | List integration jobs |
| GET | `/v1/low-code/integration-logs` | Query logs |
| GET | `/v1/low-code/audit-log` | Configuration change history |

All routes require `tenant_id` (header or query) and RBAC permissions (future: `LOW_CODE_ADMIN`, `LOW_CODE_VIEWER`).

---

## UI Draft

> **Future web-admin pages — NOT implemented.**

| Route | Purpose |
| ----- | ------- |
| `/settings/low-code` | Overview hub: active configs, publish status, quick links |
| `/settings/forms` | Form Builder: schemas, custom fields, preview |
| `/settings/processes` | BPMN Process Builder: templates, versions, publish |
| `/settings/rules` | Rule Engine: rule sets, editor, simulation |
| `/settings/connectors` | Connector catalog + tenant instances |
| `/settings/integration-logs` | Job history, errors, replay |
| `/settings/config-audit` | Full configuration audit trail |
| `/settings/templates` | Global / marketplace template browser |

Existing operational pages (`/transport-orders`, `/rfx`, etc.) will **consume** rendered forms from configuration; they are not replaced.

---

## Security Guardrails

Non-negotiable constraints for the Low-code Layer:

| # | Guardrail |
| - | --------- |
| 1 | Low-code **cannot bypass RBAC** — all actions checked against `identity-service` roles |
| 2 | Low-code **cannot access another tenant's** configuration or data |
| 3 | Low-code **cannot change protected financial statuses** directly (billing close, payment, UPD creation bypass) |
| 4 | Low-code **cannot create/sign documents** without `document-service` domain validation |
| 5 | Low-code **cannot execute arbitrary code** — declarative DSL only |
| 6 | All rules are **declarative / safe DSL** — no scripting engine |
| 7 | **Every configuration change** → `configuration_audit_log` |
| 8 | **Production config requires publish** (and optional approval) step |
| 9 | **Versioning and rollback** mandatory for process, form, rule, mapping configs |
| 10 | **Secrets never in Git** or plain DB columns — vault references only |

Implementation pattern: Low-code Runtime acts as an **orchestrator** calling existing service APIs — same as smoke test scripts call services today, but driven by tenant configuration.

---

## Tenant Isolation

| Rule | Implementation |
| ---- | -------------- |
| Config bound to tenant | `tenant_id` on all tenant-scoped rows |
| Platform global templates | `tenant_id IS NULL`, managed by `PLATFORM_ADMIN` |
| Tenant customization | Copy global template → tenant draft → publish |
| Cross-tenant visibility | **Denied** at API and DB RLS layer |
| Connector credentials | Per `connector_instances.tenant_id`, encrypted |
| Audit log | Filtered by `tenant_id`; platform admin sees global only |

Aligns with existing multi-tenant model in `core.tenants` and service-level `tenant_id` query parameters.

---

## Risks

| Risk | Mitigation |
| ---- | ---------- |
| Over-scoping MVP | Strict v0.1 = custom fields + form schemas only |
| Replacing core domain with low-code | Architecture doc + code review gate; domain owns state machines |
| Security from rule execution | Safe DSL, no eval, action whitelist |
| Performance overhead | Cache published configs; async rule evaluation |
| Migration complexity | Dedicated `lowcode` schema; phased migrations |
| Integration duplicates | Idempotency keys + connector job dedup |
| UI builder complexity | Start read-only config UI; Form Builder v0.2 |
| Customer customization chaos | Governance: publish approval, naming conventions, template catalog |

---

## Relationship to Core TMS

| Core module | Low-code enhances | Low-code does NOT |
| ----------- | ----------------- | ----------------- |
| Transport orders | Custom fields, approval process overlay | Change submit validation invariants |
| RFx / freight requests | Templates, tender process, winner rules | Accept bid without domain checks |
| Bids | Custom bid lines, evaluation rules | Bypass carrier uniqueness constraints |
| Shipments | Checkpoints, SLA alerts | Illegal status jumps |
| Documents | Signing process orchestration | Skip signature domain rules |
| Billing / UPD | Close checklist, validation rules | Create UPD without approved register |
| Companies / RBAC | Extended profile fields | Grant roles not in RBAC model |

Reference flows validated today: `tests/integration/smoke-test.sh` (full TO → FR → bid → shipment → document → billing → UPD) remains the **regression baseline** for any Low-code Runtime change.

---

## Recommended Next Action

1. **Do not write Low-code service code yet** — stabilize Core TMS / RFx / Billing first.
2. **This roadmap is the baseline** — review with product + architecture stakeholders.
3. **Next pack:** `Low-code Data Model Design Pack v0.1` — detailed ERD, RLS, indexing for `lowcode` schema.
4. **First MVP implementation (after data model):** Custom Fields + Form Templates (Configuration Foundation v0.1).
5. **Defer** BPMN Process Builder, full Rule Engine, and Connectors until v0.1 MVP is deployed and smoke-tested.
6. **Keep** `make integration-smoke-test` green after every Low-code increment — domain guardrails must not regress.

---

## Related Documentation

| Document | Relevance |
| -------- | --------- |
| `docs/PROJECT_MAP.md` | Current service layout |
| `docs/AUTH_RBAC.md` | Role model for process task bindings |
| `docs/CONTROL_TOWER.md` | Operational dashboard (SLA rules target) |
| `docs/DEMO_SEED.md` | Demo data for UI verification of future settings pages |
| `docs/CI.md` | CI baseline for future low-code service |
| `AGENTS.md` | AI working rules (no domain rewrite) |

---

## Document History

| Version | Date | Author | Notes |
| ------- | ---- | ------ | ----- |
| v0.1 | 2026-06-22 | Roadmap pack | Initial Low-code Layer roadmap — docs only |
