# TRACKING-BINDING-AUTHORITY-DECISION-0.1H — Use-Time Authorization

`USE_TIME_RECHECK=CURRENT_ROWS_OR_EMPTY`

`EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`

A stored binding and a previously issued `decision_id` are not the authorization for `ListActiveTargetsForActors`. The read evaluates current rows. `BINDING_IS_AUTHORIZATION_PROOF=NO`. An execution target from this read is denied while the owner epoch is unconfirmed, even when the driver, link, and revision match.

## 1. Input

The read receives the provider code and the provider device id. It does not receive a tenant, a driver list, or a vehicle list. The current function takes driver and vehicle ids and has no tenant predicate (`execution_tracking_repository.go:196-215`). Callers are `approach.go:69` and `eta_ingest_service.go:192`. Those callers change only when this contract is implemented.

`FindActiveBindingByDeviceAnyTenant` orders by `active_from` and keeps one row (`tracking_repository.go:51-53`). That `LIMIT 1` is not used. The active unique index allows the same device in more than one tenant (`000026:28-30`).

## 2. Evaluation

For every active binding row of that device:

1. Skip the row when it has no `decision_id`. Historical inserts are in this set.
2. Re-run the authority checks against current shipment, driver, vehicle, active execution link, revision, and required pin. Do not trust the columns copied onto the binding.
3. Keep the row only when the driver still matches and the vehicle rule matches. Driver-only does not match through `vehicle_id`.
4. Keep an execution target only when all of these hold together: the active link, the current revision, the execution driver, a claimed vehicle when one was claimed, the operating tenant from the execution row, a confirmed `owner_epoch`, an unexpired decision, and this use-time recheck. `owner_epoch=ABSENT` denies the target. `EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`. A shipment-version or revision mismatch is `REAUTHORIZATION_REQUIRED` for the location right and `EXECUTION_SCOPE_DENIED` for the target. The old decision is not a version-independent location proof.
5. Emit one scope per passing row. Scopes are not merged and not reduced to one tenant.

A shared driver or a shared vehicle therefore remains a set. Two passing scopes are two results. Approach and ETA perform a side effect only when that set contains exactly one execution. Zero or several produce no mark and no ETA write. The count is a refusal to guess. It is not a substitute for the checks above.

## 3. Concurrent reassignment

The re-read sees the latest committed assignment. A shipment version or revision version that differs from the row observed at the start of the statement drops that scope. The read does not take the execution command lock and does not authorize a later write. A write still follows the PR #375 order. An epoch that advances after the read invalidates any later effect. While the epoch column is absent, the read already returned no execution target.

If two sessions change the assignment while the read is in progress and the version cannot be confirmed, that scope is denied. The other device scopes are evaluated on their own.

## 4. Authority source unavailable

If the shipment, driver, execution, or pin rows required for a scope cannot be read, that scope is denied. If the authority source as a whole cannot be read, the result is an empty set. The function does not call the current unscoped actor query and does not use `status = 'active'` as a fallback.

The caller receives an empty result or unavailable. The body does not contain coordinates, a driver id, a vehicle id, or a tenant id.

## 5. Not granted

- A caller-supplied tenant selecting one of the scopes.
- `GetState`.
- An ETA read by a holder of the shared token.
- A vehicle lookup from a driver-only binding.
- Promotion of a historical row.
