# 0.1U: Infrastructure decision

Status: `PENDING`. The three-server check is in `THREE_SERVER_INDEPENDENCE.md`. S3 is `NOT_SUITABLE` as an anchor. A managed log at a second provider has no confirmed product, specification, or price. No class is purchased, deployed, or approved for implementation.

`RECOMMENDED_CLASS=B_MANAGED_OUTSIDE_BINTRANS_ADMIN`.
That is a class to verify next. It is not a statement that a managed log solves P3B.

## Decision

| Option | Label | Decision |
| --- | --- | --- |
| A. Existing infrastructure | S1, S2, E2E design host, Selectel registry, unconfigured object storage | `NOT_SUITABLE` |
| B. Independent managed anchor | External append-only log, other administrators, other backup | `REQUIRES_PROVIDER_VERIFICATION` |
| C. Dedicated anchor infrastructure | Own host and disk outside the BINTRANS administrative domain | `REQUIRES_PROVIDER_VERIFICATION`, and only if B cannot meet the checks below |
| D. Quorum | Several members with a majority rule | `NOT_SUITABLE` as the next step |

`REQUIRES_NEW_SERVER=NO`.

Ordering another virtual machine in the account that already runs `161.104.57.152` does not create a failure domain. It adds a disk the same administrators can snapshot. Option C is allowed only when the new host's panel, storage account, backup product, and root credentials are not the BINTRANS ones. That fact is not true of anything in the inventory, so C is not selected today.

## Why B is the class to verify first

The missing property is not CPU. It is a log that BINTRANS operators cannot restore to an earlier sequence. A managed append-only service is the smallest place that property can exist, if and only if the provider can show:

1. BINTRANS has no restore, delete, or rewrite right on the log.
2. A second administrator, not a BINTRANS operator, holds the service root.
3. The service accepts an append of `(namespace, incarnation, seq, digest)` only as the exact successor and rejects a lower position.
4. The acknowledgement is durable before it is returned, and it names the predecessor digest.
5. A read returns the same history, bound to a caller-supplied challenge, or it fails.
6. Unavailability is an error, not an empty log that the issuer may treat as a new start.
7. Backup of that service cannot be applied by the BINTRANS staging restore path (`/protected/bintrans/deploy-snapshots/` or the Selectel panel used for S2).

If any line fails, B is `NOT_SUITABLE` for that provider and the question returns to C under the same seven lines. D is not a shortcut around a missing domain.

No price was available. A provider that cannot state a price can still be examined, and the decision stays `NOT_VERIFIED` on cost. Cost is not a reason to reuse S2.

## What this decision does not do

- It does not open `ISSUER_ISSUANCE_GATE`, `IMPLEMENTATION_GATE`, or `DECISION_CONSUMPTION_GATE`.
- It does not change `P3B_ALL_STORES_ROLLBACK=NOT_FIXED`.
- It does not change `ANCHOR_FAILURE_DOMAIN=NOT_PROVEN`.
- It does not change `OLD_PRIMARY_FENCING=NOT_PROVEN` or `J_DML_001=NOT_CLOSED`.
- It does not choose Selectel object storage. F-004 says S3/WORM is not configured, and a bucket in the same account would fail check 1.
- It does not authorize an implementation, a migration, or a role.

## Facts still required before a provider may be accepted

| Fact | State |
| --- | --- |
| Selectel account id and region for S1 and for S2 | `NOT_VERIFIED` |
| Whether one IAM principal can snapshot S1 and S2 | `NOT_VERIFIED` |
| Whether S1 is still allocated | `NOT_VERIFIED` |
| Live address and account of any E2E host | `NOT_VERIFIED` |
| A named managed log that meets checks 1–7 | `NOT_VERIFIED` |
| A price for that log | `NOT_VERIFIED` |

`INFRASTRUCTURE_DECISION=PENDING`.
