# Control Tower Shadow Observation v0.6 — Release Manifest

**Immutable release reference for Selectel staging observation.**

Do not retag images as `latest` without digest or Git SHA.

---

## Release identifiers

| Field | Value |
|-------|-------|
| Feature SHA | `a5163c3` |
| Observation tooling SHA | `8708f2e` (+ handoff docs commit on observation branch) |
| Migration target | `000018` |
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
IDENTITY_IMAGE=<registry>/identity-service:git-a5163c3
SHIPMENT_IMAGE=<registry>/shipment-service:git-a5163c3
READ_MODEL_IMAGE=<registry>/control-tower-read-model-service:git-a5163c3
GATEWAY_IMAGE=<registry>/api-gateway:git-a5163c3
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

Target: **version 18**. Down migrations on staging are **not** permitted.

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
| `feat/control-tower-projection-rebuild-activation-v0.3` | `a5163c3` | Feature functionality |
| `test/control-tower-staging-shadow-observation-v0.6` | `8708f2e+` | Observation tooling and ops handoff |

PR status: **Feature branch pushed — PR creation pending** / **Observation branch pushed — PR creation pending**

---

## Observation window

- Duration: **7 consecutive calendar days** (minimum 5 business days)
- Starts: after deployment healthy + migration 18 + approved cohort + baseline snapshot
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
