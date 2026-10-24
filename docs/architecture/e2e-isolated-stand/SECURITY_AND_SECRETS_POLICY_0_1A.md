# BINTRANS E2E Isolated Stand 0.1A — Security and Secrets Policy

Stage: `BINTRANS-E2E-ISOLATED-STAND-0.1A-DISCOVERY`

Baseline: `fb9dc6c409f048c4b9f913bb40f315c2a11b4d6a`. This stage creates no credentials, prints no secret values, and does not read `/protected/**`.

## Classification

| Material | Classification for this stage | Stand rule |
| --- | --- | --- |
| `POSTGRES_PASSWORD` | `MISSING` from this branch. The base Compose file contains a committed dev default. | Generate it in the runner process for the stand project only. Do not commit it. Do not reuse the base-file default or a staging value. |
| `JWT_SECRET` | `MISSING` for the stand. The base file contains `dev_secret_change_me`. | Not required while api-gateway and identity-service are out of the scenario. Do not copy the dev literal into the stand file. |
| `INTERNAL_SERVICE_TOKEN` | The Go harness contains the fixed test token `bintrans-e2e-transport-flow-v0.1b`. | That string is a test fixture already in git. A Compose-started service must receive the same value the test sends. Do not replace it with `INTERNAL_SERVICE_TOKEN` from staging. Do not invent a second production token. |
| `DATABASE_URL` | `MISSING` as a populated stand URL. | Built at run time from the ephemeral user, password, host `postgres`, and stand database name. `sslmode=disable` is acceptable only on the internal stand network. |
| Staging env file | `MISSING` here, and it must stay unread. | Never pass `--env-file` to a path under `/protected`. |
| Registry credentials | `MISSING` | The stand builds local Dockerfiles or uses the public images already named in the repo. No registry login. |
| SSH keys | `MISSING` | No SSH. |

Allowed report words for any later run are only `PRESENT`, `MISSING`, `EMPTY`, `NONEMPTY_NONPLACEHOLDER`, and `PLACEHOLDER`.

## Network boundary

- Project network `bintrans-e2e-stand` with `internal: true`.
- No host port for Postgres, Redpanda, or any service.
- No route to `freight-platform-network`.
- No route to the staging VM, staging Compose project, or the public internet from stand containers.
- Redpanda advertises only `redpanda:9092`. The base file's external listener `localhost:19092` is not part of the stand.
- SASL and TLS stay disabled because the broker is reachable only on that internal network. Do not add a SASL password in this design.

`internal: true` blocks container egress. Image pulls happen before the network is used for application traffic. The runner image has to be present locally before the test container joins the network.

## Identity and tenancy

The happy path uses one tenant and two companies, the shipper and the awarded carrier. The fixture also inserts a foreign tenant and extra companies so the test can prove denial. Those extra rows are not a license to skip the negative checks.

The test sends `X-Tenant-ID` and `X-User-ID` straight to the service. Billing calls also send `X-Company-ID` and `X-Actor-Kind`. That matches the current harness. It is the trusted-header path used when the caller is already inside the platform, not a browser JWT.

api-gateway is not in the scenario. The dev file sets `AUTH_ENABLED=false`. A later gateway probe must not copy that setting and must not be described as proof of this flow. The flow's public-boundary check is the transport-order create that returns HTTP 403 when the body tenant and the header tenant differ. That call is already in `TestBintransE2ETransportFlow`.

Negative checks that the stand run has to keep green:

- Another carrier and a foreign tenant cannot execute the order.
- Another carrier's driver or vehicle, and a foreign driver or vehicle, cannot be assigned.
- A foreign tenant cannot convert the award.
- Another driver and a foreign driver cannot post pickup events.
- A foreign tenant does not receive a control-tower projection and receives HTTP 404 from the status route.
- Another driver and a foreign tenant cannot create a POD. A foreign tenant cannot read the document.
- A foreign tenant and another carrier cannot create the settlement. Another buyer cannot approve it. Denied include responses do not contain register or amount fields.
- Register calculate, approve, and package creation reject a foreign `tenant_id` in the body.

## Database roles

Migrations `000103`, `000104`, and `000105` create `freight_migrations`, `company_runtime`, and `shipment_runtime` as `NOLOGIN`. `000105` states that it does not create a `LOGIN`, does not change the shared application login, and does not install `trg_new_activation_authorization`.

The Postgres image creates the stand superuser from the ephemeral `POSTGRES_USER`. That login exists only inside the stand container. The stand must not `ALTER` or `CREATE` a login on any other instance, and it must not change `pg_hba.conf`.

Do not point service `DATABASE_URL` at `shipment_runtime` or at a `shipment_login` session. `ExecuteAwardOrder` under `shipment_login` fails closed with SQLSTATE `42501`, and the award-lock function is intentionally absent. The current test, and CI, connect as the database owner. The stand keeps that owner for this scenario.

`TMS_RELATIONSHIP_ACTIVATION_ENABLED` stays `false`. Do not apply `infrastructure/postgres/tms_activation_enforcement_gate_v0.1.sql`.

## Service-to-service authentication

The harness passes one internal token to contract-rate, transport-order, rfx, shipment, billing, and control-tower. Document-service is started without that token because the POD routes in this test use the tenant header and the upload token returned by the intent call.

The stand must not log the token, the database URL, or the Postgres password. Shipment and control-tower logs at `info` are required for event proof; those lines carry event ids and shipment ids, not credentials.

## Data leaving the stand

- No production or staging connection string.
- No dump of the stand database to a shared volume.
- Cleanup drops only the stand project volumes.
- The test's POD body is the fixed byte string already in the test. It is not a customer document.

## Enforcement checklist for a later implementation PR

1. Diff contains no populated `.env`, no `staging.env`, and no secret value.
2. Compose project, network, volume, and container names do not match `freight_postgres`, `freight_postgres_data`, `freight-platform-network`, or `bintrans-ct-staging`.
3. `ports:` is absent or empty for every stand service.
4. `000105` is the migration head. The enforcement trigger is absent.
5. Activation flag is `false`.
6. No `CREATE ROLE ... LOGIN` and no `pg_hba` change.
7. Payment service is absent and `billing.payment_obligations` stays empty.
8. Broker client in the stand run is Redpanda, not `kfake`.
