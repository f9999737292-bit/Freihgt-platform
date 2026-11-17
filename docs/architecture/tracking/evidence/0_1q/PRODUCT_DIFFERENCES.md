# 0.1Q proof model vs. BINTRANS product

The 0.1Q evidence comes from a disposable, test-only model. Every result holds only inside that model. The differences below limit what the results say about the product.

| Area | 0.1Q model | Product / 0.1P contract | Consequence |
| --- | --- | --- | --- |
| Database | One disposable PostgreSQL 16.15 container (`postgres:16`, `--network none`), fresh per attempt | Shared platform PostgreSQL with migrations up to the current head | No platform schema, migration, or data was used. Nothing was proven about the real cluster. |
| Schema | `proof` / `proof_fence` test schemas from `sql/01_schema.sql` | Future issuer schema not yet designed as DDL; no migration reserved | The functions are models of 0.1P rules, not product DDL. |
| Issuance | `issue_ticket` checks digest, gate, epoch (`FOR UPDATE`), holder (`session_user`) and inserts one row | 0.1P reserve-before-commit with `RESERVED` / `COMMIT_CONFIRMED` / `ABORT_CONFIRMED` / `IN_DOUBT` / `PERMANENTLY_DENIED` and an external witness | Reservation states and witness ordering are not modeled. |
| Witness | None. `handover_state.witness_confirmed` is never set and only keeps the gate closed | External append-only witness outside the PostgreSQL data directory | All witness scenarios are `NOT_RUN`. A PostgreSQL column is not a witness. |
| Primary topology | Single primary, no replicas, no failover | Possible replicas / restored primaries | `OLD_PRIMARY_FENCING` stays `NOT_PROVEN`. A ban in one cluster does nothing to another primary. |
| Login identities | `proof_*` roles with per-attempt random passwords | Real service logins, including the shared `freight` login | The real `freight` login and its password were not used. Test E models a shared privileged login with `proof_shared_login`. |
| Cluster admin | `proof_admin` bootstrap superuser, `local ... trust` for the harness only | Managed admin access | The harness admin is the independent observer. It is not a product component. |
| Fencing controller | `proof_fencing_login` acting only through `proof_fence.*` SECURITY DEFINER functions owned by NOLOGIN `proof_fence_owner` (CREATEROLE, ADMIN on issuer roles, `pg_signal_backend`) | Future `binding_authority_fencing` role (0.1P R10), not created | This is one candidate shape. The direct CREATEROLE variant was shown to be unsafe (finding F1). |
| Connection pooling | Repeated one-shot reconnects (3 TCP, 2 Unix socket) | Application pools, possibly PgBouncer | Pooler-held server connections and pooler auth caching were not tested. |
| Authentication | `scram-sha-256` for all proof logins over loopback TCP and Unix socket | Platform `pg_hba` / network policy | `pg_hba` changes were not tested as a ban mechanism. |
| Tenant authorization | Not modeled | Gateway JWT → trusted tenant context | Role checks are not tenant authorization. `TRUSTED_TENANT_AUTHORITY` stays `NOT_PROVEN`. |
| Presenter binding | Not modeled | 0.1P presenter binding options | `PRESENTER_IDENTITY_BOUND` stays `NOT_IMPLEMENTED`. |
| Writers | One `proof_writer_login` with column-level UPDATE | Several services writing tracking/business tables | `ALL_WRITERS_FENCED` stays `NO`. `J_DML_001` is not closed. |
| Timing | Local Docker Desktop on Windows, millisecond-scale steps | Production latency and load | Races were not stress-tested. The signal-vs-gone race was never observed (F5), and that does not prove it is absent. |
