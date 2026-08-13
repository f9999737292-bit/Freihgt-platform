# Task Contract

## Task ID

CT-AA-001

## Title

Control Tower alert acknowledgement — API contract and architecture freeze

## Owner

orchestrator

## Role

architect

## Repository

D:\Projects\freight-platform-wt\ct-alert-ack-contract

## Base branch

chore/control-tower-alert-ack-orchestration-v0.1

## Base SHA

**ORCHESTRATION_BASE_SHA** — HEAD of `chore/control-tower-alert-ack-orchestration-v0.1` at worktree creation (contains pilot docs). **Do not branch from `origin/main`.**

## Product reference SHA

**PRODUCT_BASE_SHA:** `02208106e494afcaa46372e44b417761d6613daf` (`origin/main` at pilot design — underlying product code only)

## Working branch

arch/control-tower-alert-ack-contract-v0.1

## Worktree

D:\Projects\freight-platform-wt\ct-alert-ack-contract

---

## Objective

Freeze the v0.1 API contract and architecture for Control Tower critical event acknowledgement: public POST endpoint, summary enrichment schema, internal read-model endpoints, **proven stable event identity**, existence validation boundary, mutation authorization decision, idempotency semantics, persistence ownership, and migration outline. Deliver **CONTRACT_FREEZE_SHA** for downstream parallel implementation.

## In scope

- OpenAPI: `POST /api/v1/control-tower/critical-events/{eventId}/acknowledge`
- OpenAPI: extend `ControlTowerEvent` with optional acknowledgement block
- OpenAPI: document error semantics (400/401/403/404)
- Architecture doc: `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/ARCHITECTURE.md`
- **Per-event-type identity analysis** (see Acceptance criteria §6)
- **Acknowledgement existence validation boundary** (gateway vs read-model)
- **Mutation authorization decision** (view vs acknowledge)
- **Idempotency semantics** (first/repeat/resolved event cases)
- Define internal read-model endpoint shapes (document in ARCHITECTURE.md)
- Define migration 000020 table schema (columns, PK, indexes) — spec in ARCHITECTURE.md; SQL deferred to CT-AA-002

## Out of scope

- Backend handler implementation
- Frontend UI implementation
- Migration file creation (deferred to CT-AA-002)
- Implementing a revised event ID algorithm (analysis and contract only)
- Security review execution
- QA test execution

## Allowed paths

- `packages/openapi/openapi.yaml`
- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/ARCHITECTURE.md`
- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/contracts/CT-AA-001-FROZEN.md` (optional freeze marker)

## Forbidden paths

- `services/**`
- `apps/**`
- `infrastructure/migrations/**`
- `infrastructure/docker-compose/**`
- `.github/**`
- `Makefile`, `go.work`, root `package.json`

## Dependencies

- Orchestration branch pushed; worktree created from ORCHESTRATION_BASE_SHA

## Security invariants

- No request body fields for `tenant_id`, `user_id`, or `acknowledged_by` on public API
- Tenant and actor from JWT/gateway context only
- Cross-tenant unknown event → **404 NOT_FOUND** (no existence leak)
- Read-model must not treat client-supplied identity as authoritative
- Internal endpoints trust gateway-set `X-Tenant-ID` / `X-User-ID` only

## Acceptance criteria

1. OpenAPI defines acknowledge endpoint with full request/response/error schemas.
2. `ControlTowerEvent` includes optional acknowledgement representation for summary merge.
3. ARCHITECTURE.md documents: persistence table, service boundaries, internal API paths.
4. Handoff records **CONTRACT_FREEZE_SHA** = final commit on working branch.
5. `make openapi-validate` PASS (or NOT_RUN with reason if tooling unavailable).

### 6. Derived-event identity (mandatory)

Inspect `services/api-gateway/internal/controltower/events.go` and `deterministicEventID()`.

For **each** current critical event type, classify identity semantics:

| Event type | Must analyze |
|------------|--------------|
| `SHIPMENT_CANCELLED` | `occurredAt` = `pickTime(LastUpdatedAt, now)` |
| `PICKUP_DELAY` | `occurredAt` = `PlannedPickupAt` |
| `DELIVERY_DELAY` | `occurredAt` = `PlannedDeliveryAt` |
| `STALE_UPDATES` | `occurredAt` = `LastUpdatedAt` |
| `MISSING_DOCUMENTS` | `occurredAt` = `pickTime(LastUpdatedAt, now)` |
| `TECHNICAL_PROBLEM` | `occurredAt` = `pickTime(LastUpdatedAt, now)` |

For each type, document behavior across:

- **(A)** same logical occurrence — ID stable?
- **(B)** resolved occurrence — alert disappears; ack retained or rejected?
- **(C)** newly re-triggered occurrence — new ID? new ack required?

**Do not** assume existing `deterministicEventID` is automatically safe. If safe for a type, **prove why**. If not safe, define a stable canonical occurrence key or revised identity model in ARCHITECTURE.md **before** contract freeze. Do not implement the algorithm in this task.

### 7. Acknowledgement existence validation (mandatory)

POST must **not** accept an arbitrary opaque `eventId` and blindly persist.

ARCHITECTURE.md must define how the system proves:

- the event exists (or existed as a valid occurrence per frozen identity rules);
- it belongs to the authenticated tenant;
- it represents a valid Control Tower event occurrence.

Because events are **derived in api-gateway** and persistence is in **read-model**, define the validation boundary. Preferred pattern (unless superior mechanism documented):

1. Gateway rebuilds/lookups current derived critical events for authenticated tenant (same logic as summary).
2. Gateway matches `eventId` (or canonical key) against that set per frozen rules.
3. Gateway sends **trusted** event metadata to read-model internal endpoint.

Document alternative if chosen. Foreign/unknown → **404**.

### 8. Mutation authorization decision (mandatory)

Inspect `services/api-gateway/internal/controltower/rbac.go` — `CanAccessControlTower()` gates view today.

Explicitly decide and document:

- **Option A:** all Control Tower view roles may acknowledge; or
- **Option B:** acknowledgement requires a narrower permission (document which).

Reusing view roles for v0.1 is allowed **only** with recorded rationale. CT-AA-004 Security must explicitly verify this decision. **No new RBAC seed/role** unless justified in ARCHITECTURE.md.

### 9. Idempotency semantics (mandatory)

Freeze behavior for:

| Scenario | Required definition |
|----------|---------------------|
| First acknowledgement | 200 + persisted actor/time |
| Repeat by same authorized user | Idempotent 200; original actor/time preserved |
| Repeat by different authorized user | Idempotent 200; **do not** rewrite original actor/time unless explicitly chosen |
| Acknowledgement of resolved / non-current derived event | Define: 404, 409, or allow historical ack — document rationale |

Response must not silently rewrite original `acknowledged_by` / `acknowledged_at` unless contract explicitly chooses that.

## Required validation

Level: 1

Commands:

- `git diff --check`
- `make openapi-validate` (if available)

## Required deliverables

- OpenAPI diff
- ARCHITECTURE.md (identity matrix, validation boundary, RBAC decision, idempotency table)
- Handoff with CONTRACT_FREEZE_SHA

## Integration target

int/control-tower-alert-ack-v0.1 → main

## Handoff requirements

- ORCHESTRATION_BASE_SHA and CONTRACT_FREEZE_SHA recorded
- PRODUCT_BASE_SHA referenced
- Changed files list
- Contracts changed: packages/openapi/openapi.yaml
- Validation results
