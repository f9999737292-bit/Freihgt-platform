# TRACKING-BINDING-AUTHORITY-0.1G — Protected Read Integration

PR #376 remains the DML decision: `tracking_runtime` does not receive table `SELECT`. PR #377 remains the read-authority decision: a caller tenant argument is not a grant, `GetState` is blocked, and stop ETA is blocked because the shared token is not a tenant. This document says how a future binding decision would feed those reads. It creates no function and no route.

`J-DML-001` stays `NOT_CLOSED` until a reviewer confirms an implemented read path. This design is not that confirmation.

## 1. ListActiveTargetsForActors

Current signature takes driver and vehicle ids and applies no tenant predicate (`execution_tracking_repository.go:196-215`). Callers are `approach.go:69` and `eta_ingest_service.go:192`.

Proposed interface, not created:

- Input: provider code and provider device id. No tenant, driver, or vehicle argument.
- The function loads binding candidates for that device.
- It keeps a candidate only when the lifecycle label is ACTIVE: current shipment assignment matches, and the execution row's operating tenant and actor match that candidate.
- The shipment and execution rows are read by the read-function owner. The runtime role does not receive table `SELECT` on `transport.shipments`, `transport.drivers`, `transport.transport_executions`, or `tracking.execution_tracking_state`.
- Output is the set of scopes that passed. Empty is a successful denial.
- A caller-supplied tenant is ignored if it is ever present on an older call path.
- Unavailable assignment rows produce an empty set, not the current unscoped query.

PR #377 called the per-binding join necessary and not sufficient because the binding writer trusted the header. This stage names the missing check as the shipment and execution re-read. The join to a self-written binding remains insufficient.

## 2. GetExecutionStopETA

Current route: `services/tracking-service/internal/http/router.go:75`. The handler checks the shared token (`eta_handler.go:282-288`) and then `X-Tenant-ID` (`eta_handler.go:296-306`).

Proposed interface, not created:

- The shared token remains proof of possession of the secret only.
- A tenant header is not an argument.
- A named service identity is required before any internal caller can receive an ETA. No such identity exists in the configuration or the handler. This stage does not define a new token, certificate, or header.
- `NAMED_SERVICE_IDENTITY_DEPENDENCY=UNRESOLVED`.
- Until that identity exists, and until the named caller presents a server-owned execution scope rather than a stop UUID, the read stays blocked as in PR #377.
- System-wide read of any stop is not granted.

## 3. GetState

`GetState` (`execution_tracking_repository.go:177-183`) still has no production caller. The only in-tree call is the integration test at `execution_stop_tracking_integration_test.go:661`.

It stays blocked. No public route is added. A binding decision does not become a `GetState` caller. A later caller has to be a concrete production path whose tenant and user come from gateway `MustAuthContext`, with shipment confirming execution participation. That caller is not in the tree.

## 4. PostgreSQL and the server

| Check | Where it can be enforced |
| --- | --- |
| Current shipment driver and tenant | Read of shipment-owned rows at use time |
| Current execution owner and actor | Read of the execution row at use time |
| Empty set when the decision is missing, revoked, expired, or reassigned | The read function |
| The HTTP caller is the gateway user rather than a token holder | The writer and the gateway. A function argument does not prove it. |
| Named internal service | Unresolved. Not a PostgreSQL predicate on the shared secret. |

The read is not the PR #375 owner-epoch fence. A scope returned by the read does not authorize a later write after the execution owner changes.

## 5. Follow-up documents, not edited here

- PR #377 actor-scope and protected-read text, to replace `BINDING_CREATION_TRUST=OPEN` with the shipment re-check once a controller accepts option B.
- PR #376 read-function owner, to add the shipment and execution columns used by the re-check, still without table `SELECT` for `tracking_runtime`.
- PR #375 only if a later implementation changes the lock caller. This read does not.
- PR #374 and PR #357 stay as they are. Broker safety is not proven.

## 6. Gates

`PROTECTED_READ_INTEGRATION=SPECIFIED_NOT_IMPLEMENTED`

`IMPLEMENTATION_GATE=CLOSED`
