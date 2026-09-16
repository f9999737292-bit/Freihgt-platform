# RFx v3.0E7 Phase 2 — ERP API Implementation Waves and Test Ownership

**Status:** `FROZEN_PENDING_REVIEW`
**Parent:** [RFX_V3_0E7_ERP_API_IMPLEMENTATION_PLAN.md](./RFX_V3_0E7_ERP_API_IMPLEMENTATION_PLAN.md)

| Marker | Value |
|---|---|
| `ERP_TEST_IDS` | `E7P2-INT-120..195` |
| `ERP_TEST_COUNT` | `76` |
| `ERP_TEST_IDS_COMPLETE` | `YES` |
| `ERP_TEST_IDS_UNIQUE_PRIMARY_ASSIGNMENT` | `YES` |
| `ERP_TEST_ID_GAPS` | `NONE` |
| `ERP_TEST_ID_DUPLICATES` | `NONE` |
| `NEXT_FREE_TEST_ID` | `E7P2-INT-196` |
| `NEW_TEST_IDS_RESERVED` | `NO` |

Cross-wave regression references are allowed; **primary owner** is exactly one wave per ID.

---

## 1. Wave summary

| Wave | Name | Primary test count | Branch |
|---|---|---|---|
| **E1** | Schema + integration principals | 1 | `feat/rfx-erp-schema-principals-e1-v3.0e7` |
| **E2** | Machine authentication | 14 | `feat/rfx-erp-machine-auth-e2-v3.0e7` |
| **E3** | ERP parser + Preview | 22 | `feat/rfx-erp-preview-e3-v3.0e7` |
| **E4** | CREATE Commit | 13 | `feat/rfx-erp-create-commit-e4-v3.0e7` |
| **E5** | UPDATE Commit | 15 | `feat/rfx-erp-update-commit-e5-v3.0e7` |
| **E6** | Read APIs + contract closure | 11 | `feat/rfx-erp-contract-closure-e6-v3.0e7` |
| **Total** | | **76** | |

Additional migration harness tests (pattern E7P2-INT-01..05 for 000074) are **E1** non-matrix obligations.

---

## 2. Primary test ownership matrix

| Test ID | Scenario (short) | Primary wave | Package/suite | Required dependency |
|---|---|---|---|---|
| E7P2-INT-120 | OAuth client_credentials valid | E2 | `identity-service` + gateway integration | E1 principals |
| E7P2-INT-121 | OAuth invalid client_secret | E2 | auth integration | E1 |
| E7P2-INT-122 | OAuth expired token | E2 | auth integration | E1 |
| E7P2-INT-123 | Revoked API key | E2 | auth integration | E1 credentials |
| E7P2-INT-124 | API key hash-only storage | E2 | credential repo + auth | E1 credentials |
| E7P2-INT-125 | Missing Authorization | E2 | gateway middleware | E2 auth path |
| E7P2-INT-126 | Credential rotation grace | E2 | credential lifecycle | E1 |
| E7P2-INT-127 | IP allowlist violation | E2 | gateway/principal | E1 allowed_cidrs |
| E7P2-INT-128 | Spoofed X-Tenant-ID stripped | E2 | gateway strip/inject | E2 |
| E7P2-INT-129 | Spoofed X-Company-ID stripped | E2 | gateway strip/inject | E2 |
| E7P2-INT-130 | Cross-tenant 404 | E2 | rfx tenant predicate | E2 + E6 read optional |
| E7P2-INT-131 | Cross-company 404 | E2 | principal company bind | E2 |
| E7P2-INT-132 | Missing scope on commit | E4 | scope middleware + CREATE commit | E2 scopes, E4 route |
| E7P2-INT-133 | Missing scope on preview | E3 | scope middleware + preview | E2, E3 |
| E7P2-INT-134 | Capabilities without auth 401 | E6 | capabilities route | E6 route exists |
| E7P2-INT-135 | CREATE preview valid | E3 | `internal/integration/erp/` preview | E1, E2, E3 |
| E7P2-INT-136 | CREATE preview validation 422 | E3 | preview validation | E3 |
| E7P2-INT-137 | CREATE commit 201 | E4 | create commit integration | E3 analysis |
| E7P2-INT-138 | Stable external link inserted | E4 | external link repo | E1, E4 |
| E7P2-INT-139 | Duplicate CREATE 409 | E4 | stable unique index | E1, E4 |
| E7P2-INT-140 | CREATE idempotent replay | E4 | idempotency store | E1 idempotency ext, E4 |
| E7P2-INT-141 | Commit without preview 404 | E4 | commit guard | E4 |
| E7P2-INT-142 | Unknown schema_version 422 | E3 | parser | E3 |
| E7P2-INT-143 | Mass assignment 422 | E3 | allowlist parser | E3 |
| E7P2-INT-144 | Missing Idempotency-Key 400 | E4 | commit handler | E4 |
| E7P2-INT-145 | Idempotency conflict 409 | E4 | idempotency | E4 |
| E7P2-INT-146 | Feature flag disabled 404 | E3 | `RFX_ERP_INTEGRATION_ENABLED` | E3 |
| E7P2-INT-147 | Oversized payload 413 | E3 | body limit | E3 |
| E7P2-INT-148 | UPDATE preview on DRAFT | E3 | update preview | E3 |
| E7P2-INT-149 | UPDATE preview on PUBLISHED 409 | E3 | state guard | E3 |
| E7P2-INT-150 | UPDATE commit applies graph | E5 | update commit + reconcile | E3, E5 |
| E7P2-INT-151 | Stale event_row_version 409 | E5 | stale baseline | E5 |
| E7P2-INT-152 | Stale draft_row_version 409 | E5 | stale baseline | E5 |
| E7P2-INT-153 | Canonical hash tampered 409 | E5 | hash verify | E5 |
| E7P2-INT-154 | Consumed analysis replay 409 | E5 | analysis lifecycle | E5 |
| E7P2-INT-155 | Expired analysis 409 | E5 | expiry check | E5 |
| E7P2-INT-156 | Principal binding mismatch 409 | E5 | XOR/binding verify | E1, E5 |
| E7P2-INT-157 | UPDATE idempotent replay | E5 | idempotency | E5 |
| E7P2-INT-158 | Concurrent UPDATE one wins | E5 | row locks | E5 |
| E7P2-INT-159 | Rollback no partial writes | E5 | transaction | E5 |
| E7P2-INT-160 | GET by stable external ID | E6 | read handler | E4 link, E6 |
| E7P2-INT-161 | GET unknown external 404 | E6 | read handler | E6 |
| E7P2-INT-162 | GET internal ErpRfxDraftSummary | E6 | DTO allowlist | E6 |
| E7P2-INT-163 | Analysis status PREVIEWED | E6 | analysis status GET | E3, E6 |
| E7P2-INT-164 | Analysis status CONSUMED | E6 | analysis status GET | E4/E5, E6 |
| E7P2-INT-165 | Unknown currency 422 | E3 | mapping fail-closed | E1 mapping tables |
| E7P2-INT-166 | Unknown unit 422 | E3 | mapping fail-closed | E1 |
| E7P2-INT-167 | Unknown cargo warning | E3 | mapping warning | E1 |
| E7P2-INT-168 | Tenant override mapping | E3 | mapping resolve | E1 |
| E7P2-INT-169 | Retired mapping at Preview | E3 | mapping status | E1 |
| E7P2-INT-170 | Platform default mapping | E3 | mapping resolve | E1 |
| E7P2-INT-171 | mapping_context in hash | E3 | canonical hash | E3 |
| E7P2-INT-172 | Duplicate section_code 422 | E3 | questionnaire validation | E3 |
| E7P2-INT-173 | Rule cycle 422 | E3 | questionnaire validation | E3 |
| E7P2-INT-174 | Max lots exceeded 422 | E3 | limits | E3 |
| E7P2-INT-175 | JSON depth bomb | E3 | depth limit | E3 |
| E7P2-INT-176 | Invalid requested_operation 422 | E3 | parser routing | E3 |
| E7P2-INT-177 | Commit does not publish | E4 | commit side-effect | E4 |
| E7P2-INT-178 | No carrier mutation | E5 | side-effect guard | E5 |
| E7P2-INT-179 | No scoring/award mutation | E5 | side-effect guard | E5 |
| E7P2-INT-180 | Audit CREATE commit | E4 | audit recorder | E4 |
| E7P2-INT-181 | Audit UPDATE commit | E5 | audit recorder | E5 |
| E7P2-INT-182 | No secrets in audit | E6 | audit redaction review | E4/E5, E6 |
| E7P2-INT-183 | ERP + UI coexistence stale | E5 | baseline tokens | E5 |
| E7P2-INT-184 | ERP + XLSX coexistence stale | E5 | cross-channel stale | E5 + XLSX |
| E7P2-INT-185 | Route/gateway/OpenAPI parity | E6 | shared manifest + parity tests | E3–E6 routes |
| E7P2-INT-186 | Rate limit 429 | E2 | per-principal rate limit | E2 |
| E7P2-INT-187 | Migration 000074 up/down/up | E1 | `migration_integration_test.go` | E1 authorized |
| E7P2-INT-188 | Competitor data absent GET | E6 | DTO allowlist | E6 |
| E7P2-INT-189 | Capabilities schema + deferred | E6 | capabilities handler | E6 |
| E7P2-INT-190 | Auth scheme downgrade denied | E2 | scheme binding | E2 |
| E7P2-INT-191 | Analysis principal binding | E4 | commit binding | E1, E4 |
| E7P2-INT-192 | GET without revision deterministic | E6 | stable lookup | E1 index, E6 |
| E7P2-INT-193 | stale_mapping_context on commit | E4 | mapping pin commit | E3, E4 |
| E7P2-INT-194 | Concurrent CREATE race | E4 | unique + transaction | E1, E4 |
| E7P2-INT-195 | Deferred lanes[] rejected | E3 | allowlist | E3 |

### Cross-wave regression notes

| ID | Secondary verification |
|---|---|
| E7P2-INT-191 | E5 UPDATE commit binding regression |
| E7P2-INT-193 | E5 UPDATE commit mapping pin regression |
| E7P2-INT-156 | E4 CREATE commit binding smoke optional |
| E7P2-INT-130, 131 | Re-verified on E6 read endpoints |
| E7P2-INT-177 | Re-verified on E5 UPDATE (must stay DRAFT) |

---

## 3. Wave E1 — detail

**Worktree:** `D:\Projects\freight-platform-wt\rfx-erp-schema-e1-v3.0e7-phase2`

**Logical commits (when authorized):**

1. `feat(rfx): add migration 000074 integration principal schema`
2. `feat(rfx): add integration principal and mapping repositories`
3. `test(rfx): add migration 000074 up/down integration tests`

**Unit tests:** repository CRUD, XOR constraint violations, credential hash storage interface.

**Integration tests:** INT-187; extend INT-01..05 pattern for 000074.

**Forbidden:** HTTP routes, OpenAPI ERP paths, OAuth handlers.

---

## 4. Wave E2 — detail

**Worktree:** `D:\Projects\freight-platform-wt\rfx-erp-machine-auth-e2-v3.0e7-phase2`

**Logical commits:**

1. `feat(identity): add integration OAuth token endpoint`
2. `feat(gateway): add ERP machine auth middleware and header injection`
3. `test(integration): add ERP auth INT-120..131, 190, 186`

**Forbidden:** RFx preview/commit handlers, OpenAPI ERP RFx ops (token endpoint only).

**Merge prerequisite:** E1 merged to `main`.

---

## 5. Wave E3 — detail

**Logical commits:**

1. `feat(rfx): add ERP JSON canonical parser and mapping pin`
2. `feat(rfx): add ERP CREATE and UPDATE preview routes`
3. `test(rfx): add ERP preview integration tests E3 matrix subset`

**Forbidden:** Commit handlers, external link creation, event creation.

---

## 6. Wave E4 — detail

**Logical commits:**

1. `feat(rfx): add ERP CREATE commit orchestration`
2. `feat(rfx): wire stable external identity on create`
3. `test(rfx): add ERP CREATE commit integration tests`

---

## 7. Wave E5 — detail

**Logical commits:**

1. `feat(rfx): add ERP UPDATE commit orchestration`
2. `test(rfx): add ERP UPDATE commit and coexistence tests`

---

## 8. Wave E6 — detail

**Logical commits:**

1. `feat(rfx): add ERP read APIs and capabilities`
2. `docs(openapi): add ERP integration operations`
3. `test(rfx): add ERP matrix guards INT-120..195 and route parity`

**Final gates:** `make openapi-check`, full ERP integration package, Buyer/Carrier XLSX regression CI jobs unchanged/passing.

---

## 9. ID range integrity

```
E7P2-INT-120..195  — 76 tests, all assigned
NEXT_FREE_TEST_ID  — E7P2-INT-196 (reserved; not allocated)
```

No new IDs proposed. If controller requires additional IDs during implementation, start at **INT-196** with explicit approval.
