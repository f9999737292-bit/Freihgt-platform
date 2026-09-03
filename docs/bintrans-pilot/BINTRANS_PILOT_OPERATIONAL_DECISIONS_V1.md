# BINTRANS Pilot Operational Decisions v1

**Status:** PARTIALLY CLOSED — OPS-BLK-002/003/004 closed; OPS-BLK-005 remains open
**Wave:** P0.1 Critical Operations Blocker Closure
**Last updated:** 2026-09-03

---

## Blocker summary

```text
OPS_BLK_001=CLOSED
OPS_BLK_002_STATUS=CLOSED
OPS_BLK_003_STATUS=CLOSED
OPS_BLK_004_STATUS=CLOSED
OPS_BLK_005_STATUS=OPEN_MEDIUM
CRITICAL_BLOCKERS_REMAINING=0
HIGH_BLOCKERS_REMAINING=0
MEDIUM_BLOCKERS_REMAINING=1
PILOT_OPERATIONAL_READINESS=CONDITIONAL_PENDING_OPS_BLK_005
CONTROLLED_PILOT_GO_LIVE_RECOMMENDATION=PENDING_CONTROLLER_FINAL_READINESS_REVIEW
REAL_USER_PILOT_ALLOWED=NO
```

---

## DECISION-001 — Critical ownership assignments

| Field | Value |
|---|---|
| **Decision** | Assign named accountable individuals for pilot-critical roles |
| **Status** | **CLOSED** |

| Role | ASSIGNED | ACKNOWLEDGED | SOURCE |
|---|---|---|---|
| PILOT_BUSINESS_OWNER | Феликс | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| PILOT_TECHNICAL_OWNER | Марина | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| PILOT_OPERATIONS_OWNER | Люба | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| P1_INCIDENT_COMMANDER | Люба | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| INFRASTRUCTURE_OWNER | Люба | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| DATABASE_OWNER | Люба | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| SECURITY_OWNER | Марина | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |
| GO_LIVE_AUTHORITY | Феликс | YES | BINTRANS_PILOT_OPS_TELEGRAM (2026-09-03) |

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
| DAILY_BACKUP_AUTOMATION | ACTIVE |

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
| **Decision** | Accept local-image fallback or publish immutable registry images for targeted deploys |
| **Status** | **ACTION_REQUIRED_FROM_MANAGEMENT** (OPS-BLK-005) |

| Service | OCI revision | Current image form |
|---|---|---|
| rfx-service | `704ecbc3008e228c66046edc856a9de6dd6440c7` | local tag `bintrans-rfx-r3.1c1:704ecbc` |
| shipment-service | `48ef3e56d428aa1ae53ae83e436fee493d7450a7` | local tag `bintrans-shipment-r3.1e1:48ef3e5` |
| web-admin | `48ef3e56d428aa1ae53ae83e436fee493d7450a7` | local tag `bintrans-web-admin-r3.1e1:48ef3e5` |

---

## DECISION-007 — Alert external receiver (OPS-BLK-001)

| Field | Value |
|---|---|
| **Decision** | Provide approved Alertmanager external receiver |
| **Status** | **CLOSED** (OPS-BLK-001) |

Protected env keys (when approved): `ALERT_WEBHOOK_URL`, `SLACK_WEBHOOK_URL`, `TELEGRAM_BOT_TOKEN`, `ALERT_EMAIL_TO`

---

**MANAGEMENT_ACTION_REQUIRED=YES** (OPS-BLK-005 registry/local-image acceptance only)
