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
| **v3.0D** | Scoring + Knockout | Score models, knockout, explainability | **IMPLEMENTED_PENDING_CONTROLLER_ACCEPTANCE** |
| **v3.0E** | Templates + Versioning | Template library, immutable published versions, compare/restore | Planned |
| **v3.0F** | Qualification Pool | Qualification results, pools, RFI→RFQ handoff | Planned |
| **v3.0G** | Carrier 360 | Profile autofill, freshness, confirmation | Planned |
| **v3.0H** | Analytics + Explainability | Dashboards, score drill-down, audit views | Planned |
| **v3.0I** | AI | Bounded assist — extraction, suggestions, explanations | Planned |
| **v3.0J** | Enterprise Hardening | Approval chains, notifications, outbox, OpenAPI parity, performance | Planned |

**STOP_AFTER_V3_0A=YES** — no implementation tasks authorized from this stream.

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
