# ADR-PLATFORM-001: Revoke an accepted carrier relationship

## Status

Proposed. Architecture only. Implementation, a migration file, a gateway route, TMS activation, and deployment are not authorized.

## Context

An accepted cross-tenant carrier relationship is an `ACTIVE` head on `company.carrier_relationship_heads`. PR #290, merged as `3551b960585f58d7a86c29d3b12fb4677caa6f88`, can cancel or expire a `PENDING` request. It does not revoke a relationship. `REVOKED` is already a legal relationship version and head status, and `carrier_relationship.revoked` is already a legal outbox event type. No command inserts that status.

Cancellation and revocation are different facts. Cancel ends a request that never became the current relationship, or records that a pending token expired. Revoke ends the current operating authority of a relationship that was accepted. Historical executions that already pinned a version must keep that pin. A later activation must not treat a revoked head as authority.

0.1A and 0.1B already froze the authority, the idempotency scope, the transaction boundary, and the reactivation rule. This ADR binds those decisions to the schema and commands on `main`. It does not replace them.

## Decision

Revoke is a company-service command on an existing relationship. It inserts the next append-only version with `status = REVOKED`, moves the mutable head to that version, and writes one audit row and one `carrier_relationship.revoked` outbox row in the same transaction. It does not update the previous version, the request head, or any shipment, execution, finance, POD, driver, or vehicle row.

```text
REVOKE_STATE_MACHINE=ACTIVE_HEAD_TO_NEW_REVOKED_VERSION
REQUEST_STATUS_ON_REVOKE=UNCHANGED
REVOKED_IS_NOT_CANCELLED=YES
HARD_DELETE=NO
ACTIVE_RELATIONSHIP_REVOKE_AUTHORITY=SHIPPER_SOURCE_CREATOR_AUTHORIZATION|SHIPPER_TENANT_GLOBAL_PLATFORM_ADMIN|OPERATING_SELECTED_COMPANY_CARRIER_ADMIN|OPERATING_TENANT_GLOBAL_PLATFORM_ADMIN
REVOKE_REQUIRES_SELECTED_COMPANY=YES
REVOKE_IDEMPOTENCY_SCOPE=relationship_id+acting_tenant_id+selected_company_id+idempotency_key
REVOKE_TRANSACTION_BOUNDARY=LOCK_HEAD_INSERT_REVOKED_VERSION_MOVE_HEAD_INSERT_AUDIT_AND_OUTBOX
REACTIVATION_POLICY=NEW_ACTIVE_VERSION_AFTER_NEW_TWO_SIDED_ACCEPTANCE
REVOKED_RELATIONSHIP_REUSE_POLICY=REUSE_SAME_ROOT_INSERT_NEXT_VERSION
RELATIONSHIP_REVOKED_BEFORE_ACTIVATE_POLICY=REFUSE_NO_EXECUTION
RELATIONSHIP_REVOKED_AFTER_EXECUTION_POLICY=HISTORICAL_EXECUTION_UNCHANGED
HISTORICAL_PINS_PRESERVED=YES
DRIVER_AND_VEHICLE_UNCHANGED=YES
GLOBAL_PARTY_INTRODUCED=NO
TMS_ACTIVATION_RUNTIME_IN_THIS_STAGE=NO
NLO_CHANGED=NO
```

The shipper actor is a verified user in `shipper_tenant_id` whose selected company is the requesting company of an `ACCEPTED` event for this relationship, with the same creator role that company-service already uses for that source type. The operating actor is a verified user in `operating_tenant_id` whose selected company is `operating_carrier_company_id`, with company-scoped `CARRIER_ADMIN`. Tenant-global `PLATFORM_ADMIN` replaces only that role check. It does not replace the active user, the active membership, or the selected company. `GOV_INSPECTOR` is not a revoke role.

A second authorized revoke does not insert a second `REVOKED` version. The same idempotency scope replays the stored version. A different key, after the head is already `REVOKED`, returns that same version and stores only an idempotency row. Restoration is a later accept of a new pending request, which the current accept path already implements by inserting the next `ACTIVE` version on the same root. There is no un-revoke command.

The future activation function must lock the relationship head, refuse a `REVOKED` head with `RELATIONSHIP_REVOKED`, and leave committed pin rows unchanged. This ADR does not create that function, the pin table, or an NLO change.

## Consequences

A runtime stage still has to add the revoke idempotency table, a nullable `revoke_reason` on new `REVOKED` versions, and a head transition trigger. That migration number is not assigned here. Accept's existing `REVOKED` to `ACTIVE` increment must remain legal, because that is reactivation. Public revoke responses contain the relationship id, the revoked version, and `REVOKED`. They do not contain a foreign tenant, a foreign company, or an acceptance token.

## Alternatives rejected

Editing the `ACTIVE` version in place to `REVOKED` breaks the append-only trigger and destroys the version that an execution pin names. Writing `CANCELLED` or `REVOKED` onto the request head collapses two lifecycles and violates the request status check. Deleting the relationship, the execution, or the pin rewrites history. A global party id is a new identity model and is not required to name either endpoint. An un-revoke command would edit a terminal version. Implementing activation or changing NLO in the revoke stage would couple two contracts that 0.1B kept apart.
