# 0.1L remediation options

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`.

Neither option is selected. `RECOMMENDATION_STATUS=PROPOSED_NOT_APPROVED`.

The audit found no proven product deadlock with the proposed reader order. It found writers that change a shipment or can change the active link without locking the existing execution. Those writers are outside the fence.

## Option A — keep the test order and fence the rows it already locks

Keep the future read order as execution, shipment, active link, driver, vehicle.

This matches the product command and successor paths, which lock the execution first and, for commands, the shipment second. It does not deadlock with flag-on activation, because that path inserts a new execution instead of locking the existing one. AssignDriver and AssignVehicle only update the shipment, so the reader's shipment lock waits for them. A later `FOR SHARE` of the active link waits for a direct link update, which is the isolated result already recorded in PR #381.

Required change is in the future read function and its privilege proof, not in current writers. The `shipment_runtime` `UPDATE` grant on the active link stays open. A caller can still update `execution_id` without taking the execution lock; the fence works only while the reader holds the link row.

Deadlock risk with current writers: low against the observed edges. The risk appears if a later change locks the active link before the shipment, or makes activation lock an existing execution after it has locked the shipment.

Service compatibility: no current handler changes. Verifiable by extending the isolated proof, not by treating the 0.1K fixture as production.

## Option B — change the writer protocol and close direct DML

Make every writer that changes an existing execution, its revision, its shipment assignment, or its active link take one agreed order in one transaction. Remove or narrow `UPDATE` on `transport.transport_execution_active_shipments` so `execution_id` cannot be changed outside that protocol.

The agreed order cannot be "shipment first, then the existing execution." Flag-on activation already holds the shipment before it would reach that execution, and the command path already holds the execution before the shipment. Adding the missing lock at the end of either path creates the cycle the isolated tests demonstrated.

A safe direction for this option is execution first for any transaction that touches an existing execution, including a replacement of the active link. Flag-on activation would have to lock that existing execution before the shipment, or it would have to keep creating a new execution and conflict only on the link under the same earlier lock. AssignDriver and AssignVehicle would join that transaction or take the same prefix. This is a product-protocol change across shipment commands, assignment, and activation. It needs its own contract, migration, and grant review. This document does not write that SQL.

Deadlock risk: higher during the change, and lower only after every observed writer uses the same prefix. Service compatibility: breaks the current split between shipment assignment and execution commands. Verifiable only with tests that use the real statements, not the 0.1K stand-ins.

## Comparison

| Question | Option A | Option B |
|---|---|---|
| Deadlock with observed writers | no proven cycle | cycle if the existing execution is locked after the shipment |
| Product edits now | none | shipment, command, activation, and grants |
| Direct `UPDATE` of the link | remains possible | can be closed |
| AssignDriver and AssignVehicle | waited on by the shipment lock, not by the execution lock | brought into the same prefix |
| Check | isolated reader against the real statement order | new writer tests plus grant tests |

The controller has to choose. This audit stops before that choice.
