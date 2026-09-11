# RFx v3.0A — Implementation Roadmap

**Status:** Architecture freeze
**Normative companion:** [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)

---

## 1. Release train (v3.0A → v3.0J)

| Release | Name | Scope | Status |
|---|---|---|---|
| **v3.0A** | Architecture Freeze | Docs, ADRs, gap matrix, diagrams — **no implementation** | **THIS STREAM** |
| **v3.0B** | Questionnaire Core | Sections, questions, types, conditional rules, buyer Studio builder | **IMPLEMENTED_ACCEPTED** |
| **v3.0C** | Carrier Response | Autosave, resume, error UX, submit gate | **IMPLEMENTED_ACCEPTED** |
| **v3.0D** | Scoring + Knockout | Score models, knockout, explainability | **IMPLEMENTED_ACCEPTED** |
| **v3.0E** | Templates + Versioning | Template library, immutable published versions, compare/restore, late submission | **IMPLEMENTATION_IN_PROGRESS** (E1–E6 accepted; **E7 Phase 1 late submission** implemented pending validation; browser acceptance not started) |
| **v3.0F** | Qualification Pool | Qualification results, pools, RFI→RFQ handoff | Planned |
| **v3.0G** | Carrier 360 | Profile autofill, freshness, confirmation | Planned |
| **v3.0H** | Analytics + Explainability | Dashboards, score drill-down, audit views | Planned |
| **v3.0I** | AI | Bounded assist — extraction, suggestions, explanations | Planned |
| **v3.0J** | Enterprise Hardening | Approval chains, notifications, outbox, OpenAPI parity, performance | Planned |

**STOP_AFTER_V3_0A=YES** applies only to the completed v3.0A architecture-freeze stream. Subsequent release implementation requires an explicit controller gate per release.

---

## 2. Non-deferrable capabilities (must not slip past core)

These are assigned to the **earliest appropriate wave** — not deferred beyond enterprise RFx core:

| Capability | Wave |
|---|---|
| Buyer draft / resume | v3.0B |
| Buyer autosave | v3.0B |
| Server-authoritative validation | v3.0B |
| Publish readiness gate | v3.0B |
| Carrier response draft / resume | v3.0C |
| Atomic autosave safety | v3.0C |
| Valid-only persistence | v3.0C |
| Carrier error UX (inline, section, global, deep link) | v3.0C |
| Pre-submit validation gate | v3.0C |
| Preview-as-carrier | v3.0C |
| Warning vs validation vs knockout classification | v3.0C–D |
| Scoring/knockout on persisted valid answers | v3.0D |
| Version history & compare | v3.0E |
| Post-publish material change / new version | v3.0E |
| Change impact analysis & re-scoring | v3.0E |

---

## 3. v3.0B — Questionnaire Core

**Goal:** Buyer can define structured questionnaire with conditional logic.

| Deliverable | Architecture refs |
|---|---|
| `rfx_sections`, `rfx_questions`, `rfx_question_options`, `rfx_question_rules` | Data model §3.2 |
| Rule engine (visibility, required, validation) | Questionnaire engine, ADR-003 |
| Buyer Studio builder UX | UX §8 |
| Buyer draft autosave | API §5, validation contract |

---

## 4. v3.0C — Carrier Response

**Goal:** End-to-end draft response with safe persistence and error UX.

| Deliverable | Architecture refs |
|---|---|
| Domain model: `Answer`, `AnswerDraft`, `ResponseVersion` | Domain model §3–5 |
| PATCH autosave + optimistic concurrency | API §2 |
| 422 structured validation | API §2.3 |
| Four validation layers (L1–L3 on save, L4 on submit) | Questionnaire §2 |
| Carrier workspace UX flags | UX §2–6 |
| Preview-as-carrier sandbox | API §6, validation contract §16–17 |
| Pre-submit gate | Validation contract §19 |
| State machine: `NOT_STARTED` → `IN_PROGRESS` → `SUBMITTED` | State machines §2 |

---

## 5. v3.0D — Scoring + Knockout

| Deliverable | Architecture refs |
|---|---|
| `ScoreModel`, criteria, answer_scores | Scoring engine, data model §3.5 |
| Knockout on valid answers | Scoring engine §3, §8 |
| Explainability payload | Scoring engine §7 |
| `rfx.score.calculated` event | Events §3 |

---

## 6. v3.0E — Templates + Versioning

| Deliverable | Architecture refs |
|---|---|
| `RfxTemplate`, `RfxVersion` | Data model §3.1, ADR-002, ADR-008 |
| Immutable published versions | Domain model §7 |
| `COMPARE_VERSIONS`, `RESTORE_DRAFT_VERSION` | Validation contract §21 |
| Change impact analysis | API §7 |

### v3.0E wave status

| Wave | Scope | Status |
|---|---|---|
| **E1 Backend Foundation** | Questionnaire publish/supersede, fork draft, idempotency, version list/detail API, carrier response continuity, migration 000068 | **IMPLEMENTED_ACCEPTED** — PR #107 merged via `1b880d0` |
| **E2 Compare + Restore** | Compare versions, restore-as-draft | **IMPLEMENTED_ACCEPTED** — PR #109 merged via `e1780e7` (head `5f223b1`, CI `34241367509`) |
| **E3 Change Impact** | Change-impact preview, publish confirmation | **IMPLEMENTED_ACCEPTED** — PR #111 merged via `69eacb3` (head `d9aa539`, CI `34259692961`) |
| **E4 Template Library** | Template library CRUD, publish, fork-draft, archive, draft graph | **IMPLEMENTED_ACCEPTED** — PR #113 merged via `5243bb5` (head `c701ab3`, CI `34383868950`) |
| **E5 Template Clone + Provenance** | Clone event from template, provenance | **IMPLEMENTED_ACCEPTED** — PR #115 merged via `81ff86b` (head `5c26d34`, CI `34401634396`, migration 000071) |
| **E6 Studio Frontend** | Studio UI: library, history, compare, restore, clone, republish impact | **IMPLEMENTED_ACCEPTED** — PR #117 merged via `1764617` (head `60d5fd2`, CI `34509790393`) |
| **E7 Phase 1 Late Submission** | Per-carrier late submission backend (migration 000072, API, RBAC, OpenAPI, E7-INT-01..40) | **IMPLEMENTED_PENDING_CONTROLLER_ACCEPTANCE** |
| **E7 Browser Acceptance** | Browser acceptance gate | **NOT_STARTED** |

Notes:

- The E1 implementation expanded the backend foundation while remaining inside the accepted v3.0E architecture.
- E2 compare + restore backend is accepted and merged to `main` at `e1780e78425f9624b6ad956e24510260a2d9efcf`.
- E3 change-impact backend is accepted and merged to `main` at `69eacb3c1f1e9cea5e9fd735f40d1b256bc33587` (PR #111 head `d9aa539ae1288ddbcdfd4427d595b353a1db0c57`, CI `34259692961`).
- E4 template library backend is accepted and merged to `main` at `5243bb5b8752e6d94bcdf7403697b64ff88def96` (PR #113 head `c701ab32af38c0f0ef4db5bd51210634198d583a`, CI `34383868950`, migration 000070).
- E5 template clone + provenance backend is accepted and merged to `main` at `81ff86b0f54b1053491d20dc06f51c4dfef537fd` (PR #115 head `5c26d34e3e623ac1d1be414455c8a28b18ae1c62`, CI `34401634396`, migration 000071). E5 delivers only the **template-to-event clone** channel; manual creation, Excel, SAP/1C, late submission, and carrier import/export remain future gates.
- E6 Studio frontend is accepted and merged to `main` at `176461729dc2200d458eefad70ccdc126a2041b3` (PR #117 head `60d5fd28fb4601814a44ff4b7fb8d815b1635130`, CI `34509790393`). E6 delivers buyer Studio UI and acceptance evidence; late submission, Excel, ERP/TMS, and training remain future gates.
- E7 Phase 1 late submission backend is implemented on branch `feat/rfx-final-acceptance-v3.0e7` (migration 000072, five HTTP routes, carrier submit gate, OpenAPI, integration tests E7-INT-01..40 + E7-REM-001..008). Status: **IMPLEMENTED_PENDING_CONTROLLER_ACCEPTANCE** — controller remediation applied; exact-head CI on remediation HEAD (see PR #119). Historical green at `c045fd8` retained for audit only.
- E7 browser acceptance, Excel/ERP, frontend, and training have not started.
- Each implementation wave requires a separate controller gate.
- Complete v3.0E remains **IMPLEMENTATION_IN_PROGRESS** until E7 is accepted.

### Mandatory future gates

| Area | Markers | Target |
|---|---|---|
| Late submission workflow | `LATE_SUBMISSION_AND_DEADLINE_EXCEPTIONS=REQUIRED` — Phase 1 backend **IMPLEMENTED_PENDING_CONTROLLER_ACCEPTANCE**; Excel/ERP/frontend/training deferred | Before E7 browser acceptance |
| Buyer RFQ channels | `BUYER_RFQ_MANUAL_CREATION`, `BUYER_RFQ_TEMPLATE_CREATION` (E5: backend clone-from-template only), `BUYER_RFQ_EXCEL_IMPORT`, `BUYER_RFQ_ERP_INTEGRATION` (SAP/1C/ERP/TMS) — Excel/SAP/1C/manual UX **not implemented** | Post-E6 / pre-pilot |
| Carrier offer channels | `CARRIER_DIRECT_OFFER_ENTRY`, `CARRIER_OFFER_EXCEL_EXPORT_IMPORT`; `CARRIER_ERP_INTEGRATION=NOT_REQUIRED_CURRENT_SCOPE` | Post-E6 / pre-pilot |
| Competitor confidentiality | `CARRIER_CAN_VIEW_COMPETITOR_*=NO`, backend enforcement + cross-carrier isolation tests; carrier must not see participants, competitor identities, bids, submission times, or late-submission requests | Before pilot |
| Excel import/export | `EXCEL_IMPORT_EXPORT=REQUIRED` | Post-E6 |
| Training course | `USER_TRAINING_COURSE=REQUIRED` (RU/EN/ZH) | After UI stabilisation, before pilot |

See [RFX_V3_0E2_COMPARE_RESTORE.md](./implementation/RFX_V3_0E2_COMPARE_RESTORE.md) §5 for full requirement text.

---

## 7. v3.0F — Qualification Pool

| Deliverable | Architecture refs |
|---|---|
| `QualificationResult`, pools, members | Data model §3.5, ADR-005 |
| RFI qualification flow | Diagrams §3, functional baseline §4 |
| Pool update on qualify event | Events §3 |

---

## 8. v3.0G — Carrier 360

| Deliverable | Architecture refs |
|---|---|
| Aggregation API | Carrier 360 |
| Autofill + confirmation UX | Carrier 360 §5–6 |
| Provenance on answers | ADR-007 |

---

## 9. v3.0H — Analytics + Explainability

| Deliverable | Architecture refs |
|---|---|
| Evaluation dashboards | Gap matrix |
| Score drill-down from `explanation_json` | Scoring engine §7 |
| Audit panel in web-admin | Gap matrix §4 |

---

## 10. v3.0I — AI

| Deliverable | Architecture refs |
|---|---|
| Bounded assist capabilities | AI doc §2 |
| AI safety prohibitions | AI doc §3, ADR-010 |
| Review state on extracted values | AI doc §4 |

---

## 11. v3.0J — Enterprise Hardening

| Deliverable | Architecture refs |
|---|---|
| Transactional outbox + Kafka | Events, data model §3.7 |
| Notifications + reminders | Events, data model §3.6 |
| Approval gates | ADR-009 |
| OpenAPI parity with router | Gap matrix §4 |
| Security hardening, performance | Security |

---

## 12. Gap-driven priorities

See [RFX_V3_GAP_MATRIX.md](./RFX_V3_GAP_MATRIX.md) for repository-backed current vs target assessment.

---

## 13. References

- [README.md](./README.md)
- [RFX_V3_GAP_MATRIX.md](./RFX_V3_GAP_MATRIX.md)
- [ADR index](./adr/)

