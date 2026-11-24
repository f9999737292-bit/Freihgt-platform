# 0.1R — Failure-domain limits of the local model

`WITNESS_FAILURE_DOMAIN=NOT_PROVEN`.

The test witness and the test PostgreSQL ran on one computer. Separate
directories, a separate process and a separate container do **not** make
separate failure domains. Nothing in this pack may be cited as evidence that a
witness is independent of PostgreSQL.

## Shared between witness and PostgreSQL in this model

| Shared element | Consequence |
| --- | --- |
| Physical host, power supply, OS kernel, clock | One host failure or power loss affects both; their relative state after such an event was not tested |
| Disk / filesystem (witness on the host filesystem, PostgreSQL on the Docker Desktop VM disk on the same host) | A host-level snapshot or backup restore can roll both back together, or roll back the witness alone; neither case is detectable by the witness after its last accepted checkpoint (F-TAIL-LOSS) |
| Operator / administrator | The same account could start, stop, edit or delete both stores |
| Key material | The orchestrator generated and held every HMAC key (issuers, operator, auditor, harness) and the IPC authkey in one process. Anyone with that process's memory could forge any principal, including operator checkpoints |
| Orchestrator | One harness process plays operator, reconciler and fault injector |

## What the local model does show

- **Logic of the protocol:** reservation before PostgreSQL; one reservation per key under concurrent requests; `IN_DOUBT` is never released or turned into `ABORT_CONFIRMED`; a stale PostgreSQL restore with intact witness history yields no new authorization; unavailable or failed witness means issuance is denied.
- **Single writer:** the server process keeps an exclusive OS lock on its store and serializes requests.
- **Tamper evidence:** changes to records **up to the last accepted checkpoint** are detected on load.

## What it cannot show

- **Independence:** no evidence that the witness survives, or fails separately from, PostgreSQL, host, disk, region, operator error or key compromise.
- **Durability:**
  - Records are fsynced per append; the parent directory is not fsynced; atomic rename of the checkpoint uses `os.replace`.
  - No power-cut, kernel-crash or storage-controller cache test was run.
  - Docker Desktop and Windows I/O semantics were not characterized.
- **Rollback of the witness itself:** a rollback of the witness store after the last checkpoint is undetectable (F-TAIL-LOSS, reproduced). Rollback of the witness store **and** its checkpoint directory together is also undetectable, because both live on one machine. This was not separately executed.
- **Production-grade append-only storage:** the file lock, the hash-chained JSONL file and the operator checkpoint are test-only mechanisms and not a production append-only witness. SQLite or another local engine would not change that without dedicated verification.
- **Fencing of an old primary's PostgreSQL COMMIT:** the witness has no control over PostgreSQL. A transaction started under an older epoch can still commit (`SS-OLD-COMMIT-NOT-FENCED`). `OLD_PRIMARY_FENCING=NOT_PROVEN`.
- **Prevention of PostgreSQL-only writes:** they are only detected by the gate afterwards (`SS-BYPASS-DETECTED-NOT-PREVENTED`).
- **Network partitions, latency, timeouts, availability, throughput:** IPC was a local named pipe.
- **Credential isolation:** see the key-material row above.
- **Replay after restart:** nonce replay protection is in memory only.
