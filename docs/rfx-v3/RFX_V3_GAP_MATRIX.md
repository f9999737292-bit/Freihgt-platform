# RFx v3.0A — Gap Matrix

**Status:** Architecture freeze artifact  
**Discovery date:** 2026-09-03  
**Repository evidence:** `services/rfx-service/`, `apps/web-admin/`, `packages/openapi/`, `infrastructure/migrations/`

---

## 1. Purpose

Repository-backed comparison of **current BINTRANS implementation** vs **target Enterprise RFx v3.0A**. Status claims require file-path evidence; absent capabilities are marked **ABSENT**, not inferred.

**BINTRANS primary UI:** `apps/web-admin/` (pilot). Fuller RFx UI exists in `apps/web-procurement/` but is outside BINTRANS pilot scope.

---

## 2. Legend

| Status | Meaning |
|---|---|
| **EXISTS** | Implemented end-to-end or materially present in scoped paths |
| **PARTIAL** | Backend or data exists; UI/contract incomplete, or feature subset only |
| **ABSENT** | No implementation found in repository |

| Priority | Meaning |
|---|---|
| **P0** | v3.0B–C core (questionnaire + carrier response) |
| **P1** | v3.0D–F (scoring, templates, qualification) |
| **P2** | v3.0G–I (Carrier 360, analytics, AI) |
| **P3** | v3.0J enterprise hardening |

---

## 3. Capability matrix

| Capability | CURRENT_BINTRANS | TARGET_V3 | GAP | BACKEND | FRONTEND | DATA | SECURITY | PRIORITY |
|---|---|---|---|---|---|---|---|---|
| **RFI** | Type enum in DB + create modal (`000004`, `RfxCreateModal.vue`) | Full RFI qualification lifecycle | No RFI-specific questionnaire or qualification flow | `rfx_type` CHECK includes RFI | Type picker only | Column exists | Tenant + owner scoping | P0 |
| **RFQ** | Same as RFI; spot path via `freight_requests` | RFQ with commercial + technical sections | Dual product paths; no unified questionnaire | Events + FR services | web-admin FR/bids UI | `rfx_events`, `freight_requests` | RBAC on both paths | P0 |
| **RFP** | Type enum only | RFP with weighted evaluation | No proposal questionnaire | Type in lifecycle | Badge only | Enum | Same as RFx event | P1 |
| **Questionnaire Builder** | **ABSENT** for RFx | RFx Studio builder | Entire builder missing | No questionnaire API | No builder UI | No `rfx_questions` tables | — | P0 |
| **Sections** | Low-code only (`000011`, `lowcode.form_sections`) | RFx-native sections | Low-code not wired to carrier responses | low-code-service only | Low-code admin | Separate schema | Tenant isolation | P0 |
| **Question Types** | Low-code field types only | Full RFx question catalogue | No RFx question model | Low-code validation | Low-code preview | `form_fields.field_type` | — | P0 |
| **Conditional Logic** | Low-code JSON rules (`visibility_rule_json`) | RFx rule engine | Rules not in rfx-service | low-code conditional | Preview in low-code only | JSON on low-code | — | P0 |
| **Draft** | Event DRAFT + response DRAFT status | Buyer + carrier draft with resume | No carrier draft UI in web-admin | `rfx_response.go`, status enums | Manual save only | `status`, `version` columns | Carrier draft RBAC tests | P0 |
| **Autosave** | **ABSENT** | Atomic server-validated autosave | No debounce/PATCH autosave | Explicit POST/PATCH only | No autosave | — | — | P0 |
| **Resume** | DRAFT rows persist server-side | Resume UX + version indicator | No resume UI in web-admin | `GET .../own-response` exists | UI absent | DRAFT responses | Carrier isolation | P0 |
| **Preview-as-Carrier** | **ABSENT** | Mandatory buyer sandbox | No preview endpoints/UI | No preview routes | — | — | Preview isolation spec only | P0 |
| **Participants** | List + add (`RfxParticipantsTable.vue`) | Groups, access rules, reminders | No supplier groups | Full CRUD + audit | web-admin implemented | `rfx_participants` | Buyer-manage RBAC | P1 |
| **Carrier Response** | Spot bids UI; no enterprise response UI | Full questionnaire response | Backend exists; web-admin missing | `carrier_rfx_service.go`, offer lines | web-procurement only | `rfx_responses`, offer lines | Security tests exist | P0 |
| **Scoring** | Commercial 70% + manual 30% (`rfx_evaluation.go`) | Configurable score models | Fixed formula; no questionnaire scoring | `evaluation_service.go` | web-procurement evaluation | Score columns | Buyer-only evaluation | P1 |
| **Knockout** | **ABSENT** | Knockout on valid answers | No knockout domain | — | — | — | — | P1 |
| **Templates** | Low-code form templates only | RFx tender templates + clone | No RFx template reuse | No template API | Low-code admin | `lowcode.form_templates` | — | P1 |
| **Versioning** | Row `version` optimistic lock | Immutable published versions | No `rfx_versions` table | Entity version columns | — | `version` on entities | — | P1 |
| **Qualification** | **ABSENT** | Qualification results + gates | No qualification model | — | — | — | — | P1 |
| **Qualification Pool** | **ABSENT** | Prequalified carrier pools | No pool tables/API | — | — | — | — | P2 |
| **Carrier 360** | Company list pages only | Reusable carrier profile autofill | No 360 aggregation | — | — | Company tables only | — | P2 |
| **Notifications** | Lifecycle transition only (`send-invitations`) | Email/push/outbox reminders | No RFx notification worker | Status transition | Toast only | Shipment notifications unrelated | — | P2 |
| **Approval** | Direct publish action | Approval gates before publish | No approver chain | Publish without gate | `canPublishTenders` in web-procurement | — | Role-based publish | P2 |
| **Audit** | Backend + migration (`000037`, audit routes) | Full audit + explainability | No audit panel in web-admin | `audit_support.go` | web-procurement only | `rfx.audit_events` | Tenant-scoped reads | P1 |
| **Analytics** | **ABSENT** for RFx | Tender analytics + KPI linkage | No RFx dashboards | — | — | — | — | P2 |
| **AI** | **ABSENT** | Bounded AI assist (see AI doc) | No LLM integration | — | — | — | — | P2 |

---

## 4. Layer summary

### Backend (`services/rfx-service/`)

| EXISTS | PARTIAL | ABSENT |
|---|---|---|
| Event CRUD, lifecycle, lots/lanes, participants, carrier API, responses (draft→submit), evaluation/award, freight requests/bids, deadline worker, audit | OpenAPI contract drift; no questionnaire; fixed scoring | Knockout, qualification pool, templates, preview, notifications, AI |

### Frontend — BINTRANS (`apps/web-admin/`)

| EXISTS | PARTIAL | ABSENT |
|---|---|---|
| Event list/create/detail, participants, publish/cancel, freight requests/bids | Low-code custom fields on RFx detail | Questionnaire builder, carrier response, evaluation, autosave, preview, audit panel |

### Data (`infrastructure/migrations/`)

| EXISTS | ABSENT (v3 target) |
|---|---|
| `rfx_events`, lots, lanes, participants, responses, offer lines, awards, audit_events, freight_requests, bids | `rfx_versions`, sections, questions, answers, score_models, qualification_*, pools, preview_sessions |

### OpenAPI (`packages/openapi/rfx-service.yaml`)

**PARTIAL** — stub ~12 paths; missing lots, responses, evaluation, audit, carrier routes documented in `router.go`.

---

## 5. Architecture freeze closure

This gap matrix closes controller finding **F101-002** (architecture freeze incomplete) by documenting repository-backed current state vs v3.0A target across all mandated capability areas.

---

## 6. References

- [RFX_V3_DATA_MODEL.md](./RFX_V3_DATA_MODEL.md) — target schema extensions
- [RFX_V3_ROADMAP.md](./RFX_V3_ROADMAP.md) — implementation waves v3.0A–J
- [RFX_V3_FUNCTIONAL_BASELINE.md](./RFX_V3_FUNCTIONAL_BASELINE.md) — functional baseline
- Evidence: `infrastructure/migrations/000004_create_rfx_tables.up.sql`, `services/rfx-service/internal/http/router.go`
