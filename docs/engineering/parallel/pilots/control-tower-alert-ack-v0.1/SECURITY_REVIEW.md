# Control Tower Alert Acknowledgement v0.1 — Security Review

**Task ID:** CT-AA-004
**Reviewer:** security-auditor agent
**Date:** 2026-08-13
**Worktree:** `D:\Projects\freight-platform-wt\ct-alert-ack-security`
**Branch:** `review/control-tower-alert-ack-security-v0.1`

## Result

**PASS** — No CRITICAL or HIGH findings. Two LOW operational/test findings documented below; neither blocks integration.

---

## SHAs reviewed

| Artifact | SHA | Notes |
|----------|-----|-------|
| Contract freeze | `4167b0fba849350cbd633330d39ad01d4567d4ce` | OpenAPI + `ARCHITECTURE.md` |
| Backend (CT-AA-002) | `40be356dd13e843068a3ae72275d3d5e848a2ee7` | `feat/control-tower-alert-ack-backend-v0.1` |
| Frontend (CT-AA-003) | `54aa73a5e5d8106bb90e55b8345700c043d2a1d8` | `feat/control-tower-alert-ack-frontend-v0.1` |
| Integration branch | — | `int/control-tower-alert-ack-v0.1` not present locally; review performed on backend + frontend branch heads above |

---

## Scope (REVIEW_TRIGGERS.md)

This feature touches:

- API gateway identity propagation to read-model (`X-Tenant-ID`, `X-User-ID`)
- Authorization / RBAC (`CanAccessControlTower`)
- Tenant-scoped persistence (`tenant_id` predicates)
- Object lookup by `eventId` (IDOR risk)
- Cross-tenant visibility (404 vs 403 on unknown events)

---

## Security invariant checklist

| Invariant | Result | Evidence |
|-----------|--------|----------|
| IDOR on `eventId` | **PASS** | Gateway derives tenant-wide critical events and matches `eventId` before calling read-model (`acknowledge.go`: `buildTenantCriticalEvents` → `FindCriticalEventByID`). Unknown IDs → `404 NOT_FOUND`. |
| Cross-tenant acknowledgement | **PASS** | Event derivation uses JWT tenant only; downstream fetches scoped via `reqCtx.TenantID`. Spoofed `X-Tenant-ID` on public request ignored — covered by `TestAcknowledgeHandlerUsesVerifiedTenantInReadModelCall`. |
| Tenant spoofing (headers / body) | **PASS** | Public POST body must be empty (`validateAcknowledgeRequestBody`); non-empty properties rejected. Tenant from `MustAuthContext`, not client headers. Frontend uses `skipTenant: true` on ack POST (JWT-only). |
| Identity spoofing (client-supplied actor) | **PASS** | No public fields for `user_id` / `acknowledged_by`. Gateway requires `reqCtx.UserID != ""`. Read-model sets `acknowledged_by_user_id` from trusted `X-User-ID` header set by gateway client. |
| RBAC bypass | **PASS** | `ensureAccess` → `CanAccessControlTower(roles)` on acknowledge handler (same as summary). Frozen Option A in `ARCHITECTURE.md` §9 accepted for v0.1. Frontend gates UI via `canAccessControlTower()` with matching role list. |
| Foreign-resource disclosure (403 vs 404) | **PASS** | Non-current / unknown / cross-tenant `eventId` → `404` (`NotFound("critical event not found")`). Lacks Control Tower role → `403`. |
| Actor attribution integrity | **PASS** | DB: `ON CONFLICT (tenant_id, event_id) DO NOTHING` then SELECT — first ack wins; no overwrite of `acknowledged_at` / `acknowledged_by_user_id`. |
| DB tenant isolation | **PASS** | All repository queries include `WHERE tenant_id = $1`. PK `(tenant_id, event_id)`. Migration check constraint on `event_id` format. |

---

## Findings

| ID | Severity | Title | Description | Recommendation |
|----|----------|-------|-------------|----------------|
| CT-AA-004-001 | **LOW** | Internal ack API trusts network boundary | Read-model `/internal/v1/control-tower/critical-events/{eventId}/acknowledge` accepts `X-Tenant-ID` / `X-User-ID` without service-to-service auth. A caller with network access could persist acknowledgements **without** gateway event-derivation validation. | **Accepted for v0.1** (matches existing internal-API pattern). Deployment must keep read-model off public ingress; local compose already binds `127.0.0.1:8089`. Document in staging/prod runbooks. Consider service token / mTLS in a future hardening task. |
| CT-AA-004-002 | **LOW** | Missing RBAC negative test for acknowledge | `acknowledge_test.go` covers 400/401/404 and tenant header spoofing but has no test asserting `403` when identity returns roles outside `CanAccessControlTower`. | Add test in CT-AA-005 or backend follow-up (non-blocking). |

**CRITICAL:** 0
**HIGH:** 0
**MEDIUM:** 0
**LOW:** 2

---

## Component notes

### Public API (`api-gateway`)

- Route: `POST /api/v1/control-tower/critical-events/{eventId}/acknowledge` behind global `Auth` middleware.
- Handler enforces RBAC when `AUTH_ENABLED=true`; service layer fails closed on missing `UserID`.
- Event validation uses **unfiltered** tenant shipment dataset (correct per architecture §8.2 — not limited by summary list filters).
- Empty JSON object `{}` allowed on public body (consistent with optional empty body; no identity fields accepted).

### Read-model (`control-tower-read-model-service`)

- Internal routes registered without additional auth middleware — by design; gateway is sole public entry.
- `UpsertAcknowledgement` and `LookupAcknowledgements` always predicate on `tenant_id`.
- Idempotent insert semantics match frozen contract §10.

### Frontend (`web-admin`)

- Ack button shown only when `canAccessControlTower()` and not in demo mode.
- POST sends no body; relies on bearer token via standard `apiPost` path.
- Error mapping covers 403 / 404 / 5xx without leaking cross-tenant details.

### RBAC decision (§9)

**Option A confirmed acceptable for v0.1:** acknowledgement reuses view access roles (`PLATFORM_ADMIN`, `CARRIER_DISPATCHER`, `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, `FORWARDER_MANAGER`). Backend and frontend role sets align.

---

## Validation

| Check | Result |
|-------|--------|
| Read-only code inspection (backend + frontend branches) | **PASS** |
| Contract / architecture alignment | **PASS** |
| `git diff --check` on this report | **PASS** (see below) |

---

## Integration recommendation

**PASS** — Safe to merge backend + frontend into `int/control-tower-alert-ack-v0.1` subject to normal integration validation (CT-AA-006). No security waiver required.

---

## Reviewer sign-off

| Field | Value |
|-------|-------|
| Task | CT-AA-004 |
| Recommendation | **PASS** |
| Blocking findings | None |
| Conditions | CT-AA-004-001 accepted as deployment invariant; CT-AA-004-002 optional test follow-up |
