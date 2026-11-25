# BINTRANS Smart Tender Allocation — Technical Spec v0.9 (W0 Freeze)

**Status:** ARCHITECTURE SPEC FREEZE CANDIDATE (docs only)  
**Date:** 2026-11-25  
**Baseline main:** `892361db87985f35132aef86550a025909af648a`  
**Migration head (unchanged):** `000105`  
**Prior:** TZ v0.7 + W0 remediation 0.8A + Controller Decisions 0.9  
**Implementation:** NOT AUTHORIZED  

Companion docs in this directory: Legacy Compatibility, KPI Catalog, Money Policy, RBAC Matrix, Migration/API Plan, J Findings Closure, Controller Decisions.

---

## 0. Purpose

Freeze architecture for Smart Tender Allocation so Agent J can perform final W0 review toward v1.0 approval.  
Product intent remains TZ v0.7 §1 (campaigns, rounds, eligibility, scoring, corridors, KPI, multi-carrier shares, budgets, commission, carrier confirmations, guarantees, appendices, EDO, TMS activation, plan/fact).  
Competitive claims vs Transporeon are **out of scope**.

---

## 1. Campaign types (Decision 1)

| Type | Behavior |
|---|---|
| `LEGACY_SINGLE_WINNER` | Existing RFx: evaluation → `AwardResponse` → one `rfx.rfx_awards` row (`uq_rfx_award_event`) → Award→TO conversion |
| `ALLOCATION_CAMPAIGN` | Parallel domain: Proposal → Decision → Commitment → Appendix → Activation → TO with ledger |

No silent conversion of legacy events. New model does not replace Single Winner lifecycle.

---

## 2. Multi-carrier path (Decision 2 — OPTION_A)

```
AllocationPolicyVersion (Rule AST)
  → AllocationInputSnapshot (immutable)
  → AllocationProposal (+ lines MAIN/RESERVE/…)
  → Commission ApprovalWorkflow (version-bound votes)
  → AllocationDecision (immutable when APPROVED)
  → CarrierCommitmentProposal / Response / Guarantees
  → ContractAppendixVersion → EDO evidence (mock until GO)
  → Activation gates → AllocationLedger → TransportOrder
```

**Forbidden:** synthetic legacy Award per carrier; dropping `uq_rfx_award_event`.

---

## 3. Business requirements preserved (Phase 3 checklist)

| # | Requirement | Spec location |
|---|---|---|
| 1 | Automatic + manual ad-hoc rounds | §5 Commercial rounds |
| 2 | Late questionnaire submission (existing) | Legacy compat; unchanged E7 |
| 3 | Separate late **commercial** bid authorization | §5.3 |
| 4 | HARD/SCORE/PREFERENCE/CAP/FLOOR/FALLBACK/APPROVAL/ACTION AST | §6 |
| 5 | Overall + lane KPI | KPI catalog + §7 |
| 6 | Price traffic light vs target (min/max zones) | §8 |
| 7 | Configurable MAIN + RESERVE counts (EXACT_N / UP_TO_N) | §9 |
| 8 | Preliminary global quotas 0–100% | §10; DEFAULT CAP_ONLY |
| 9 | Quota only on eligible lanes | §10 |
| 10 | May receive less than cap if expensive | §10 TARGET/CAP |
| 11 | Budgets: tender, lane, carrier, period | §8; Money policy |
| 12–13 | Commission + quorum / mandatory roles | RBAC matrix |
| 14–15 | Carrier confirm + policy-allowed partial | RBAC / §11 |
| 16 | Guarantees % / trips / day / week / month / calendar | §12 |
| 17 | Contract appendix from immutable snapshot | §13 |
| 18 | EDO after security + legal | §13; mock until GO |
| 19 | Plan/fact in TMS | §14 |
| 20 | Full legacy Single Winner preserved | Legacy compat |

Nothing from this agreed set is dropped in v0.9.

---

## 4. Logical data ownership

| Domain | Owner service (proposed) |
|---|---|
| RFx event, questionnaire versions, legacy award, late questionnaire | `rfx-service` |
| Commercial rounds, policies, snapshots, proposals, decisions, commission | `rfx-service` (+ schema extensions) |
| Optimizer job | in-process / worker under rfx-service (not `network-optimizer-service`) |
| Approved rates | `contract-rate-service` (read) |
| Commitments / guarantees / ledger | `rfx-service` allocation schema |
| Appendix / EDO adapter | `document-service` |
| TO create + quota consume | `transport-order-service` + ledger |
| Shipment execution | `shipment-service` |
| Identity / membership | `identity-service` / `company-service` |
| Gateway policies | `api-gateway` under `/api/v1/rfx/allocations` |
| UI | `web-procurement`, `web-carrier` |

No microservice per rule. Final placement audit remains Agent J’s remit before W1 code.

---

## 5. Commercial rounds (≠ questionnaire versioning)

**FSM:** DRAFT → SCHEDULED → OPEN → CLOSED → EVALUATED → APPROVED; terminals CANCELLED / SUPERSEDED.  
Modes: PREDEFINED | MANUAL_AD_HOC | HYBRID.  
Round config: round_id, campaign_id, sequence, type, participants, open/close, editable_fields, participation/transition rules, approval_policy_version, notification_policy, source_round_id, snapshot_version.

**Late commercial bid:** separate grant (carrier, tender, round, fields, window, approver, immutable response version). Does not rewrite approved Decision without re-approval procedure.  
**Late questionnaire:** existing E7 — unchanged.

**PARTIALLY_AWARDED (Decision 7):** not used for `ALLOCATION_CAMPAIGN`. Allocation uses its own Proposal/Decision FSM (incl. FEASIBLE_WITH_SHORTFALL). Legacy FSM string may remain for old transitions only.

---

## 6. Rule AST

Blocks: FILTER, SCORE, PREFERENCE, CAP, FLOOR, FALLBACK, APPROVAL, ACTION.  
Operators: AND/OR/NOT, EQ/NE, GT/GTE, LT/LTE, IN/NOT_IN, BETWEEN, EXISTS, IS_MISSING.  
Forbidden: arbitrary SQL, shell, user scripts, undocumented HTTP, unbounded recursion.  
`schema_version` required. Priority: authz → legal/ops HARD → commercial HARD → missing-data policy → score → optimize → approval rights. Score never bypasses HARD.

---

## 7. KPI (Decision 4)

Evidence-based only (see KPI catalog). Missing ≠ 0 ≠ average.  
Fallbacks: FAIL_ELIGIBILITY, FALLBACK_OVERALL_KPI, FALLBACK_COMPARABLE_LANE, RESERVE_ONLY, PILOT_VOLUME_CAP, MANUAL_REVIEW — always explained.  
W1/W2 without real sources: fail-closed or **labeled test simulation** — not production Award.

---

## 8. Price corridors & budgets (Decision 5)

Lane settings: target/min/max rates, green/yellow/red zones, currency, tax_basis, rate_unit, included services, validity.  
Comparability required or `RATE_NOT_COMPARABLE`.  
Budgets: target, projected, lane, carrier, MAIN/RESERVE pools, period, guarantee cost, committed, reallocation forecast, **separately labeled** uncovered volume cost.  
Arithmetic: decimal (`shopspring/decimal`); ProjectedCost = Σ x·r; Variance = Projected − Target.

---

## 9. MAIN / RESERVE

Shipper configures MAIN count, RESERVE count, min scores, zones, contracts, KPI thresholds, share caps, reserve activation.  
EXACT_N vs UP_TO_N. Do not pad with ineligible carriers. RESERVE ≠ effective TO.

---

## 10. Shares & optimization (Decision 3)

Modes: CAP_ONLY (default), TARGET_WITH_CAP, CONTRACTUAL_MINIMUM (legal basis only).  
Constraints: Σ_c x[c,l,t] ≤ V[l,t]; Σ x[c] ≤ q[c]·V_total; ineligible ⇒ x=0; capacity caps; shortfall[l,t] explicit.  
Lexicographic objectives: HARD → min shortfall → commercial HARD → cost/quality/risk/share balance → deterministic tie-break.  
INFEASIBLE / FEASIBLE_WITH_SHORTFALL — no hidden partial Award.

---

## 11. Simulation

No side effects: no Award, legal obligation, TO, Shipment, real quota reserve, EDO. Immutable result + input hash + algorithm version.

---

## 12. Commission, commitments, guarantees

See RBAC matrix. Version-bound votes on subject_hash. Partial carrier accept ⇒ new response version, budget recalc, possible re-commission, reallocation of freed volume. Silence ≠ consent by default.

---

## 13. Appendix & EDO

Appendix from immutable snapshot (ids, hashes, template_version).  
EDO statuses per TZ §16; idempotent submit; trusted evidence only.  
**Live EDO / EFFECTIVE from UI:** forbidden until document security P0 + legal + Controller GO.

---

## 14. TMS activation & plan/fact

Activation after commission + commitment + contract conditions + signatures + effective date + re-authz.  
TO create: active policy, carrier scope, remaining quota, reserve ledger entry, allocation_decision_id, idempotent.  
Carrier refusal: event, release reservation, reallocation / reserve policy.  
Plan/fact buckets: planned/offered/reserved/accepted/executed/cancelled/failed/remaining/disputed — shipper cancel ≠ carrier breach by default.

---

## 15. Allocation Proposal / Decision FSM (Decision 7)

DRAFT → VALIDATED → SIMULATED → SUBMITTED_FOR_APPROVAL → APPROVED → CARRIER_CONFIRMATION → CONTRACT_PREPARATION → EDO_SIGNING → EFFECTIVE  
Reject paths: REJECTED, CHANGES_REQUESTED, EXPIRED, SUPERSEDED, CANCELLED.

---

## 16. API namespace (Decision 6)

Base: **`/api/v1/rfx/allocations`**.  
Examples (design):  
- `.../policies`, `.../policies/{id}/versions`  
- `.../events/{rfxEventId}/rounds`  
- `.../simulations`, `.../proposals`, `.../decisions`  
- `.../approval-workflows/{id}/votes`  
- `.../carrier/commitment-proposals`  
- `.../contract-appendices` (or document-service ownership with gateway route under same prefix policy)

Legacy `POST /api/v1/rfx-events/{id}/award-response` unchanged.

---

## 17. Waves W1–W9

Unchanged intent from TZ §25; each wave needs Controller GO after J review.  
W0 = this freeze (no runtime). W7 live EDO blocked on P0/legal. W1 = Policy Builder without Award side effects.

---

## 18. Non-claims

`PRODUCT_CODE_CHANGED=NO` · `MIGRATIONS=NO` · `MAIN_MERGE=NO` · `STAGING/PROD=NO` · `LIVE_EDO=NO` · `LIVE_AWARD=NO` · `AUTOPILOT=NO` · `PILOT_A1_GO=NO` · `W1_IMPLEMENTATION=NO` · `IMPLEMENTATION_ALLOWED=NO`
