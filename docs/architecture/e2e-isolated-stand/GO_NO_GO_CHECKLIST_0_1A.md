# BINTRANS E2E Isolated Stand 0.1A — Go/No-Go Checklist

Stage: `BINTRANS-E2E-ISOLATED-STAND-0.1A-DISCOVERY`

Baseline: `fb9dc6c409f048c4b9f913bb40f315c2a11b4d6a`. A row is Go only when the evidence column can be filled from the stand run. This discovery stage does not mark the stand Go.

## Discovery verdict

| Gate | Result now | Evidence |
| --- | --- | --- |
| Docs describe an isolated project | Go for review | Architecture and dependency matrix |
| Stand deployed | No-Go | Docker was not started |
| Scenario executed on Redpanda | No-Go | Harness always starts `kfake` |
| Ready to merge this docs PR | No, until exact-head CI is green and Agent I finishes | Merge is not authorized by this stage |
| Ready to deploy | No-Go | Deployment is not authorized |

## Stand run gates

All of the following are required before anyone calls a future stand run successful.

| # | Gate | Pass condition |
| --- | --- | --- |
| 1 | Project identity | Compose project is `bintrans-e2e-stand-v0.1a`. Staging project `bintrans-ct-staging` is untouched. |
| 2 | Network | Network is internal. `docker compose port` shows no published ports. Postgres is not reachable from the host network. |
| 3 | Volume | Postgres data volume name contains `bintrans-e2e-stand` and is not `freight_postgres_data`. |
| 4 | Postgres | `pg_isready` succeeds inside the container. Image is PostgreSQL 16. |
| 5 | Migrations | Versions `000001` through `000105` applied with no error. Head is `000105`. |
| 6 | Activation | `TMS_RELATIONSHIP_ACTIVATION_ENABLED=false`. Trigger `trg_new_activation_authorization` is absent. Gate SQL was not applied. |
| 7 | Roles | Runtime roles from `000103`–`000105` are `NOLOGIN`. No `pg_hba` change. Services use the ephemeral database owner, not `shipment_login`. |
| 8 | Redpanda | `rpk cluster info` succeeds. Topics `shipment.status.v1` and `driver.events.v1` exist before the consumer starts. |
| 9 | Process health | contract-rate, transport-order, rfx, shipment, document, billing, and control-tower each return HTTP 200 from `GET /ready`. |
| 10 | Consumer | Control-tower log contains `starting shipment status consumer` for group `control-tower-e2e-<run-id>`. `kfake` is absent from the test log. |
| 11 | Seed only | Before the first RFx POST, shipment, order, POD, settlement, and projection business tables for the scenario are empty. |
| 12 | One chain | One operating tenant's shipper and awarded carrier produce one RFx, one award, one transport order, and one shipment. |
| 13 | Delivery | Shipment HTTP status is `DELIVERED`. One history row. Replay does not add a second row. |
| 14 | Broker | An independent reader of `shipment.status.v1` finds the shipment id and `DELIVERED`. |
| 15 | Projection | Operating-tenant control-tower HTTP status is `DELIVERED`. One projection row. Foreign tenant has zero rows and HTTP 404. |
| 16 | POD | One Document Service POD for that shipment and tenant. Replay does not add a file. |
| 17 | Settlement | One settlement, status `APPROVED`, linked to the same order and shipment. |
| 18 | Register | One register and one item. Final register status `CLOSING_DOCUMENTS_CREATED`. |
| 19 | Package | One closing package, one act, one VAT invoice. Replay returns the same ids. |
| 20 | No payment | `billing.payment_obligations` count is 0. No register is `PAID` or `CLOSED`. |
| 21 | No duplicates | Order, shipment, award link, POD, settlement, register, package, act, and VAT invoice counts stay at one after every replay. |
| 22 | Negative tests | The foreign-tenant and other-company denials in the test pass, including the transport-order tenant mismatch HTTP 403. |
| 23 | Secrets | Logs and the PR diff contain no database URL with a password, no staging env, and no newly created credential. |
| 24 | Cleanup | Only project `bintrans-e2e-stand-v0.1a` is removed, with its volumes. Staging containers and `freight_postgres_data` remain. |

## No-Go conditions

Stop the later run, do not retry against another database, and leave the stand up only long enough to collect the failing evidence if any of these are true:

- The broker address is a `kfake` listen address.
- The database host is not the stand container.
- A host port is published.
- Migration head is not `000105`, or the enforcement trigger exists.
- Activation is enabled.
- `ExecuteAwardOrder` is attempted as `shipment_login`.
- A shipment, POD, or settlement row appears before the matching HTTP call.
- Control-tower shows `DELIVERED` while the independent topic read found no `DELIVERED` record.
- A payment obligation appears.
- Cleanup would select a project other than `bintrans-e2e-stand-v0.1a`.

## Agent I

Review is limited to the seven documents and the confirmation that product code, migrations, and Compose files are unchanged. Agent I does not need a running stand to review this discovery PR. A later implementation PR is a new Go/No-Go against the table above.
