# BINTRANS E2E Isolated Stand 0.1A — Rollback and Cleanup

Stage: `BINTRANS-E2E-ISOLATED-STAND-0.1A-DISCOVERY`

Baseline: `fb9dc6c409f048c4b9f913bb40f315c2a11b4d6a`. Nothing was deployed in this stage, so there is nothing to roll back on a server. This plan binds a later implementation.

## Scope of cleanup

Cleanup may remove only resources whose Compose project is `bintrans-e2e-stand-v0.1a`.

| Resource | Allowed action | Forbidden action |
| --- | --- | --- |
| Stand containers | `docker compose -p bintrans-e2e-stand-v0.1a -f <stand-file> down -v --remove-orphans` | `down` without `-p`, or `-f infrastructure/docker-compose/docker-compose.yml` |
| Stand network `bintrans-e2e-stand` | Removed by that `down` | `docker network rm freight-platform-network` |
| Stand Postgres volume | Removed by `down -v` | `docker volume rm freight_postgres_data` |
| Stand document volume | Removed by `down -v` | Deleting a host directory used by staging or the dev stack |
| Stand database inside the container | Dropped with the volume, or `DROP DATABASE` only when the name is `bintrans_e2e_flow_<id>` or the dedicated stand database | `DROP DATABASE` against `freight_platform` on any shared instance |
| Topics | Removed with the Redpanda container | `rpk topic delete` against a broker that is not `redpanda:9092` on this project |
| Images built for the stand | Left in the local image cache, or removed by explicit image id after the run | `docker image prune -a` |
| Staging project `bintrans-ct-staging` | No action | Any `up`, `down`, `stop`, or volume delete |
| Host `pg_hba.conf` and host Postgres | No action | Any edit or restart |

`docker volume prune`, `docker system prune`, `docker network prune`, and `docker compose down` without a project name are forbidden. They can select the dev stack or a staging project running on the same daemon.

## Order

1. Stop the runner so it stops opening new HTTP calls and stops its own child processes.
2. If the harness created `bintrans_e2e_flow_<id>`, allow its existing cleanup to drop that database. If that drop fails, continue with project removal; the volume delete is the backstop.
3. Run the project `down -v` command above once.
4. Confirm:
   - `docker compose -p bintrans-e2e-stand-v0.1a ps` lists nothing;
   - no container name starts with `bintrans-e2e-stand-v0.1a`;
   - volume list has no `bintrans-e2e-stand` volume;
   - `freight_postgres` and `bintrans-ct-staging` containers, if they were already running, still have the same ids as before the stand run.
5. Confirm no stand process remains bound to a published host port. The stand should not have published one.

## Failure during migrations

If migrate fails, do not rerun it against staging and do not use `migrate force` on any database outside the stand volume. Fix-forward is a new implementation task. Immediate recovery is `down -v`, which discards the empty or partial stand data directory.

If the harness `applyMigrations` fails, its `t.Cleanup` still drops the private database. Do not point that cleanup at `TEST_DATABASE_URL` when that URL is a shared server. The later runner must refuse a URL whose host is not the stand service name `postgres`.

## Failure during the scenario

Keep the stand only if an operator is collecting logs. Collect:

- `GET /ready` status for each required service;
- shipment log lines `outbox event published`;
- control-tower lines `starting shipment status consumer` and `shipment status event processed`;
- outbox status counts for the shipment id;
- projection counts for the operating tenant and the foreign tenant.

Do not copy logs that contain a database URL. Then run the same project `down -v`.

## Rollback of this discovery PR

This PR adds documentation only. Rollback is reverting the docs commit. It does not require a database down migration, a feature flag, or a deployment.

## Roles and activation

Cleanup does not connect as `shipment_login`, does not drop `freight_migrations`, `company_runtime`, or `shipment_runtime` on a shared server, and does not install or remove `trg_new_activation_authorization`. Those objects disappear only because the stand volume is deleted.
