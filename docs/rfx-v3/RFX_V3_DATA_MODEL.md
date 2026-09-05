# RFx v3.0A — Data Model

**Status:** Architecture draft (target schema)  
**Current schema evidence:** `infrastructure/migrations/000004`, `000037`, `000038`, `000039`

---

## 1. Principles

1. **Preserve existing RFx tables** where practical — extend, do not replace.
2. **Every tenant-owned table** includes `tenant_id` with query predicates.
3. **Soft delete** via `deleted_at` where entities are user-facing.
4. **Optimistic concurrency** via `version` column on mutable aggregates.
5. **Published snapshots immutable** — new rows for new versions, never in-place overwrite.

---

## 2. Current schema (verified)

| Table | Schema | Purpose | Key constraints |
|---|---|---|---|
| `rfx_events` | `rfx` | RFx header | `uq_rfx_number (tenant_id, rfx_number)`; `owner_company_id` |
| `rfx_lots` | `rfx` | Lots | FK → `rfx_events`; `uq_rfx_lot_number` |
| `rfx_lanes` | `rfx` | Lanes | FK → `rfx_lots` |
| `rfx_participants` | `rfx` | Invitations | `uq_rfx_participant (rfx_event_id, company_id)` |
| `rfx_responses` | `rfx` | Carrier responses | `uq_rfx_response (rfx_event_id, participant_company_id)` |
| `rfx_response_offer_lines` | `rfx` | Commercial lines | Unique per response/lot |
| `rfx_awards` | `rfx` | Award record | `uq_rfx_award_event` |
| `audit_events` | `rfx` | Audit trail | `(tenant_id, entity_type, entity_id)` index |
| `freight_requests` | `rfx` | Spot FR path | `shipper_company_id` |
| `bids` | `rfx` | Spot bids | `carrier_company_id` |

All above include `tenant_id`. Indexes on `tenant_id`, FK columns, and status fields exist per migrations.

---

## 3. Target extensions (v3.0A architecture)

### 3.1 Versioning & templates

#### `rfx.rfx_versions`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rfx_event_id` | UUID FK → `rfx_events` | |
| `version_number` | INT NOT NULL | Monotonic per event |
| `status` | VARCHAR | `DRAFT`, `PUBLISHED`, `SUPERSEDED`, `ARCHIVED` |
| `questionnaire_snapshot_json` | JSONB | Immutable published definition |
| `published_at` | TIMESTAMPTZ | NULL until published |
| `published_by` | UUID | |
| `created_at`, `updated_at` | TIMESTAMPTZ | |
| `version` | INT | Optimistic lock |

**Unique:** `(rfx_event_id, version_number)`  
**Index:** `(tenant_id, rfx_event_id, status)`

#### `rfx.rfx_templates`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `template_code` | VARCHAR | Human code |
| `name`, `description` | VARCHAR/TEXT | |
| `rfx_type` | VARCHAR | RFI/RFQ/RFP |
| `owner_company_id` | UUID | Optional tenant-wide template |
| `status` | VARCHAR | `DRAFT`, `ACTIVE`, `ARCHIVED` |
| `created_at`, `updated_at`, `deleted_at` | TIMESTAMPTZ | |
| `version` | INT | |

**Unique:** `(tenant_id, template_code)` where `deleted_at IS NULL`

#### `rfx.rfx_template_versions`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `template_id` | UUID FK | |
| `version_number` | INT | |
| `definition_json` | JSONB | Sections/questions snapshot |
| `published_at` | TIMESTAMPTZ | |

**Unique:** `(template_id, version_number)`

---

### 3.2 Questionnaire definition

#### `rfx.rfx_sections`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rfx_version_id` | UUID FK → `rfx_versions` | Version-scoped |
| `section_code` | VARCHAR | Stable within version |
| `title`, `description` | VARCHAR/TEXT | |
| `sort_order` | INT | |
| `visibility_rule_json` | JSONB | Conditional display |
| `created_at`, `updated_at` | TIMESTAMPTZ | |

**Unique:** `(rfx_version_id, section_code)`  
**Index:** `(tenant_id, rfx_version_id, sort_order)`

#### `rfx.rfx_questions`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `section_id` | UUID FK → `rfx_sections` | |
| `question_code` | VARCHAR | |
| `question_type` | VARCHAR | TEXT, NUMBER, BOOLEAN, SELECT, MULTI_SELECT, DATE, FILE, … |
| `label`, `help_text` | TEXT | i18n keys preferred |
| `required` | BOOLEAN | |
| `validation_rule_json` | JSONB | L1 rules |
| `scoring_binding_json` | JSONB | Link to score criteria |
| `sort_order` | INT | |
| `created_at`, `updated_at` | TIMESTAMPTZ | |

**Unique:** `(section_id, question_code)`

#### `rfx.rfx_question_options`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `question_id` | UUID FK | |
| `option_code` | VARCHAR | |
| `label` | TEXT | |
| `sort_order` | INT | |
| `score_weight` | NUMERIC | Optional default weight |

**Unique:** `(question_id, option_code)`

#### `rfx.rfx_question_rules`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rfx_version_id` | UUID FK | |
| `rule_type` | VARCHAR | VISIBILITY, REQUIRED, VALIDATION, KNOCKOUT |
| `rule_json` | JSONB | Engine definition |
| `rule_version` | INT | For audit replay |

---

### 3.3 Responses & answers

Extend **`rfx_responses`** (migration ALTER):

| New column | Type | Notes |
|---|---|---|
| `save_version` | BIGINT | Optimistic concurrency token |
| `last_saved_at` | TIMESTAMPTZ | Server autosave timestamp |
| `rfx_version_id` | UUID FK | Published version responded against |
| `completion_percent` | NUMERIC(5,2) | Derived |

Map existing `status` DRAFT → architecture `IN_PROGRESS`; retain DB enum for compatibility.

#### `rfx.rfx_answers`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rfx_response_id` | UUID FK → `rfx_responses` | |
| `question_id` | UUID FK → `rfx_questions` | |
| `answer_value_json` | JSONB | Typed value |
| `answer_source` | VARCHAR | Authoritative sources only (domain model §4.4) |
| `validation_version` | INT | |
| `rule_version` | INT | Nullable |
| `score_model_version` | INT | Nullable |
| `updated_by` | UUID | |
| `updated_at` | TIMESTAMPTZ | |
| `version` | INT | Per-answer optimistic lock |

**Unique:** `(rfx_response_id, question_id)`  
**Index:** `(tenant_id, rfx_response_id)`

> Invalid client values **never** become rows in this table.

#### `rfx.rfx_answer_evidence`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `answer_id` | UUID FK → `rfx_answers` | |
| `document_id` | UUID | FK → document service |
| `evidence_type` | VARCHAR | UPLOAD, EXTRACTED, VERIFIED |
| `provenance_json` | JSONB | Source metadata |
| `created_at` | TIMESTAMPTZ | |

---

### 3.4 Preview isolation (non-production)

#### `rfx.rfx_preview_sessions`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rfx_event_id` | UUID FK | |
| `draft_version_id` | UUID FK → `rfx_versions` | |
| `buyer_user_id` | UUID | |
| `session_data_json` | JSONB | Ephemeral preview answers |
| `expires_at` | TIMESTAMPTZ | TTL |
| `created_at` | TIMESTAMPTZ | |

**No FK to `rfx_responses` or `rfx_answers`.**  
`PREVIEW_DATA_ONLY=YES`, `REAL_RESPONSE_CREATED=NO`.

Alternative: Redis/ephemeral cache with same contract.

---

### 3.5 Scoring & qualification

#### `rfx.rfx_score_models`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rfx_version_id` | UUID FK | |
| `model_type` | VARCHAR | AUTOMATIC, MANUAL, HYBRID |
| `model_version` | INT | |
| `definition_json` | JSONB | Weights, thresholds |
| `created_at` | TIMESTAMPTZ | |

#### `rfx.rfx_score_criteria`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `score_model_id` | UUID FK | |
| `criterion_code` | VARCHAR | |
| `weight` | NUMERIC(8,4) | |
| `normalization_json` | JSONB | |
| `knockout_threshold_json` | JSONB | Optional |

#### `rfx.rfx_answer_scores`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `answer_id` | UUID FK | |
| `criterion_id` | UUID FK | |
| `raw_score` | NUMERIC | |
| `weighted_contribution` | NUMERIC | |
| `explanation_json` | JSONB | Source, rule, weight, knockout reason |
| `calculated_at` | TIMESTAMPTZ | |
| `score_model_version` | INT | |

#### `rfx.rfx_qualification_results`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rfx_response_id` | UUID FK | |
| `status` | VARCHAR | QUALIFIED, CONDITIONALLY_QUALIFIED, REJECTED, PENDING_REVIEW |
| `knockout_reason_json` | JSONB | |
| `total_score` | NUMERIC | |
| `calculated_at` | TIMESTAMPTZ | |
| `score_model_version` | INT | |

#### `rfx.rfx_qualification_pools`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `pool_code` | VARCHAR | |
| `name` | VARCHAR | |
| `owner_company_id` | UUID | Buyer company |
| `criteria_json` | JSONB | Entry rules |
| `created_at`, `updated_at`, `deleted_at` | TIMESTAMPTZ | |

**Unique:** `(tenant_id, pool_code)` where `deleted_at IS NULL`

#### `rfx.rfx_pool_members`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `pool_id` | UUID FK | |
| `carrier_company_id` | UUID | |
| `status` | VARCHAR | ACTIVE, SUSPENDED, EXPIRED |
| `qualified_at` | TIMESTAMPTZ | |
| `expires_at` | TIMESTAMPTZ | |

**Unique:** `(pool_id, carrier_company_id)`

---

### 3.6 Collaboration, notifications, approvals

#### `rfx.rfx_collaborators`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rfx_event_id` | UUID FK | |
| `user_id` | UUID | |
| `role` | VARCHAR | OWNER, EDITOR, REVIEWER, VIEWER |
| `company_id` | UUID | Server-resolved membership |
| `created_at` | TIMESTAMPTZ | |

**Unique:** `(rfx_event_id, user_id, role)`

#### `rfx.rfx_notifications`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rfx_event_id` | UUID FK | |
| `recipient_user_id` | UUID | |
| `notification_type` | VARCHAR | INVITE, REMINDER, DEADLINE, STATUS |
| `channel` | VARCHAR | EMAIL, IN_APP |
| `status` | VARCHAR | PENDING, SENT, FAILED |
| `payload_json` | JSONB | No secrets |
| `created_at`, `sent_at` | TIMESTAMPTZ | |

#### `rfx.rfx_approvals`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `rfx_event_id` | UUID FK | |
| `approval_type` | VARCHAR | PUBLISH, AWARD |
| `requested_by` | UUID | |
| `approver_user_id` | UUID | |
| `status` | VARCHAR | PENDING, APPROVED, REJECTED |
| `decided_at` | TIMESTAMPTZ | |
| `comment` | TEXT | |

---

### 3.7 Event outbox (reuse platform pattern)

#### `rfx.rfx_event_outbox`

Follows `transport.shipment_event_outbox` pattern (`docs/SHIPMENT_STATUS_OUTBOX.md`):

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL | |
| `aggregate_type` | VARCHAR | `RFX_EVENT`, `RFX_RESPONSE`, … |
| `aggregate_id` | UUID | |
| `aggregate_version` | INT | |
| `event_type` | VARCHAR | e.g. `rfx.response.submitted.v1` |
| `schema_version` | INT | Envelope version |
| `payload` | JSONB | |
| `status` | VARCHAR | PENDING, PUBLISHED, FAILED |
| `created_at`, `published_at` | TIMESTAMPTZ | |

**Index:** partial on `(status, available_at)` where `status = 'PENDING'`

---

## 4. Archive semantics

| Entity | Archive behavior |
|---|---|
| Published `rfx_versions` | `SUPERSEDED` or `ARCHIVED`; never deleted |
| `rfx_events` | `ARCHIVED` status + `deleted_at` soft delete |
| Preview sessions | TTL expiry; no archive |
| Qualification pools | `deleted_at` soft delete; members retained for audit |

---

## 5. Tenant & company ownership summary

| Table | `tenant_id` | Company ownership |
|---|---|---|
| All `rfx.*` | Required | `owner_company_id` on events/templates; `participant_company_id` / `carrier_company_id` on responses |
| Preview sessions | Required | Buyer user only; no carrier company |
| Pool members | Required | `carrier_company_id` |

---

## 6. References

- [RFX_V3_DOMAIN_MODEL.md](./RFX_V3_DOMAIN_MODEL.md)
- [RFX_V3_GAP_MATRIX.md](./RFX_V3_GAP_MATRIX.md)
- [RFX_V3_EVENTS.md](./RFX_V3_EVENTS.md)
- Migrations: `infrastructure/migrations/000004_create_rfx_tables.up.sql`
