# W1 AST Schema Contract (R2)

**schema_version:** `allocation_policy_ast.v1`  
**Remediates:** J-W1-H001 (score), J-W1-M001 (fingerprint), J-W1-M002 (share mode), J-W1-M004 (operator types)  
**Money note:** Score arithmetic uses decimal but is **not** money; currency rounding from W0 Money Policy does **not** apply to scores. Money paths remain separate (`shopspring/decimal`, NUMERIC money scales).

---

## 1. Top-level document

```json
{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "opaque-string",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "share_policy_mode": "CAP_ONLY",
  "tie_break": { "strategy": "CARRIER_ID_ASC" },
  "root": { "op": "AND", "args": [] }
}
```

| Field | Rules |
|---|---|
| `schema_version` | Required; unknown ⇒ reject |
| `scoring_model` | W1: only `ADDITIVE_WEIGHTS` |
| `share_policy_mode` | `CAP_ONLY` \| `TARGET_WITH_CAP`; **DEFAULT=`CAP_ONLY`** if omitted (backend + frontend identical) |
| `tie_break.strategy` | W1: `CARRIER_ID_ASC` (lexicographic string/UUID). Optimizer objectives = W5 |
| `root` | Single node |

**Share mode semantics (D3):** Preliminary share is **not** a commercial/legal obligation. `CAP_ONLY` = hard max only. `TARGET_WITH_CAP` may bias toward target without violating HARD price/KPI/contract/capacity/budget. Missing field ⇒ treat as `CAP_ONLY` before fingerprint defaults normalization (see §8).

---

## 2. Node kinds

FILTER · SCORE · PREFERENCE · CAP · FLOOR · FALLBACK · APPROVAL · ACTION  
Boolean: AND / OR / NOT (NOT arity 1).  
ACTION may only set flags (shortlist/reserve/exception) — **never** Award/TO/EDO.

---

## 3. Score normalization (J-W1-H001) — normative

### 3.1 Scale and type

| Rule | Value |
|---|---|
| Score scale | **[0, 100]** inclusive |
| Type | **decimal** string literals in AST; runtime `shopspring/decimal` |
| Out of range part | Validation/eval error — do not clamp silently |

### 3.2 Weights

| Rule | Value |
|---|---|
| Storage | decimal strings in AST, e.g. `"0.30"` |
| Sum (ADDITIVE_WEIGHTS) | Exactly **1.00** after normalize; tolerance **0** (exact decimal) |
| Invalid sum | Reject at validate (W1-008) |
| Negative / NaN / Inf weights | Reject (W1-017) |

### 3.3 Computation order (fixed)

1. Resolve AuthContext + tenant isolation (abort if deny).  
2. Evaluate **HARD** FILTER / FLOOR eligibility (and HARD CAP eligibility checks).  
3. If HARD fail ⇒ `eligible=false`; **stop scoring** (no PREFERENCE, no SCORE total used for ranking).  
4. Resolve FALLBACK for missing attrs (never fabricate KPI).  
5. Compute each **SCORE** node contribution: `part_i = weight_i * componentScore_i` with `componentScore_i ∈ [0,100]`.  
6. `score_raw = Σ part_i` (decimal).  
7. Round **final** score only: see §3.4. Intermediate sums keep full decimal precision (no early float).  
8. Apply **PREFERENCE** (§4) — does **not** change `score_total` numeric value in W1.  
9. Emit explanation.

**HARD cannot be bypassed by high Score** (W1-005).

### 3.4 Rounding (scores only)

| Stage | Precision | Mode |
|---|---|---|
| Intermediate | exact decimal rational via shopspring/decimal | no round |
| Final `score_total` | **2 decimal places** | `Round(2)` half-away-from-zero |
| Emitted `score_parts[].value` | 2 dp same mode | after final assembly |

Do **not** use W0 money `NUMERIC(18,2)` money rounding rules interchangeably; document separately in Money Policy for currency.

### 3.5 Worked examples

**Example A — weights OK**  
Weights: price `0.40`, lane_kpi `0.35`, contract `0.25` (sum 1.00).  
Components: 80, 60, 100.  
`score_raw = 0.40*80 + 0.35*60 + 0.25*100 = 32 + 21 + 25 = 78`.  
`score_total = 78.00`.

**Example B — HARD blocks score**  
FILTER `contract_status EQ ACTIVE` fails; SCORE would be 99.  
Result: `eligible=false`, `score_total=null`, hard_failures non-empty.

**Example C — missing KPI**  
`lane_kpi` NOT_AVAILABLE; FILTER requiring lane_kpi without FALLBACK ⇒ fail or IS_MISSING path; never substitute `0` as measured KPI (W1-006).

**Example D — invalid weights**  
`0.50 + 0.50 + 0.10 = 1.10` ⇒ validate reject.

---

## 4. PREFERENCE rules (normative)

| Question | W1 rule |
|---|---|
| Changes numeric `score_total`? | **NO** |
| Used for? | Deterministic ordering among equal `score_total` **after** tie_break key, or as ordered preference flags in explanation |
| When applied? | Only if `eligible=true` |
| Aggregation | Evaluate PREFERENCE predicates; collect ordered list of matched preference ids (stable AST walk order) |
| Tie-break interaction | Sort key: `(score_total DESC, preference_rank ASC, carrier_id ASC)` where `preference_rank` = index of first matching PREFERENCE or `+∞` if none |
| Explanation | List matched preference node_ids; state that numeric score unchanged |

---

## 5. CAP application order

1. CAP nodes that are **HARD eligibility** (e.g. max share exceeded vs fixture attrs) run in step 2 with FILTER.  
2. CAP metadata (`share_policy_mode`, preliminary share %) is validated and echoed in explanation; **quota optimization** is W5.  
3. SCORE/PREFERENCE never increase a CAP.

---

## 6. Operator / type compatibility (J-W1-M004)

### 6.1 Three-valued logic

Eval of comparisons returns `TRUE` | `FALSE` | `UNKNOWN`.

| Situation | Result |
|---|---|
| Any operand MISSING / NULL where op needs value | `UNKNOWN` (unless op is EXISTS/IS_MISSING) |
| Attribute status UNKNOWN/NOT_AVAILABLE | `UNKNOWN` |
| Incompatible types | **Reject policy eval** (fail-closed) — not silent coerce |

**AND:** FALSE wins; else UNKNOWN if any UNKNOWN; else TRUE.  
**OR:** TRUE wins; else UNKNOWN if any UNKNOWN; else FALSE.  
**NOT:** NOT TRUE=FALSE; NOT FALSE=TRUE; NOT UNKNOWN=UNKNOWN.  

HARD FILTER: outcome `FALSE` or `UNKNOWN` ⇒ **not eligible** (fail-closed for HARD).  
SCORE component encountering UNKNOWN ⇒ that component contribution rejected unless FALLBACK rewrites attr first.

**No implicit conversions** that change commercial meaning (e.g. string `"10"` vs decimal `10`).

### 6.2 Compatibility table

| Operator | decimal | string | boolean | enum | timestamp | missing/null | unknown-status |
|---|---|---|---|---|---|---|---|
| EQ / NE | ✓ | ✓ | ✓ | ✓ | ✓ | UNKNOWN | UNKNOWN |
| GT/GTE/LT/LTE | ✓ | ✗ | ✗ | ✗ | ✓ | UNKNOWN | UNKNOWN |
| IN / NOT_IN | ✓ (set) | ✓ | ✓ | ✓ | ✓ | UNKNOWN | UNKNOWN |
| BETWEEN | ✓ | ✗ | ✗ | ✗ | ✓ | UNKNOWN | UNKNOWN |
| EXISTS | attr present (incl. null?) — **present & not MISSING** | same | same | same | same | FALSE if MISSING | FALSE if NOT_AVAILABLE mapped as missing |
| IS_MISSING | TRUE if MISSING/NOT_AVAILABLE | same | same | same | same | TRUE | TRUE for NOT_AVAILABLE |

✗ = incompatible operand types ⇒ **validation/eval error**.

---

## 7. Bounds

| Limit | Value |
|---|---|
| Max AST depth | 16 |
| Max nodes | 256 |
| Max IN-set size | 64 |
| Max string length | 256 |
| Eval step budget | 10_000 |
| **Max JSON payload** | **65536 bytes (64 KiB)** raw request/policy document **before** AST parse |

Payload gate: reject if `len(bytes) > 65536` **before** JSON unmarshal (W1-016).

Forbidden: SQL, shell, scripts, HTTP, undocumented ops, recursion beyond tree.

---

## 8. Policy fingerprint (J-W1-M001)

### Algorithm `allocation_policy_fingerprint.v1`

1. Parse + validate policy.  
2. Apply defaults: missing `share_policy_mode` → `CAP_ONLY`; missing `tie_break` → `{strategy:CARRIER_ID_ASC}`.  
3. Drop non-semantic fields: `policy_id` **excluded** from hash (identity ≠ content). Include `schema_version`, `scoring_model`, `share_policy_mode`, `tie_break`, `root`.  
4. Canonicalize:
   - UTF-8 JSON  
   - Object keys sorted lexicographically at every level  
   - Arrays preserve AST order (semantic)  
   - Decimal literals normalized to canonical decimal string (no exponent; no trailing zeros except `"0"`; no `+` sign)  
   - No pretty-print whitespace: compact separators  
5. Hash: **SHA-256** over canonical bytes.  
6. Fingerprint string: `sha256:<hex>`.

| Invariant | Rule |
|---|---|
| Key order independence | Same semantic object ⇒ same hash |
| Pretty JSON | Must not be sole fingerprint source |
| Material change | Any change to scored/filter tree or weights/mode ⇒ new hash |
| Identical semantics | Same canonical form ⇒ same hash |

Implementation MUST unit-test W1-022.

---

## 9. Attribute catalog (unchanged honesty)

overall_kpi / lane_kpi* remain NOT_AVAILABLE without evidence sources. Fixtures may mark `TEST_ONLY`. Never fabricate production KPI.

---

## 10. Legacy scoring

`ComputeCommercialScores` / 70/30 in `rfx_evaluation.go` untouched. W1 AST scoring is separate until a later GO.
