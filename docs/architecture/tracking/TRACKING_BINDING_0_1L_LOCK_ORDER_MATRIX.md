# 0.1L lock-order compatibility matrix

Proposed test order, not a production contract:

`execution → shipment → active link → driver → vehicle`

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`.

An edge exists only when one product transaction performs those operations in that order. A new `INSERT` of an execution is not an edge from an existing execution row. The isolated 0.1K deadlocks are not edges in this graph.

## Edges

```text
Execute command:        Execution -> Stop -> Shipment -> Execution event_seq
Successor:              Execution -> Revision -> Execution
Authorize return/redirect: Execution -> Disposition case -> Execution -> Revision
Record disposition:     Execution -> Disposition case
Flag-on activation:     Relationship head -> Shipment -> Cargo -> Driver -> Vehicle -> new Execution insert -> Active link insert
Flag-off activation:    new Execution insert -> Active link insert
AssignDriver:           Shipment
AssignVehicle:          Shipment
Shipment status/accept/cancel: Shipment
Award execution:        Award link -> Transport order -> new Shipment insert
Tracking consumer:      Tracking state
```

There is no product edge `Shipment -> existing Execution`. There is no product edge `Active link -> Execution`. There is no product `UPDATE` of the active-link row.

## Graph result

1. No cycle was found among the observed product edges.
2. The opposite order that would matter is flag-on activation's `Shipment` before a later active-link `INSERT`, set against a reader that locks the active link before the shipment. The proposed reader locks the shipment before the active link, so that pair does not close.
3. AssignDriver, AssignVehicle, status update, accept, and cancel have a transaction, but their only lock is the shipment `UPDATE`. They have no explicit `FOR UPDATE`.
4. The active-link grant has no transaction and no row lock until some caller issues SQL. That order is `UNKNOWN`.
5. Foreign keys and the revision-mismatch trigger can take locks this audit did not execute. Those waits are `UNKNOWN`, not compatible by assumption.
6. The tracking consumer and the shipment writers commit separately. An outbox publish is not one transaction with `Project` or `Execute`.

## Compatibility

| Writer | Same order as the proposal | Opposite order | Fenced by an execution lock | Verdict |
|---|---|---|---|---|
| Flag-off `Project` | new execution, then active link | no | no | YES for deadlock with the proposed reader. The link insert is not fenced by the existing execution. |
| Flag-on `Project` | shipment, cargo, driver, vehicle, then a new execution | shipment before the new execution insert | no existing execution lock | YES against the proposed reader. NO against a link-before-shipment reader. |
| AssignDriver | shipment only | no | no | YES for deadlock. Not fenced. |
| AssignVehicle | shipment only | no | no | YES for deadlock. Not fenced. |
| Status, accept, cancel | shipment only | no | no | YES for deadlock. Not fenced. |
| Award-order execution | no shared execution or link | no | no | YES |
| Driver-stop `Execute` | execution, then shipment | no | yes for its own execution | YES |
| Successor | execution, then revision | no | yes | YES |
| Record disposition | execution first | no | yes | YES |
| Authorize return or redirect | execution, then revision in the same transaction | no | yes | YES |
| Tracking consumer | different table and transaction | no | no | YES on the transport rows |
| `shipment_runtime` `UPDATE` of the active link | unknown | unknown | no | UNKNOWN |

`YES` here means no proven deadlock with a reader that uses the proposed order. It does not mean the writer is fenced. `ALL_WRITERS_FENCED` stays `NO`.

## Activation check

The earlier claim is true only for the optional path. `TMS_RELATIONSHIP_ACTIVATION_ENABLED` defaults to false (`services/shipment-service/internal/config/config.go:101`). Migration `000104` says it does not enable that flag.

When the flag is on, `authorizeRelationshipActivation` (`transport_execution_activation.go:67-71`) locks the shipment, cargo, driver, and vehicle before `Project` inserts a new execution (`transport_execution_repository.go:80-99`). That insert creates a new row. It is not `FOR UPDATE` of an execution that already exists.

The shared objects with a proposed reader of an existing execution are the shipment, and the active-link row when that shipment already has one. They are not the same execution row. The reader takes the execution lock first and then waits on the shipment. The activation transaction does not need that existing execution lock, so the wait does not form a cycle.

When the flag is off, activation does not lock the shipment, driver, or vehicle at all.

## Active-link ownership

`shipment-service` `Project` is the only observed inserter. There is no second service writing the table. There is also no single database owner: `shipment_runtime` can `UPDATE` the row directly. Transactions that must follow a future lock order are `Project`, the command `Execute` path, successor, and the disposition authorization that calls the successor. AssignDriver and AssignVehicle stay outside that fence today. Revoking the direct `UPDATE` is possible later. This audit does not design that grant change.

## Deadlock records

```text
WRITER_A=flag-on Project
WRITER_B=proposed read_binding_assignment
LOCK_A_HELD=shipment FOR UPDATE
LOCK_A_WAITS_FOR=existing execution row
LOCK_B_HELD=existing execution FOR SHARE
LOCK_B_WAITS_FOR=shipment FOR SHARE
CYCLE_POSSIBLE=NO_CYCLE_IN_OBSERVED_PATH
CODE_EVIDENCE=activation never selects the existing execution FOR UPDATE; it inserts a new id at transport_execution_repository.go:80-84
REPRODUCTION_NEEDED=NO for this pair
```

```text
WRITER_A=flag-on Project
WRITER_B=reader that locks the active link before the shipment
LOCK_A_HELD=shipment FOR UPDATE
LOCK_A_WAITS_FOR=active-link row on INSERT primary-key conflict
LOCK_B_HELD=active link
LOCK_B_WAITS_FOR=shipment
CYCLE_POSSIBLE=POTENTIAL_CYCLE
CODE_EVIDENCE=shipment lock at transport_execution_activation.go:263; link insert at transport_execution_repository.go:336
REPRODUCTION_NEEDED=YES before any reader uses link-before-shipment order
```

```text
WRITER_A=DriverStop Execute
WRITER_B=proposed read_binding_assignment
LOCK_A_HELD=execution FOR UPDATE
LOCK_A_WAITS_FOR=shipment FOR UPDATE
LOCK_B_HELD=execution FOR SHARE
LOCK_B_WAITS_FOR=shipment FOR SHARE
CYCLE_POSSIBLE=NO_CYCLE_IN_OBSERVED_PATH
CODE_EVIDENCE=command locks execution at transport_execution_command_repository.go:74 and shipment at line 1147
REPRODUCTION_NEEDED=NO for order; the link remains unlocked by the command
```

```text
WRITER_A=AssignDriver
WRITER_B=proposed read_binding_assignment
LOCK_A_HELD=shipment UPDATE
LOCK_A_WAITS_FOR=nothing in the five-row order
LOCK_B_HELD=execution FOR SHARE
LOCK_B_WAITS_FOR=shipment FOR SHARE
CYCLE_POSSIBLE=NO_CYCLE_IN_OBSERVED_PATH
CODE_EVIDENCE=shipment_repository.go:433 has no execution lock
REPRODUCTION_NEEDED=NO for deadlock. The shipment can still change while the reader holds only the execution.
```

```text
WRITER_A=shipment_runtime direct active-link UPDATE
WRITER_B=proposed read_binding_assignment
LOCK_A_HELD=unknown
LOCK_A_WAITS_FOR=unknown
LOCK_B_HELD=active link FOR SHARE after execution and shipment
LOCK_B_WAITS_FOR=none from this grant alone
CYCLE_POSSIBLE=INSUFFICIENT_EVIDENCE
CODE_EVIDENCE=000104 line 18 grants UPDATE; no product SQL uses it
REPRODUCTION_NEEDED=YES if that grant remains
```

The 0.1K shipment-first and link-first `40P01` results used test scripts that lock an existing execution after the shipment or the link. Those scripts are not product writers.
