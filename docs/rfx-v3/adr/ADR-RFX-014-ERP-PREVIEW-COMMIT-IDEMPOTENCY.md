# ADR-RFX-014: ERP Preview/Commit and Idempotency

**Status:** FROZEN_PENDING_REVIEW
**Date:** 2026-09-16
**Deciders:** E7 Phase 2 ERP architecture stream
**Normative detail:** [RFX_V3_0E7_ERP_API.md](../implementation/RFX_V3_0E7_ERP_API.md) §4, §8

---

## Context

Buyer XLSX P3/P4 establishes Preview/Commit with `rfx_import_analyses`, 24h TTL, idempotency, and human `actor_id` binding.

ERP must reuse machinery with **separate M2M principal binding** — no silent mapping of integration principal to human user.

---

## Decision

### 1. Operation constants (M-01 — frozen)

| Constant | Route |
|---|---|
| `ERP_BUYER_CREATE_PREVIEW` | POST create preview |
| `ERP_BUYER_CREATE_COMMIT` | POST create commit |
| `ERP_BUYER_UPDATE_PREVIEW` | POST update preview |
| `ERP_BUYER_UPDATE_COMMIT` | POST update commit |

Idempotency scope: `{tenant_id, integration_principal_id, operation_constant, target_id?}`.

### 2. Actor / principal binding (H-02)

| Path | Analysis ownership column |
|---|---|
| Human XLSX | `actor_id` (required), `integration_principal_id` NULL |
| ERP M2M | `integration_principal_id` (required), `actor_id` NULL |

XOR CHECK proposed in migration 000074. Commit verifies:

- ERP: `analysis.integration_principal_id == authenticated principal`
- XLSX: `analysis.actor_id == actor.UserID` (unchanged)

Machine code: `actor_binding_denied`. Test: **E7P2-INT-191**.

**Forbidden:** synthesizing human `actor_id` from integration principal without explicit ADR.

### 3. Preview → Commit

| Rule | Value |
|---|---|
| Preview input | Full `BINTRANS_RFX_ERP_JSON_V1` |
| Commit input | `{ "analysis_id" }` only |
| Persistence | OPTION A: analysis only when `ready_to_commit=true` |
| Source type | `ERP_BUYER_JSON` |
| Schema | `BINTRANS_RFX_ERP_JSON_V1` |
| Mapping | Resolved values + `mapping_context` pinned in canonical payload |

### 4. Idempotency

- **Required** on both Commit operations.
- Max key length 128.
- Same key + same body hash → replay stored response.
- Same key + different body → 409 `idempotency_conflict`.
- External stable identity does **not** replace Idempotency-Key.

### 5. Concurrency

Body tokens: `event_row_version`, `draft_row_version`, `baseline_lots_fingerprint`. No HTTP ETag v1.

### 6. Concurrent CREATE (M-06)

- Preview does not reserve stable identity.
- Commit atomic: event + stable link + CONSUMED + idempotency.
- Unique stable key → one winner; loser 409; no orphan RFx.
- Test: **E7P2-INT-194**.

### 7. Failed transaction

Analysis stays PREVIEWED; idempotency record only on success.

---

## References

- [RFX_V3_0E7_BUYER_XLSX_IMPORT_P4_COMMIT.md](../implementation/RFX_V3_0E7_BUYER_XLSX_IMPORT_P4_COMMIT.md)
- `services/rfx-service/internal/service/excel_exchange_commit.go`
- `infrastructure/migrations/000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql`
