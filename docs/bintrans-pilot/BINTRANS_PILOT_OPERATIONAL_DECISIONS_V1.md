# BINTRANS Pilot Operational Decisions v1

**Status:** ACTIVE — controlled real-user pilot authorized (see `BINTRANS_PILOT_GO_LIVE_AUTHORIZATION_V1.md`)
**Wave:** P0.1 Critical Operations Blocker Closure
**Last updated:** 2026-09-03

---

## Blocker summary

```text
OPS_BLK_001=CLOSED
OPS_BLK_002_STATUS=CLOSED
OPS_BLK_003_STATUS=CLOSED
OPS_BLK_004_STATUS=CLOSED
OPS_BLK_005_STATUS=CLOSED
CRITICAL_BLOCKERS_REMAINING=0
HIGH_BLOCKERS_REMAINING=0
MEDIUM_BLOCKERS_REMAINING=0
PILOT_TECHNICAL_READINESS=PASS
PILOT_OPERATIONAL_READINESS=PASS
PILOT_SECURITY_READINESS=PASS
CONTROLLED_PILOT_GO_LIVE_RECOMMENDATION=GO
REAL_USER_PILOT_ALLOWED=YES
CONTROLLED_REAL_USER_PILOT=AUTHORIZED
PILOT_STATUS=LIVE_CONTROLLED
PRODUCTION_GO_LIVE=NOT_AUTHORIZED
BROAD_CUSTOMER_ROLLOUT=NOT_AUTHORIZED
```

---

## DECISION-001 — Critical ownership assignments

| Field | Value |
|---|---|
| **Decision** | Assign named accountable individuals for pilot-critical roles |
| **Status** | **CLOSED** |

| Role | ASSIGNED | ACKNOWLEDGED | SOURCE |
|---|---|---|---|
| PILOT_BUSINESS_OWNER | Феликс | YES | CONTROLLER_CHAT (2026-09-03) |
| PILOT_TECHNICAL_OWNER | Марина | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| PILOT_OPERATIONS_OWNER | Люба | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| P1_INCIDENT_COMMANDER | Люба | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| INFRASTRUCTURE_OWNER | Люба | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| DATABASE_OWNER | Люба | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| SECURITY_OWNER | Марина | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| GO_LIVE_AUTHORITY | Феликс | YES | CONTROLLER_CHAT (2026-09-03) |

---

## DECISION-002 — Support / escalation channel

| Field | Value |
|---|---|
| **Decision** | Activate primary support and escalation contact route for pilot users |
| **Status** | **CLOSED** |

| Field | CURRENT | APPROVED |
|---|---|---|
| PRIMARY_SUPPORT_CHANNEL | BINTRANS Pilot Ops | YES |
| ESCALATION_CHANNEL | BINTRANS Pilot Ops | YES |

---

## DECISION-003 — Support window / SLA

| Field | Value |
|---|---|
| **Decision** | Approve pilot support window and P1/P2 response targets |
| **Status** | **CLOSED** |

| Field | CURRENT | APPROVED |
|---|---|---|
| PILOT_SUPPORT_WINDOW | 09:00–18:00 MSK, Monday–Friday | YES |
| P1_RESPONSE_TARGET | 15 minutes ACK target | YES |
| P2_RESPONSE_TARGET | 30 minutes ACK target | YES |

---

## DECISION-004 — Pilot RPO

| Field | Value |
|---|---|
| **Decision** | Approve recovery point objective for controlled pilot |
| **Status** | **CLOSED** |

| Field | Value |
|---|---|
| RPO_TARGET | 24h (daily validated backup via systemd timer) |
| FIRST_SCHEDULED_BACKUP_PROVEN | YES (2026-09-03T00:00:16Z / 03:00 MSK) |
| RPO_APPROVED | YES |
| BACKUP_AUTOMATED | YES |
| DAILY_BACKUP_AUTOMATION | ACTIVE |
| RPO_OPERATIONALLY_SATISFIED | YES |
| DB_MIGRATION_VERSION | 64 |
| DB_DIRTY | false |

---

## DECISION-005 — Pilot RTO

| Field | Value |
|---|---|
| **Decision** | Approve recovery time objective for controlled pilot |
| **Status** | **CLOSED** |

| Field | Value |
|---|---|
| RTO_TARGET | ≤30 minutes |
| RTO_APPROVED | YES |
| OBSERVED_DB_RESTORE_DURATION | 6 seconds (P0 isolated drill) |

---

## DECISION-006 — Local-image acceptance / registry publish

| Field | Value |
|---|---|
| **Decision** | Publish exact running images to Selectel registry and pin staging by immutable digest |
| **Status** | **CLOSED** (OPS-BLK-005) |

Evidence: `BINTRANS_PILOT_OPS_BLK005_CLOSURE_EVIDENCE_V1.md`

| Service | OCI revision | Published digest | IMAGE_ID_MATCH |
|---|---|---|---|
| rfx-service | `704ecbc3008e228c66046edc856a9de6dd6440c7` | `sha256:f950809d7b99a9f3a16c1d851fdd05c81fb6b6bde57a7949dde5e846c98597e9` | YES |
| shipment-service | `48ef3e56d428aa1ae53ae83e436fee493d7450a7` | `sha256:0b693de9f12c92e557a0dfb6a33a49d8ee14a600c8783918fa59169c2c46e457` | YES |
| web-admin | `6e4d3e22ec09c91d7d2a57e189918db15c564e69` | `sha256:e670c6cc4016f2e2ea90543ec03a2a460e3367c54b50da27a89094a7f2086204` | YES |

```text
REGISTRY_DIGEST_PINNED_SERVICE_COUNT=14
LOCAL_ONLY_SERVICE_COUNT=0
OVERALL_REPRODUCIBLE_WITHOUT_LOCAL_CACHE=YES
OPS_BLK_005_TECHNICAL_CLOSURE=PASS
```

---

## DECISION-007 — Alert external receiver (OPS-BLK-001)

| Field | Value |
|---|---|
| **Decision** | Provide approved Alertmanager external receiver |
| **Status** | **CLOSED** (OPS-BLK-001) |

Protected env keys (when approved): `ALERT_WEBHOOK_URL`, `SLACK_WEBHOOK_URL`, `TELEGRAM_BOT_TOKEN`, `ALERT_EMAIL_TO`

---

## DECISION-008 — Controlled real-user pilot go-live

| Field | Value |
|---|---|
| **Decision** | Authorize controlled real-user pilot on BINTRANS staging |
| **Status** | **AUTHORIZED** |
| GO_LIVE_AUTHORITY | Феликс |
| GO_LIVE_DECISION | GO |
| GO_LIVE_DECISION_DATE | 2026-09-03 |
| GO_LIVE_DECISION_SOURCE | CONTROLLER_CHAT |
| CONTROLLER_FINAL_PILOT_READINESS_GATE | PASS |

Evidence: `BINTRANS_PILOT_GO_LIVE_AUTHORIZATION_V1.md`

---

**MANAGEMENT_ACTION_REQUIRED=NO** (controlled pilot authorized; production rollout not authorized)
