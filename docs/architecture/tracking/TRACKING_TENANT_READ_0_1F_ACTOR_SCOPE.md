# TRACKING-TENANT-READ-AUTHORITY-0.1F — Actor Scope

`ListActiveTargetsForActors` is the read that crosses tenants today. This document chooses a scope shape. It does not change the function.

## 1. Path

1. A provider location is accepted by `IngestProviderLocations`. The device lookup is `FindActiveBindingByDeviceAnyTenant` (`services/tracking-service/internal/service/ingest_service.go:101`).
2. The location event copies `tenant_id`, `shipment_id`, `vehicle_id`, and `driver_id` from that one binding (`ingest_service.go:133-136`). The provider payload is not the source of those four fields.
3. When approach is enabled, `AcceptLocation` is called (`ingest_service.go:157`).
4. `emitApproach` calls `ListActiveTargetsForActors` with the event's driver and vehicle ids (`services/tracking-service/internal/service/approach.go:62-69`).
5. The SQL has no tenant predicate (`services/tracking-service/internal/repository/execution_tracking_repository.go:206-215`).
6. `actorMatchesTarget` compares driver and vehicle ids (`eta_ingest_service.go:311-327`). It does not compare `event.TenantID` with `target.OperatingTenantID`.
7. Approach continues only when exactly one target remains (`approach.go:83-85`). That count is a uniqueness filter.
8. ETA ingest uses the other binding read, `ListActiveBindingsByDevice` (`eta_ingest_service.go:177`), collects every driver and vehicle id (`:182-191`), and calls the same unscoped target query (`:192`). It continues only when the result contains one execution id (`:201-203`).

`FindActiveBindingByDeviceAnyTenant` orders by `active_from DESC` and keeps one row (`tracking_repository.go:51-53`). The active unique index is per tenant (`000026:28-30`), so two active rows for one device are legal. The `LIMIT 1` chooses a tenant by recency.

## 2. Decision

Three shapes were compared.

| Shape | Result |
| --- | --- |
| 1. One tenant-scoped read for each authorized binding | Selected as the only shape that keeps each tenant separate. |
| 2. One server-owned set of tenant scopes, each checked on its own | Allowed only as the list of shape-1 results. A single SQL statement without a per-scope predicate is not this shape. |
| 3. Move target resolution into shipment-service | Not selected. `authorizeCommand` (`transport_execution_command_repository.go:159-189`) authorizes a shipment command against a locked execution. It does not return tracking targets, and tracking-service does not call it. |

The selected read contract, specified and not created:

- Input is the provider code and the provider device id. There is no tenant argument.
- The function reads every binding row for that device with `status = 'active'` and (`active_to IS NULL` OR `active_to > statement_timestamp()`).
- Each surviving binding is one scope: its `tenant_id`, `driver_id`, and `vehicle_id`.
- A tracking-state row is returned for that scope only when all of the following hold: `operating_tenant_id` equals that binding's `tenant_id`; the state driver or vehicle equals that binding's driver or vehicle; `live_eta_stop_id` is not null.
- The execution row in `transport.transport_executions` must agree on `operating_tenant_id` and on the same driver or vehicle. The runtime role does not receive table `SELECT` on that table. The comparison runs inside the protected function, whose owner is the only role with the column `SELECT`. This is the PR #376 read-owner boundary, extended to those execution columns. PR #376 is not edited by this stage.
- Rows from different scopes are not merged into one tenant. Zero scopes returns an empty set.
- The current `ORDER BY active_from DESC LIMIT 1` lookup is not used as the authorization choice.

`BINDING_CREATION_TRUST=OPEN`. The join stops an actor id from returning a tenant that has no matching active binding. It does not make the binding tenant authoritative, because `EnsureActiveDriverMobileBinding` stores the caller-supplied tenant (`tracking_repository.go:472-477`) after the internal route has accepted `X-Tenant-ID` (`tracking_handler.go:97-105`). A later implementation has to restrict that insert to a writer that copies the gateway auth context. Until that writer exists, this document does not say the actor read is safe.

`LIST_ACTIVE_TARGETS_AUTHORIZATION=PER_BINDING_SCOPE_NOT_SUFFICIENT`

`MULTI_TENANT_BINDING_POLICY=PER_BINDING_NO_ARBITRARY_PICK`

`SHARED_ACTOR_SCOPE=SEPARATE_VERIFIED_SCOPES`

## 3. Required cases

### A — One actor, one tenant

One active, in-window binding whose tenant and actor match one execution and one state row. The result contains that target only.

### B — One actor, several tenants

Two active bindings for one device, or the same driver UUID on two executions, produce two independent scopes. The function returns each scope that passes its own join. It does not keep the newer `active_from` and drop the other. A caller that needs one execution still has to name a scope that passed. The uniqueness test at `eta_ingest_service.go:201-203` is not that decision.

### C — Binding tenant differs from the tracking owner

The state row's `operating_tenant_id` differs from the binding `tenant_id`, or the execution row disagrees. That pair produces no target. Approach must not call `MarkApproachEmitted` for it.

### D — Revoked binding

`status` is `revoked` or `inactive`. The row is not a scope. The constant `BindingStatusRevoked` (`telemetry.go:29`) and the check constraint (`000026:18`) already name the value. No tracking-service writer of that value was found, so a row that is still `active` remains a candidate. Revocation is effective only after `status` changes.

### E — Missing tenant context

No active in-window binding. The result is empty. The function has no tenant argument to invent one.

### F — Tenant in the payload

Provider ingest already copies the tenant from the binding (`ingest_service.go:133`). The protected read does not accept a payload tenant or a header tenant. A forged field on the location body cannot select a scope.

### G — Operating tenant changes

The binding is re-read at use time. If the execution `operating_tenant_id` no longer equals the binding tenant, the scope fails. A binding read taken before the execution lock is not a grant. Any later state or ETA write still follows the PR #375 order: execution lock, owner-epoch check, then the effect. `owner_epoch` is specified by PR #375 and is not stored by the current migration head. This read does not create it and does not replace that check.

## 4. PostgreSQL and the trusted server

PostgreSQL can enforce the binding window, the per-binding join, and the empty result. It cannot see whether the binding row was inserted from `MustAuthContext` or from a header presented by a holder of the shared token. That distinction belongs to the binding writer, which is a gate, not a new identity system.

## 5. Unresolved

- Which existing writer, if any outside tracking-service, sets binding `status` to `revoked`.
- Whether `active_to` is populated by a current writer. The column exists. The reads do not use it.
- The binding-insert privilege change is a later implementation task. It is not authorized here.
