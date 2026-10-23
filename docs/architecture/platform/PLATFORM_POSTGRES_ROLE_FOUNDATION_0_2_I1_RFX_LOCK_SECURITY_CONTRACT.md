# Award-link lock security contract

Blocker for `rfx.lock_award_transport_order_for_execution(uuid, uuid)`. The function is not created by migration `000105`.

## Caller

`lockAwardLink` in `services/shipment-service/internal/repository/order_execution_repository.go` runs:

```sql
SELECT id, tenant_id, rfx_event_id, rfx_award_id, rfx_response_id, rfx_lot_id,
    transport_order_id, carrier_company_id, buyer_company_id,
    amount::float8, currency_code, converted_at
FROM rfx.rfx_award_transport_orders
WHERE tenant_id = $1 AND transport_order_id = $2
FOR UPDATE
```

Both identifiers are arguments from `ExecuteAwardOrder`. PostgreSQL `current_user` is the service login, not a tenant. The shipment service does not set a tenant session attribute. The only `current_setting` uses in that service read `server_version`, the database name, or `session_replication_role`.

## Why a definer is not safe yet

A `SECURITY DEFINER` function owned by `freight_migrations` would lock the award row with the owner's rights. The existing activation definers are different: they write evidence and pins only after the stored relationship head, version, and subject identity match, and the runtime role cannot insert those rows itself.

The award function would have no equivalent server-owned caller identity. Passing the two UUIDs back into the function only repeats the caller's arguments. Any session that can execute it can name any tenant and any transport order. Checking `transport.transport_orders.tenant_id` proves the pair is consistent. It does not prove the session is allowed to act for that tenant.

`GRANT EXECUTE` is required to stay limited to `shipment_runtime`. The current shared login would then be unable to run a rewritten `lockAwardLink`, and this stage does not change that login or `DATABASE_URL`. The Go statement therefore stays the direct `SELECT ... FOR UPDATE`.

## Privilege left in place

`shipment_runtime` may `SELECT` `rfx.rfx_award_transport_orders`. It may not `INSERT`, `UPDATE`, or `DELETE` that table. `SELECT ... FOR UPDATE` and a direct `UPDATE` both fail with `42501` for `shipment_login`. The function is absent from `pg_proc`.

`ExecuteAwardOrder` still cannot run as `shipment_login` until a later contract supplies a caller identity the session cannot choose. That contract has to name the server-owned check, the function owner, the `search_path`, the execute grant, and the regression that a foreign tenant or a foreign order locks nothing. This document is not that implementation.
