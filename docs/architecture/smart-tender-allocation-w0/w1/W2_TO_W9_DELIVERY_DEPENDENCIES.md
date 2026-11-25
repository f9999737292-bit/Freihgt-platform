# W2–W9 Delivery Dependencies (R2)

**Rule:** `AUTOMATIC_WAVE_ADVANCEMENT=NO`  
**W0 on main:** `5432e144…` pack under `docs/architecture/smart-tender-allocation-w0/`  
**W1 contracts R2:** this directory — docs Draft PR only until Controller GO  
**Pin:** `000105` until migration GO  

---

## Graph

```
W1 AST/engine (+ fingerprint, ABAC, score rules)
  → W2 KPI/budget (Agent E; decimal money ≠ score)
  → W3 rounds (closes J-STA-0.9-001)
  → W4 commission
  → W5 allocation (Agent D; CAP_ONLY default)
  → W6 commitments
  → W7 EDO (Agent B; P0/legal gate)
  → W8 TMS (Agent C)
  → W9 E2E
```

---

## Wave briefs

**W2 (E):** Evidence KPI or fail-closed; price traffic light; decimal **money** budgets; explainable ranking. No fake production KPI Award.

**W3 (A):** Immutable bid history; versioned offer lines; rebid; carry-forward vs re-entry; planned/ad-hoc; late commercial submit; exception approval. ≠ questionnaire E7.

**W4:** Quorum/mandatory roles; subject_hash votes; re-approval on material change.

**W5 (D):** MAIN/RESERVE; global cap; multi-lane; eligible only; capacity; shortfall; tie-break; idempotent decisions.  
Mandatory: 1000 trips, 30% cap → max 300; win 180 → share 18%; NoForcedQuotaFill=YES.

**W6:** Accept/partial/reject/counter; guarantees schedules; guarantee budget.

**W7 (B):** Immutable appendix; verified EDO evidence; no EFFECTIVE without evidence; legal GO.

**W8 (C):** Allocation→TO; ledger idempotency; reserve; reallocation; plan/fact; legacy Award→TO preserved.

**W9:** Real gateway/DB; three-app browser E2E; cross-tenant; performance; audit; controlled pilot.

---

## Agent cards

| Agent | Now |
|---|---|
| F | `W1_AGENT_F_UI_HANDOFF.md` after UI GO |
| D | W5 prep from this brief |
| E | W2 KPI/budget prep |
| B | W7 / document P0 tracking |
| C | W8 TMS bridge prep |
| J | Review W1 R2 docs PR exact HEAD; later code PRs |

**Carry-forward:** J-STA-0.9-001→W3 · 002→docs hygiene · 003→W1 ABAC (this pack).
