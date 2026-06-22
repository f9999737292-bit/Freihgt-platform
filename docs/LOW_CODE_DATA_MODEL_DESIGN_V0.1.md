# Low-code Data Model Design v0.1

Date: 2026-06-22  
Project: `D:\Projects\freight-platform`  
Status: **Design document only — no migrations, no services**  
Related: `docs/LOW_CODE_LAYER_ROADMAP_V0.1.md`  
Git baseline: `35d8bd3`

---

## Summary

This document defines the **future PostgreSQL data model** for the 7Rights Low-code Layer: forms, processes, rules, connectors, runtime events, and governance.

Important constraints for this pack:

- **No tables are created** in this step.
- **No migrations** are written.
- **No backend services or API contracts** are changed.
- The design is required **before** MVP implementation (Custom Fields + Form Templates + simple rules).

Every entity in this design assumes:

- **Tenant isolation** (`tenant_id` on tenant-owned rows)
- **Versioning** for production configuration
- **Draft → review → published → archived** lifecycle where applicable
- **Audit log** for every configuration change
- **Security guardrails** — low-code cannot bypass Core TMS domain validation

Suggested future schema namespace: `lowcode` (alongside existing `core`, `transport`, `rfx`, `documents`, `billing`).

---

## Design Principles

| Principle | Description |
| --------- | ----------- |
| **Tenant-first design** | Every tenant-owned row carries `tenant_id`; queries always filter by tenant |
| **No cross-tenant access** | RLS or service-layer enforcement; no shared mutable config between tenants |
| **Declarative configuration** | JSON / safe DSL only — no arbitrary code stored or executed |
| **Versioning** | Published configs are immutable snapshots; edits create new draft versions |
| **Draft / published / archived lifecycle** | Production runtime reads **published** versions only |
| **Audit log** | Append-only log for CREATE, UPDATE, PUBLISH, ROLLBACK, etc. |
| **Rollback support** | Restore prior published version without losing history |
| **Protected core domain** | Low-code orchestrates API calls; cannot bypass shipment/billing/document invariants |
| **Secrets protection** | Credentials via `credentials_secret_ref` — never plain text in DB or Git |
| **Config ≠ validation** | Core services retain authoritative business validation |

---

## Entity Groups

| # | Group | Purpose |
| - | ----- | ------- |
| 1 | Configuration Registry | Central index of all low-code artifacts per tenant |
| 2 | Form Builder | Form templates, sections, fields, custom values |
| 3 | BPMN / Process Builder | Process templates, versions, steps, transitions, runtime instances |
| 4 | Rule Engine | Rule sets, versions, rules, evaluation logs |
| 5 | No-code Connectors | Global definitions, tenant instances, mappings, jobs |
| 6 | Runtime Execution | Events and notifications emitted by low-code runtime |
| 7 | Audit and Governance | Audit log and publish approval workflow |
| 8 | Templates and Marketplace | Global templates and tenant copies |

---

## 1. Configuration Registry

### `low_code_configurations`

**Purpose:** Master registry of low-code configuration artifacts for a tenant (or platform-global when `tenant_id IS NULL`).

| Column | Type (draft) | Notes |
| ------ | ------------ | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NULL | NULL = platform-global template registry entry |
| `code` | VARCHAR | Unique per tenant (or global namespace) |
| `name` | VARCHAR | Display name |
| `description` | TEXT NULL | |
| `config_type` | VARCHAR | See enum below |
| `status` | VARCHAR | DRAFT, REVIEW, PUBLISHED, ARCHIVED |
| `version` | INTEGER | Monotonic version number |
| `owner_company_id` | UUID NULL | Optional owning company within tenant |
| `created_by_user_id` | UUID | |
| `updated_by_user_id` | UUID NULL | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |
| `published_at` | TIMESTAMPTZ NULL | |
| `archived_at` | TIMESTAMPTZ NULL | |

**`config_type` values:**

- `FORM_TEMPLATE`
- `PROCESS_TEMPLATE`
- `RULE_SET`
- `CONNECTOR`
- `DOCUMENT_TEMPLATE`
- `DASHBOARD_TEMPLATE`

**`status` values:**

- `DRAFT` — editable, not used in production runtime
- `REVIEW` — submitted for approval
- `PUBLISHED` — immutable for runtime (new edits → new version)
- `ARCHIVED` — retired, not selectable for new instances

**Indexes:**

- `(tenant_id)`
- `(tenant_id, code)` UNIQUE WHERE `tenant_id IS NOT NULL`
- `(tenant_id, config_type)`
- `(tenant_id, status)`

**Guardrails:**

- `code` unique per tenant (and per global namespace when `tenant_id IS NULL`)
- Published rows immutable or versioned via linked version tables
- All mutations → `configuration_audit_log`

---

## 2. Form Builder Data Model

### `form_templates`

**Purpose:** Form layout template bound to a domain entity type.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `configuration_id` | UUID FK → `low_code_configurations` | |
| `entity_type` | VARCHAR | See enum |
| `code` | VARCHAR | Unique per tenant + entity_type |
| `name` | VARCHAR | |
| `description` | TEXT NULL | |
| `status` | VARCHAR | DRAFT, PUBLISHED, ARCHIVED |
| `version` | INTEGER | |
| `locale_default` | VARCHAR | e.g. `ru-RU` |
| `created_by_user_id` | UUID | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |
| `published_at` | TIMESTAMPTZ NULL | |

**`entity_type` values:**

- `TRANSPORT_ORDER`
- `RFX`
- `FREIGHT_REQUEST`
- `BID`
- `SHIPMENT`
- `DOCUMENT`
- `BILLING_REGISTER`
- `COMPANY_PROFILE`
- `DRIVER_TASK`

**Indexes:** `(tenant_id, entity_type)`, `(tenant_id, code)`, `(configuration_id)`

---

### `form_sections`

**Purpose:** Grouping of fields within a form (tabs, panels).

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `form_template_id` | UUID FK → `form_templates` | |
| `code` | VARCHAR | Unique within template |
| `title` | VARCHAR | |
| `description` | TEXT NULL | |
| `sort_order` | INTEGER | |
| `visibility_rule_id` | UUID NULL FK → `rules` | Optional Rule Engine ref |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**Indexes:** `(form_template_id, sort_order)`, `(tenant_id)`

---

### `form_fields`

**Purpose:** Field definition (system + custom) within a form section.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `form_template_id` | UUID FK | |
| `section_id` | UUID FK → `form_sections` | |
| `code` | VARCHAR | Stable field key |
| `label` | VARCHAR | Default label |
| `field_type` | VARCHAR | See enum |
| `required` | BOOLEAN | Custom fields only; system fields use domain rules |
| `read_only` | BOOLEAN | |
| `system_field` | BOOLEAN | TRUE = maps to core entity column, cannot delete |
| `default_value_json` | JSONB NULL | |
| `options_json` | JSONB NULL | Select options, lookups |
| `validation_rule_id` | UUID NULL FK → `rules` | |
| `visibility_rule_id` | UUID NULL FK → `rules` | |
| `sort_order` | INTEGER | |
| `localization_json` | JSONB NULL | `{ "ru-RU": { "label": "..." }, "en-US": {...} }` |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**`field_type` values:**

`TEXT` · `NUMBER` · `DATE` · `DATETIME` · `SELECT` · `MULTI_SELECT` · `CHECKBOX` · `FILE` · `COMPANY_REFERENCE` · `ROUTE` · `ADDRESS` · `VEHICLE` · `MONEY` · `CURRENCY` · `VAT_TAX` · `DOCUMENT_REFERENCE`

**Guardrails:**

- `system_field = true` rows cannot be removed or marked optional if domain requires the field
- Custom fields (`system_field = false`) stored values go to `custom_field_values`

**Indexes:** `(form_template_id, section_id, sort_order)`, `(tenant_id, code)`

---

### `custom_field_values`

**Purpose:** Runtime storage of custom field values for a specific domain entity instance.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `entity_type` | VARCHAR | Same enum as `form_templates.entity_type` |
| `entity_id` | UUID NOT NULL | Logical reference to domain entity |
| `form_template_id` | UUID FK | Template version used at save time |
| `field_id` | UUID FK → `form_fields` | |
| `field_code` | VARCHAR | Denormalized for query performance |
| `value_json` | JSONB NOT NULL | Typed value envelope `{ "type": "MONEY", "amount": 100, "currency": "RUB" }` |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**Indexes:**

- `(tenant_id, entity_type, entity_id)` — primary lookup
- `(tenant_id, field_code, entity_type)` — reporting
- UNIQUE `(tenant_id, entity_type, entity_id, field_id)`

**Polymorphic reference note:**

- `entity_id` is a **logical** reference — no FK to all domain tables (transport_orders, shipments, etc.)
- Integrity enforced at **application layer** when saving (entity must exist in owning service)
- **Risk:** orphaned values if domain entity deleted — recommend soft-delete sync or cascade job
- **Mitigation:** periodic cleanup job; `entity_type` + `entity_id` validation API on write

---

## 3. BPMN / Process Builder Data Model

### `process_templates`

**Purpose:** Named process definition (approval, tender, billing close overlay).

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `configuration_id` | UUID FK | |
| `code` | VARCHAR | |
| `name` | VARCHAR | |
| `description` | TEXT NULL | |
| `process_type` | VARCHAR | See enum |
| `status` | VARCHAR | DRAFT, PUBLISHED, ARCHIVED |
| `active_version_id` | UUID NULL FK → `process_versions` | Points to current published version |
| `created_by_user_id` | UUID | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**`process_type` values:**

- `TRANSPORT_ORDER_APPROVAL`
- `RFX_TENDER`
- `BID_EVALUATION`
- `SHIPMENT_LIFECYCLE`
- `DOCUMENT_SIGNING`
- `BILLING_CLOSING`
- `CLAIMS_DISPUTE`

---

### `process_versions`

**Purpose:** Immutable snapshot of process definition (BPMN XML + normalized JSON).

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `process_template_id` | UUID FK | |
| `version` | INTEGER | |
| `status` | VARCHAR | DRAFT, PUBLISHED, ARCHIVED |
| `bpmn_xml` | TEXT NULL | Optional BPMN 2.0 XML export |
| `definition_json` | JSONB NOT NULL | Normalized internal model |
| `published_by_user_id` | UUID NULL | |
| `published_at` | TIMESTAMPTZ NULL | |
| `created_at` | TIMESTAMPTZ | |

**Indexes:** `(process_template_id, version)` UNIQUE, `(tenant_id, status)`

---

### `process_steps`

**Purpose:** Step/node within a process version.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `process_version_id` | UUID FK | |
| `code` | VARCHAR | |
| `name` | VARCHAR | |
| `step_type` | VARCHAR | See enum |
| `assigned_role` | VARCHAR NULL | RBAC role code |
| `assigned_user_rule_id` | UUID NULL FK → `rules` | Dynamic assignee |
| `timeout_rule_id` | UUID NULL FK → `rules` | SLA escalation |
| `sort_order` | INTEGER NULL | For linear display |
| `metadata_json` | JSONB NULL | UI hints, form bindings |

**`step_type` values:**

`START` · `TASK` · `APPROVAL` · `SYSTEM_ACTION` · `WAIT` · `NOTIFICATION` · `END`

**Indexes:** `(process_version_id)`, `(tenant_id)`

---

### `process_transitions`

**Purpose:** Directed edge between steps with optional conditions.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `process_version_id` | UUID FK | |
| `from_step_id` | UUID FK → `process_steps` | |
| `to_step_id` | UUID FK → `process_steps` | |
| `condition_rule_id` | UUID NULL FK → `rules` | |
| `action_rule_id` | UUID NULL FK → `rules` | Side effects (notify, call API) |
| `transition_label` | VARCHAR NULL | |
| `metadata_json` | JSONB NULL | |

**Indexes:** `(process_version_id, from_step_id)`

---

### `process_instances`

**Purpose:** Running or completed process bound to a domain entity.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `process_template_id` | UUID FK | |
| `process_version_id` | UUID FK | Locked at start |
| `entity_type` | VARCHAR | |
| `entity_id` | UUID | Polymorphic ref |
| `status` | VARCHAR | RUNNING, COMPLETED, CANCELLED, FAILED |
| `current_step_id` | UUID NULL FK → `process_steps` | |
| `started_by_user_id` | UUID | |
| `started_at` | TIMESTAMPTZ | |
| `completed_at` | TIMESTAMPTZ NULL | |
| `cancelled_at` | TIMESTAMPTZ NULL | |

**Indexes:** `(tenant_id, entity_type, entity_id)`, `(tenant_id, status)`, `(process_template_id)`

---

### `process_tasks`

**Purpose:** User/system task within a process instance.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `process_instance_id` | UUID FK | |
| `step_id` | UUID FK → `process_steps` | |
| `assigned_user_id` | UUID NULL | |
| `assigned_role` | VARCHAR NULL | |
| `status` | VARCHAR | PENDING, IN_PROGRESS, COMPLETED, CANCELLED, EXPIRED |
| `due_at` | TIMESTAMPTZ NULL | |
| `completed_by_user_id` | UUID NULL | |
| `completed_at` | TIMESTAMPTZ NULL | |
| `result_json` | JSONB NULL | Outcome, comments |
| `created_at` | TIMESTAMPTZ | |

**Indexes:** `(process_instance_id, status)`, `(tenant_id, assigned_user_id, status)`

**Guardrail:** Completing a task triggers **domain API calls** — process engine never writes domain tables directly.

---

## 4. Rule Engine Data Model

### `rule_sets`

**Purpose:** Named collection of business rules.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `configuration_id` | UUID FK | |
| `code` | VARCHAR | |
| `name` | VARCHAR | |
| `description` | TEXT NULL | |
| `rule_set_type` | VARCHAR | See enum |
| `status` | VARCHAR | DRAFT, PUBLISHED, ARCHIVED |
| `active_version_id` | UUID NULL FK → `rule_versions` | |
| `created_by_user_id` | UUID | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**`rule_set_type` values:**

- `APPROVAL`
- `SLA_KPI`
- `CARRIER_SELECTION`
- `RFX_WINNER_EVALUATION`
- `PRICING_TARIFF`
- `BILLING_VALIDATION`
- `DOCUMENT_VALIDATION`
- `NOTIFICATION`
- `ANTIFRAUD`
- `SLOT_BOOKING`

---

### `rule_versions`

**Purpose:** Immutable published snapshot of rules in a set.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rule_set_id` | UUID FK | |
| `version` | INTEGER | |
| `status` | VARCHAR | DRAFT, PUBLISHED, ARCHIVED |
| `rules_json` | JSONB | Denormalized full set for fast load |
| `safe_dsl_json` | JSONB | Validated DSL schema version |
| `test_cases_json` | JSONB NULL | Simulation fixtures |
| `published_by_user_id` | UUID NULL | |
| `published_at` | TIMESTAMPTZ NULL | |
| `created_at` | TIMESTAMPTZ | |

---

### `rules`

**Purpose:** Individual rule within a version (normalized for editing UI).

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rule_version_id` | UUID FK | |
| `code` | VARCHAR | |
| `name` | VARCHAR | |
| `description` | TEXT NULL | |
| `priority` | INTEGER | Lower = higher priority |
| `condition_json` | JSONB | Safe DSL condition tree |
| `action_json` | JSONB | Whitelisted actions only |
| `enabled` | BOOLEAN | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**Action whitelist (design):** `REQUIRE_APPROVAL`, `CREATE_ALERT`, `SEND_NOTIFICATION`, `BLOCK_TRANSITION`, `SET_FIELD_VALUE`, `EXCLUDE_CARRIER` — **never** `EXECUTE_SQL`, `ELEVATE_ROLE`, `CROSS_TENANT_READ`.

---

### `rule_evaluation_logs`

**Purpose:** Audit trail of rule evaluations (production + simulation).

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rule_set_id` | UUID FK | |
| `rule_version_id` | UUID FK | |
| `entity_type` | VARCHAR NULL | |
| `entity_id` | UUID NULL | |
| `input_json` | JSONB | Context snapshot |
| `output_json` | JSONB | Actions fired |
| `matched_rules_json` | JSONB | List of matched rule codes |
| `result` | VARCHAR | MATCH, NO_MATCH, ERROR |
| `evaluated_at` | TIMESTAMPTZ | |

**Indexes:** `(tenant_id, evaluated_at DESC)`, `(rule_set_id, evaluated_at DESC)`

**Guardrails:**

- No arbitrary code execution
- Only safe JSON DSL validated against schema
- Simulation writes to same table with `result = SIMULATION` flag (optional column) or separate retention policy
- Production rules must be **published** version only

---

## 5. No-code Connectors Data Model

### `connector_definitions`

**Purpose:** **Global** catalog of connector types (platform-managed, not tenant-owned).

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `code` | VARCHAR UNIQUE | e.g. `ONE_C_EXPORT_V1` |
| `name` | VARCHAR | |
| `description` | TEXT NULL | |
| `connector_type` | VARCHAR | See enum |
| `provider` | VARCHAR NULL | Vendor name |
| `version` | VARCHAR | Semantic version |
| `schema_json` | JSONB | Settings schema, capability flags |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**`connector_type` values:**

`API` · `WEBHOOK` · `EDI` · `ERP` · `SAP` · `ONE_C` · `EMAIL` · `FILE_EXCHANGE` · `TELEMATICS` · `E_SIGNATURE` · `GOVERNMENT`

**Note:** No `tenant_id` — global read-only catalog.

---

### `connector_instances`

**Purpose:** Tenant-specific deployed connector.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `connector_definition_id` | UUID FK | |
| `configuration_id` | UUID NULL FK | Optional link to registry |
| `code` | VARCHAR | Unique per tenant |
| `name` | VARCHAR | |
| `status` | VARCHAR | DRAFT, ACTIVE, DISABLED, ERROR |
| `environment` | VARCHAR | DEV, STAGING, PRODUCTION |
| `credentials_secret_ref` | VARCHAR NOT NULL | Vault/KMS path — **never plain credentials** |
| `settings_json` | JSONB | Non-secret settings (URLs, timeouts) |
| `created_by_user_id` | UUID | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**Indexes:** `(tenant_id, code)` UNIQUE, `(tenant_id, status)`

**Guardrails:**

- `credentials_secret_ref` only — secrets in HashiCorp Vault / cloud secret manager
- Secrets **never** in Git or `settings_json`
- Rotate credentials without changing mapping version

---

### `connector_mappings`

**Purpose:** Field mapping between platform entity and external system object.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `connector_instance_id` | UUID FK | |
| `code` | VARCHAR | |
| `name` | VARCHAR | |
| `source_entity_type` | VARCHAR | Platform entity |
| `target_system_object` | VARCHAR | External object name |
| `mapping_json` | JSONB | Field map |
| `transformation_rules_json` | JSONB NULL | Format conversions |
| `status` | VARCHAR | DRAFT, PUBLISHED, ARCHIVED |
| `version` | INTEGER | |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**Indexes:** `(connector_instance_id, code, version)` UNIQUE

---

### `integration_jobs`

**Purpose:** Async integration work unit.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `connector_instance_id` | UUID FK | |
| `mapping_id` | UUID NULL FK | |
| `job_type` | VARCHAR | IMPORT, EXPORT, SYNC, WEBHOOK_RECEIVE, WEBHOOK_SEND |
| `status` | VARCHAR | PENDING, RUNNING, SUCCESS, FAILED, RETRYING, DEAD_LETTER |
| `trigger_type` | VARCHAR | SCHEDULE, EVENT, MANUAL |
| `entity_type` | VARCHAR NULL | |
| `entity_id` | UUID NULL | |
| `request_payload_json` | JSONB NULL | Redacted if sensitive |
| `response_payload_json` | JSONB NULL | |
| `error_message` | TEXT NULL | |
| `retry_count` | INTEGER DEFAULT 0 | |
| `next_retry_at` | TIMESTAMPTZ NULL | |
| `created_at` | TIMESTAMPTZ | |
| `completed_at` | TIMESTAMPTZ NULL | |

**Indexes:** `(tenant_id, status, created_at DESC)`, `(connector_instance_id, status)`, `(next_retry_at)` WHERE status = `RETRYING`

**Idempotency:** recommend `idempotency_key` column (future) to prevent duplicate imports.

---

### `integration_logs`

**Purpose:** Line-level log for integration job execution.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `integration_job_id` | UUID FK | |
| `level` | VARCHAR | DEBUG, INFO, WARN, ERROR |
| `message` | TEXT | |
| `details_json` | JSONB NULL | |
| `created_at` | TIMESTAMPTZ | |

**Indexes:** `(integration_job_id, created_at)`, `(tenant_id, created_at DESC)`

**Retention:** partition by month; PII redaction in `details_json`.

---

## 6. Runtime Execution Data Model

### `low_code_runtime_events`

**Purpose:** Event stream for low-code runtime observability and Control Tower integration.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `event_type` | VARCHAR | See enum |
| `entity_type` | VARCHAR NULL | |
| `entity_id` | UUID NULL | |
| `process_instance_id` | UUID NULL FK | |
| `rule_evaluation_id` | UUID NULL FK → `rule_evaluation_logs` | |
| `connector_job_id` | UUID NULL FK | |
| `payload_json` | JSONB | |
| `created_at` | TIMESTAMPTZ | |

**`event_type` values:**

`FORM_SUBMITTED` · `PROCESS_STARTED` · `PROCESS_STEP_COMPLETED` · `RULE_EVALUATED` · `CONNECTOR_JOB_STARTED` · `CONNECTOR_JOB_FAILED` · `NOTIFICATION_SENT`

**Indexes:** `(tenant_id, event_type, created_at DESC)`, `(entity_type, entity_id)`

---

### `low_code_notifications`

**Purpose:** Outbound notifications triggered by rules or process steps.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `notification_type` | VARCHAR | EMAIL, IN_APP, WEBHOOK, SMS |
| `target_user_id` | UUID NULL | |
| `target_role` | VARCHAR NULL | |
| `entity_type` | VARCHAR NULL | |
| `entity_id` | UUID NULL | |
| `template_id` | UUID NULL | Future notification template FK |
| `payload_json` | JSONB | |
| `status` | VARCHAR | PENDING, SENT, FAILED |
| `sent_at` | TIMESTAMPTZ NULL | |
| `created_at` | TIMESTAMPTZ | |

**Indexes:** `(tenant_id, status, created_at)`, `(target_user_id, status)`

---

## 7. Audit and Governance Data Model

### `configuration_audit_log`

**Purpose:** Immutable audit trail for **all** low-code configuration changes.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `configuration_id` | UUID NULL FK | |
| `entity_type` | VARCHAR | Table/artifact type |
| `entity_id` | UUID | Row ID |
| `action` | VARCHAR | See enum |
| `old_value_json` | JSONB NULL | |
| `new_value_json` | JSONB NULL | |
| `changed_by_user_id` | UUID | |
| `changed_at` | TIMESTAMPTZ | |
| `request_id` | VARCHAR NULL | Correlation ID |
| `ip_address` | INET NULL | |
| `user_agent` | TEXT NULL | |

**`action` values:**

`CREATE` · `UPDATE` · `DELETE` · `PUBLISH` · `ARCHIVE` · `ROLLBACK` · `TEST` · `SIMULATE`

**Indexes:** `(tenant_id, changed_at DESC)`, `(configuration_id, changed_at DESC)`, `(entity_type, entity_id)`

**Retention:** append-only; no UPDATE/DELETE on audit rows.

---

### `configuration_approvals`

**Purpose:** Publish approval workflow (optional governance gate).

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `configuration_id` | UUID FK | |
| `requested_by_user_id` | UUID | |
| `approved_by_user_id` | UUID NULL | |
| `status` | VARCHAR | PENDING, APPROVED, REJECTED, CANCELLED |
| `comment` | TEXT NULL | |
| `requested_at` | TIMESTAMPTZ | |
| `approved_at` | TIMESTAMPTZ NULL | |
| `rejected_at` | TIMESTAMPTZ NULL | |

**Indexes:** `(tenant_id, status)`, `(configuration_id)`

---

## 8. Templates and Marketplace

### `global_templates`

**Purpose:** Platform-admin curated templates (forms, processes, rules, connectors).

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `code` | VARCHAR UNIQUE | |
| `name` | VARCHAR | |
| `description` | TEXT NULL | |
| `template_type` | VARCHAR | FORM, PROCESS, RULE_SET, CONNECTOR_MAPPING, DOCUMENT |
| `template_json` | JSONB NOT NULL | Full portable definition |
| `version` | INTEGER | |
| `status` | VARCHAR | DRAFT, PUBLISHED, ARCHIVED |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

**Note:** No `tenant_id` — platform scope.

---

### `tenant_template_copies`

**Purpose:** Record when tenant copies a global template into tenant-owned configuration.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `global_template_id` | UUID FK → `global_templates` | |
| `local_configuration_id` | UUID FK → `low_code_configurations` | |
| `copied_by_user_id` | UUID | |
| `copied_at` | TIMESTAMPTZ | |

**Indexes:** `(tenant_id, global_template_id)`, `(local_configuration_id)`

---

## Relationships

```
low_code_configurations (1) ──< form_templates
                          ──< process_templates
                          ──< rule_sets
                          ──< connector_instances (optional)

form_templates (1) ──< form_sections (1) ──< form_fields
custom_field_values ──> form_fields (field_id)
                    ──> entity (entity_type + entity_id) [logical]

process_templates (1) ──< process_versions (1) ──< process_steps
                                               ──< process_transitions
process_instances ──> process_versions (locked)
                ──< process_tasks

rule_sets (1) ──< rule_versions (1) ──< rules
rule_evaluation_logs ──> rule_sets / rule_versions

connector_definitions (global, 1) ──< connector_instances (tenant)
connector_instances (1) ──< connector_mappings
                        ──< integration_jobs (1) ──< integration_logs

global_templates (1) ──< tenant_template_copies ──> low_code_configurations

configuration_audit_log ──> any config entity (entity_type + entity_id)
configuration_approvals ──> low_code_configurations
```

**Key rules:**

- `tenant_id` mandatory on all tenant-owned tables
- `low_code_configurations` is the **hub** linking typed artifacts
- Runtime tables (`process_instances`, `custom_field_values`, `integration_jobs`) reference **published** versions only
- Global tables (`connector_definitions`, `global_templates`) have no `tenant_id`

---

## Status Lifecycle

### Configuration artifacts (`low_code_configurations`, templates, rule sets, mappings)

```
DRAFT ──submit──> REVIEW ──approve──> PUBLISHED ──retire──> ARCHIVED
                     │
                     └──reject──> DRAFT
```

Published versions are **immutable**; edits create new draft version linked to same `code`.

### Process instance

```
(start) ──> RUNNING ──complete──> COMPLETED
              │
              ├──cancel──> CANCELLED
              └──error───> FAILED
```

### Integration job

```
PENDING ──> RUNNING ──success──> SUCCESS
              │
              ├──fail(retry)──> RETRYING ──> RUNNING
              └──fail(max)───> DEAD_LETTER

FAILED (terminal, no retry configured)
```

---

## Tenant Isolation Rules

| Rule | Enforcement |
| ---- | ----------- |
| `tenant_id` required | NOT NULL on tenant-owned tables; CHECK constraints |
| Query filtering | Every SELECT/UPDATE/DELETE includes `tenant_id = :current_tenant` |
| Global templates | Read-only for tenants until copied via `tenant_template_copies` |
| Connector credentials | Scoped to `connector_instances.tenant_id` + secret ref |
| Audit log | `(tenant_id, changed_at)` — tenants see own log only |
| Cross-tenant mapping | **Forbidden** — no FK across tenants |
| Platform admin | Explicit `PLATFORM_ADMIN` role for global tables; audited separately |

Align with existing pattern: services pass `tenant_id` query param / header (`X-Tenant-ID`) as in current API Gateway flows.

---

## Security Guardrails

| # | Guardrail | Data model support |
| - | --------- | ------------------ |
| 1 | No arbitrary code execution | `safe_dsl_json`, `condition_json`, `action_json` — schema-validated only |
| 2 | Safe DSL only | `rule_versions.safe_dsl_json` stores DSL version + schema hash |
| 3 | No direct DB mutation from rules | Actions whitelist; no SQL action type |
| 4 | No RBAC bypass | Process tasks store `assigned_role`; completion checks identity-service |
| 5 | No protected financial status bypass | Process `SYSTEM_ACTION` maps to known domain API endpoints only |
| 6 | No hidden document signing | Document actions require `document-service` API |
| 7 | No cross-tenant access | `tenant_id` on all rows + RLS |
| 8 | Secrets via secret manager | `credentials_secret_ref` only |
| 9 | Production publish requires approval | `configuration_approvals` + status REVIEW gate |
| 10 | All changes audited | `configuration_audit_log` trigger or service-layer write |

---

## MVP Data Model Recommendation

### MVP v0.1 — implement first (Configuration Foundation)

| Table | Rationale |
| ----- | --------- |
| `low_code_configurations` | Central registry |
| `form_templates` | Form Builder core |
| `form_sections` | Layout groups |
| `form_fields` | Field definitions |
| `custom_field_values` | Runtime custom data |
| `rule_sets` | Simple approval / validation rules |
| `rule_versions` | Versioning |
| `rules` | Individual rules |
| `configuration_audit_log` | Governance from day one |

**Delivers:** Custom Fields + Form Templates + Rule Engine lite + audit.

**Defer:** BPMN tables, connector tables, runtime events, marketplace.

### MVP v0.2 — Process and Rule Studio

Add:

- `process_templates`, `process_versions`, `process_steps`, `process_transitions`
- `process_instances`, `process_tasks`
- `rule_evaluation_logs`
- `configuration_approvals`
- `low_code_runtime_events`, `low_code_notifications`

### MVP v0.3 — Integration Studio

Add:

- `connector_definitions` (seed global catalog)
- `connector_instances`, `connector_mappings`
- `integration_jobs`, `integration_logs`

### MVP v0.4 — Marketplace

Add:

- `global_templates`, `tenant_template_copies`

---

## Open Questions

| # | Question | Options / notes |
| - | -------- | --------------- |
| 1 | Separate `low-code-service`? | **Recommended:** dedicated Go service with `lowcode` schema; avoids bloating existing services |
| 2 | Config tables in existing services? | Possible for MVP but splits ownership; harder to enforce uniform audit/versioning |
| 3 | Custom field storage: JSONB vs typed columns? | **JSONB** in `custom_field_values.value_json` for flexibility; index hot `field_code` paths if needed |
| 4 | BPMN engine: Camunda / Zeebe / Temporal / custom? | Start **custom lightweight** engine for v0.2; evaluate Temporal if long-running workflows dominate |
| 5 | Connector secrets storage? | HashiCorp Vault or cloud KMS; `credentials_secret_ref` pattern |
| 6 | Who can publish production config? | `PLATFORM_ADMIN` + tenant `SHIPPER_ADMIN` with `LOW_CODE_PUBLISHER` role (new role TBD) |
| 7 | Approval workflow required? | Optional for v0.1; mandatory for enterprise tier in v0.4 |
| 8 | RLS vs application filtering? | **Both:** RLS as defense-in-depth + service-layer tenant checks |
| 9 | Polymorphic `entity_id` integrity? | Validation service or domain event sync on entity delete |
| 10 | BPMN XML vs JSON-only? | Store both: `bpmn_xml` for export/interop, `definition_json` for runtime |

---

## Recommended Next Action

1. **Do not create migrations now** — this document is the design baseline.
2. **Do not implement low-code services yet** — keep `make integration-smoke-test` green on Core TMS.
3. **Next pack:** `Low-code MVP Scope Pack v0.1` — API surface, service boundaries, UI wireframes for Custom Fields + Form Templates.
4. **First implementation slice:** Custom Fields + Form Templates + `configuration_audit_log`.
5. **Only after MVP scope approval:** write migrations for `lowcode` schema (MVP v0.1 tables only) and draft OpenAPI routes under `/v1/low-code/*`.
6. **Regression gate:** every low-code increment must pass existing smoke test — domain guardrails unchanged.

---

## Related Documentation

| Document | Relationship |
| -------- | ------------ |
| `docs/LOW_CODE_LAYER_ROADMAP_V0.1.md` | Strategic roadmap and module scope |
| `docs/AUTH_RBAC.md` | Role codes for process tasks and publish permissions |
| `docs/PROJECT_MAP.md` | Existing service and schema layout |
| `AGENTS.md` | AI rules — no domain rewrite |

---

## Document History

| Version | Date | Notes |
| ------- | ---- | ----- |
| v0.1 | 2026-06-22 | Initial data model design — docs only, no migrations |
