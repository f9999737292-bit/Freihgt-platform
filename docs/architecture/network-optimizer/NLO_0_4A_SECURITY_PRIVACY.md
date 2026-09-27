# NLO-0.4A security and privacy

Status: proposed. No runtime change.

```text
CALLER_SUPPLIED_EXECUTION_STATE_ALLOWED=NO
TENANT_ISOLATION_DESIGN=PASS
CROSS_SHIPPER_PRIVACY_DESIGN=PASS
```

## Trusted context

Evaluate for `CURRENT_TRIP` takes `shipment_id` and candidate load ids. The server builds position, onboard cargo, residual capacity, vehicle, and destination through the existing `currenttrip` provider. A body field for those facts is rejected as an unknown field, the same way NLO-0.3 rejects client budgets.

Depot-start planning takes an owned `capacity_id`. The capacity must belong to the caller tenant. A foreign capacity is `404`, matching consolidation search.

Gateway `X-Tenant-ID` is the tenant. The handler does not fall back to a query parameter.

## Versions

Accept and activate send `plan_id` and the `version` the caller read. The dependency rows must still match shipment, capacity, and load versions. Mismatch is `409` `PLAN_STALE`. The caller cannot overwrite a version to force a match.

## Idempotency

`Idempotency-Key` is required for evaluate, accept, and activate. Same key and same body replays. Same key and different body conflicts. Activate after `EXECUTION_LINKED` does not create a second driver-task request even if a new key is presented. The existing activation is returned.

## Marketplace projection

A carrier-visible plan includes:

- stop ordinal, canonical location identity allowed by the load's marketplace view, planned arrival, and planned departure
- action type and the load's public marketplace view
- road distance and duration for legs
- feasibility status and the public reason codes

A carrier-visible plan omits:

- other tenants' catalog overlays and private rule text
- private comments and commercial prices
- raw owner tenant ids on marketplace loads (`OwnerTenantID` is cleared the same way consolidation views clear it)
- tracking raw samples and driver identity beyond what the shipment already exposes to that carrier
- compatibility trace internals

Cross-shipper loads in one onboard interval use the NLO-0.3 rule. If ownership of the catalog scope cannot be proved, the sequence is `INDETERMINATE` with `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`. It is not `FEASIBLE`. One tenant's overlay is not applied to another tenant's cargo. That sequence cannot be activated.

## Audit fingerprint

The evaluation fingerprint covers ordered stop location ids, action types and load ids with versions, capacity or vehicle version, current-trip context fingerprint, per-leg routing fingerprints, compatibility fingerprints, catalog and rule versions, window inputs, `nlo-0.4a-insert-v1`, and the budget policy version.

The accepted plan version is the immutable snapshot id plus `version`. It is not recomputed by editing the evaluated row. A successor has its own fingerprint and `supersedes_plan_id`.
