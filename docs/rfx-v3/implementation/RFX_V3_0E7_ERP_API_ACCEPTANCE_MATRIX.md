# RFx v3.0E7 Phase 2 — ERP API Acceptance Test Matrix

**Status:** FROZEN_ACCEPTED (tests not implemented)
**Parent:** [RFX_V3_0E7_ERP_API.md](./RFX_V3_0E7_ERP_API.md)

| Marker | Value |
|---|---|
| `ERP_TEST_IDS` | `E7P2-INT-120..195` |
| `ERP_TEST_COUNT` | `76` |
| `ERP_TEST_IDS_UNIQUE` | `YES` |
| `INT_120_WAS_FREE` | `YES` (at initial freeze) |
| `NEXT_FREE_TEST_ID` | `E7P2-INT-196` |
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
| E7P2-INT-138 | CREATE stable external link inserted | Link matches stable identity key | external_id |
| E7P2-INT-139 | Duplicate CREATE same stable external ID | 409 `external_id_conflict` | external_id |
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
| E7P2-INT-156 | UPDATE actor/principal binding mismatch | 409 `actor_binding_denied` | security |
| E7P2-INT-157 | UPDATE idempotent replay | Stored 200 response | idempotency |
| E7P2-INT-158 | UPDATE concurrent ERP requests | One succeeds; one stale | concurrency |
| E7P2-INT-159 | UPDATE no partial writes on failure | Transaction rollback verified | rollback |

---

## 4. External ID and lookup (INT-160..164)

| ID | Scenario | Expected | Category |
|---|---|---|---|
| E7P2-INT-160 | GET by stable external_system + object_id | 200; correct single RFx | read |
| E7P2-INT-161 | GET external unknown stable ID | 404 | read |
| E7P2-INT-162 | GET internal ID returns ErpRfxDraftSummary only | 200; allowlisted fields | read |
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
| E7P2-INT-169 | Retired mapping set at Preview | 422 or mapping error | mapping |
| E7P2-INT-170 | Platform default mapping | Unmapped tenant uses default | mapping |
| E7P2-INT-171 | Mapping applied before canonical hash | Hash reflects resolved codes + mapping_context | mapping |

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
| E7P2-INT-188 | Competitor data absent in ERP GET DTO | No participant/bid fields | confidentiality |
| E7P2-INT-189 | Capabilities returns schema version | `BINTRANS_RFX_ERP_JSON_V1` + deferred_fields | capabilities |

---

## 8. Remediation tests (INT-190..195)

| ID | Scenario | Expected | Finding closed |
|---|---|---|---|
| E7P2-INT-190 | OAuth-only principal attempts API key | 401 `auth_scheme_denied` | M-02 |
| E7P2-INT-191 | ERP commit rejects analysis bound to different principal | 409 `actor_binding_denied` | H-02 |
| E7P2-INT-192 | GET by stable external ID without revision is deterministic | Single RFx; revision history optional | H-03 |
| E7P2-INT-193 | Commit after mapping set RETIRED post-Preview | 409 `stale_mapping_context` | H-04 |
| E7P2-INT-194 | Concurrent CREATE commits same stable external ID | One 201; others 409; one RFx | M-06 |
| E7P2-INT-195 | Deferred field `lanes[]` in payload | 422 `unsupported_field_v1` | H-05 |

---

## 9. Capabilities (uses INT-134 for unauthenticated denial)

Authenticated capabilities returns `BINTRANS_RFX_ERP_JSON_V1` and `deferred_fields[]` — covered at implementation time alongside INT-134 inverse.

---

## 10. Finding traceability table

| ID | Exact requirement | Endpoint/layer | Expected result | Finding closed |
|---|---|---|---|---|
| E7P2-INT-143 | Mass assignment blocked | CREATE preview | 422 `unknown_field` | H-01 |
| E7P2-INT-147 | Payload size limit | CREATE preview | 413 | H-01 |
| E7P2-INT-175 | JSON depth limit | CREATE preview | 400/422 | H-01 |
| E7P2-INT-177 | No auto-publish | CREATE commit | DRAFT status | H-01 |
| E7P2-INT-188 | Competitor data excluded | ERP GET DTO | No bid/participant fields | H-01, M-05 |
| E7P2-INT-195 | Deferred lanes rejected | CREATE preview | 422 `unsupported_field_v1` | H-05 |
| E7P2-INT-190 | Auth downgrade blocked | Token endpoint + API | 401 `auth_scheme_denied` | M-02 |
| E7P2-INT-191 | Principal binding on analysis | CREATE/UPDATE commit | 409 if mismatch | H-02 |
| E7P2-INT-192 | Stable external lookup | GET by-external-id | Single RFx | H-03 |
| E7P2-INT-193 | Mapping version pinned | UPDATE commit | 409 if mapping retired | H-04 |
| E7P2-INT-194 | Concurrent CREATE race | CREATE commit | One winner | M-06 |
| E7P2-INT-138 | Stable link on CREATE | CREATE commit | One link per stable key | H-03 |
| E7P2-INT-139 | Duplicate stable CREATE | CREATE commit | 409 | H-03 |
| E7P2-INT-134 | Capabilities requires auth | GET capabilities | 401 | M-03 |
| E7P2-INT-162 | ERP GET DTO allowlist | GET by internal ID | ErpRfxDraftSummary only | M-05 |
| E7P2-INT-171 | mapping_context in hash | CREATE preview | Pinned in analysis | H-04 |
| E7P2-INT-187 | Migration reversible | DB migration | up/down/up | M-08 |

---

## 11. Threat model cross-reference (H-01)

| Threat | Test IDs |
|---|---|
| Mass assignment | E7P2-INT-143 |
| Payload bomb | E7P2-INT-147, E7P2-INT-175 |
| Auto-publish | E7P2-INT-177 |
| Competitor leak | E7P2-INT-188 |
| Deferred freight fields | E7P2-INT-195 |
| Auth downgrade | E7P2-INT-190 |
| Principal binding | E7P2-INT-191, E7P2-INT-156 |
| Stable identity | E7P2-INT-138, 139, 192, 194 |
| Mapping drift | E7P2-INT-193 |

---

## 12. ID range summary

```
E7P2-INT-120..195  — ERP API (76 tests)
NEXT_FREE_TEST_ID  — E7P2-INT-196
```
