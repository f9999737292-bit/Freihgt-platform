# W1 Test Matrix (R2) — J-W1-H003

**Rule:** Documentation ≠ executed evidence. PASS only after W1 GO with CI/test logs.  
**Package (planned):** `services/rfx-service/internal/allocationpolicy/`  
**Payload limit:** 65536 bytes before parse (`W1_AST_SCHEMA.md` §7).

---

## A. Core acceptance (W1-001 … W1-015)

| ID | Title | Type | Pass criteria |
|---|---|---|---|
| W1-001 | Valid AST parses | unit | v1 schema accepted |
| W1-002 | Invalid AST denied | unit | Malformed rejected |
| W1-003 | Unknown operator denied | unit | Fail-closed |
| W1-004 | Nested AND/OR | unit/golden | Expected eligibility |
| W1-005 | HARD not bypassed by Score | golden | HARD fail ⇒ ineligible |
| W1-006 | Missing KPI not fabricated | unit | No numeric fake KPI |
| W1-007 | Weighted score reproducible | golden | Same score_total |
| W1-008 | Invalid weight sum | unit | Sum ≠ 1.00 rejected |
| W1-009 | Depth/size bounds | unit | Over limit rejected |
| W1-010 | Unauthorized attribute | unit | Unknown attr denied |
| W1-011 | Cross-tenant input | unit | Deny before score |
| W1-012 | Determinism | golden | Identical results |
| W1-013 | Legacy Award/score unchanged | guard | No edits to legacy paths; existing tests green |
| W1-014 | No TO from eval | unit | Zero TO side effects |
| W1-015 | No external API from AST | unit | No HTTP in package |

---

## B. Extended security / robustness (W1-016 … W1-023)

### W1-016 — Oversized JSON payload

| Field | Value |
|---|---|
| TEST_ID | W1-016 |
| INPUT | Byte slice length `65537` (or `>65536`) of `{...}` padding **before** parse |
| EXPECTED_RESULT | Reject; no AST build |
| SECURITY_INVARIANT | DoS/resource bound; limit enforced pre-unmarshal |
| TEST_TYPE | unit |
| ACCEPTANCE_CRITERIA | Error class `payload_too_large`; 65536 accepted if otherwise valid |

### W1-017 — Negative / NaN / invalid decimal weights

| Field | Value |
|---|---|
| TEST_ID | W1-017 |
| INPUT | Weights `-0.1`, `"NaN"`, `"Infinity"`, `"1e309"` |
| EXPECTED_RESULT | Validate fail each |
| SECURITY_INVARIANT | No IEEE poison in score |
| TEST_TYPE | unit |
| ACCEPTANCE_CRITERIA | All rejected; no eval |

### W1-018 — Incompatible operand types

| Field | Value |
|---|---|
| TEST_ID | W1-018 |
| INPUT | `GT` between string and decimal; `BETWEEN` on boolean |
| EXPECTED_RESULT | Reject per operator matrix |
| SECURITY_INVARIANT | No silent coerce |
| TEST_TYPE | unit |
| ACCEPTANCE_CRITERIA | Error `incompatible_types` |

### W1-019 — Concurrent validation of identical policies

| Field | Value |
|---|---|
| TEST_ID | W1-019 |
| INPUT | N=32 goroutines validate same policy bytes |
| EXPECTED_RESULT | All succeed or all fail identically; no panic/race |
| SECURITY_INVARIANT | Thread-safe validator; deterministic outcome |
| TEST_TYPE | unit `-race` |
| ACCEPTANCE_CRITERIA | `go test -race` clean; identical fingerprints |

### W1-020 — Input policy immutability

| Field | Value |
|---|---|
| TEST_ID | W1-020 |
| INPUT | Caller retains original JSON/struct; engine evaluates |
| EXPECTED_RESULT | Caller copy unchanged after Evaluate/Validate |
| SECURITY_INVARIANT | No in-place mutation of caller policy |
| TEST_TYPE | unit |
| ACCEPTANCE_CRITERIA | Deep equal before/after on caller buffer |

### W1-021 — Mixed UNKNOWN / MISSING / NULL operators

| Field | Value |
|---|---|
| TEST_ID | W1-021 |
| INPUT | HARD FILTER with UNKNOWN attr; OR/AND mixes; IS_MISSING on NOT_AVAILABLE |
| EXPECTED_RESULT | Three-valued logic per AST §6; HARD UNKNOWN ⇒ not eligible |
| SECURITY_INVARIANT | Fail-closed HARD |
| TEST_TYPE | unit/golden |
| ACCEPTANCE_CRITERIA | Matches truth table fixtures |

### W1-022 — Canonical fingerprint stability

| Field | Value |
|---|---|
| TEST_ID | W1-022 |
| INPUT | Same policy with shuffled object keys / pretty vs compact whitespace; omitted `share_policy_mode` vs explicit `CAP_ONLY` |
| EXPECTED_RESULT | Same `sha256:` fingerprint |
| SECURITY_INVARIANT | Canonicalization; defaults applied |
| TEST_TYPE | unit |
| ACCEPTANCE_CRITERIA | Hash equality; material weight change ⇒ new hash |

### W1-023 — Unauthorized draft policy read

| Field | Value |
|---|---|
| TEST_ID | W1-023 |
| INPUT | Actor with only `tender.read` attempts draft policy read/evaluate |
| EXPECTED_RESULT | DENY |
| SECURITY_INVARIANT | Draft not covered by tender.read (M003) |
| TEST_TYPE | unit (AuthContext stub) |
| ACCEPTANCE_CRITERIA | Deny code; no policy body leak |

---

## C. Prior ABAC negatives (still required)

W1-S01…S07: foreign tenant/company, revoked membership, missing company, forged headers, unauthorized role, stale version.

---

## Evidence format (implementation PR)

`TEST_ID` · test name · file:line · CI URL · PASS/FAIL.  
This R2 contract does **not** claim execution.
