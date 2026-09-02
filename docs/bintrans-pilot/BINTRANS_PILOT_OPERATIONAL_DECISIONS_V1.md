# BINTRANS Pilot Operational Decisions v1

**Status:** MANAGEMENT APPROVAL RECORDED — owner ACK and backup automation pending
**Wave:** Management Approval Record + Owner ACK Gate v0.2
**Base SHA:** `6e4d3e22ec09c91d7d2a57e189918db15c564e69`
**Primary management pack:** `docs/bintrans-pilot/BINTRANS_PILOT_MANAGEMENT_APPROVAL_PACK_V1.md`

---

## Blocker register

| Blocker | Topic | Status | Notes |
|---|---|---|---|
| OPS-BLK-001 | Telegram alert delivery | **CLOSED** | FIRING + RESOLVED lifecycle PASS |
| OPS-BLK-002 | On-call ownership | **OPEN_PENDING_OWNER_ACK** | Management approved; Марина/Люба ACK pending |
| OPS-BLK-003 | Support / escalation | **OPEN_PENDING_OWNER_ACK_DEPENDENCY** | Policy approved; ops ownership ACK pending |
| OPS-BLK-004 | RPO / RTO | **OPEN_PENDING_DAILY_BACKUP_AUTOMATION** | Targets approved; RPO gap not accepted |
| OPS-BLK-005 | Registry / image pinning | **OPEN_MEDIUM** | Unchanged |

```text
PILOT_OPERATIONAL_READINESS=FAIL
CONTROLLED_PILOT_GO_LIVE_RECOMMENDATION=NO_GO
REAL_USER_PILOT_ALLOWED=NO
OPS_OBS_TELEGRAM_EGRESS_DNS_WORKAROUND=OPEN_NONBLOCKING
PILOT_REPEAT_INTERVAL_REVIEW=RECOMMENDED
```

---

## HISTORICAL — OPS-BLK-001 (CLOSED)

| Field | Value |
|---|---|
| **Status** | **CLOSED** |
| **Synthetic test** | `telegram-clean-lifecycle-20260902T210510Z` |
| **Root cause** | Upstream partial Telegram IP blocking |
| **Remediation** | Alertmanager `extra_hosts` DNS override (staging) |
| **Closure date** | 2026-09-02 |

---

## DECISION-001 — Critical ownership (OPS-BLK-002)

| Field | Value |
|---|---|
| **Management approval** | **YES** — 2026-09-03, APPROVED_BY=Феликс |
| **Status** | **OPEN_PENDING_OWNER_ACK** |
| **Closed** | **NO** |

| Role | Assigned | MANAGEMENT_APPROVED | OWNER_ACKNOWLEDGED |
|---|---|---|---|
| PILOT_BUSINESS_OWNER | Феликс | YES | YES |
| PILOT_TECHNICAL_OWNER | Марина | YES | NO — PENDING |
| PILOT_OPERATIONS_OWNER | Люба | YES | NO — PENDING |
| P1_INCIDENT_COMMANDER | Люба | YES | NO — PENDING |
| INFRASTRUCTURE_OWNER | Люба | YES | NO — PENDING |
| DATABASE_OWNER | Люба | YES | NO — PENDING |
| SECURITY_OWNER | Марина | YES | NO — PENDING |
| GO_LIVE_AUTHORITY | Феликс | YES | YES |

---

## DECISION-002 — Support / escalation channel (OPS-BLK-003)

| Field | Value |
|---|---|
| **Policy management approved** | **YES** — 2026-09-03 |
| **Status** | **OPEN_PENDING_OWNER_ACK_DEPENDENCY** |
| **Closed** | **NO** |

| Field | APPROVED |
|---|---|
| ESCALATION_CHANNEL | BINTRANS Pilot Ops — **YES** |
| CHANNEL_ROLE | ALERT_AND_INCIDENT_COORDINATION_CHANNEL — **YES** |

---

## DECISION-003 — Support window / SLA (OPS-BLK-003)

| Field | APPROVED |
|---|---|
| PILOT_SUPPORT_WINDOW | 09:00–18:00 MSK, Mon–Fri — **YES** |
| P1_INITIAL_ACK_TARGET | 15 minutes — **YES** |
| P2_INITIAL_ACK_TARGET | 30 minutes — **YES** |
| P3_INITIAL_ACK_TARGET | Next support window / backlog — **YES** |

```text
SUPPORT_POLICY_MANAGEMENT_APPROVED=YES
```

---

## DECISION-004 — Pilot RPO (OPS-BLK-004)

| Field | Value |
|---|---|
| **RPO target management approved** | **YES** — 24h |
| **RPO operationally satisfied** | **NO** |
| **RPO gap accepted** | **NO** |
| **Status** | **OPEN_PENDING_DAILY_BACKUP_AUTOMATION** |

| Field | Value |
|---|---|
| RPO_TARGET | 24 hours |
| BACKUP_CURRENT_MODE | MANUAL_OPERATOR_INVOCATION |
| BACKUP_AUTOMATED | NO |
| RPO_IMPLEMENTATION_GAP | YES |
| BACKUP_FOLLOWUP_REQUIRED | YES |
| DAILY_BACKUP_TIME_PROPOSED | 03:00 MSK (technical proposal) |

---

## DECISION-005 — Pilot RTO (OPS-BLK-004)

| Field | Value |
|---|---|
| **RTO management approved** | **YES** — 30 minutes |
| **RTO_APPROVED** | **YES** |

| Field | Value |
|---|---|
| COMMITTED_OPERATIONAL_RTO | 30 minutes |
| OBSERVED_TECHNICAL_RESTORE_DURATION | ~6 seconds (isolated drill — not operational RTO) |

Blocker OPS-BLK-004 remains open pending daily backup automation despite RTO approval.

---

## DECISION-006 — Local-image acceptance (OPS-BLK-005)

**Status:** OPEN_MEDIUM — unchanged. No work in v0.2.

---

```text
MANAGEMENT_APPROVAL_RECORDED=YES
NEXT_ACTION=WAIT_FOR_MARINA_AND_LYUBA_OWNER_ACK
```
