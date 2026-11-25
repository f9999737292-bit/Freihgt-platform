# RFX-POLICY-BUILDER-UI-0.1 — Handoff for Agent F (R2)

**Owner:** Agent F  
**Backend:** Agent A (after W1 GO + J PASS)  
**App:** `apps/web-procurement`  
**Persistence:** NONE — labeled mock/test data only  

---

## 1. Mandatory banner

> **TEST PROTOTYPE — not a live Award. Data is synthetic. Server policy persistence not authorized.**

---

## 2. Authority contract (forbidden)

| Forbidden | Required instead |
|---|---|
| Client-selected tenant as authority | Tenant only from server session / gateway |
| Client-selected company without membership check | Company picker ⊆ server-provided memberships; server re-validates |
| Showing test KPI as real | Label `TEST_ONLY` / `NOT_AVAILABLE`; never imply production KPI |
| localStorage / browser DB as production SoR | Local cache OK if marked ephemeral draft; SoR = future server |
| Preview / simulate result as Award | Copy: “Preview only — not an Award or commitment” |

---

## 3. Shared formats (must match backend)

| Concept | Contract |
|---|---|
| Score scale | 0–100; display 2 decimal places; decimal arithmetic |
| Weights | Sum exactly 1.00; reject otherwise |
| `share_policy_mode` | `CAP_ONLY` \| `TARGET_WITH_CAP`; **default CAP_ONLY** if unset (same as backend) |
| Policy fingerprint | Display `sha256:<hex>` when backend/WASM helper available; do not invent alternate hash |
| HARD vs SCORE | Visual; HARD failure blocks score use |

Preliminary share % is **not** shown as a legal/commercial obligation.

---

## 4. Features (unchanged intent)

Policy list/create · visual AND/OR · HARD/SCORE/PREFERENCE · weights · price zones · KPI as unavailable · MAIN/RESERVE metadata · CAP mode · validation errors · explanation view · version compare.

---

## 5. Acceptance (F)

| ID | Criterion |
|---|---|
| F-01 | Banner present |
| F-02 | Nested AND/OR HARD+SCORE |
| F-03 | Weight sum validation |
| F-04 | KPI unavailable / test-labeled |
| F-05 | No Award label on preview |
| F-06 | Default CAP_ONLY |
| F-07 | No client tenant authority |
| F-08 | Company selection membership-bound |

---

## 6. Out of scope

Real allocation write APIs · commission UI · commitment UI · EDO · live gateway E2E claims.
