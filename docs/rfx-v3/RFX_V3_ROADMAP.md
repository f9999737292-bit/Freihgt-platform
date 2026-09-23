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
| **v3.0E** | Templates + Versioning | Template library, immutable published versions, compare/restore, late submission, Excel/ERP | **IMPLEMENTATION_IN_PROGRESS** (E1–E6 + E7 Phase 1 + E7 Phase 2 foundation + Buyer XLSX Export V1 + Buyer XLSX Import Preview P2/P2.1/P3 + Buyer XLSX Import P4 Commit + Frontend F1–F4 + **E7 browser acceptance IMPLEMENTED_ACCEPTED** + **F5 Create-from-XLSX backend W1 IMPLEMENTED_ACCEPTED** + **F5 frontend upload-only IMPLEMENTED_ACCEPTED** + **F5 template acquisition W1 backend IMPLEMENTED_ACCEPTED**; E7 Phase 2 overall in progress; F5 template acquisition W2 frontend **IMPLEMENTED_ACCEPTED**; W3 browser acceptance **IMPLEMENTED_ACCEPTED** (PR #158, product HEAD `c7f5ae934eda50340836960ea04fc5f2b3c6a2e1`, CI `35783111497`, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W3`); F5 overall and Frontend Phase 2 remain in progress; training not started) |
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
| **E7 Phase 1 Late Submission** | Per-carrier late submission backend (migration 000072, API, RBAC, OpenAPI, E7-INT-01..40 + E7-REM-001..008) | **IMPLEMENTED_ACCEPTED** — PR #119 merged via `598b0b3` (head `3a2f4b9`, CI `34599209374`, migration 000072) |
| **E7 Phase 2 Foundation + Buyer XLSX Export V1** | Migration 000073, ZIP security, import-analysis/external-link repos, buyer draft XLSX export (7-sheet `BINTRANS_RFX_BUYER_XLSX_V1`, Excelize v2.11.0, E7P2-INT-01..20) | **IMPLEMENTED_ACCEPTED** — PR #123 merged via `a6b66be` (head `9a75029`, CI `34639373871`, migration 000073) |
| **E7 Phase 2 Buyer XLSX Import Preview P2/P2.1/P3** | Parser/validator, preview HTTP, OPTION A analysis persistence (`UPDATE_EXISTING_DRAFT`, E7P2-INT-21..42) | **IMPLEMENTED_ACCEPTED** — PR #125 merged via `b2df9ac` (head `db5bb9ef`, CI `34719259509`, migration 000073 reused) |
| **E7 Phase 2 Buyer XLSX Import Commit (P4)** | Atomic/single-use apply of preview analysis to DRAFT (`UPDATE_EXISTING_DRAFT`, E7P2-INT-43..70) | **IMPLEMENTED_ACCEPTED** — PR #129 merged via `0756c3fc` (head `43924472`, CI `34773513465` attempt 2, migration 000073 reused) |
| **E7 Phase 2 Carrier XLSX Architecture** | Frozen discovery (8-sheet `BINTRANS_RFX_CARRIER_XLSX_V1`, response-scoped routes, competitor confidentiality, zero-lot, no auto-submit) | **IMPLEMENTED_ACCEPTED** — PR #131 merged via `7454b949` (head `1859872b`, CI `34812566043`) |
| **E7 Phase 2 Carrier XLSX C1 Export** | Own-response GET export (DRAFT + SUBMITTED read-only, E7P2-INT-71..79, INT-42 remediation) | **IMPLEMENTED_ACCEPTED** — PR #132 merged via `fa850826` (head `479d1fd7`, CI `34833481110`, migration 000073 reused) |
| **E7 Phase 2 Carrier XLSX C2 Preview** | Carrier import preview parser + analysis persistence (E7P2-INT-80..90) | **MERGED_ACCEPTED** — PR #134 merged via `66241e67` (head `4a04e7ec`, product head `5b2fa47a`, CI `34864516651`, migration 000073 reused) |
| **E7 Phase 2 Carrier XLSX C3 Commit** | Atomic commit from stored proposal (DRAFT-only, mandatory Idempotency-Key, E7P2-INT-91..119) | **MERGED_ACCEPTED** — PR #135 merged via `e3341f05` (head `b48af2ff`, product head `5326ff8c`, CI `34884705304`, migration 000073 reused) |
| **E7 Phase 2 ERP API Architecture** | Generic ERP JSON contract + controller acceptance (ADR-012..015, E7P2-INT-120..195, PR #137) | **FROZEN_ACCEPTED** |
| **E7 Phase 2 ERP API Implementation Plan** | Wave plan E1–E6, INT-120..195 ownership, migration/auth/API sequencing, per-wave OpenAPI parity | **FROZEN_ACCEPTED** |
| **E7 Phase 2 ERP API E1 Foundation** | Migration 000074, integration principals, credentials, scopes, XOR ownership, stable external identity, reference mapping | **IMPLEMENTED_ACCEPTED** — PR #139 (head `5713d7c5`, CI `35066148105`) |
| **E7 Phase 2 ERP API E2 Machine Auth** | OAuth/API-key integration auth, trusted header strip/inject | **IMPLEMENTED_ACCEPTED** — PR #142 |
| **E7 Phase 2 ERP API E3 CREATE/UPDATE Preview** | Canonical JSON parser, mapping pin, CREATE/UPDATE Preview only | **IMPLEMENTED_ACCEPTED** — PR #143 (head `f60e1917`, CI `35382305777`) |
| **E7 Phase 2 ERP API E3.1 Hardening** | Strict Content-Type, nested IngestJSON, per-type mapping pins, stable `rfx.erp.*` errors | **IMPLEMENTED_ACCEPTED** — PR #144 (head `f4a0ed73`, CI `35390138371`) |
| **E7 Phase 2 ERP API E4 CREATE Commit** | Atomic DRAFT create from preview analysis, stable external link, idempotency, no auto-publish | **IMPLEMENTED_ACCEPTED** — PR #145 (head `3ab1c8e5`, CI `35459414961`) |
| **E7 Phase 2 ERP API E5 UPDATE Commit** | Atomic DRAFT update from preview analysis, existing-link revision policy B, no auto-publish | **IMPLEMENTED_ACCEPTED** — PR #146 (head `1a12c999`, CI `35466989829`) |
| **E7 Phase 2 ERP API E6** | GET/status/capabilities (no 000075) | **IMPLEMENTED_ACCEPTED** — PR #147 (head `72950d64`, CI `35491362938`) |
| **E7 Phase 2 Frontend F1 Buyer XLSX UPDATE_DRAFT** | web-procurement `/tenders/:id` export / preview / commit for `UPDATE_EXISTING_DRAFT` | **IMPLEMENTED_ACCEPTED** — PR #148 (head `b3b0b9e4`, CI `35506563407`, 6/6 browser PASS 0 skipped) |
| **E7 Phase 2 Frontend F2 Carrier XLSX UPDATE_CARRIER_DRAFT** | web-procurement `/carrier/tenders/:id` export / preview / commit for `UPDATE_CARRIER_DRAFT` | **IMPLEMENTED_ACCEPTED** — PR #149 (product head `00849e3a`, controller CI `35516181597`) |
| **E7 Phase 2 Frontend F3 Late Submission** | web-procurement carrier request/mine, buyer queue approve/reject, late questionnaire submit of an existing DRAFT in an APPROVED window | **IMPLEMENTED_ACCEPTED** — PR #150 (product head `22536fb2`, controller CI `35523932949`, verdict `ACCEPT_FRONTEND_PHASE2_F3`) |
| **E7 Phase 2 Frontend F4 Human Provenance** | web-procurement `/tenders/:id` shows stored `creation_channel` from human JWT `GET /api/v1/rfx-events/{id}` (RU/EN/ZH; no `external_link`; no `/integrations/erp/*`) | **IMPLEMENTED_ACCEPTED** — PR #151 (product head `ee14a606`, controller CI `35532693100`, verdict `ACCEPT_FRONTEND_PHASE2_F4`) |
| **E7 Phase 2 Frontend F5 Create-from-XLSX upload-only** | web-procurement `/tenders/new-from-xlsx` human-JWT upload-only preview/commit; HTTP 201 → DRAFT `creation_channel=EXCEL`; no publish/submit/ERP; no blank-template download | **IMPLEMENTED_ACCEPTED** — PR #154 (product head `2acb7c0e`, controller CI `35731621656`, verdict `ACCEPT_F5_FRONTEND_UPLOAD_ONLY`) |
| **E7 Phase 2 F5 XLSX template acquisition discovery** | Supported way for a buyer to obtain a CREATE-compatible blank XLSX | **DISCOVERY_ACCEPTED** — verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_SCOPE`, accepted HEAD `c0e39487`, CI `35745948306` attempt 2; Variant A `GET /api/v1/rfx-events/xlsx-create/template`; OpenAPI SoT `scripts/openapi/generate_openapi.py`; `F5-TA-1=FIXED_ACCEPTED`; `F5-TA-2=NOTE_CLOSED` |
| **E7 Phase 2 F5 XLSX template acquisition W1 backend** | Read-only Variant A blank BUYER XLSX V1 download | **IMPLEMENTED_ACCEPTED** — PR #156, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W1`, product HEAD `c835d19e`, CI `35760651845`; `GET /api/v1/rfx-events/xlsx-create/template`; human JWT + `PolicyBuyerManage`; flag-off 404; no event/analysis writes |
| **E7 Phase 2 F5 XLSX template acquisition W2 frontend** | web-procurement `/tenders/new-from-xlsx` blank template download next to the XLSX upload | **IMPLEMENTED_ACCEPTED** — PR #157, product HEAD `c51b39f5d7d2a683b4c1e3261cce93f8dee38231`, CI `35773932856`, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W2` |
| **E7 Phase 2 F5 XLSX template acquisition W3 browser** | Live download of the blank CREATE template inside `rfx-buyer-xlsx-create-browser-e2e` | **IMPLEMENTED_ACCEPTED** — PR #158, product HEAD `c7f5ae934eda50340836960ea04fc5f2b3c6a2e1`, CI `35783111497`, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W3`; isolated 1 passed, full suite 10 passed, skipped 0, retries 0; live download errors stay on the W2 component tests |
| **E7 Phase 2 F5 Create-from-XLSX overall** | One live chain: blank download, upload of that file, preview, commit, DRAFT/EXCEL, zero participants, scoped DB proof | **IMPLEMENTED_ACCEPTED** — [RFX_V3_0E7_CREATE_FROM_XLSX_OVERALL_FINAL_ACCEPTANCE.md](./implementation/RFX_V3_0E7_CREATE_FROM_XLSX_OVERALL_FINAL_ACCEPTANCE.md); PR #159, product HEAD `3c3e37ce8b7d28b17799f20ade8c5c7246282986`, CI `35886064236`, verdict `ACCEPT_F5_OVERALL_FINAL_ACCEPTANCE`; isolated overall-chain 1 passed, scoped DB assertions PASS, full CREATE suite 11 passed, skipped 0, retries 0 |
| **E7 Phase 2 Frontend** | Remaining Frontend Phase 2 after F5 upload-only (W2 template download UX, not an ERP browser client) | **IMPLEMENTATION_IN_PROGRESS** |
| **E7 Phase 2 Training** | RU/EN/ZH training course for accepted F5 Create-from-XLSX | **DISCOVERY_ACCEPTED** — [RFX_V3_0E7_CREATE_FROM_XLSX_TRAINING_DISCOVERY.md](./implementation/RFX_V3_0E7_CREATE_FROM_XLSX_TRAINING_DISCOVERY.md); PR #160, discovery HEAD `5b716dc01904616f4650e0f310fac6a42ea2c2ba`, CI `35893849771`, verdict `ACCEPT_F5_TRAINING_SCOPE`; `TRAINING_STATUS=NOT_STARTED`; `TRAINING_IMPLEMENTATION_AUTHORIZED=NO` |
| **E7 Browser Acceptance** | Final real browser acceptance gate (one event / one response through Award; no ERP browser client; no Award→TO) | **IMPLEMENTED_ACCEPTED** — PR #152 (product head `fd5b615f`, product CI `35644222482`, 13/13 passed 0 skipped retries 0, verdict `ACCEPT_E7_BROWSER_ACCEPTANCE`) |

Notes:

- The E1 implementation expanded the backend foundation while remaining inside the accepted v3.0E architecture.
- E2 compare + restore backend is accepted and merged to `main` at `e1780e78425f9624b6ad956e24510260a2d9efcf`.
- E3 change-impact backend is accepted and merged to `main` at `69eacb3c1f1e9cea5e9fd735f40d1b256bc33587` (PR #111 head `d9aa539ae1288ddbcdfd4427d595b353a1db0c57`, CI `34259692961`).
- E4 template library backend is accepted and merged to `main` at `5243bb5b8752e6d94bcdf7403697b64ff88def96` (PR #113 head `c701ab32af38c0f0ef4db5bd51210634198d583a`, CI `34383868950`, migration 000070).
- E5 template clone + provenance backend is accepted and merged to `main` at `81ff86b0f54b1053491d20dc06f51c4dfef537fd` (PR #115 head `5c26d34e3e623ac1d1be414455c8a28b18ae1c62`, CI `34401634396`, migration 000071). E5 delivers only the **template-to-event clone** channel; manual creation, Excel, SAP/1C, late submission, and carrier import/export remain future gates.
- E6 Studio frontend is accepted and merged to `main` at `176461729dc2200d458eefad70ccdc126a2041b3` (PR #117 head `60d5fd28fb4601814a44ff4b7fb8d815b1635130`, CI `34509790393`). E6 delivers buyer Studio UI and acceptance evidence; late submission, Excel, ERP/TMS, and training remain future gates.
- E7 Phase 1 late submission backend is accepted and merged to `main` at `598b0b3f19861480ec2716e174692eb1d2425bed` (PR #119 head `3a2f4b91c131c2fefe1056821618c7661fd4f4bc`, CI `34599209374`, migration 000072). Delivers migration 000072, five HTTP routes, carrier submit gate, OpenAPI, integration tests E7-INT-01..40 + E7-REM-001..008. Historical green at `c045fd8` retained for audit only.
- E7 Phase 2 foundation + Buyer XLSX Export V1 is accepted and merged to `main` at `a6b66bea51579d689e98188e31446e436524927d` (PR #123 head `9a7502902d4ddbdedbb4e54817c183b8a00a7140`, CI `34639373871`, migration 000073). Delivers buyer draft XLSX export only.
- E7 Phase 2 Buyer XLSX Import Preview P2/P2.1/P3 is accepted and merged to `main` at `b2df9ac20ae9bf9dbbb48f5e78199766643ad2ef` (PR #125 head `db5bb9ef5e2abc527af2f6f50cc2ea5b1497d754`, CI `34719259509`, E7P2-INT-21..42 PASS). Delivers parser, preview HTTP, and immutable analysis persistence.
- E7 Phase 2 Buyer XLSX Import Commit (P4) is accepted and merged to `main` at `0756c3fc5fb0a24f095859abb9008a447fa2dfe2` (PR #129 head `43924472c091a5e0e4845c92fac31c4b374ee391`, CI `34773513465` attempt 2, E7P2-INT-43..70 PASS). Delivers atomic commit apply for **UPDATE_EXISTING_DRAFT** only — Create from XLSX, carrier Excel, ERP, frontend, and training remain future gates.
- E7 Phase 2 Carrier XLSX architecture is accepted and merged to `main` at `7454b94916fe76a9712ed80e0631db70bba5539b` (PR #131 head `1859872b`, CI `34812566043`). Delivers frozen discovery only.
- E7 Phase 2 Carrier XLSX C1 Export is accepted and merged to `main` at `fa850826c6f7ed7f25e948dcc8fc8cad41eb2d74` (PR #132 head `479d1fd7`, CI `34833481110`, E7P2-INT-71..79 PASS, INT-42 remediation PASS). Delivers carrier own-response GET export only.
- E7 Phase 2 Carrier XLSX C2 Preview is accepted and merged to `main` at `66241e67d58c411efb9fa425902e459df65772fc` (PR #134 head `4a04e7ec`, product head `5b2fa47a`, CI `34864516651`, E7P2-INT-80..90 PASS). Delivers carrier import preview parser, valid-only analysis persistence, and Preview HTTP route. Non-blocking findings deferred: INT-113 closed in C3, partial C1 no-write snapshot (LOW), theoretical `target_version` INTEGER overflow guard (LOW).
- E7 Phase 2 Carrier XLSX C3 Commit is accepted and merged to `main` at `e3341f0594cb749e5bb27d70053a495838eed3c8` (PR #135 head `b48af2ff`, product head `5326ff8c`, CI `34884705304`, E7P2-INT-91..119 PASS). Delivers atomic commit from persisted preview analysis to DRAFT response (no auto-submit). Open findings: C1 LOW-02 extended no-write snapshot (REMAINS_OPEN), `target_version` INTEGER overflow guard (REMAINS_LOW), dedicated named JSONB round-trip integration test absent (implicit runtime/unit coverage).
- E7 Phase 2 Carrier XLSX backend (C1 Export + C2 Preview + C3 Commit) is **IMPLEMENTED_ACCEPTED** on `main`. F5 frontend upload-only is **IMPLEMENTED_ACCEPTED**. F5 template acquisition W1 backend is **IMPLEMENTED_ACCEPTED**. W2 frontend is **IMPLEMENTED_ACCEPTED** (PR #157, product HEAD `c51b39f5d7d2a683b4c1e3261cce93f8dee38231`, CI `35773932856`, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W2`). W3 live-browser acceptance is **IMPLEMENTED_ACCEPTED** (PR #158, product HEAD `c7f5ae934eda50340836960ea04fc5f2b3c6a2e1`, CI `35783111497`, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W3`; isolated 1 passed, full suite 10 passed, skipped 0, retries 0). F5 overall is **IMPLEMENTED_ACCEPTED** (PR #159). Frontend Phase 2 remains **IMPLEMENTATION_IN_PROGRESS**. Training implementation remains **NOT_STARTED**. Training discovery is **DISCOVERY_ACCEPTED** (PR #160, verdict `ACCEPT_F5_TRAINING_SCOPE`) and is not implementation authorization.
- E7 Phase 2 ERP API **architecture** frozen accepted (PR #137): [RFX_V3_0E7_ERP_API.md](./implementation/RFX_V3_0E7_ERP_API.md), ADR-012..015 Accepted, acceptance matrix E7P2-INT-120..195. Controller verdict `ACCEPT_ERP_API_ARCHITECTURE`. `ERP_API_IMPLEMENTATION_AUTHORIZED=NO`; migration 000074 proposed not created.
- E7 Phase 2 ERP API **implementation plan** frozen accepted (PR #138): [RFX_V3_0E7_ERP_API_IMPLEMENTATION_PLAN.md](./implementation/RFX_V3_0E7_ERP_API_IMPLEMENTATION_PLAN.md), [RFX_V3_0E7_ERP_API_IMPLEMENTATION_WAVES.md](./implementation/RFX_V3_0E7_ERP_API_IMPLEMENTATION_WAVES.md). Controller verdict `ACCEPT_ERP_API_IMPLEMENTATION_PLAN`. `PUBLIC_ROUTE_CONTRACT_POLICY=INCREMENTAL_PER_WAVE`.
- E7 Phase 2 ERP API **E1 foundation** accepted (PR #139 head `5713d7c5`, CI `35066148105`, controller `ACCEPT_ERP_API_E1`): [RFX_V3_0E7_ERP_API_E1_IMPLEMENTATION.md](./implementation/RFX_V3_0E7_ERP_API_E1_IMPLEMENTATION.md). Delivers migration 000074 and repository foundations only — no public ERP routes, OAuth, or OpenAPI changes.
- E7 Phase 2 ERP API **E3 CREATE/UPDATE Preview** accepted (PR #143 head `f60e1917979cb2133f7d5fcd74cbdd8d66ddea79`, CI `35382305777`, controller `ACCEPT_ERP_API_E3`): [RFX_V3_0E7_ERP_API_E3_IMPLEMENTATION.md](./implementation/RFX_V3_0E7_ERP_API_E3_IMPLEMENTATION.md). Delivers parser, mapping pin, and Preview only — Commit/GET/capabilities were out of E3.
- E7 Phase 2 ERP API **E3.1 hardening** accepted (PR #144 head `f4a0ed73c733198b451561e4b49a9acde46eba37`, CI `35390138371`, controller `ACCEPT_ERP_API_E3_1`). Delivers Content-Type enforcement, unified nested ingest, per-mapping-type pins, and stable public error keys. Non-blocking residuals: `F_E31_L1`, `F_E31_L2`.
- E7 Phase 2 ERP API **E4 CREATE Commit** accepted (PR #145 head `3ab1c8e51f694583579af504c2eb24ac2cf75729`, CI `35459414961`, controller `ACCEPT_ERP_API_E4`): [RFX_V3_0E7_ERP_API_E4_IMPLEMENTATION.md](./implementation/RFX_V3_0E7_ERP_API_E4_IMPLEMENTATION.md). Delivers `POST /api/v1/integrations/erp/rfx/drafts/commit` only — event stays DRAFT.
- E7 Phase 2 ERP API **E5 UPDATE Commit** accepted (PR #146 head `1a12c999d5cea2b2cc72d185773b22090f8ed261`, CI `35466989829`, controller `ACCEPT_ERP_API_E5`): [RFX_V3_0E7_ERP_API_E5_IMPLEMENTATION.md](./implementation/RFX_V3_0E7_ERP_API_E5_IMPLEMENTATION.md). Delivers `POST /api/v1/rfx-events/{id}/erp-import/commit` with revision policy B for existing links. Event stays DRAFT.
- E7 Phase 2 ERP API **E6 GET/status/capabilities** accepted (PR #147 head `72950d64a0f16251f2b94abfe02c3cfd346f6d45`, CI `35491362938`, controller `ACCEPT_ERP_API_E6`): [RFX_V3_0E7_ERP_API_E6_IMPLEMENTATION.md](./implementation/RFX_V3_0E7_ERP_API_E6_IMPLEMENTATION.md). Four integration-auth GET operations; migration `000075` and auto-publish remain unauthorized; INT-196 remains free. Non-blocking: `F_E6_L1` (OpenAPI lists `409 event_not_draft` on analysis status and capabilities).
- E7 Phase 2 Frontend **F1 Buyer XLSX UPDATE_DRAFT UI** accepted (PR #148 head `b3b0b9e48c18a96085a6f742e8ba7b10d4773c41`, CI `35506563407`, 6/6 browser PASS 0 skipped, verdict `ACCEPT_FRONTEND_PHASE2_F1`): web-procurement `/tenders/:id` human-JWT export/preview/commit for `UPDATE_EXISTING_DRAFT` only.
- E7 Phase 2 Frontend **F2 Carrier XLSX UPDATE_CARRIER_DRAFT UI** accepted (PR #149 product head `00849e3a2f7ddef809766458c92f1a961d1b3dec`, CI `35516181597`, verdict `ACCEPT_FRONTEND_PHASE2_F2`): web-procurement `/carrier/tenders/:id` human-JWT export/preview/commit for `UPDATE_CARRIER_DRAFT` only after own `response.id`; carrier-scoped `GET /api/v1/carrier/rfx-events/{id}`; Commit stays DRAFT; no `/submit` from XLSX.
- E7 Phase 2 Frontend **F3 Late Submission UI** accepted (PR #150 product head `22536fb2c4aa1fbc53080fba3d75e876277bc65b`, controller CI `35523932949`, 11/11 late-submission browser PASS 0 skipped, verdict `ACCEPT_FRONTEND_PHASE2_F3`): carrier Create → GET `/mine` → buyer approve/reject → late questionnaire submit of an existing DRAFT inside an active APPROVED window (`valid_from` inclusive, `valid_until` exclusive). Commercial `/rfx-responses/{id}/submit` stays closed after the deadline. Follow-up LOW (do not block F3): `F3_L1` `/mine` error shows wait-for-approval; `F3_L2` HTTP classifier omits 422 `response_deadline`; `F3_L3` panel and workspace do not share late-request state.
- E7 Phase 2 Frontend **F4 Human Provenance** accepted (PR #151 product head `ee14a6062b47e287f5a1981def40fe0e516741d2`, controller CI `35532693100`, 7/7 human-provenance browser PASS 0 skipped, verdict `ACCEPT_FRONTEND_PHASE2_F4`): web-procurement `/tenders/:id` shows stored `creation_channel` from buyer human JWT `GET /api/v1/rfx-events/{id}` (MANUAL/TEMPLATE/EXCEL/ERP; empty historical value omitted; no invented MANUAL). No `external_link`; browser never calls `/integrations/erp/*`. Follow-up LOW (do not block F4): `F4_R1` duck-typed 403/404 helper and synthesized `FORBIDDEN`/`NOT_FOUND`; `F4_R2` browser gate is `en-US` only (RU/ZH unit + i18n); `F4_R3` live fixture asserts MANUAL only; `F4_R4` OpenAPI `operationId` renamed by generate summary.
- E7 Phase 2 Frontend **F5 Create-from-XLSX upload-only** accepted (PR #154 product head `2acb7c0e27381c69707bde409447452a6e54d2f7`, controller CI `35731621656`, 5/5 create browser PASS 0 skipped retries 0, verdict `ACCEPT_F5_FRONTEND_UPLOAD_ONLY`): web-procurement `/tenders/new-from-xlsx` human-JWT upload-only preview/commit; stable `Idempotency-Key=buyer-xlsx-create-commit:<analysis_id>`; HTTP 201 → created tender `DRAFT` `creation_channel=EXCEL`; zero participants; no publish/submit/ERP. Role/flag/tenant/company stay fail-closed. Blank-template acquisition/generation stays out of this wave. Follow-up NOTE/LOW (do not block F5 upload-only): `F5-CR-1` machine code must not be the primary user string; `F5-CR-2` optional Vue mount coverage; `F5-CR-3` optional live invalidation for remaining metadata; `F5-CR-4` optional live retry with the same Idempotency-Key; `F5-CR-5` optional CREATE-specific code localization.
- E7 Phase 2 **F5 XLSX template acquisition W1 backend** accepted (PR #156 product HEAD `c835d19ee5d22bacd852852c7d4a3c723f5d4f5c`, controller CI `35760651845`, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W1`): read-only Variant A `GET /api/v1/rfx-events/xlsx-create/template` → downstream `GET /v1/rfx-events/xlsx-create/template`; human JWT + `PolicyBuyerManage`; flag-off 404; `tenant_id` query 403; shared 429; blank BUYER XLSX V1 with schema-only metadata and no event/analysis writes. Non-blocking notes: Chi segment matching after `/{id}`; cosmetic OpenAPI 400/409; existing Excel flag-off text/plain 404; constant filename is safe. W2 frontend is **IMPLEMENTED_ACCEPTED** (PR #157, product HEAD `c51b39f5d7d2a683b4c1e3261cce93f8dee38231`, CI `35773932856`, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W2`). W3 live-browser acceptance is **IMPLEMENTED_ACCEPTED** (PR #158, product HEAD `c7f5ae934eda50340836960ea04fc5f2b3c6a2e1`, CI `35783111497`, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W3`; isolated 1 passed, full suite 10 passed, skipped 0, retries 0; live download errors stay on the W2 component tests).
- E7 Phase 2 overall remains **IMPLEMENTATION_IN_PROGRESS**. Frontend Phase 2 as a whole is not closed: F5 template acquisition W1 backend is **IMPLEMENTED_ACCEPTED** (Variant A `GET /api/v1/rfx-events/xlsx-create/template`, product HEAD `c835d19e`, CI `35760651845`, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W1`); W2 frontend is **IMPLEMENTED_ACCEPTED** (PR #157, product HEAD `c51b39f5d7d2a683b4c1e3261cce93f8dee38231`, CI `35773932856`, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W2`); W3 live-browser acceptance is **IMPLEMENTED_ACCEPTED** (PR #158, product HEAD `c7f5ae934eda50340836960ea04fc5f2b3c6a2e1`, CI `35783111497`, verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_W3`; isolated 1 passed, full suite 10 passed, skipped 0, retries 0). Training is **NOT_STARTED**. F5 overall acceptance is not recorded. F5 is not an ERP browser client. TMS and Award→Transport Order are outside the E7 browser gate.
- **E7 Browser Acceptance** is **IMPLEMENTED_ACCEPTED** (PR #152 product head `fd5b615ff408f4d6e9b471cf687211d3ec7fcf80`, product CI `35644222482`, verdict `ACCEPT_E7_BROWSER_ACCEPTANCE`, 13 passed / 0 failed / 0 skipped / retries 0). Main-chain uses one event and one response through Award. Scoped contract: new Studio draft defaults `questionnaire_enabled=false`; successful `PublishQuestionnaire` enables and publishes atomically; failed publish leaves `false`; GET workspace unbound → 422; carrier start pinning keeps the same response ID and offer. Findings: `F-QE-DEFAULT=FIXED`, `F-OAPI-422=FIXED`, `F-LIVE-OFFER-PIN=FIXED`, `F-INPUT-NUMBER=NOTE`, `F-QE-VALIDATE-PUBLISH=NOTE`.
- Each implementation wave requires a separate controller gate.
- Complete v3.0E remains **IMPLEMENTATION_IN_PROGRESS** until F5 overall acceptance and training are accepted. W3 browser acceptance does not close F5 overall or Frontend Phase 2.

### Mandatory future gates

| Area | Markers | Target |
|---|---|---|
| Late submission workflow | `LATE_SUBMISSION_AND_DEADLINE_EXCEPTIONS=REQUIRED` — Phase 1 backend **IMPLEMENTED_ACCEPTED** (PR #119); F3 carrier late submission UI **IMPLEMENTED_ACCEPTED** (PR #150); E7 live late scenario **IMPLEMENTED_ACCEPTED** (PR #152) | Closed for F3 UI and E7 browser gate |
| Buyer approve/reject UI | Late-submission buyer approve/reject workflow UI — **IMPLEMENTED_ACCEPTED** (PR #150); SHIPPER_LOGIST read-only; E7 live late scenario **IMPLEMENTED_ACCEPTED** (PR #152) | Closed for F3 UI and E7 browser gate |
| Buyer RFQ channels | `BUYER_RFQ_MANUAL_CREATION`, `BUYER_RFQ_TEMPLATE_CREATION` (E5: backend clone-from-template only), `BUYER_RFQ_EXCEL_IMPORT` (export **IMPLEMENTED_ACCEPTED** PR #123; preview **IMPLEMENTED_ACCEPTED** PR #125; P4 Commit apply **IMPLEMENTED_ACCEPTED** PR #129; F1 buyer UPDATE_DRAFT UI **IMPLEMENTED_ACCEPTED** PR #148; F5 Create-from-XLSX upload-only **IMPLEMENTED_ACCEPTED** PR #154; blank-template acquisition W1 backend **IMPLEMENTED_ACCEPTED** PR #156; W2 frontend **IMPLEMENTED_ACCEPTED** (PR #157); W3 **IMPLEMENTED_ACCEPTED** (PR #158)), `BUYER_RFQ_ERP_INTEGRATION` — ERP CREATE/UPDATE Commit **IMPLEMENTED_ACCEPTED** (PR #145/#146); ERP GET **IMPLEMENTED_ACCEPTED** (PR #147); TMS **NOT_STARTED** | Post-E6 / pre-pilot |
| Carrier offer channels | `CARRIER_DIRECT_OFFER_ENTRY`; `CARRIER_OFFER_EXCEL_EXPORT` (**MERGED_ACCEPTED** PR #132 C1); `CARRIER_OFFER_EXCEL_IMPORT_PREVIEW` (**MERGED_ACCEPTED** PR #134 C2); `CARRIER_OFFER_EXCEL_IMPORT_COMMIT` (**MERGED_ACCEPTED** PR #135 C3); F2 carrier XLSX UI **IMPLEMENTED_ACCEPTED** PR #149; `CARRIER_ERP_INTEGRATION=NOT_REQUIRED_CURRENT_SCOPE` | Post-E6 / pre-pilot |
| Competitor confidentiality | `CARRIER_CAN_VIEW_COMPETITOR_*=NO`, backend enforcement + cross-carrier isolation tests; carrier must not see participants, competitor identities, bids, submission times, or late-submission requests; buyer XLSX export enforces exclusion (E7P2-INT-18) | Before pilot |
| Excel import/export | `EXCEL_IMPORT_EXPORT=REQUIRED` — buyer export **IMPLEMENTED_ACCEPTED**; buyer import preview **IMPLEMENTED_ACCEPTED** (PR #125); P4 Commit apply **IMPLEMENTED_ACCEPTED** (PR #129); F1 buyer UPDATE_DRAFT UI **IMPLEMENTED_ACCEPTED** (PR #148); carrier export **MERGED_ACCEPTED** (PR #132 C1); carrier import preview **MERGED_ACCEPTED** (PR #134 C2); carrier import commit **MERGED_ACCEPTED** (PR #135 C3); carrier XLSX overall **IMPLEMENTED_ACCEPTED**; F2 carrier XLSX UI **IMPLEMENTED_ACCEPTED** (PR #149); F5 Create-from-XLSX upload-only **IMPLEMENTED_ACCEPTED** (PR #154); blank-template acquisition W1 backend **IMPLEMENTED_ACCEPTED** (PR #156); W2 frontend **IMPLEMENTED_ACCEPTED** (PR #157); W3 **IMPLEMENTED_ACCEPTED** (PR #158) | Post-E6 |
| P4 Commit gates | Atomic/single-use Commit; stale baseline rejection; consumed/expired analysis rejection — **IMPLEMENTED_ACCEPTED** (PR #129) | Closed for UPDATE_EXISTING_DRAFT |
| ERP integration | Generic ERP JSON contract API — architecture **FROZEN_ACCEPTED** (PR #137); E1–E6 **IMPLEMENTED_ACCEPTED**; TMS **NOT_STARTED** | Post-E6 / pre-pilot |
| Training course | `USER_TRAINING_COURSE=REQUIRED` (RU/EN/ZH) — Phase 2 training **NOT_STARTED** | After UI stabilisation, before pilot |
| Final browser acceptance | Real browser acceptance gate for E7 — **IMPLEMENTED_ACCEPTED** (PR #152 product head `fd5b615f`, CI `35644222482`, 13/13) | Closed for E7 browser gate; F5 W2 frontend **IMPLEMENTED_ACCEPTED** (PR #157); W3 live-browser acceptance is **IMPLEMENTED_ACCEPTED** (PR #158, CI `35783111497`, isolated 1 passed, full suite 10 passed, skipped 0, retries 0); F5 overall and Frontend Phase 2 remain in progress; training remains |

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

## 13. E7 Phase 2 buyer XLSX status markers

```
BUYER_XLSX_EXPORT_V1_STATUS=IMPLEMENTED_ACCEPTED
BUYER_XLSX_IMPORT_PREVIEW_STATUS=IMPLEMENTED_ACCEPTED
BUYER_XLSX_IMPORT_P4_STATUS=IMPLEMENTED_ACCEPTED
BUYER_XLSX_IMPORT_V1_UPDATE_DRAFT_STATUS=IMPLEMENTED_ACCEPTED

E7_PHASE2_STATUS=IMPLEMENTATION_IN_PROGRESS
E7_STATUS=IMPLEMENTATION_IN_PROGRESS
V3_0E_STATUS=IMPLEMENTATION_IN_PROGRESS

CREATE_FROM_XLSX_STATUS=IMPLEMENTATION_IN_PROGRESS
F5_OVERALL_STATUS=IMPLEMENTED_ACCEPTED
F5_BACKEND_W1_STATUS=IMPLEMENTED_ACCEPTED
F5_CREATE_FROM_XLSX_BACKEND_W1_STATUS=IMPLEMENTED_ACCEPTED
F5_CREATE_FROM_XLSX_CONTRACT_VERDICT=ACCEPT_F5_BACKEND_W1
F5_BACKEND_W1_PRODUCT_HEAD=1f5deadaa4e81e352c041a5b97318b1a640ac48f
F5_BACKEND_W1_CI_RUN=35695381395
F5_FRONTEND_UPLOAD_ONLY_STATUS=IMPLEMENTED_ACCEPTED
F5_FRONTEND_STATUS=IMPLEMENTATION_IN_PROGRESS
F5_FRONTEND_IMPLEMENTATION_AUTHORIZED=YES
F5_FRONTEND_UPLOAD_ONLY_PRODUCT_HEAD=2acb7c0e27381c69707bde409447452a6e54d2f7
F5_FRONTEND_UPLOAD_ONLY_CI_RUN=35731621656
F5_FRONTEND_UPLOAD_ONLY_CONTRACT_VERDICT=ACCEPT_F5_FRONTEND_UPLOAD_ONLY
F5_TEMPLATE_ACQUISITION_DECISION=A_UPLOAD_ONLY_FIRST_WAVE
F5_TEMPLATE_ACQUISITION_DECISION_REQUIRED=YES
F5_TEMPLATE_ACQUISITION_STATUS=IMPLEMENTED_ACCEPTED
F5_TEMPLATE_ACQUISITION_CONTROLLER_VERDICT=ACCEPT_F5_TEMPLATE_ACQUISITION_SCOPE
F5_TEMPLATE_ACQUISITION_DISCOVERY_HEAD=c0e39487482b9ffeca3706c089b15adc762007c6
F5_TEMPLATE_ACQUISITION_CI_RUN=35745948306
F5_TEMPLATE_ACQUISITION_CI_ATTEMPT=2
F5_TEMPLATE_ACQUISITION_VARIANT=A_BACKEND_GENERATED_BLANK_WORKBOOK
F5_TEMPLATE_ACQUISITION_ENDPOINT=GET /api/v1/rfx-events/xlsx-create/template
F5_TEMPLATE_ACQUISITION_OPENAPI_SOT=scripts/openapi/generate_openapi.py
F5_TA_1=FIXED_ACCEPTED
F5_TA_2=NOTE_CLOSED
F5_TEMPLATE_ACQUISITION_W1_STATUS=IMPLEMENTED_ACCEPTED
F5_TEMPLATE_ACQUISITION_W1_CONTROLLER_VERDICT=ACCEPT_F5_TEMPLATE_ACQUISITION_W1
F5_TEMPLATE_ACQUISITION_W1_PRODUCT_HEAD=c835d19ee5d22bacd852852c7d4a3c723f5d4f5c
F5_TEMPLATE_ACQUISITION_W1_CI_RUN=35760651845
F5_TEMPLATE_ACQUISITION_W2_STATUS=IMPLEMENTED_ACCEPTED
F5_TEMPLATE_ACQUISITION_W2_CONTROLLER_VERDICT=ACCEPT_F5_TEMPLATE_ACQUISITION_W2
F5_TEMPLATE_ACQUISITION_W2_PRODUCT_HEAD=c51b39f5d7d2a683b4c1e3261cce93f8dee38231
F5_TEMPLATE_ACQUISITION_W2_CI_RUN=35773932856
F5_TEMPLATE_ACQUISITION_W3_STATUS=IMPLEMENTED_ACCEPTED
F5_TEMPLATE_ACQUISITION_W3_CONTROLLER_VERDICT=ACCEPT_F5_TEMPLATE_ACQUISITION_W3
F5_TEMPLATE_ACQUISITION_W3_PRODUCT_HEAD=c7f5ae934eda50340836960ea04fc5f2b3c6a2e1
F5_TEMPLATE_ACQUISITION_W3_CI_RUN=35783111497
F5_TEMPLATE_ACQUISITION_W3_PR=158
F5_TEMPLATE_ACQUISITION_W3_BROWSER=ISOLATED_1_PASSED_FULL_SUITE_10_PASSED_SKIPPED_0_RETRIES_0
F5_TEMPLATE_ACQUISITION_W3_LIVE_DOWNLOAD_ERROR_PATH=W2_COMPONENT_TESTS
F5_OVERALL_FINAL_ACCEPTANCE_STATUS=IMPLEMENTED_ACCEPTED
F5_OVERALL_FINAL_ACCEPTANCE_CONTROLLER_VERDICT=ACCEPT_F5_OVERALL_FINAL_ACCEPTANCE
F5_OVERALL_FINAL_ACCEPTANCE_PRODUCT_HEAD=3c3e37ce8b7d28b17799f20ade8c5c7246282986
F5_OVERALL_FINAL_ACCEPTANCE_CI_RUN=35886064236
F5_OVERALL_FINAL_ACCEPTANCE_PR=159
F5_OVERALL_FINAL_ACCEPTANCE_BROWSER=ISOLATED_1_PASSED_DB_ASSERTIONS_PASS_FULL_SUITE_11_PASSED_SKIPPED_0_RETRIES_0
F5_TEMPLATE_ACQUISITION_IMPLEMENTATION_AUTHORIZED=YES
F5_TEMPLATE_ACQUISITION_IMPLEMENTATION_STARTED=YES
F5_N1=WALL_CLOCK_VALIDATION_PLUS_COMMIT_NOWFN
F5_N2=CROSS_TENANT_FIXTURE_NOT_FOREIGN_BUYER_MANAGE
F5_N3=COMMIT_SHARES_RBAC_FLAG_MIDDLEWARE_WITH_PREVIEW
FOLLOW_UP_FINDINGS_BLOCK_F5_W1_MERGE=NO
CARRIER_XLSX_ARCHITECTURE_STATUS=IMPLEMENTED_ACCEPTED
CARRIER_XLSX_C1_EXPORT_STATUS=MERGED_ACCEPTED
CARRIER_XLSX_C2_PREVIEW_STATUS=MERGED_ACCEPTED
CARRIER_XLSX_C3_COMMIT_STATUS=MERGED_ACCEPTED
CARRIER_XLSX_C3_COMMIT_STARTED=YES
CARRIER_XLSX_OVERALL_STATUS=IMPLEMENTED_ACCEPTED
AUTO_SUBMIT_IMPLEMENTED=NO
INT_113_STATUS=CLOSED_IN_C3
C1_LOW_02_STATUS=REMAINS_OPEN
TARGET_VERSION_OVERFLOW_STATUS=REMAINS_LOW
CARRIER_ERP_INTEGRATION=NOT_REQUIRED_CURRENT_SCOPE
ERP_API_DISCOVERY_STATUS=COMPLETE
ERP_API_ARCHITECTURE_STATUS=FROZEN_ACCEPTED
ERP_API_IMPLEMENTATION_PLAN_STATUS=FROZEN_ACCEPTED
PUBLIC_ROUTE_CONTRACT_POLICY=INCREMENTAL_PER_WAVE
PUBLIC_ROUTE_OPENAPI_PER_WAVE=YES
ERP_API_IMPLEMENTATION_STATUS=IMPLEMENTATION_IN_PROGRESS
ERP_API_IMPLEMENTATION_AUTHORIZED=YES
ERP_API_IMPLEMENTATION_STARTED=YES
ERP_API_E1_AUTHORIZED=YES
ERP_API_E1_STATUS=IMPLEMENTED_ACCEPTED
MIGRATION_000074_AUTHORIZED=YES
MIGRATION_000074_CREATED=YES
ERP_API_E2_AUTHORIZED=YES
ERP_API_E2_STATUS=IMPLEMENTED_ACCEPTED
ERP_API_E3_STATUS=IMPLEMENTED_ACCEPTED
ERP_API_E3_AUTHORIZED=YES
ERP_API_E4_STATUS=IMPLEMENTED_ACCEPTED
ERP_API_E4_AUTHORIZED=YES
ERP_API_E5_STATUS=IMPLEMENTED_ACCEPTED
ERP_API_E5_AUTHORIZED=YES
ERP_API_E6_STATUS=IMPLEMENTED_ACCEPTED
ERP_API_E6_AUTHORIZED=YES
MIGRATION_000075_AUTHORIZED=NO
AUTO_PUBLISH_AUTHORIZED=NO
BUYER_RFQ_ERP_INTEGRATION=ARCHITECTURE_FROZEN_ACCEPTED
CONTROLLER_PREVIOUS_VERDICT=ACCEPT_F5_TEMPLATE_ACQUISITION_W1
CONTROLLER_VERDICT=ACCEPT_F5_TEMPLATE_ACQUISITION_W3
CONTROLLER_REVIEW_HEAD=c7f5ae934eda50340836960ea04fc5f2b3c6a2e1
CONTROLLER_CI_RUN=35783111497
CONTROLLER_CI_RESULT=SUCCESS
E7_BROWSER_ACCEPTANCE_STATUS=IMPLEMENTED_ACCEPTED
E7_BROWSER_ACCEPTANCE_PRODUCT_HEAD=fd5b615ff408f4d6e9b471cf687211d3ec7fcf80
E7_BROWSER_ACCEPTANCE_CI_RUN=35644222482
E7_BROWSER_ACCEPTANCE_RESULT=13_PASSED_0_FAILED_0_SKIPPED_RETRIES_0
F_QE_DEFAULT=FIXED
F_OAPI_422=FIXED
F_LIVE_OFFER_PIN=FIXED
F_INPUT_NUMBER=NOTE
F_QE_VALIDATE_PUBLISH=NOTE
ERP_BROWSER_CLIENT_STATUS=NOT_STARTED
TMS_STATUS=OUT_OF_SCOPE
AWARD_TO_TRANSPORT_ORDER_IN_E7_BROWSER_GATE=NO
FRONTEND_PHASE2_F1_BUYER_XLSX_UPDATE_DRAFT_STATUS=IMPLEMENTED_ACCEPTED
FRONTEND_PHASE2_F1_BROWSER_STATUS=IMPLEMENTED_ACCEPTED
FRONTEND_PHASE2_F2_CARRIER_XLSX_STATUS=IMPLEMENTED_ACCEPTED
FRONTEND_PHASE2_F2_BROWSER_STATUS=IMPLEMENTED_ACCEPTED
FRONTEND_PHASE2_F3_STATUS=IMPLEMENTED_ACCEPTED
FRONTEND_PHASE2_F3_BROWSER_STATUS=IMPLEMENTED_ACCEPTED
FRONTEND_PHASE2_F4_STATUS=IMPLEMENTED_ACCEPTED
FRONTEND_PHASE2_F4_BROWSER_STATUS=IMPLEMENTED_ACCEPTED
FRONTEND_PHASE2_F4_SCOPE=CREATION_CHANNEL_ONLY
FRONTEND_PHASE2_F4_HUMAN_GET=/api/v1/rfx-events/{id}
FRONTEND_PHASE2_F4_PRODUCT_HEAD=ee14a6062b47e287f5a1981def40fe0e516741d2
FRONTEND_PHASE2_F4_CI_RUN=35532693100
FRONTEND_PHASE2_F5_UPLOAD_ONLY_STATUS=IMPLEMENTED_ACCEPTED
FRONTEND_PHASE2_F5_UPLOAD_ONLY_BROWSER_STATUS=IMPLEMENTED_ACCEPTED
FRONTEND_PHASE2_F5_UPLOAD_ONLY_PRODUCT_HEAD=2acb7c0e27381c69707bde409447452a6e54d2f7
FRONTEND_PHASE2_F5_UPLOAD_ONLY_CI_RUN=35731621656
F5_CR_1=MACHINE_CODE_NOT_PRIMARY_USER_STRING
F5_CR_2=OPTIONAL_VUE_MOUNT_COVERAGE
F5_CR_3=OPTIONAL_LIVE_INVALIDATION_REMAINING_METADATA
F5_CR_4=OPTIONAL_LIVE_RETRY_SAME_IDEMPOTENCY_KEY
F5_CR_5=OPTIONAL_CREATE_SPECIFIC_CODE_LOCALIZATION
FOLLOW_UP_FINDINGS_BLOCK_F5_FRONTEND_UPLOAD_ONLY_MERGE=NO
F3_L1=MINE_ERROR_SHOWS_WAIT_FOR_APPROVAL
F3_L2=CLASSIFY_HTTP_ERROR_OMITS_RESPONSE_DEADLINE
F3_L3=PANEL_WORKSPACE_LATE_REQUEST_STATE_NOT_SHARED
FOLLOW_UP_FINDINGS_BLOCK_F3_MERGE=NO
F4_R1=DUCK_TYPED_403_404_HELPER_AND_SYNTHESIZED_FORBIDDEN_NOT_FOUND
F4_R2=BROWSER_GATE_EN_US_ONLY
F4_R3=LIVE_FIXTURE_ASSERTS_MANUAL_ONLY
F4_R4=OPENAPI_OPERATION_ID_RENAMED_BY_GENERATE_SUMMARY
FOLLOW_UP_FINDINGS_BLOCK_F4_MERGE=NO
F_E6_L1=OPENAPI_409_EVENT_NOT_DRAFT_ON_ANALYSIS_AND_CAPABILITIES
FOLLOW_UP_FINDINGS_BLOCK_E6_MERGE=NO
F_E3_C1=CONTENT_TYPE_ENFORCEMENT
F_E3_C2=NESTED_UNKNOWN_FIELD_REJECTION
F_E3_C3=UNIFIED_INGEST_PATH
F_E3_C4=PER_MAPPING_TYPE_PINNING
F_E3_C5=STABLE_ERROR_MESSAGE_KEYS
FOLLOW_UP_FINDINGS_BLOCK_E3_MERGE=NO
ERP_TEST_IDS=E7P2-INT-120..195
ERP_TEST_COUNT=76
NEXT_FREE_TEST_ID=E7P2-INT-221
MERMAID_VALIDATION=MANUAL_ONLY
FRONTEND_PHASE2_STATUS=IMPLEMENTATION_IN_PROGRESS
TRAINING_STATUS=NOT_STARTED
TRAINING_DISCOVERY_STATUS=DISCOVERY_ACCEPTED
TRAINING_IMPLEMENTATION_AUTHORIZED=NO
TRAINING_DISCOVERY_CONTROLLER_VERDICT=ACCEPT_F5_TRAINING_SCOPE
TRAINING_DISCOVERY_HEAD=5b716dc01904616f4650e0f310fac6a42ea2c2ba
TRAINING_DISCOVERY_CI_RUN=35893849771
TRAINING_DISCOVERY_PR=160
BROWSER_ACCEPTANCE_STATUS=IMPLEMENTED_ACCEPTED
NEXT_ACTION=AUTHORIZE_F5_TRAINING_T1_RU
NEXT_STAGE_SEQUENCE=F5_TRAINING_T1_RU_AUTHORIZATION
```

---

## 14. References

- [README.md](./README.md)
- [RFX_V3_GAP_MATRIX.md](./RFX_V3_GAP_MATRIX.md)
- [ADR index](./adr/)

