# TRACKING-BINDING-AUTHORITY-DECISION-0.1H — Binding Write

`BINDING_WRITE_ENFORCEMENT=REVOKE_INSERT_PLUS_DECISION_FUNCTION`

`DUPLICATE_BINDING_POLICY=CONFLICT_UNLESS_SAME_DECISION_SHIPMENT`

`REVOCATION_POLICY=RECHECK_AND_OMIT`

The enforcement is a privilege boundary. A documented call order is not the control. Nothing here is applied. `DIRECT_BINDING_INSERT_BYPASS=OPEN` until that privilege change exists. `IMPLEMENTATION_GATE=CLOSED`.

## 1. The only write

The future write is one function. `tracking_runtime` receives `EXECUTE` on that function and no `INSERT`, `UPDATE`, or `DELETE` on `tracking.shipment_tracking_binding`. The same revoke applies to the shared application login when the role split from PR #376 is in force. The function owner is the only role that writes the table.

The function arguments are `decision_id` and the provider device identity. They are not tenant, driver, vehicle, or shipment. The function loads the decision row. It then re-reads the shipment, driver, vehicle, active execution link, and current revision. It inserts only when all of the following hold:

- the decision is unconsumed and `expires_at` is in the future
- the loaded shipment version and revision version equal the decision
- the driver still matches
- a claimed vehicle still matches the current shipment vehicle and, when an execution is in scope, the current execution vehicle
- a driver-only decision still has a null vehicle scope
- a required relationship pin is still valid
- `owner_epoch` is still `ABSENT` or, once the column exists, equals the current epoch

The function then sets `consumed_at`. A second use of the same `decision_id` inserts nothing.

The current insert (`tracking_repository.go:472-477`) accepts the caller's tenant, shipment, driver, and vehicle and is the bypass. The HTTP handler at `tracking_handler.go:96-135` is that bypass. Closing it means that handler no longer writes from headers. A request without a `decision_id` writes nothing. `REVOKE` is what stops a SQL session from calling `INSERT` directly. Removing the Go call without the `REVOKE` leaves the bypass open.

## 2. Cases

| Case | Control |
| --- | --- |
| Direct internal insert | No `decision_id`, or a session without `EXECUTE`, writes nothing. Token equality (`tracking_handler.go:97-99`) does not become a decision. |
| Forged tenant | The function has no tenant argument. The decision's tenants come from the shipment and execution rows. |
| Forged driver or vehicle | The function has no driver or vehicle argument. A header value never reaches the insert. |
| Duplicate binding | One active row per tenant, provider, and device remains the unique index (`000026_add_shipment_telemetry_foundation_v0.7.0.up.sql:28-30`). |
| `ON CONFLICT DO NOTHING` race | That clause is not used. The current clause is `tracking_repository.go:476`. It can return success when another shipment won. The function locks the unique key, and on conflict reads the stored shipment. A different shipment is a conflict error. Success means the stored shipment is the decision's shipment. |
| Stale assignment | The re-read inside the function fails the version, driver, vehicle, or revision check. No insert. |
| Revoked assignment | The driver no longer matches, the driver is inactive, or a required pin is gone. No insert. A row already stored is not a grant on later reads. |
| Historical unverified rows | Rows written by the current insert have no `decision_id`. The function does not attach one. Target search skips them. `HISTORICAL_BINDING_POLICY=NOT_A_GRANT`. |

Two tenants may each have an active row for one device. That is not a duplicate. The unique index is per tenant.

## 3. What the revoke does not stop

The table owner and a superuser are not limited by `GRANT` and `REVOKE`. PR #376 left that bypass open. `SHARED_FREIGHT_RISK=OPEN`. `ALL_WRITERS_FENCED=NO`. This contract does not claim the owner is fenced.

`ON_CONFLICT_DO_NOTHING_RACE=OPEN` until the function replaces the current insert. The design forbids that clause. The running code still has it.

## 4. Failure behavior

The caller sees conflict or not found, with the same privacy rule as the authority contract. A database error is unavailable. The function does not insert a row marked active and hope a later check corrects it.
