# ADR-RFX-014: ERP Preview/Commit and Idempotency

**Status:** Accepted (architecture freeze — pending controller review)  
**Date:** 2026-09-16  
**Deciders:** E7 Phase 2 ERP architecture stream  
**Normative detail:** [RFX_V3_0E7_ERP_API.md](../implementation/RFX_V3_0E7_ERP_API.md) §4, §8

---

## Context

Buyer XLSX import (P3 Preview + P4 Commit) establishes a proven **two-phase apply** pattern:

| Phase | Behavior | Evidence |
|---|---|---|
| Preview | Parse → validate → canonical hash → optional analysis row | `PreviewBuyerImportWorkbook` |
| Commit | `analysis_id` only → lock → revalidate → atomic apply → CONSUMED | `CommitBuyerImportAnalysis` |
| TTL | 24 hours | `importAnalysisTTL` |
| Idempotency | Required on commit; 24h record TTL | `rfx_idempotency_records` |
| Immutability | DB trigger on payload | migration `000073` |

ERP JSON must reuse this machinery without mixing Create-from-XLSX or carrier flows.

---

## Decision

### 1. Preview → Commit (mandatory two-phase)

| Rule | Value |
|---|---|
| Preview input | Full `BINTRANS_RFX_ERP_JSON_V1` body |
| Commit input | `{ "analysis_id": "<uuid>" }` only |
| Analysis persistence | OPTION A: only when `ready_to_commit=true` |
| Target types | `NEW_EVENT` (CREATE), `DRAFT_EVENT` (UPDATE) |
| Workbook type | `BUYER_TENDER` (until 000074 adds ERP-specific type) |
| Schema version | `BINTRANS_RFX_ERP_JSON_V1` |
| Actor binding | `analysis.integration_principal_id` must match commit actor |
| Single-use | PREVIEWED → CONSUMED; replay blocked |

### 2. CREATE vs UPDATE separation

| Mode | Preview route | Commit route | Post-commit |
|---|---|---|---|
| `CREATE_DRAFT` | `POST .../erp/rfx/drafts/preview` | `POST .../erp/rfx/drafts/commit` | Create event, link external ID, `creation_channel=ERP` |
| `UPDATE_DRAFT` | `POST .../rfx-events/{id}/erp-import/preview` | `POST .../rfx-events/{id}/erp-import/commit` | Reconcile graph/lots only |

ERP Create **must not** use `POST /v1/rfx-events/from-template` or XLSX routes.

### 3. Idempotency policy

| Aspect | Policy |
|---|---|
| Header | `Idempotency-Key` required on all Commit operations |
| Max length | 128 characters (matches OpenAPI + RFx existing) |
| Scope key | `{tenant_id, integration_principal_id, operation, target_id}` |
| Operation constants | `ERP_BUYER_CREATE_COMMIT`, `ERP_BUYER_UPDATE_COMMIT` |
| Request fingerprint | SHA-256 of canonical commit body JSON |
| Same key + same fingerprint | Return stored HTTP response (200/201) |
| Same key + different fingerprint | 409 `idempotency_conflict` |
| Retention | 24 hours (align `rfx_idempotency_records.expires_at`) |
| External ID | Complements but does **not replace** Idempotency-Key |

Preview idempotency: **optional** (stateless re-validation acceptable); no requirement to deduplicate preview requests.

### 4. Concurrency policy

HTTP `ETag` / `If-Match`: **not used in v1** (no ETag in rfx-service today).

| Token | Location | Purpose |
|---|---|---|
| `event_row_version` | Canonical payload | Detect event metadata change |
| `draft_row_version` | Canonical payload | Detect draft graph change |
| `baseline_lots_fingerprint` | Canonical payload | Detect lot set change |
| `expected_version` | Questionnaire PATCH (UI) | Parallel human edit path |

Commit revalidation compares stored analysis baseline against live DRAFT state. Mismatch → 409 `stale_target` or `proposal_revalidation_failed`.

Concurrent scenarios:

| Scenario | Outcome |
|---|---|
| ERP commit vs UI save | First wins; second gets stale error |
| ERP commit vs XLSX commit | Same stale baseline protection |
| Duplicate concurrent commits (same analysis) | Row lock + single-use analysis |
| Retry after timeout (same Idempotency-Key) | Stored response replay |

### 5. Failed transaction behavior

- Commit runs in single DB transaction
- On failure: analysis remains PREVIEWED (not CONSUMED)
- Idempotency record written only on success (match XLSX pattern)

---

## Alternatives considered

### A. Single-phase POST (no preview)

**Rejected.** No safe validation window; no canonical hash audit trail; inconsistent with accepted XLSX architecture.

### B. External ID as idempotency key

**Rejected.** External ID identifies business object; Idempotency-Key identifies HTTP retry semantics. Colliding semantics caused ambiguous 409 handling.

### C. HTTP ETag concurrency

**Deferred.** Body version tokens already proven in XLSX commit; ETag adds gateway/service complexity without new capability.

---

## Consequences

### Positive

- Maximum reuse of `ImportAnalysisRepository`, reconcile functions, idempotency repo
- Identical operational playbook for support (expired analysis, consumed analysis)
- Clear test matrix parity with E7P2-INT-43..70 patterns

### Negative / cost

- ERP clients must implement two-step flow
- 24h window requires re-preview after expiry

---

## References

- [RFX_V3_0E7_BUYER_XLSX_IMPORT_P4_COMMIT.md](../implementation/RFX_V3_0E7_BUYER_XLSX_IMPORT_P4_COMMIT.md)
- `services/rfx-service/internal/service/excel_exchange_commit.go`
- `infrastructure/migrations/000068_rfx_version_lifecycle_v3_0e1.up.sql`
