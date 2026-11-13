# TRACKING-BINDING-AUTHORITY-DECISION-0.1H — Binding Write

`BINDING_WRITE_ENFORCEMENT=REVOKE_INSERT_PLUS_DECISION_FUNCTION`

`DUPLICATE_BINDING_POLICY=CONFLICT_UNLESS_SAME_DECISION_SHIPMENT`

`REVOCATION_POLICY=RECHECK_AND_OMIT`

`BINDING_WRITE=BLOCKED_UNTIL_TRUSTED_PRESENTER`

`ATOMIC_CONSUMPTION_MODEL=SINGLE_FUNCTION_TRANSACTION_SPECIFIED_WRITE_BLOCKED`

`DECISION_REPLAY_POLICY=IDENTICAL_REPLAY_RETURNS_PRIOR_RESULT`

The enforcement is a privilege boundary. A documented call order is not the control. Nothing here is applied. `DIRECT_BINDING_INSERT_BYPASS=OPEN` on the current insert. The designed consume function is not granted, because the presenter cannot be passed in as a trusted value. `IMPLEMENTATION_GATE=CLOSED`.

## 1. One transaction, when the presenter exists

Shipment-service and tracking-service do not share a transaction. Two commits, one in each service, are not atomic. This contract does not call that pair atomic.

`transport` and `tracking` are schemas in one database. The designed consume function runs in one session and one transaction. It is `SECURITY DEFINER`, owned by `binding_authority_fence`, and it is the only writer of the binding and of `consumed_at`. `tracking_runtime` does not receive `EXECUTE` until a presenter channel exists that is not a header, a payload, or a function argument. That channel does not exist. The write stays blocked.

Inside that single transaction the order is fixed:

1. Load the decision by `decision_id` and lock the row `FOR UPDATE`.
2. Reject unless the trusted presenter equals `caller_user_id`.
3. Reject unless the presented device equals `provider_device_id` stored on the decision. The presented value is compared. It is not the source of the device.
4. Reject when `expires_at` has passed or `use_status` is already `CONSUMED` with different bound identifiers.
5. Re-read the shipment, driver, vehicle, active link, revision, and required pin.
6. Insert the binding only when that re-read matches the decision. `ON CONFLICT DO NOTHING` is not used. A conflicting active device row whose shipment differs is a conflict, and the decision stays unconsumed.
7. Set `consumed_at` and `use_status = CONSUMED`.
8. Commit.

A crash before commit leaves the decision unconsumed and inserts no binding. A second session blocks on the row lock. After commit, the same `decision_id` with the same presenter, device, and bound identifiers returns the binding created in step 6 and inserts nothing. The same `decision_id` with any other identifier is a conflict and does not change the binding.

The function arguments do not include tenant, driver, vehicle, shipment, or presenter. Device is an input only for the equality check in step 3.

`owner_epoch=ABSENT` does not satisfy an execution target. The function may record a driver-only binding when the location checks pass. It does not record an execution-target grant while the epoch is absent. `EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`.

The current insert (`tracking_repository.go:472-477`) accepts the caller's tenant, shipment, driver, and vehicle and is the bypass. The HTTP handler at `tracking_handler.go:96-135` is that bypass. `REVOKE INSERT` on the binding table, from `tracking_runtime` and from `freight`, is what stops a SQL session from calling `INSERT` directly. Removing the Go call without the `REVOKE` leaves the bypass open. Neither revoke is applied.

## 2. Cases

| Case | Control |
| --- | --- |
| Direct internal insert | No `decision_id`, or a session without `EXECUTE`, writes nothing. Token equality (`tracking_handler.go:97-99`) does not become a decision. |
| Forged tenant | The function has no tenant argument. The decision's tenants come from the shipment and execution rows. |
| Forged driver or vehicle | The function has no driver or vehicle argument. A header value never reaches the insert. |
| Duplicate binding | One active row per tenant, provider, and device remains the unique index (`000026_add_shipment_telemetry_foundation_v0.7.0.up.sql:28-30`). |
| `ON CONFLICT DO NOTHING` race | That clause is not used. The current clause is `tracking_repository.go:476`. It can return success when another shipment won. The function locks the unique key, and on conflict reads the stored shipment. A different shipment is a conflict error. Success means the stored shipment is the decision's shipment. |
| Stale assignment | The re-read inside the same transaction fails the version, driver, vehicle, or revision check. No insert. Outcome `REAUTHORIZATION_REQUIRED`. The old decision is not consumed. |
| Revoked assignment | The driver no longer matches or the driver is inactive. `HARD_DENIAL`. No insert. A row already stored is not a grant on later reads. |
| Historical unverified rows | Rows written by the current insert have no `decision_id`. The function does not attach one. Target search skips them. `HISTORICAL_BINDING_POLICY=NOT_A_GRANT`. |
| Forged presenter | Presenter differs from `caller_user_id`. No insert. The channel that would supply the presenter is unavailable, so this function is not granted. |
| Forged device | Presented device differs from `provider_device_id` on the decision. No insert. |
| Replay | Identical presenter, device, and bound identifiers return the prior binding. Different identifiers conflict and leave the binding unchanged. |

Two tenants may each have an active row for one device. That is not a duplicate. The unique index is per tenant.

## 3. Privileges

| Role | Decision table | Binding table | Consume function |
| --- | --- | --- | --- |
| `binding_authority_owner` | Owner. Can change `consumed_at` and every identifier. | No requirement to own it. | Owns the issue function. |
| `binding_authority_fence` | `UPDATE` of `consumed_at` and `use_status` only. No `UPDATE` of identifiers. | `INSERT` of the binding columns the function writes. | Owns the consume function. |
| `shipment_runtime` | No DML. | No DML. | No `EXECUTE` on consume. Future `EXECUTE` on issue only. |
| `tracking_runtime` | No `INSERT`, `UPDATE`, or `DELETE`. | No `INSERT`, `UPDATE`, or `DELETE`. | `EXECUTE` withheld. |
| `freight` | No membership and no grant in this design. The shared login remains able to bypass grants on tables it owns (`000103:6-7`). | Same open bypass where it is the owner. | No grant. |
| `freight_migrations` | Create, then `REVOKE` DML. Not a runtime writer. | Same. | No remaining `EXECUTE`. |
| `PUBLIC` | `REVOKE ALL`. | `REVOKE ALL` on the new function. | Not granted. |
| Operator superuser | Can update `consumed_at` and identifiers. | Can insert. | Not a designed grant. |

The column grant on `consumed_at` is what stops `binding_authority_fence` and both runtime roles from changing bound identifiers. It does not stop `binding_authority_owner` or a superuser. `TABLE_OWNER_DECISION_BYPASS=OPEN`. `SHARED_FREIGHT_RISK=OPEN`. `NOLOGIN` on the owner does not close either risk.

## 4. What the revoke does not stop

The table owner and a superuser are not limited by `GRANT` and `REVOKE`. PR #376 left that bypass open. `SHARED_FREIGHT_RISK=OPEN`. `ALL_WRITERS_FENCED=NO`. This contract does not claim the owner is fenced.

`ON_CONFLICT_DO_NOTHING_RACE=OPEN` until the function replaces the current insert. The design forbids that clause. The running code still has it.

## 5. Failure behavior

The caller sees conflict or not found, with the same privacy rule as the authority contract. A database error is unavailable. The function does not insert a row marked active and hope a later check corrects it.
