# TMS analytics source contracts 0.4B

STAGE=SOURCE-CONTRACT-C-ANALYTICS-FACTS-0.4B
BASE_SHA=69371e34303b92cab188d89110c58c96156c69d8
ORIGIN_MAIN_AT_INVENTORY=69371e34303b92cab188d89110c58c96156c69d8
IMPLEMENTATION_AUTHORIZED=NO
PRODUCT_CODE_CHANGED=NO
ANALYTICS_FORMULAS_CHANGED=NO

This document freezes the shipment-service / TMS source contracts that Analytics 0.4A requested for GAP-C-001, GAP-C-002, and GAP-C-005. It does not implement KPIs, change Analytics formulas, add a migration, or add a runtime API.

Analytics request: [ANALYTICS_0_4A_KPI_SOURCE_CONTRACTS.md](../analytics/ANALYTICS_0_4A_KPI_SOURCE_CONTRACTS.md), requests AN-SRC-C-001, AN-SRC-C-002, and AN-SRC-C-005.

```
SOURCE_CONTRACT_VERSION=0.4B
TENANT_SERVER_DERIVED=YES
CLIENT_SUPPLIED_TENANT_AUTHORITY=NO
CROSS_TENANT_READ=DENY
INTERNAL_SOURCE_READ=YES
UNKNOWN_TO_FALSE=NO
UNKNOWN_TO_ZERO=NO
ANALYTICS_DIRECT_DB_ACCESS=NO
```

Unknown facts stay unknown. A missing row is not false, not zero, and not in-full.

## Current evidence

Inventory is against `origin/main` `69371e34`.

### Delivery identity and quantity

Execution delivery identity already exists as `execution_id`, `source_revision_id`, `source_execution_stop_id`, `source_action_id`, `shipment_id`, and `cargo_id`. `RecordDeliveryDisposition` rejects a command that omits execution, revision, operating tenant, stop, shipment, or cargo.

`transport.delivery_attempt_facts` stores `attempted_quantity`, `accepted_quantity`, `rejected_quantity`, and `uom`. The check constraint requires `attempted_quantity = accepted_quantity + rejected_quantity`, quantities non-negative, attempted greater than zero, and `uom = 'PALLET'`. The repository sets attempted to accepted plus rejected. The caller cannot supply a separate shipped quantity. The onboard ceiling is remaining `transport.cargoes.pallet_count`, and attempted may be less than that remaining quantity.

A disposition case is inserted only when `rejected_quantity > 0`. A zero-rejection command still inserts an attempt fact and does not open a case. Driver delivery completion does not call `RecordDeliveryDisposition`. `ReportDeliveryDisposition` is a separate command. A delivery can complete with no attempt row.

Return, redirect, and hold reserve rejected quantity on a case and can build a successor revision. They do not turn rejected quantity into accepted quantity. A successor delivery is a later delivery identity, not a rewrite of the source stop and action.

`CUSTOMER_REFUSAL` is a cargo disposition reason. It is not a carrier assignment decision.

### POD

`AllowedDriverMilestoneActions` sets `RequiresPOD: false` on `DELIVERY_COMPLETED`. No shipment policy, contract, rate, stop, action, or tenant policy in this tree states that POD is required. Document-service can store a document of type `POD`. That row is evidence of a file, not a requirement and not satisfaction of a requirement.

### Carrier assignment

Shipment status includes `CARRIER_ASSIGNED`, `ACCEPTED_BY_CARRIER`, and `CANCELLED`. Assigning a driver from `CARRIER_ASSIGNED` can move the shipment to `ACCEPTED_BY_CARRIER`. `transport.shipments.carrier_company_id` is the current carrier pointer. No table or command in this tree has a stable assignment-offer id or a recorded decision `ACCEPTED` or `REJECTED` with actor, time, and reason. RFx bid and invite state stays in rfx-service. Shipment cancellation does not record a carrier rejection.

## Shared read contract

Future internal reads, when a later stage implements them, use trusted service authentication and a server-derived tenant. The caller does not supply tenant authority. A foreign tenant receives denial without disclosure. Analytics does not read these tables directly.

Each contract below is row-level. A shipment-level boolean or a tenant-wide count is not the source fact.

```
EXPOSURE=ROW_LEVEL_FACTS
SOURCE_AGGREGATE=NO
```

An aggregate would drop the delivery identity that OTIF needs. On-time and in-full, when both exist, must be evaluated for the same delivery identity. This stage does not define the on-time formula. Analytics already owns that comparison. This contract only requires that in-full use the same delivery population.

## GAP-C-001 delivery in-full

```
GAP_C_001_CLASSIFICATION=NEEDS_SOURCE_IMPLEMENTATION
DELIVERY_IN_FULL_SOURCE=NOT_DERIVABLE
DELIVERY_QUANTITY_FACTS_PRESENT=PARTIAL
MULTI_DELIVERY_POINT_SEMANTICS_FROZEN=YES
SOURCE_CONTRACT_VERSION=0.4B
SOURCE_OWNER=shipment-service / TMS execution
EVENT_TIME_FIELD=delivery_completed_at
EVALUATED_TIME_FIELD=evaluated_at
COMPLETENESS_STATE=KNOWN or UNKNOWN
TENANT_SCOPE=operating_tenant_id
CARRIER_DIMENSION_AVAILABLE=CURRENT_SHIPMENT_POINTER_ONLY
HISTORICAL_REPLAY_AVAILABLE=NO
```

`DELIVERY_QUANTITY_FACTS_PRESENT=PARTIAL` because an attempt row, when the disposition command actually runs, stores accepted, rejected, and pallet UOM for one stop action and cargo. It does not store an independent shipped quantity, and a completed delivery with no command has no row.

### Population

One row is one delivery point: one execution action on one stop for one cargo on one shipment. Multiple delivery points on one shipment stay separate. A shipment-level boolean is not in-full.

### Rule

```
IN_FULL =
  accepted_quantity == shipped_quantity
  AND rejected_quantity == 0
```

This comparison is valid only when shipped quantity, accepted quantity, rejected quantity, and UOM are all known and the UOM values are the same. Current runtime does not meet that condition:

- `attempted_quantity` is defined as accepted plus rejected, so accepted equals attempted whenever rejected is zero. That identity is not an independent shipped quantity.
- Remaining `pallet_count` is a ceiling. It is not the quantity shipped to that delivery point.
- Status `DELIVERED` is not in-full.
- Absence of a disposition case is not in-full.
- Absence of an attempt row is not in-full.

```
IN_FULL_DERIVED_FROM_QUANTITY_FACTS=YES
IN_FULL_INFERRED_FROM_MISSING_CASE=NO
IN_FULL_INFERRED_FROM_DELIVERED_STATUS=NO
IN_FULL_INFERRED_FROM_ATTEMPTED_EQUALS_ACCEPTED=NO
```

When any required quantity or the UOM is missing, or the units differ:

```
COMPLETENESS_STATE=UNKNOWN
IN_FULL=UNKNOWN
```

`UNKNOWN` is not false and not zero. A known comparison with rejected quantity greater than zero, or accepted quantity not equal to shipped quantity, is `IN_FULL=false`.

Return, redirect, and hold do not change that result for the source delivery point. A later successor stop is a new delivery identity and is evaluated on its own facts.

`carrier_company_id` on the shipment is the current pointer. It is not a historical carrier-at-delivery fact. A future in-full row may copy the carrier id that the server resolves at completion. Until that copy exists, carrier-dimension history is not available. Replaying current shipment status or current carrier id does not reconstruct missing quantity rows.

`delivery_completed_at` is the server-owned completion event time of that delivery action. `evaluated_at` is when the source evaluated the quantities. They are different fields. Processing time does not replace the completion event time.

## GAP-C-002 POD requirement and satisfaction

```
GAP_C_002_CLASSIFICATION=NEEDS_SOURCE_IMPLEMENTATION
POD_REQUIREMENT_SOURCE=NONE
POD_REQUIREMENT_SOURCE_GAP=YES
POD_SATISFACTION_CLASSIFICATION=NEEDS_CROSS_DOMAIN_COMPOSITION
AGENT_B_DOCUMENT_EVIDENCE_DEPENDENCY=YES
SOURCE_CONTRACT_VERSION=0.4B
SOURCE_OWNER=shipment-service / TMS execution
DOCUMENT_EVIDENCE_OWNER=document-service / Agent B
EVENT_TIME_FIELD=requirement_decided_at
EVALUATED_TIME_FIELD=evaluated_at
COMPLETENESS_STATE=KNOWN or UNKNOWN
TENANT_SCOPE=operating_tenant_id
CARRIER_DIMENSION_AVAILABLE=CURRENT_SHIPMENT_POINTER_ONLY
HISTORICAL_REPLAY_AVAILABLE=NO
```

No current shipment policy, contract, rate, customer requirement, execution stop, execution action, or tenant policy is an authoritative POD requirement. `RequiresPOD: false` on the driver milestone map is a hardcoded action flag. It is not completeness and it is not proof that POD is not required.

```
POD_REQUIRED_DEFAULT_FALSE_IS_NOT_COMPLETENESS_PROOF=YES
DOCUMENT_ROW_COUNT_IS_NOT_POD_REQUIREMENT=YES
POD_REQUIREMENT_SERVER_OWNED=YES
```

Until a server-owned requirement source exists, every delivery has:

```
POD_REQUIRED=UNKNOWN
POD_SATISFIED=UNKNOWN
```

A future requirement record is owned by TMS execution, one row per delivery identity. It states whether POD is required, the canonical required document type when it is required, the server time the requirement was decided, and the server-derived tenant. The policy that sets the requirement must be an explicit server-owned source named by a later stage. This contract does not choose shipment policy, contract, rate, stop, or tenant policy, because none of those sources exists today.

Satisfaction is a composition, not a TMS write into document-service and not a document-service write into TMS:

```
delivery identity
+ POD requirement from Agent C
+ canonical document evidence from Agent B
→ POD satisfaction
```

`POD_SATISFIED` is true only when `POD_REQUIRED` is true and Agent B reports one canonical document of the required type for that delivery. `SATISFIED_DOCUMENT_ID` and `SATISFIED_AT` come from that document evidence. `POD_SATISFIED` is false only when `POD_REQUIRED` is true and the evaluated document evidence says the required document is absent. If the requirement is unknown, or the document read is unavailable, satisfaction stays `UNKNOWN`. Counting POD rows is not the denominator and is not satisfaction.

This stage does not implement that composition or any cross-domain write.

## GAP-C-005 carrier assignment decision

```
GAP_C_005_CLASSIFICATION=NEEDS_SOURCE_IMPLEMENTATION
CARRIER_ASSIGNMENT_OFFER_IDENTITY_PRESENT=NO
CARRIER_ASSIGNMENT_DECISION_SOURCE=NONE
CARRIER_ASSIGNMENT_DECISION_SOURCE_GAP=YES
SOURCE_CONTRACT_VERSION=0.4B
SOURCE_OWNER=shipment-service / TMS
EVENT_TIME_FIELD=decided_at
EVALUATED_TIME_FIELD=evaluated_at
COMPLETENESS_STATE=KNOWN or UNKNOWN
TENANT_SCOPE=operating_tenant_id
CARRIER_DIMENSION_AVAILABLE=ON_THE_DECISION_ROW
HISTORICAL_REPLAY_AVAILABLE=NO
```

The required fact is one server-owned offer and one canonical decision:

| Field | Contract |
| --- | --- |
| ASSIGNMENT_OFFER_ID | Stable server-owned identity. Absent today |
| SHIPMENT_ID | Required |
| CARRIER_COMPANY_ID | Required. The carrier the offer names |
| DECISION | `ACCEPTED` or `REJECTED` only |
| DECIDED_AT | Server event time of the decision |
| DECISION_ACTOR | Server-derived actor identity or class |
| REASON_CODE | Nullable versioned domain reason |
| TENANT_SCOPE | Server-derived operating tenant |

A decision row exists only when a server command records it. Current shipment status is not that command.

```
DELIVERY_REJECTION_IS_CARRIER_ASSIGNMENT_REJECTION=NO
RFX_NON_RESPONSE_IS_CARRIER_ASSIGNMENT_REJECTION=NO
CUSTOMER_REFUSAL_IS_CARRIER_ASSIGNMENT_REJECTION=NO
GENERIC_CANCELLED_STATUS_IS_REJECTION=NO
CARRIER_REMOVAL_WITHOUT_DECISION_IS_REJECTION=NO
ACCEPTED_BY_CARRIER_STATUS_IS_NOT_AN_OFFER_DECISION=YES
```

`ACCEPTED_BY_CARRIER` means the shipment status machine moved, including after driver assignment. It is not an offer id, not an actor, and not a reason. Removing or replacing `carrier_company_id` without a decision row is not a rejection. An unanswered RFx invite is not a rejection. Cargo rejection and `CUSTOMER_REFUSAL` stay on the delivery disposition contract. `CANCELLED` is not a carrier rejection.

The future rejection-rate denominator is the population of recorded offers, not all shipments. Offers with no decision stay `DECISION=UNKNOWN`. They are not rejected. This stage does not manufacture historical decisions from current status.

## Security

```
TENANT_SERVER_DERIVED=YES
CLIENT_SUPPLIED_TENANT_AUTHORITY=NO
CROSS_TENANT_READ=DENY
INTERNAL_SOURCE_READ=YES
UNKNOWN_TO_FALSE=NO
UNKNOWN_TO_ZERO=NO
```

Operating tenant on execution commands is the tenant scope. Shipment tenant is a participant attribute and is not a substitute for the caller choosing a tenant. A cross-tenant read is denied. Internal readers use the trusted internal service credential. No secret is created here.

## Future acceptance tests

These tests are `FUTURE_REQUIRED`. This stage does not implement them.

### C-001

- A delivery whose shipped, accepted, and rejected quantities are known in the same UOM, with accepted equal to shipped and rejected equal to zero, is in-full.
- A partial delivery, including rejected quantity greater than zero or accepted quantity less than shipped quantity, is not in-full.
- A missing shipped, accepted, or rejected quantity, or a UOM mismatch, is `UNKNOWN`, not false and not zero.
- Two delivery points on one shipment remain two rows. One in-full point does not make the other in-full.
- Status `DELIVERED` without quantity facts stays `UNKNOWN`.
- A missing disposition case stays `UNKNOWN`.
- An attempt row whose attempted quantity is only accepted plus rejected does not by itself prove in-full.

### C-002

- The POD-required denominator is the server-owned requirement rows, not delivered shipments and not a count of POD documents.
- Satisfaction is true only when the requirement is true and Agent B links one canonical document of the required type.
- A missing requirement policy is `POD_REQUIRED=UNKNOWN` and `POD_SATISFIED=UNKNOWN`.
- `RequiresPOD: false` on the driver milestone map is not a completeness result.
- No document-row count is used as the requirement or as satisfaction.

### C-005

- A recorded accepted offer is `DECISION=ACCEPTED` and is not inferred from `ACCEPTED_BY_CARRIER`.
- A recorded rejected offer is `DECISION=REJECTED` with server actor and decision time.
- An offer with no decision, including an unanswered RFx invite, is not a rejection.
- `CUSTOMER_REFUSAL` and cargo rejection are not carrier assignment rejections.
- Shipment `CANCELLED`, and carrier removal without a decision row, are not rejections.
- A cross-tenant read of any of the three facts is denied.

## Non-authorization

```
IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_CREATED=NO
OPENAPI_CHANGED=NO
GATEWAY_CHANGED=NO
STAGING_CHANGED=NO
KPI_PUBLISHED=NO
```

`OPS_OTIF`, `CAR_OTIF`, `EXEC_OTIF`, `OPS_POD_COMPLETENESS`, `CAR_POD_COMPLETENESS`, and `CAR_REJECTION_RATE` stay unpublished. GAP-B-001 remains an Agent B document-completeness contract and is not decided here.
