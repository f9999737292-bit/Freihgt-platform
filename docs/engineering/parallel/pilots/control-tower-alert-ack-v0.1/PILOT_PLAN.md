# Control Tower — Alert Acknowledgement v0.1 — Pilot Plan

**Phase:** 3B.1 orchestration
**Orchestrator branch:** `chore/control-tower-alert-ack-orchestration-v0.1`
**Status:** PLANNED — contract not frozen

### Git lineage (two SHAs)

| Concept | SHA / ref | Meaning |
|---------|-----------|---------|
| **PRODUCT_BASE_SHA** | `02208106e494afcaa46372e44b417761d6613daf` | `origin/main` at pilot design time — underlying product code base |
| **ORCHESTRATION_BASE_SHA** | HEAD of `chore/control-tower-alert-ack-orchestration-v0.1` | Final orchestration commit containing PILOT_PLAN, Task Contracts, prompts, registry |

**CT-AA-001 worktree/branch MUST start from ORCHESTRATION_BASE_SHA**, not `origin/main`, because orchestration artifacts are not yet on main.

Downstream implementation still targets product semantics from PRODUCT_BASE_SHA; orchestration docs travel on the orchestration branch until merged.

---

## 1. Architecture audit summary

### Alert source

Critical events are **derived at request time** in `services/api-gateway/internal/controltower/events.go` via `BuildCriticalEvents()`. They are computed from filtered shipment rows (SLA status, document completeness, cancellation) — **not persisted as alert records**.

The Control Tower summary BFF aggregates shipments, transport orders, companies, and documents, then derives `criticalEvents` in-process (`services/api-gateway/internal/controltower/service.go`).

### Alert identity

Each derived event receives a **deterministic ID**:

```text
event_id = hex( sha256( "{shipmentId}:{eventType}:{occurredAt.Unix()}" )[:16] )
```

Implementation: `deterministicEventID()` in `events.go`. The ID is a hash of `(shipmentId, eventType, occurredAt.Unix())`.

**CT-AA-001 guardrail:** Do **not** assume this ID is automatically a permanent acknowledgement identity. `occurredAt` sources differ per event type and may change when shipment state updates:

| Event type | `occurredAt` source | Identity stability concern |
|------------|---------------------|----------------------------|
| `SHIPMENT_CANCELLED` | `pickTime(LastUpdatedAt, now)` | ID shifts if `LastUpdatedAt` changes or `now` used as fallback |
| `PICKUP_DELAY` | `PlannedPickupAt` | Stable while planned pickup unchanged |
| `DELIVERY_DELAY` | `PlannedDeliveryAt` | Stable while planned delivery unchanged |
| `STALE_UPDATES` | `LastUpdatedAt` | ID shifts on each shipment update while stale |
| `MISSING_DOCUMENTS` | `pickTime(LastUpdatedAt, now)` | ID shifts with `LastUpdatedAt` / fallback |
| `TECHNICAL_PROBLEM` | `pickTime(LastUpdatedAt, now)` | ID shifts with `LastUpdatedAt` / fallback |

CT-AA-001 must classify each type for: (A) same logical occurrence, (B) resolved occurrence, (C) newly re-triggered occurrence — and define intentional acknowledgement key behavior across transitions. Prove safety or define a revised canonical occurrence key **before** contract freeze.

### Persistence today

| Asset | Persisted? | Owner |
|-------|------------|-------|
| Critical events | No (derived) | api-gateway BFF |
| Shipment status projection | Yes | control-tower-read-model-service |
| Alert acknowledgement | **No (gap)** | — |

### Control Tower ownership

| Layer | Component | Role |
|-------|-----------|------|
| Public API | `services/api-gateway/internal/controltower/` | BFF: auth, RBAC, aggregation, event derivation |
| Read model | `services/control-tower-read-model-service/` | Tenant-scoped projection + internal APIs |
| Frontend | `apps/web-admin/pages/control-tower/`, `components/control-tower/CriticalEventsPanel.vue` | Displays `criticalEvents` from summary API |
| OpenAPI | `packages/openapi/openapi.yaml` | `GET /api/v1/control-tower/summary`, `ControlTowerEvent` schema |

### Gateway / BFF

- Route: `GET /api/v1/control-tower/summary` registered in `services/api-gateway/internal/http/router.go`
- Auth: JWT → `AuthContext` (tenant_id, user_id from verified token)
- RBAC: `CanAccessControlTower()` — roles `PLATFORM_ADMIN`, `CARRIER_DISPATCHER`, `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, `FORWARDER_MANAGER`
- Gateway is **stateless** (no production PostgreSQL)

### Frontend source

- `useControlTower()` → `apiGet('/api/v1/control-tower/summary')` with `skipTenant: true` (tenant from JWT)
- `CriticalEventsPanel.vue` renders events; **no acknowledge action today**

### RBAC / tenant boundary

- View access: role list above (gateway + frontend mirror)
- Tenant: derived from JWT only; gateway strips client-supplied `X-Tenant-ID` / `X-User-ID` on ingress
- Read-model internal APIs trust `X-Tenant-ID` header set by gateway after JWT verification

---

## 2. Key architecture decision

**Where will acknowledgement state live?**

```text
PostgreSQL table control_tower.critical_event_acknowledgement
owned by control-tower-read-model-service
```

**Why:**

1. Acknowledgement is **operational state** tied to Control Tower, not shipment domain writes.
2. `control_tower` schema already exists with tenant-scoped tables (migrations 000015–000019).
3. api-gateway has no production DB — keeping persistence in the read-model service preserves clean ownership per `OWNERSHIP.md`.
4. Public mutation still flows through api-gateway BFF (auth/RBAC/trusted identity), which calls an internal read-model endpoint.

---

## 3. Stable alert identity model

**Acknowledgement key:** `(tenant_id, event_id)`

Where `event_id` is the deterministic hash already emitted in `ControlTowerEvent.id`.

**Stored metadata (audit + validation):**

| Column | Purpose |
|--------|---------|
| `tenant_id` | Tenant isolation PK component |
| `event_id` | Stable occurrence identity PK component |
| `shipment_id` | Audit: which shipment |
| `event_type` | Audit: PICKUP_DELAY, etc. |
| `source` | Always `control-tower` for v0.1 |
| `occurred_at` | Audit: occurrence timestamp used in ID |
| `acknowledged_at` | When ack was recorded |
| `acknowledged_by_user_id` | Trusted actor from gateway context |

**Idempotency:** `INSERT ... ON CONFLICT (tenant_id, event_id) DO NOTHING` (or equivalent). Repeated POST returns **200** with existing acknowledgement (first ack wins for `acknowledged_by`).

**Cross-tenant:** Lookup always includes `tenant_id` predicate. Foreign `event_id` → **404 NOT_FOUND** (no existence leak).

---

## 4. Migration

```text
REQUIRED
```

Next migration number: **000020** (`infrastructure/migrations/000020_create_control_tower_critical_event_acknowledgement_v0.1.{up,down}.sql`).

Evidence: no existing acknowledgement table in `control_tower` schema (verified migrations 000015–000019).

Migration is owned by **CT-AA-002** (backend), coordinated after contract freeze defines exact columns.

---

## 5. API contract plan (NOT FROZEN — CT-AA-001 owns freeze)

### Public (api-gateway)

| Item | Planned value |
|------|---------------|
| Method | `POST` |
| Path | `/api/v1/control-tower/critical-events/{eventId}/acknowledge` |
| Auth | Bearer JWT (required) |
| RBAC | **CT-AA-001 must decide:** view-only vs acknowledge permission (see §7) |
| Request body | Empty (no client-supplied tenant_id, user_id, acknowledged_by) |
| Success | `200` — `ControlTowerEventAcknowledgement` |
| Idempotent repeat | `200` — same acknowledgement |
| Invalid eventId format | `400` |
| No access | `403` |
| Foreign / unknown in tenant | `404` |

### Summary enrichment

Extend `ControlTowerEvent` (OpenAPI + gateway model) with optional:

```text
acknowledgement?: {
  acknowledgedAt: date-time
  acknowledgedBy: { userId: uuid, displayName?: string }
}
```

`GET /api/v1/control-tower/summary` merges acknowledgement state by batch lookup from read-model for derived event IDs.

### Internal (read-model-service)

| Item | Planned value |
|------|---------------|
| Method | `POST` |
| Path | `/internal/v1/control-tower/critical-events/{eventId}/acknowledge` |
| Headers | `X-Tenant-ID`, `X-User-ID` (trusted, gateway-only) |
| Body | Optional metadata: `shipmentId`, `eventType`, `occurredAt`, `source` for audit storage |

Internal batch lookup for summary merge:

```text
POST /internal/v1/control-tower/critical-events/acknowledgements/lookup
Body: { eventIds: string[] }
```

(Exact shape frozen in CT-AA-001.)

---

## 6. Dependency graph

```text
CT-AA-001 Contract/Architecture
        │
        ▼ CONTRACT_FREEZE_SHA (recorded in handoff)
        │
   ┌────┴────┐
   │         │
CT-AA-002  CT-AA-003   ← parallel after freeze
 Backend   Frontend
   │         │
   └────┬────┘
        ▼
   CT-AA-004 Security Review (readonly)
        ▼
   CT-AA-005 QA Verification
        ▼
   CT-AA-006 Integration → PR → main
```

---

## 7. Contract freeze strategy

1. **ORCHESTRATION_BASE_SHA** = pushed HEAD of `chore/control-tower-alert-ack-orchestration-v0.1` (contains pilot docs).
2. **CT-AA-001** worktree branches from **ORCHESTRATION_BASE_SHA** (NOT `origin/main`):

```powershell
git fetch origin
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack-contract -b arch/control-tower-alert-ack-contract-v0.1 <ORCHESTRATION_BASE_SHA>
```

3. **PRODUCT_BASE_SHA** (`02208106…`) remains the documented product-code reference; CT-AA-001 implements contract on top of orchestration lineage.
4. Architect commits OpenAPI + `ARCHITECTURE.md`; handoff records **`CONTRACT_FREEZE_SHA`** = contract task final commit.
5. **CT-AA-002** and **CT-AA-003** create worktrees from **`CONTRACT_FREEZE_SHA`**, not floating main:

```powershell
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack-backend -b feat/control-tower-alert-ack-backend-v0.1 <CONTRACT_FREEZE_SHA>
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack-frontend -b feat/control-tower-alert-ack-frontend-v0.1 <CONTRACT_FREEZE_SHA>
```

6. Integration (**CT-AA-006**) merges: orchestration branch (or rebased lineage) → contract → backend → frontend.

### Acknowledgement validation boundary (CT-AA-001 must freeze)

Events are derived in api-gateway; persistence is in read-model. POST must **not** blindly persist an opaque `eventId`.

Gateway must define validation — preferred pattern:

1. Rebuild/lookup current derived critical events for authenticated tenant (same derivation as summary).
2. Match `eventId` against that set (or apply frozen canonical key rules).
3. Only then call read-model with **gateway-trusted** metadata (`tenant_id`, `user_id`, `shipment_id`, `event_type`, `occurred_at`, `source`).

Read-model must **not** trust client-supplied identity fields on the public API. Cross-tenant / unknown → **404**.

### Mutation authorization (CT-AA-001 must decide)

`CanAccessControlTower()` today gates **view**. CT-AA-001 must explicitly decide whether the same role set may **acknowledge**, or whether acknowledgement requires a narrower permission. Reusing view roles is allowed for v0.1 only with documented rationale; CT-AA-004 Security must verify. No new RBAC seed unless justified.

### Idempotency (CT-AA-001 must freeze)

Define behavior for: first ack; repeat by same user; repeat by different authorized user; ack of resolved/non-current derived event. Prefer safe idempotent **200** responses. **Do not** silently rewrite original `acknowledged_by` / `acknowledged_at` unless contract explicitly chooses that.

---

## 8. Collision report

| Zone | Policy | Reason |
|------|--------|--------|
| OpenAPI | **SINGLE OWNER** (CT-AA-001) | Contract freeze before parallel impl |
| Gateway router | **SERIALIZE** | CT-AA-002 adds one route; CT-AA-001 may document path only |
| Migrations | **SINGLE OWNER** (CT-AA-002) | One numbered file 000020 |
| Shared Go models | **NONE** for v0.1 | No shared-go changes expected |
| Control Tower Go models | **SINGLE OWNER** (CT-AA-002) | Gateway + read-model in one backend task |
| Frontend API types | **SINGLE OWNER** (CT-AA-003) | Types follow frozen OpenAPI |
| Root workspace files | **NONE** | No Makefile/go.work changes |
| Integrator merge | **INTEGRATOR ONLY** (CT-AA-006) | Resolves any drift at integration |

---

## 9. Pilot success criteria (preparation)

| # | Criterion | This task |
|---|-----------|-----------|
| 1 | Usable Task Contracts | ✅ Created |
| 2 | Contract freeze | ⏳ CT-AA-001 |
| 3–12 | Implementation through merge | ⏳ Subsequent tasks |

---

## 10. Worktree plan (NOT created yet)

| Workstream | Path | Branch |
|------------|------|--------|
| Contract | `D:\Projects\freight-platform-wt\ct-alert-ack-contract` | `arch/control-tower-alert-ack-contract-v0.1` (from ORCHESTRATION_BASE_SHA) |
| Backend | `D:\Projects\freight-platform-wt\ct-alert-ack-backend` | `feat/control-tower-alert-ack-backend-v0.1` |
| Frontend | `D:\Projects\freight-platform-wt\ct-alert-ack-frontend` | `feat/control-tower-alert-ack-frontend-v0.1` |
| Security | `D:\Projects\freight-platform-wt\ct-alert-ack-security` | `review/control-tower-alert-ack-security-v0.1` |
| QA | `D:\Projects\freight-platform-wt\ct-alert-ack-qa` | `test/control-tower-alert-ack-qa-v0.1` |
| Integration | `D:\Projects\freight-platform-wt\ct-alert-ack-integration` | `int/control-tower-alert-ack-v0.1` |

`D:\Projects\freight-platform-wt\` does not exist yet — no collision with existing worktrees (24 worktrees verified).

---

## 11. Related artifacts

| Artifact | Location |
|----------|----------|
| Task Contracts | `contracts/CT-AA-*.md` |
| Agent Prompts | `prompts/CT-AA-*-prompt.md` |
| Task Registry | `docs/engineering/parallel/tasks/ct-aa-*.md` |
