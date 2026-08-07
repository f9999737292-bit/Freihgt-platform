# Control Tower Shadow Observation v0.6 — Release Manifest

**Immutable release reference for Selectel staging observation.**

Do not retag images as `latest` without digest or Git SHA.

---

## Release identifiers

| Field | Value |
|-------|-------|
| Repository release SHA | `64d218b6474d85075126cbf753fe73c1bbff94dd` |
| Feature runtime SHA | `b75eb3de751002da94a3c271fda30d09be1db450` |
| Observation tooling SHA | `9da601044f8bd5248a5fd04fb8dc7e2652e6415e` |
| Feature merge SHA | `e3cc74fb1e03a0ac92aa4a8b6d749890ce2302b4` |
| Observation merge / final main SHA | `64d218b6474d85075126cbf753fe73c1bbff94dd` |
| Feature review SHA (runtime images) | `b75eb3d` |
| Observation tooling baseline commits | `8708f2e`, `648138a` (historical) |
| Observation branch synchronization commit | `14873b7` (historical merge only; not a functional tooling commit) |
| Migration target | `000019` |
| Migration version | **19** |
| Gateway mode | `shadow` |
| Consumer enabled | `true` |
| Shipment outbox enabled | `true` |
| Primary enabled | **`false`** |
| Kafka topic | `shipment.status.v1` |
| Kafka group | `control-tower-shipment-status-v1` |

---

## Image references (placeholders)

Replace `<registry>` with approved staging registry. Tag every image with Git SHA — never deploy unverified `latest`.

```text
IDENTITY_IMAGE=<registry>/identity-service:git-b75eb3d
SHIPMENT_IMAGE=<registry>/shipment-service:git-b75eb3d
READ_MODEL_IMAGE=<registry>/control-tower-read-model-service:git-b75eb3d
GATEWAY_IMAGE=<registry>/api-gateway:git-b75eb3d
```

Record image digest at deploy time:

```text
IDENTITY_IMAGE_DIGEST=<sha256:...>
SHIPMENT_IMAGE_DIGEST=<sha256:...>
READ_MODEL_IMAGE_DIGEST=<sha256:...>
GATEWAY_IMAGE_DIGEST=<sha256:...>
```

---

## Migration sequence

| Version | Description |
|---------|-------------|
| `000016` | Rebuild core (job, stage, backup tables) |
| `000017` | Activation/rollback columns and constraints |
| `000018` | Nullable `last_event_type` on stage |
| `000019` | Nullable `last_event_type` on rebuild backup |

Target: **version 19**. Down migrations on staging are **not** permitted.

---

## Runtime configuration (required)

```text
CONTROL_TOWER_READ_MODEL_MODE=shadow
CONTROL_TOWER_CONSUMER_ENABLED=true
SHIPMENT_OUTBOX_ENABLED=true
CONTROL_TOWER_KAFKA_TOPIC=shipment.status.v1
CONTROL_TOWER_KAFKA_GROUP_ID=control-tower-shipment-status-v1
AUTH_ENABLED=true
```

**Forbidden:** `CONTROL_TOWER_READ_MODEL_MODE=primary`

---

## Branches

| Branch | SHA | Purpose |
|--------|-----|---------|
| `feat/control-tower-projection-rebuild-activation-v0.3` | `b75eb3d` | Feature functionality (reviewed HEAD) |
| `test/control-tower-staging-shadow-observation-v0.6` | resolve at deploy with `git rev-parse HEAD` | Observation tooling, ops handoff, and release alignment |

PR status: **Feature PR #1 MERGED** (`e3cc74fb1e03a0ac92aa4a8b6d749890ce2302b4`) / **Observation PR #2 MERGED** (`64d218b6474d85075126cbf753fe73c1bbff94dd`)

---

## Observation window

- Duration: **7 consecutive calendar days** (minimum 5 business days)
- Starts: after deployment healthy + migration 19 + approved cohort + baseline snapshot
- Status: **NOT STARTED**

---

## Backup retention proposal

- **ACTIVE** backup retained through entire observation window and at least 7 days after activation
- **FAILED**, **CANCELLED**, **ROLLED_BACK** jobs: cleanup only after review and explicit confirmation
- No automatic cleanup scheduler

---

## Staging execution status

**STAGING_EXECUTION_PENDING_OPS_ACCESS**

This manifest does not authorize production deployment or primary mode.
