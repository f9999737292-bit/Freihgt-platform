# RFx v3.0E7 Phase 2 — ERP API Acceptance Test Matrix

**Status:** `FROZEN_PENDING_REVIEW` (tests not implemented)  
**Parent:** [RFX_V3_0E7_ERP_API.md](./RFX_V3_0E7_ERP_API.md)

| Marker | Value |
|---|---|
| `ERP_TEST_IDS` | `E7P2-INT-120..189` |
| `ERP_TEST_COUNT` | `70` |
| `ERP_TEST_IDS_UNIQUE` | `YES` |
| `INT_120_WAS_FREE` | `YES` |
| `NEXT_FREE_TEST_ID` | `E7P2-INT-190` |
| `PRIOR_RANGE_END` | `E7P2-INT-119` (Carrier XLSX C3) |

---

## 1. Authentication and credentials (INT-120..134)

| ID | Scenario | Expected | Category |
|---|---|---|---|
| E7P2-INT-120 | OAuth client_credentials valid token | 200; trusted headers injected | auth |
| E7P2-INT-121 | OAuth invalid client_secret | 401 | auth |
| E7P2-INT-122 | OAuth expired token | 401 `credential_expired` | auth |
| E7P2-INT-123 | Revoked API key | 401 `credential_revoked` | auth |
| E7P2-INT-124 | API key hash verify (no plaintext stored) | Auth succeeds; DB has hash only | auth |
| E7P2-INT-125 | Missing Authorization header | 401 | auth |
| E7P2-INT-126 | Credential rotation grace window | Old + new both valid ≤24h | auth |
| E7P2-INT-127 | IP allowlist violation | 403 | auth |
| E7P2-INT-128 | Spoofed X-Tenant-ID header | Stripped; credential tenant used | isolation |
| E7P2-INT-129 | Spoofed X-Company-ID header | Stripped; credential company used | isolation |
| E7P2-INT-130 | Cross-tenant event access | 404 fail-closed | isolation |
| E7P2-INT-131 | Cross-company buyer access | 404 fail-closed | isolation |
| E7P2-INT-132 | Missing scope `rfx:draft:commit` on commit | 403 | scopes |
| E7P2-INT-133 | Missing scope `rfx:draft:preview` on preview | 403 | scopes |
| E7P2-INT-134 | Capabilities endpoint without auth | 401 | scopes |

---

## 2. CREATE Preview/Commit (INT-135..147)

| ID | Scenario | Expected | Category |
|---|---|---|---|
| E7P2-INT-135 | CREATE preview valid JSON | 200; `ready_to_commit=true`; analysis persisted | create |
| E7P2-INT-136 | CREATE preview validation errors | 422; zero analysis rows (OPTION A) | create |
| E7P2-INT-137 | CREATE commit from analysis | 201; event created; `creation_channel=ERP` | create |
| E7P2-INT-138 | CREATE external link inserted | Link row matches external identity key | external_id |
| E7P2-INT-139 | Duplicate CREATE same external ID | 409 `external_id_conflict` | external_id |
| E7P2-INT-140 | CREATE retry same Idempotency-Key | Same event returned | idempotency |
| E7P2-INT-141 | CREATE commit without preview | 404 `analysis_not_found` | create |
| E7P2-INT-142 | CREATE preview unknown schema_version | 422 | validation |
| E7P2-INT-143 | CREATE preview mass assignment field | 422 `unknown_field` | validation |
| E7P2-INT-144 | CREATE commit missing Idempotency-Key | 400 | idempotency |
| E7P2-INT-145 | CREATE idempotency same key different body | 409 `idempotency_conflict` | idempotency |
| E7P2-INT-146 | CREATE feature flag disabled | 404; no writes | feature_flag |
| E7P2-INT-147 | CREATE oversized payload | 413 | limits |

---

## 3. UPDATE Preview/Commit (INT-148..159)

| ID | Scenario | Expected | Category |
|---|---|---|---|
| E7P2-INT-148 | UPDATE preview on DRAFT | 200; analysis persisted | update |
| E7P2-INT-149 | UPDATE preview on PUBLISHED event | 409 `stale_target` | state |
| E7P2-INT-150 | UPDATE commit applies graph | 200; lots/questionnaire reconciled | update |
| E7P2-INT-151 | UPDATE stale event_row_version | 409 `stale_target` | concurrency |
| E7P2-INT-152 | UPDATE stale draft_row_version | 409 `proposal_revalidation_failed` | concurrency |
| E7P2-INT-153 | UPDATE canonical hash tampered in DB | 409 `canonical_hash_mismatch` | integrity |
| E7P2-INT-154 | UPDATE consumed analysis replay | 409 `analysis_already_consumed` | analysis |
| E7P2-INT-155 | UPDATE expired analysis | 409 `analysis_expired` | analysis |
| E7P2-INT-156 | UPDATE actor binding mismatch | 409 `actor_binding_denied` | security |
| E7P2-INT-157 | UPDATE idempotent replay | Stored 200 response | idempotency |
| E7P2-INT-158 | UPDATE concurrent ERP requests | One succeeds; one stale | concurrency |
| E7P2-INT-159 | UPDATE no partial writes on failure | Transaction rollback verified | rollback |

---

## 4. External ID and lookup (INT-160..164)

| ID | Scenario | Expected | Category |
|---|---|---|---|
| E7P2-INT-160 | GET by external_system + object_id | 200; correct event | read |
| E7P2-INT-161 | GET external unknown ID | 404 | read |
| E7P2-INT-162 | GET internal ID | 200; DRAFT subset only | read |
| E7P2-INT-163 | GET analysis status PREVIEWED | 200; expires_at present | status |
| E7P2-INT-164 | GET analysis CONSUMED | 200; consumed_at set | status |

---

## 5. Reference mapping (INT-165..171)

| ID | Scenario | Expected | Category |
|---|---|---|---|
| E7P2-INT-165 | Unknown currency code | 422; `external_source` in issue | mapping |
| E7P2-INT-166 | Unknown unit code (fail-closed type) | 422 | mapping |
| E7P2-INT-167 | Unknown cargo type (warning allowed) | 200 preview with warning | mapping |
| E7P2-INT-168 | Tenant override mapping | External code resolves | mapping |
| E7P2-INT-169 | Retired mapping set | 422 for codes only in retired set | mapping |
| E7P2-INT-170 | Platform default mapping | Unmapped tenant uses default | mapping |
| E7P2-INT-171 | Mapping applied before canonical hash | Hash reflects canonical codes | mapping |

---

## 6. Domain validation and limits (INT-172..176)

| ID | Scenario | Expected | Category |
|---|---|---|---|
| E7P2-INT-172 | Duplicate section_code | 422 | validation |
| E7P2-INT-173 | Rule cycle detected | 422 | validation |
| E7P2-INT-174 | Max lots exceeded | 422 | limits |
| E7P2-INT-175 | JSON depth bomb | 400/422 | limits |
| E7P2-INT-176 | Invalid requested_operation | 422 | validation |

---

## 7. Safety, audit, coexistence (INT-177..189)

| ID | Scenario | Expected | Category |
|---|---|---|---|
| E7P2-INT-177 | Commit does not publish | Event remains DRAFT | no_publish |
| E7P2-INT-178 | No carrier response mutation | Carrier rows unchanged | carrier_safe |
| E7P2-INT-179 | No scoring/award side effects | Evaluation tables unchanged | carrier_safe |
| E7P2-INT-180 | Audit event on CREATE commit | `rfx.erp.draft.created.v1` | audit |
| E7P2-INT-181 | Audit event on UPDATE commit | `rfx.erp.draft.updated.v1` | audit |
| E7P2-INT-182 | No secrets in audit payload | Redaction verified | audit |
| E7P2-INT-183 | ERP + UI edit coexistence | Stale detection works | coexistence |
| E7P2-INT-184 | ERP + XLSX edit coexistence | Stale baseline on second commit | coexistence |
| E7P2-INT-185 | Route/gateway/OpenAPI parity | Shared route manifest | parity |
| E7P2-INT-186 | Rate limit 429 | Retry-After header | limits |
| E7P2-INT-187 | Migration 000074 up/down (when authorized) | Schema extension reversible | migration |
| E7P2-INT-188 | Competitor data absent in GET | No participant/bid fields | confidentiality |
| E7P2-INT-189 | Capabilities schema version | Returns `BINTRANS_RFX_ERP_JSON_V1` | capabilities |

---

## 2. Coverage checklist

| Requirement | Covered by |
|---|---|
| Authentication | INT-120..127 |
| Credential revocation/expiry | INT-122..123 |
| Tenant/company isolation | INT-128..131 |
| Scopes | INT-132..134 |
| Create Preview/Commit | INT-135..147 |
| Update Preview/Commit | INT-148..159 |
| External ID uniqueness | INT-138..139, 160..161 |
| Duplicate Create | INT-139 |
| Idempotent replay/conflict | INT-140, 145, 157 |
| Concurrent requests | INT-158 |
| Stale version | INT-151..152 |
| Expired/consumed analysis | INT-154..155 |
| Canonical hash tampering | INT-153 |
| Mapping failures | INT-165..171 |
| Invalid reference codes | INT-165..167 |
| Domain validation | INT-172..173, 176 |
| Limits | INT-147, 174..175, 186 |
| Audit | INT-180..182 |
| Rollback | INT-159 |
| No auto-publish | INT-177 |
| No Carrier mutation | INT-178..179 |
| Route/OpenAPI parity | INT-185 |
| ERP/UI/XLSX coexistence | INT-183..184 |
| Migration up/down | INT-187 |

---

## 3. Uniqueness verification

```
PRIOR_TEST_RANGES:
  E7P2-INT-01..20   — Buyer XLSX Export
  E7P2-INT-21..42   — Buyer XLSX Preview
  E7P2-INT-43..70   — Buyer XLSX Commit
  E7P2-INT-71..119  — Carrier XLSX C1/C2/C3

ERP_RESERVED_RANGE:
  E7P2-INT-120..189 — ERP API (this matrix)

NEXT_FREE_AFTER_ERP:
  E7P2-INT-190
```

No duplicate IDs detected in repository at architecture freeze time.
