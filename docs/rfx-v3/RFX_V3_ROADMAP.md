# RFx v3.0A — Implementation Roadmap

**Status:** Architecture draft  
**Normative companion:** [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)

---

## 1. Non-deferrable capabilities

The following are **core enterprise RFx** capabilities — not optional late-phase additions:

| Capability | Wave |
|---|---|
| Buyer draft / resume | Wave 1 |
| Buyer autosave | Wave 1 |
| Carrier response draft / resume | Wave 1 |
| Server-authoritative validation | Wave 1 |
| Atomic autosave safety | Wave 1 |
| Valid-only persistence | Wave 1 |
| Carrier error UX (inline, section, global, deep link) | Wave 1 |
| Preview-as-carrier (read-only) | Wave 1 |
| Publish readiness gate | Wave 1 |
| Pre-submit validation gate | Wave 1 |
| Warning vs validation vs knockout classification | Wave 1 |
| Interactive preview test mode | Wave 2 |
| Scoring/knockout on persisted valid answers | Wave 2 |
| Version history & compare | Wave 2 |
| Post-publish material change / new version | Wave 3 |
| Change impact analysis & re-scoring | Wave 3 |

---

## 2. Wave 1 — Foundation (MVP enterprise response)

**Goal:** End-to-end draft response with safe persistence and error UX.

| Deliverable | Architecture refs |
|---|---|
| Domain model: `Answer`, `AnswerDraft`, `ResponseVersion` | Domain model §3–5 |
| PATCH autosave + optimistic concurrency | API §2 |
| 422 structured validation | API §2.3 |
| Four validation layers (L1–L3 on save, L4 on submit) | Questionnaire §2 |
| Carrier workspace UX flags | UX §2–6 |
| Buyer draft autosave (RFx Studio header/metadata) | API §5, UX §8 |
| Preview-as-carrier (static/read-only draft) | Validation contract §16 |
| Publish readiness gate (buyer) | Validation contract §18 |
| Pre-submit gate (carrier) | Validation contract §19 |
| State machine: `NOT_STARTED` → `IN_PROGRESS` → `SUBMITTED` | State machines §2 |

**Acceptance tests (minimum):** validation contract §24.1–24.3, submit/publish gate tests §24.4.

---

## 3. Wave 2 — Qualification & buyer test tooling

**Goal:** Scoring/knockout on valid answers; buyer interactive preview; version history.

| Deliverable | Architecture refs |
|---|---|
| `Warning`, `KnockoutResult`, `BusinessRuleResult` persistence | Domain model §3 |
| Scoring input: valid answers only | Questionnaire §4 |
| Interactive «Пройти как перевозчик» sandbox | Validation contract §17 |
| `VERSION_HISTORY`, `COMPARE_VERSIONS`, `RESTORE_DRAFT_VERSION` | Validation contract §21 |
| Section warning counters | UX §4 |

**Acceptance tests:** `KNOCKOUT_ANSWER_PERSISTED`, `WARNING_ANSWER_PERSISTED`, `PREVIEW_DOES_NOT_CREATE_RESPONSE`.

---

## 4. Wave 3 — Post-publish evolution

**Goal:** Safe material changes after publication.

| Deliverable | Architecture refs |
|---|---|
| Immutable published versions | State machines §4 |
| `NEW_RFX_VERSION` for material edits | Domain model §7 |
| `CHANGE_IMPACT_ANALYSIS` | API §7 |
| Reconfirmation / re-scoring workflows | Questionnaire §7 |

**Acceptance tests:** `PUBLISHED_RFX_DIRECT_EDIT_DENIED`, `NEW_VERSION_CREATED_FOR_MATERIAL_CHANGE`.

---

## 5. Explicitly out of Wave 1 (but architected)

| Item | Notes |
|---|---|
| Offline-first mobile carrier app | Local recovery optional; not authoritative |
| Advanced scoring optimisations | After Wave 2 baseline |
| Multi-language questionnaire builder | Parallel i18n track |

---

## 6. Dependencies

| Dependency | Notes |
|---|---|
| RFx v1 `rfx-service` | Extend; do not replace gateway boundary |
| Identity / company membership | Owner and participant scoping |
| Document service | Attachment validation L1/L2 |
| OpenAPI error envelope | Shared `422` contract |

---

## 7. References

- [README.md](./README.md)
- [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)
- [ADR-RFX-011](./adr/ADR-RFX-011-RESPONSE-VALIDATION-AND-DRAFT-SAFETY.md)
