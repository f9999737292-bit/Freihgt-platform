# W1 Implementation Contract — Policy Builder Foundation (R2)

**Program:** RFX-FULL-TENDER-BLOCK-1.0  
**Wave:** W1 only  
**Contract revision:** R2 (remediates Agent J W1 review)  
**Status:** DOCS ONLY — `IMPLEMENTATION_AUTHORIZED=NO`  
**Main verified at pack creation:** `5432e1444ea4ed607cc8992e967023b241315759`  
**Migration pin:** `000105` (unchanged)  
**Source:** W0 Spec 0.9 + Controller Decisions 0.9  

---

## 1. Goal (W1 only)

Pure Go declarative Rule AST: types, parse/validate, bounded deterministic evaluation, explanations, schema versioning, **policy fingerprint**, unit/golden/negative security tests.

**Not in W1:** Award, multi-carrier runtime allocation, TO, EDO, live KPI, public write APIs, DB migrations, legacy scoring changes.

---

## 2. Preserve legacy RFx (hard)

| Must keep | Path |
|---|---|
| `uq_rfx_award_event` | `infrastructure/migrations/000038_rfx_evaluation_award_v1.3.up.sql` |
| `AwardResponse` | `services/rfx-service/internal/service/evaluation_service.go` |
| Award→TO | `services/rfx-service/internal/domain/award_conversion.go` |
| Legacy 70/30 score | `services/rfx-service/internal/domain/rfx_evaluation.go` |
| Late questionnaire E7 | `late_submission_*`, `000072` |
| Gateway RFx policies | `services/api-gateway/internal/rfxrbac/policies.go` |

New model only: `ALLOCATION_CAMPAIGN`, `AllocationPolicy`, `AllocationProposal`, `AllocationDecision`, `CarrierCommitment`.  
**No** synthetic legacy awards (Decision D2 OPTION_A).

---

## 3. Code placement (after separate W1 GO)

| Artifact | Path |
|---|---|
| AST + engine | `services/rfx-service/internal/allocationpolicy/` |
| Golden tests | `.../allocationpolicy/testdata/golden/` |
| Do not touch | `rfx_evaluation.go`, award handlers, migrations, CI workflows |

---

## 4. Normative companions (this directory)

| Doc | Closes |
|---|---|
| `W1_AST_SCHEMA.md` | J-W1-H001, M001, M002, M004 |
| `W1_ABAC_AUTHORIZATION_MATRIX.md` | J-W1-H002, M003 |
| `W1_TEST_MATRIX.md` | J-W1-H003 (+ W1-001…023) |
| `W1_AGENT_F_UI_HANDOFF.md` | UI authority + shared Score/fingerprint/CAP |
| `W2_TO_W9_DELIVERY_DEPENDENCIES.md` | Wave briefs |

Reproducibility (J-W1-C001): this six-file pack is committed on a docs-only branch / Draft PR with exact HEAD + CI.

---

## 5. Non-goals / gates

No localStorage as SoR · No HTTP writers in W1 · No automatic W2+ ·  
`PRODUCT_CODE` only after Controller W1 GO **and** Agent J PASS on this R2 pack.
