# Low-code Custom Fields Technical Design v0.1

Date: 2026-06-22  
Project: `D:\Projects\freight-platform`  
Status: **Technical design only — no code, no migrations, no UI changes**  
Related:

- `docs/LOW_CODE_LAYER_ROADMAP_V0.1.md`
- `docs/LOW_CODE_DATA_MODEL_DESIGN_V0.1.md`
- `docs/LOW_CODE_MVP_SCOPE_V0.1.md`

Git baseline: `7e812fd`

---

## Summary

This document defines the **technical design** for the first Low-code MVP implementation:

**Custom Fields + Form Templates + Simple Rules + Audit Log + Tenant Isolation**

Key points:

- **Implementation comes later** — this pack is design-only.
- MVP must be **safe**, **tenant-isolated**, and **non-invasive** to Core TMS.
- Custom fields **extend** core entities via `custom_field_values` — they do **not** ALTER core domain tables.
- **Core business validation** remains in existing Go services (`transport-order-service`, `rfx-service`, etc.).
- Low-code validates **custom field payloads** and **declarative rules** only.

Future schema namespace: `lowcode` (PostgreSQL). Service: `low-code-service` (recommended, not yet created).

---

## Goals

| # | Goal |
| - | ---- |
| 1 | Allow tenant admin to define **additional fields** on supported entity types |
| 2 | Use **form templates** to layout system + custom fields per entity type |
| 3 | Store custom field **values separately** from core entity columns |
| 4 | Support **simple validation and visibility rules** (declarative JSON DSL) |
| 5 | **Log every configuration change** in append-only audit log |
| 6 | Preserve existing flows: transport orders, RFx, shipments, documents, billing registers |
| 7 | Keep `make integration-smoke-test` green after any future implementation |

---

## Non-goals

Explicitly **out of scope** for this technical design / MVP v0.1 implementation:

| Non-goal | Deferred to |
| -------- | ----------- |
| BPMN process runtime | MVP v0.2 |
| Full visual drag-and-drop Form Builder | Phase 6 UI |
| Advanced Rule Engine (pricing, carrier selection) | MVP v0.3 |
| No-code Connectors | MVP v0.4 |
| Pricing / tariff engine | MVP v0.3 |
| ALTER core domain tables for custom columns | Never (EAV/JSONB pattern) |
| Override protected business statuses | Never |
| Arbitrary code / script execution | Never |
| Global template marketplace | MVP v0.4 |
| Production OpenAPI contract changes today | Next pack (API contract draft) |

---

## Service Boundary Options

### Option A: New `low-code-service`

| Pros | Cons |
| ---- | ---- |
| Clear ownership of configuration domain | New service to deploy and monitor |
| Single place for forms, rules, audit, future BPMN/connectors | New gateway routes and health checks |
| Easier governance (versioning, publish, audit) | Additional Docker Compose entry |
| Aligns with long-term Low-code roadmap | Initial bootstrap cost |

### Option B: Add to `company-service` or `identity-service`

| Pros | Cons |
| ---- | ---- |
| Faster initial bootstrap | Violates single-responsibility |
| Fewer containers | Low-code is not identity/company domain |
| Reuse existing DB connections | Hard to add BPMN/connectors later |
| | Audit/versioning mixed with unrelated domains |

### Option C: Shared Go package + per-service storage

| Pros | Cons |
| ---- | ---- |
| Custom fields live next to entity code | Duplicated schema/logic across 6+ services |
| No new HTTP service | No unified config registry or audit |
| | Tenant isolation harder to enforce consistently |
| | Form templates fragmented per service |

---

## Recommended Service Boundary

**Recommendation: Option A — new `low-code-service`**

Rationale:

- Low-code is a **platform configuration domain**, not a transport/Rfx/billing concern.
- Owns: custom fields, form templates, simple rules, configuration audit.
- Future modules (BPMN, connectors, marketplace) extend the same service naturally.
- Core services call low-code via **API Gateway** (`/v1/low-code/*`) or internal HTTP client — they never own config tables.

**v0.1 note:** Design and migrations can be prepared **before** the service binary exists. Phase 1 creates skeleton only.

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────────┐
│  web-admin  │────▶│ api-gateway  │────▶│  low-code-service   │
└─────────────┘     └──────┬───────┘     │  (config + values)  │
                           │             └──────────┬──────────┘
                           │                        │
                           ▼                        │ validate custom fields
              ┌────────────────────────┐           │
              │ transport-order-service│◀──────────┘ (optional callback)
              │ rfx-service            │
              │ shipment-service       │
              │ document-service       │
              │ billing-register-service│
              │ company-service        │
              └────────────────────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │ PostgreSQL             │
              │ schemas: core, transport│
              │ rfx, documents, billing │
              │ lowcode (NEW)          │
              └────────────────────────┘
```

---

## Proposed Module Name

| Aspect | Name |
| ------ | ---- |
| Service directory | `services/low-code-service` |
| Docker Compose service | `low-code-service` |
| Default port (suggested) | `8088` |
| API prefix | `/v1/low-code` |
| PostgreSQL schema | `lowcode` |
| UI section (future) | **Platform Configuration Studio** |
| RU product name | **Конструктор процессов 7Rights** |
| Makefile target (future) | `run-low-code-service`, `test-low-code-service` |
| Feature flag (future) | `LOW_CODE_ENABLED=true` |

---

## Entity Types Supported in v0.1

| Entity type | Core service | Why custom fields |
| ----------- | ------------ | ----------------- |
| `TRANSPORT_ORDER` | `transport-order-service` | Loading window, cargo class, internal cost center, incoterms extension, shipper PO number |
| `RFX` | `rfx-service` | Tender strategy, evaluation criteria weights, category tags |
| `FREIGHT_REQUEST` | `rfx-service` | Lane notes, service level, packaging requirements |
| `SHIPMENT` | `shipment-service` | Temperature mode, loading contact, seal number, customs reference |
| `DOCUMENT` | `document-service` | Document category, external archive ref, signing route metadata |
| `BILLING_REGISTER` | `billing-register-service` | Cost allocation code, approval group, GL mapping reference |
| `COMPANY_PROFILE` | `company-service` | Compliance attributes (ISO cert expiry), industry codes, credit limit notes |

**Not in v0.1:** `BID` (read-only extensions deferred), `DRIVER_TASK`, dashboard widgets.

Each entity type maps to one **published form template** per tenant in v0.1 (latest published wins).

---

## Data Model Draft

> Tables live in schema `lowcode`. **No migrations in this pack.**

### `low_code_configurations`

**Purpose:** Unified registry of all low-code configuration artifacts.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `code` | TEXT NOT NULL | Stable identifier |
| `name` | TEXT NOT NULL | |
| `description` | TEXT | |
| `config_type` | TEXT NOT NULL | `FORM_TEMPLATE`, `RULE_SET`, … |
| `status` | TEXT NOT NULL | `DRAFT`, `REVIEW`, `PUBLISHED`, `ARCHIVED` |
| `version` | INT NOT NULL | Monotonic per `code` |
| `created_by_user_id` | UUID | |
| `updated_by_user_id` | UUID | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |
| `published_at` | TIMESTAMPTZ | |
| `archived_at` | TIMESTAMPTZ | |

**Constraints:**

- `UNIQUE (tenant_id, code, version)`
- `CHECK (status IN ('DRAFT','REVIEW','PUBLISHED','ARCHIVED'))`

**Indexes:** `(tenant_id)`, `(tenant_id, config_type)`, `(tenant_id, status)`

---

### `form_templates`

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `configuration_id` | UUID NOT NULL FK | → `low_code_configurations` |
| `entity_type` | TEXT NOT NULL | See entity types above |
| `code` | TEXT NOT NULL | |
| `name` | TEXT NOT NULL | |
| `description` | TEXT | |
| `status` | TEXT NOT NULL | |
| `version` | INT NOT NULL | |
| `locale_default` | TEXT | Default `ru-RU` |
| `created_by_user_id` | UUID | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |
| `published_at` | TIMESTAMPTZ | |

**Constraints:** `UNIQUE (tenant_id, entity_type, code, version)`

---

### `form_sections`

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `form_template_id` | UUID NOT NULL FK | |
| `code` | TEXT NOT NULL | |
| `title` | TEXT NOT NULL | |
| `description` | TEXT | |
| `sort_order` | INT NOT NULL | |
| `visibility_rule_id` | UUID NULL | FK → `rules` (optional) |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**Constraints:** `UNIQUE (form_template_id, code)`

---

### `form_fields`

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `form_template_id` | UUID NOT NULL FK | |
| `section_id` | UUID NULL FK | → `form_sections` |
| `code` | TEXT NOT NULL | Stable field key |
| `label` | TEXT NOT NULL | |
| `field_type` | TEXT NOT NULL | See field types section |
| `required` | BOOLEAN NOT NULL DEFAULT false | Custom fields only |
| `read_only` | BOOLEAN NOT NULL DEFAULT false | |
| `system_field` | BOOLEAN NOT NULL DEFAULT false | Protected |
| `default_value_json` | JSONB | |
| `options_json` | JSONB | SELECT options |
| `validation_rule_json` | JSONB | Inline rule or ref |
| `visibility_rule_json` | JSONB | Inline rule or ref |
| `localization_json` | JSONB | `{ "ru-RU": { "label": "..." } }` |
| `sort_order` | INT NOT NULL | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**Constraints:**

- `UNIQUE (tenant_id, form_template_id, code)`
- Application guard: `system_field = true` → cannot DELETE, cannot set `required = false` for domain-mandatory fields

---

### `custom_field_values`

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `entity_type` | TEXT NOT NULL | |
| `entity_id` | UUID NOT NULL | Polymorphic logical ref |
| `form_template_id` | UUID | Template version at save time |
| `field_id` | UUID NOT NULL FK | → `form_fields` |
| `field_code` | TEXT NOT NULL | Denormalized for queries |
| `value_json` | JSONB | Typed envelope |
| `created_by_user_id` | UUID | |
| `updated_by_user_id` | UUID | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**Indexes:**

- `(tenant_id, entity_type, entity_id)` — primary lookup
- `(tenant_id, field_id)`
- `(tenant_id, field_code)`
- `UNIQUE (tenant_id, entity_type, entity_id, field_id)`

**Polymorphic reference notes:**

- **No FK** from `entity_id` to all domain tables — integrity enforced at write time.
- On save: low-code-service (or gateway orchestration) verifies entity exists in owning service **and** `entity.tenant_id == tenant_id`.
- **Risk:** orphaned values if core entity hard-deleted → mitigation: soft-delete sync event or periodic cleanup job.
- **Risk:** cross-tenant write if validation skipped → mitigation: mandatory tenant check on every API path.

---

### `rule_sets`

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `configuration_id` | UUID FK | |
| `code` | TEXT NOT NULL | |
| `name` | TEXT NOT NULL | |
| `rule_set_type` | TEXT NOT NULL | `VALIDATION`, `VISIBILITY` for v0.1 |
| `status` | TEXT NOT NULL | |
| `version` | INT NOT NULL | |
| `created_by_user_id` | UUID | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

---

### `rules`

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rule_set_id` | UUID NOT NULL FK | |
| `code` | TEXT NOT NULL | |
| `name` | TEXT NOT NULL | |
| `priority` | INT NOT NULL | Lower = higher priority |
| `condition_json` | JSONB NOT NULL | Safe DSL |
| `action_json` | JSONB NOT NULL | Whitelisted actions |
| `enabled` | BOOLEAN NOT NULL DEFAULT true | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

---

### `configuration_audit_log`

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `configuration_id` | UUID | |
| `entity_type` | TEXT NOT NULL | Table/artifact name |
| `entity_id` | UUID | Row ID |
| `action` | TEXT NOT NULL | CREATE, UPDATE, PUBLISH, … |
| `old_value_json` | JSONB | |
| `new_value_json` | JSONB | |
| `changed_by_user_id` | UUID | |
| `request_id` | TEXT | Correlation ID |
| `ip_address` | TEXT | |
| `user_agent` | TEXT | |
| `changed_at` | TIMESTAMPTZ | |

Append-only. Partition by month in production (future).

---

## Field Types and Validation

| field_type | value_json envelope | Validation | UI rendering (future) |
| ---------- | ------------------- | ---------- | ----------------------- |
| `TEXT` | `{ "type":"TEXT", "value":"..." }` | maxLength, pattern regex | `<input type="text">` |
| `NUMBER` | `{ "type":"NUMBER", "value": 123.45 }` | min, max, integer | number input |
| `DATE` | `{ "type":"DATE", "value":"2026-08-01" }` | ISO date | date picker |
| `DATETIME` | `{ "type":"DATETIME", "value":"2026-08-01T09:00:00Z" }` | ISO8601 | datetime picker |
| `SELECT` | `{ "type":"SELECT", "value":"OPT1" }` | enum in `options_json` | dropdown |
| `MULTI_SELECT` | `{ "type":"MULTI_SELECT", "value":["A","B"] }` | subset of options | multi-select (v0.1 optional) |
| `CHECKBOX` | `{ "type":"CHECKBOX", "value": true }` | boolean | checkbox |
| `FILE` | `{ "type":"FILE", "document_id":"uuid" }` | ref exists in document-service | file upload / doc picker |
| `COMPANY_REFERENCE` | `{ "type":"COMPANY_REFERENCE", "company_id":"uuid" }` | company in tenant | company autocomplete |
| `ROUTE` | `{ "type":"ROUTE", "origin_id":"...", "dest_id":"..." }` | location refs | route widget (v0.2 UI) |
| `ADDRESS` | `{ "type":"ADDRESS", "lines":[], "city":"..." }` | required subfields | address block (v0.2 UI) |
| `VEHICLE` | `{ "type":"VEHICLE", "vehicle_id":"uuid" }` | vehicle in tenant | vehicle picker (v0.2 UI) |
| `MONEY` | `{ "type":"MONEY", "amount":1000, "currency":"RUB" }` | amount ≥ 0, currency ISO | money input |
| `CURRENCY` | `{ "type":"CURRENCY", "code":"RUB" }` | ISO 4217 | currency select |
| `VAT_TAX` | `{ "type":"VAT_TAX", "rate":20 }` | 0–100 | tax rate input |
| `DOCUMENT_REFERENCE` | `{ "type":"DOCUMENT_REFERENCE", "document_id":"uuid" }` | doc in tenant | document picker |

**MVP v0.1 implementation priority:** TEXT, NUMBER, DATE, DATETIME, SELECT, CHECKBOX, FILE, MONEY, COMPANY_REFERENCE, DOCUMENT_REFERENCE.

---

## Simple Rules v0.1

Safe rule categories only:

| # | Rule type | Purpose |
| - | --------- | ------- |
| 1 | Required rule | Conditional required fields |
| 2 | Visibility rule | Show/hide field or section |
| 3 | Read-only rule | Lock field by status/role |
| 4 | Value range rule | min/max for NUMBER/MONEY |
| 5 | Enum allowed values | SELECT validation |
| 6 | Role-based visibility | `role IN [...]` |
| 7 | Status-based lock | `entity.status >= X` → read-only |

**Format constraints:**

- JSON DSL only
- No scripting, no SQL, no arbitrary code
- No direct DB mutation actions
- Action whitelist: `required`, `visible`, `read_only`, `validation_error`, `notify_draft`

**Example DSL:**

```json
{
  "if": {
    "field": "cargo_class",
    "equals": "HAZMAT"
  },
  "then": {
    "required": ["hazmat_certificate_file"]
  }
}
```

**Role-based visibility example:**

```json
{
  "if": {
    "context.role": "CARRIER_DISPATCHER"
  },
  "then": {
    "visible": ["carrier_ops_section"]
  }
}
```

**Status-based lock example:**

```json
{
  "if": {
    "context.entity_status": { "in": ["IN_TRANSIT", "DELIVERED", "READY_FOR_BILLING"] }
  },
  "then": {
    "read_only": ["loading_contact_phone"]
  }
}
```

Rules may be stored inline in `form_fields.validation_rule_json` / `visibility_rule_json` or referenced from `rules` table via `rule_sets`.

---

## Validation Flow

### Create flow

```
1. Client: GET /v1/low-code/form-templates?entity_type=TRANSPORT_ORDER&status=PUBLISHED
2. Client: render system fields + custom fields from template
3. User: submits { core_payload, custom_fields: { field_code: value } }
4. API Gateway → core service: validate & create core entity
5. If core OK → low-code-service: POST /v1/low-code/validate (pre-save check)
6. low-code-service: validate types, rules, tenant match
7. low-code-service: PUT /v1/low-code/custom-field-values
8. low-code-service: append configuration_audit_log (if config changed) / value audit (optional v0.1.1)
9. Return combined response { entity, custom_fields }
```

**Order option (stricter):** validate custom fields **before** core create via gateway orchestration — prevents orphan core rows without custom values.

### Update flow

```
1. Load published template + existing custom_field_values
2. Merge user edits
3. Core service: PATCH core entity (domain validation)
4. low-code-service: validate custom fields (respect read_only rules for current status)
5. Upsert custom_field_values
6. Audit value changes (optional dedicated audit entity_type = custom_field_values)
```

### Read / list flow

```
1. Core service: GET entity (list or detail)
2. Client or BFF: GET /v1/low-code/custom-field-values?entity_type=&entity_id=
3. Merge for display
```

List pages: batch API `GET /v1/low-code/custom-field-values/batch?entity_type=&entity_ids=id1,id2,...` (future optimization).

### Error response format (draft)

```json
{
  "error": {
    "code": "VALIDATION_RULE_FAILED",
    "message": "Field hazmat_certificate_file is required when cargo_class is HAZMAT",
    "details": {
      "field": "hazmat_certificate_file",
      "rule_code": "REQ_HAZMAT_CERT"
    }
  }
}
```

HTTP status: `422 Unprocessable Entity` for validation errors; `409 Conflict` for publish/version conflicts.

---

## Integration with Existing Services

Integration is **read/consume + validate** — core services are **not modified in v0.1 design implementation** beyond optional gateway composition layer.

### `transport-order-service`

- Reads custom fields for `TRANSPORT_ORDER` via low-code API on detail views.
- Custom fields **cannot** change `order_number`, status transitions, or submit rules.
- Status changes remain domain-only (`POST .../submit`).

### `rfx-service`

- Custom fields on `RFX`, `FREIGHT_REQUEST` (and optionally `BID` read-only later).
- **No override** of bid accept/winner selection in v0.1.

### `shipment-service`

- Custom fields on `SHIPMENT`.
- **No bypass** of shipment lifecycle (`PATCH .../status` domain machine unchanged).

### `document-service`

- Custom fields on `DOCUMENT`.
- **No bypass** of signing workflow or `document_status` transitions.

### `billing-register-service`

- Custom fields on `BILLING_REGISTER`.
- **No bypass** of calculate → approve → UPD → close sequence (smoke-tested flow preserved).

### `company-service`

- Custom fields on `COMPANY_PROFILE`.
- Compliance / industry extension attributes.

### Integration pattern (recommended)

| Pattern | v0.1 | Notes |
| ------- | ---- | ----- |
| **Gateway orchestration** | ✅ Preferred | Gateway calls core + low-code in sequence |
| **Core service embeds low-code client** | ❌ Defer | Couples domains |
| **Async event (entity created → save custom fields)** | ⚠️ Optional | Risk of inconsistency without outbox |

---

## API Draft

> **Future routes — NOT part of current OpenAPI. Draft only.**

### Form Templates

| Method | Route | Purpose |
| ------ | ----- | ------- |
| GET | `/v1/low-code/form-templates?entity_type=TRANSPORT_ORDER` | List templates |
| POST | `/v1/low-code/form-templates` | Create draft |
| GET | `/v1/low-code/form-templates/{id}` | Get with sections/fields |
| PUT | `/v1/low-code/form-templates/{id}` | Update draft |
| POST | `/v1/low-code/form-templates/{id}/publish` | Publish version |
| POST | `/v1/low-code/form-templates/{id}/archive` | Archive |

Headers: `Authorization`, `X-Tenant-ID` (same as web-admin today).

### Form Fields

| Method | Route | Purpose |
| ------ | ----- | ------- |
| POST | `/v1/low-code/form-templates/{id}/fields` | Add field |
| PUT | `/v1/low-code/form-fields/{field_id}` | Update field |
| DELETE | `/v1/low-code/form-fields/{field_id}` | Delete (custom only) |

### Custom Field Values

| Method | Route | Purpose |
| ------ | ----- | ------- |
| GET | `/v1/low-code/custom-field-values?entity_type=&entity_id=` | Read values |
| PUT | `/v1/low-code/custom-field-values` | Upsert batch `{ entity_type, entity_id, values: {} }` |

### Validation

| Method | Route | Purpose |
| ------ | ----- | ------- |
| POST | `/v1/low-code/validate` | Dry-run validation without save |

### Audit

| Method | Route | Purpose |
| ------ | ----- | ------- |
| GET | `/v1/low-code/audit-log?configuration_id=&limit=` | Query audit trail |

### Registry (optional v0.1)

| Method | Route | Purpose |
| ------ | ----- | ------- |
| GET | `/v1/low-code/configurations` | List all configs for tenant |

---

## UI Draft

> **Future web-admin pages — not implemented in this pack.**

### Settings / admin pages

| Route | Purpose |
| ----- | ------- |
| `/settings/low-code` | Hub: config summary, links |
| `/settings/forms` | List form templates |
| `/settings/forms/:id` | Template detail, sections |
| `/settings/forms/:id/fields` | Field editor |
| `/settings/rules` | Rule sets for template |
| `/settings/config-audit` | Audit log viewer |

### Entity page integration

| Existing page | Integration |
| ------------- | ----------- |
| `/transport-orders/:id` | Extra "Custom fields" section from template |
| `/freight-requests/:id` | Same pattern |
| `/shipments/:id` | Same pattern |
| `/documents/:id` | Same pattern |
| `/billing-registers/:id` | Same pattern |
| `/companies/:id` | Company profile custom fields |

**Phase 5:** read-only preview of custom field values.  
**Phase 6:** editable fields + admin Form Builder.

---

## Tenant Isolation

| Rule | Enforcement |
| ---- | ----------- |
| Every config row has `tenant_id` | NOT NULL constraint |
| All API queries filter by tenant | Service + gateway |
| Custom field values tenant match entity tenant | Validate on write via core entity lookup |
| No cross-tenant templates | Unique constraints scoped to tenant |
| Platform global templates | **Deferred** — v0.1 tenant-only to reduce complexity |
| RBAC | New roles TBD: `LOW_CODE_ADMIN`, `LOW_CODE_VIEWER` (design in AUTH_RBAC extension pack) |

---

## Security Guardrails

| # | Guardrail |
| - | --------- |
| 1 | Custom fields **cannot override** system fields |
| 2 | `system_field = true` → read-only in low-code config UI |
| 3 | Low-code **cannot bypass RBAC** |
| 4 | Low-code **cannot change protected statuses** (billing, shipment, document) |
| 5 | Low-code **cannot approve** billing registers or create UPD |
| 6 | Low-code **cannot sign** documents |
| 7 | **No arbitrary code** execution |
| 8 | Rules **cannot run SQL** or shell commands |
| 9 | **Secrets not** in custom fields MVP |
| 10 | **Every config change audited** |
| 11 | Published templates **immutable** — edits create new draft version |
| 12 | `make integration-smoke-test` must pass unchanged |

---

## Error Handling

Standard error codes (align with existing `AppError` pattern in services):

| Code | HTTP | When |
| ---- | ---- | ---- |
| `FIELD_REQUIRED` | 422 | Missing required custom field |
| `FIELD_INVALID_TYPE` | 422 | value_json type mismatch |
| `FIELD_VALUE_OUT_OF_RANGE` | 422 | NUMBER/MONEY bounds |
| `FIELD_NOT_ALLOWED` | 422 | Field not in published template |
| `FORM_TEMPLATE_NOT_PUBLISHED` | 422 | Runtime uses draft template |
| `CONFIGURATION_ARCHIVED` | 409 | Config no longer active |
| `TENANT_MISMATCH` | 403 | entity tenant ≠ request tenant |
| `SYSTEM_FIELD_PROTECTED` | 403 | Attempt to modify/delete system field def |
| `VALIDATION_RULE_FAILED` | 422 | Rule DSL evaluation failed |
| `LOW_CODE_DISABLED` | 503 | Feature flag off |

---

## Migration Strategy Later

> **Not executed now.**

1. Create migration `0000XX_create_lowcode_schema.up.sql` — config tables only (9 tables).
2. **Do not ALTER** core domain tables.
3. Seed **default read-only templates** per entity type (platform defaults, tenant copy on first login — optional).
4. Add env `LOW_CODE_ENABLED=false` default in compose.
5. Deploy **read-only GET APIs** first.
6. Enable write APIs behind flag + role.
7. UI read-only preview → editable builder.

Rollback: drop `lowcode` schema migration down script (non-prod only); prod uses archive not drop.

---

## Testing Strategy Later

| Test type | Scope |
| --------- | ----- |
| Unit | JSON DSL parser, field type validators, rule evaluator |
| Repository | CRUD + tenant isolation + unique constraints |
| API | Form template publish, custom value upsert, validate endpoint |
| Tenant isolation | Cross-tenant read/write must fail |
| Audit | Every publish creates audit row |
| Integration | Create transport order + custom fields end-to-end |
| Regression | `make integration-smoke-test` unchanged and green |
| Performance | Batch read custom values for list page (100 entities) |

---

## Rollout Plan

| Phase | Deliverable | Type |
| ----- | ----------- | ---- |
| **Phase 0** | Docs: roadmap, data model, MVP scope, **this technical design** | ✅ Done |
| **Phase 1** | Migrations + `low-code-service` skeleton + health | Infra + backend |
| **Phase 2** | Form templates CRUD + publish API | Backend |
| **Phase 3** | Custom field values read/write API | Backend |
| **Phase 4** | Validation engine MVP (`/validate`) | Backend |
| **Phase 5** | Admin UI read-only preview | Frontend |
| **Phase 6** | Editable Form Builder MVP | Frontend |
| **Phase 7** | Entity detail page integration | Frontend + BFF |

---

## Risks

| Risk | Mitigation |
| ---- | ---------- |
| Polymorphic `entity_id` hard to query | Denormalize `field_code`; batch APIs; optional materialized view |
| JSONB indexing | GIN index on hot paths; limit custom fields per entity |
| Dynamic forms complexity | Start read-only; schema-driven renderer component |
| Tenant isolation bugs | RLS + integration tests + mandatory tenant middleware |
| Rule DSL creep | Strict schema validation; action whitelist; review gate |
| List page performance | Batch fetch custom values; cache published templates |
| Audit log growth | Monthly partitions; retention policy |
| Gateway orchestration failures | Transactional outbox or compensating delete on partial failure |

---

## Recommended Next Action

1. **Review and approve** this technical design with architecture/product stakeholders.
2. **Next pack:** `Low-code Custom Fields Migration Design Pack v0.1` — SQL draft, indexes, RLS policies, seed data.
3. **Then:** API contract draft (`packages/openapi` extension or separate low-code openapi file).
4. **Then:** Implement `low-code-service` skeleton (health, metrics, empty routes) — **only after migration design approved**.
5. **Always:** Run `make integration-smoke-test` after each backend increment.

---

## Related Documentation

| Document | Role |
| -------- | ---- |
| `docs/LOW_CODE_MVP_SCOPE_V0.1.md` | Functional MVP boundaries |
| `docs/LOW_CODE_DATA_MODEL_DESIGN_V0.1.md` | Full entity catalog including future BPMN/connectors |
| `docs/LOW_CODE_LAYER_ROADMAP_V0.1.md` | Strategic modules |
| `docs/AUTH_RBAC.md` | Role model extension point |
| `tests/integration/smoke-test.sh` | Regression baseline |

---

## Document History

| Version | Date | Notes |
| ------- | ---- | ----- |
| v0.1 | 2026-06-22 | Initial custom fields technical design — docs only |
