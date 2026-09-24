# State machines

Transitions are compare-and-swap on `(id, version)`. A lost race returns conflict. The loser does not retry as a new winner.

## Capacity

```text
PREDICTED → AVAILABLE → RESERVED → ASSIGNED → CONSUMED
                ↓           ↓          ↓
            WITHDRAWN   WITHDRAWN   WITHDRAWN
                ↓           ↓
             EXPIRED     EXPIRED
```

| State | Meaning |
|-------|---------|
| `PREDICTED` | From a provider. Not offerable until confidence and publication policy allow `AVAILABLE`. |
| `AVAILABLE` | May be matched. |
| `RESERVED` | Held for one offer or one plan until TTL. |
| `ASSIGNED` | Bound to an accepted plan. |
| `CONSUMED` | The underlying movement started or the capacity window was used. |
| `WITHDRAWN` | Owner pulled it. |
| `EXPIRED` | Time or TTL ended. |

`PREDICTED` is not visible under marketplace scopes.

## Load opportunity

```text
DRAFT → PUBLISHED → MATCHING → RESERVED → ASSIGNED
           ↓           ↓          ↓
       WITHDRAWN   WITHDRAWN  WITHDRAWN
           ↓           ↓
        EXPIRED     EXPIRED
```

`DRAFT` is owner-only. `PUBLISHED` is the first state a projection exists. `MATCHING` means at least one job is considering it; it is still not reserved.

## Offer

```text
CREATED → SENT → VIEWED → ACCEPTED
                    ↓         ↑
                COUNTERED ----+
                    ↓
                 REJECTED
        EXPIRED or CANCELLED from CREATED, SENT, VIEWED, or COUNTERED
```

`ACCEPTED` is terminal for that offer revision. A counter creates a new revision; the previous revision becomes `COUNTERED` and cannot also be accepted.

## Route / chain plan

```text
GENERATED → PROPOSED → ACCEPTED → ACTIVE → COMPLETED
                            ↓         ↓
                        CANCELLED  AT_RISK → REOPTIMIZING → PROPOSED
                                       ↓
                                   CANCELLED
```

`ACTIVE` is forbidden for multi-stop or multi-shipper plans until shipment execution can represent them (ADR-NET-005). `GENERATED` is a system draft. `PROPOSED` is what a human or carrier sees.

## Races

| Race | Rule |
|------|------|
| Two carriers accept the same load | One `RESERVED` row. Second accept conflicts. Idempotent replay of the winner returns the same assignment. |
| One carrier accepts two incompatible loads | Second accept runs hard constraints against residual capacity. Failure releases nothing of the first and rejects the second. |
| Load withdrawn while an offer is active | Offer `CANCELLED`. Reservation released. Carrier sees withdrawal, not a silent expire. |
| Capacity changed after offer | Offer binds `capacity_version`. Mismatch invalidates the offer. |
| ETA changed | Prediction superseded. Linked offers that required the old window move to `AT_RISK` or `CANCELLED` per slack policy. |
| Vehicle changed | Capacity identity changes. Old offers do not follow the new vehicle. |
| Cargo quantity changed | Opportunity version bump. Open matches `invalidated`. Reserved offers re-checked; hard failure cancels them. |
| Shipment cancelled | Consume `shipment.cancelled`. Predicted capacity from that shipment is `WITHDRAWN`. Chain `AT_RISK`. |

Mechanisms: optimistic locking (`version`), reservation TTL, idempotency key on accept, and a single transaction that assigns the load and reserves the capacity together. There is no distributed lock service in the platform today; the first implementation should keep this transaction in one database schema.

## Reoptimization

Trigger `network.chain.marked_at_risk` when predicted availability of leg N+1 is after that leg's pickup window end, or slack is below the policy minimum. The worker builds a new plan. It does not cancel the live shipment. If no feasible replacement exists, the chain stays `AT_RISK` and Control Tower can show it. The optimizer does not auto-drop cargo that is already accepted without an explicit policy `AUTO_REPLAN_ACCEPTED=false` by default.
