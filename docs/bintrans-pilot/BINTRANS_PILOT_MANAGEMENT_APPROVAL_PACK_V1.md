# BINTRANS Pilot Management Approval Pack v1

**Status:** MANAGEMENT APPROVAL RECORDED — owner ACK and backup automation pending
**Wave:** Management Approval Record + Owner ACK Gate v0.2
**Base SHA:** `6e4d3e22ec09c91d7d2a57e189918db15c564e69`
**Previous head:** `8f845e03936956b90cd44bef5b5293c72dc41bee`

This document distinguishes **MANAGEMENT_APPROVED**, **OWNER_ACKNOWLEDGED**, **OWNER_ACK_PENDING**, **HISTORICAL**, and **LEGACY_ONLY**.

**Critical:** MANAGEMENT APPROVAL ≠ OWNER ACK. Do not treat management assignment as on-call acceptance by Марина or Люба.

---

## Purpose

Management decision package for BINTRANS controlled pilot blockers:

| Blocker | Topic | Status |
|---|---|---|
| OPS-BLK-001 | Telegram alert delivery | **CLOSED** |
| OPS-BLK-002 | On-call / ownership | **OPEN_PENDING_OWNER_ACK** |
| OPS-BLK-003 | Support / escalation | **OPEN_PENDING_OWNER_ACK_DEPENDENCY** |
| OPS-BLK-004 | RPO / RTO | **OPEN_PENDING_DAILY_BACKUP_AUTOMATION** |
| OPS-BLK-005 | Registry / image pinning | **OPEN_MEDIUM** — out of scope |

---

## 1. Ownership register (management-approved)

Recorded **2026-09-03** by controller. Replaces prior TBD / LEGACY_ONLY values for the eight minimum roles.

| Role | ASSIGNED | MANAGEMENT_APPROVED | OWNER_ACKNOWLEDGED | CONTACT_METHOD | ACK_SOURCE | ACK_DATE |
|---|---|---|---|---|---|---|
| PILOT_BUSINESS_OWNER | Феликс | YES | YES | PENDING | CONTROLLER_CHAT | 2026-09-03 |
| PILOT_TECHNICAL_OWNER | Марина | YES | NO — PENDING | PENDING | — | — |
| PILOT_OPERATIONS_OWNER | Люба | YES | NO — PENDING | PENDING | — | — |
| P1_INCIDENT_COMMANDER | Люба | YES | NO — PENDING | PENDING | — | — |
| INFRASTRUCTURE_OWNER | Люба | YES | NO — PENDING | PENDING | — | — |
| DATABASE_OWNER | Люба | YES | NO — PENDING | PENDING | — | — |
| SECURITY_OWNER | Марина | YES | NO — PENDING | PENDING | — | — |
| GO_LIVE_AUTHORITY | Феликс | YES | YES | PENDING | CONTROLLER_CHAT | 2026-09-03 |

### Per-person ACK summary

| Person | Roles | OWNER_ACK | Notes |
|---|---|---|---|
| Феликс | PILOT_BUSINESS_OWNER, GO_LIVE_AUTHORITY | **YES** | Controller provided approval personally (`FELIX_OWNER_ACK=YES`) |
| Марина | PILOT_TECHNICAL_OWNER, SECURITY_OWNER | **PENDING** | `MARINA_OWNER_ACK=PENDING` — do not fabricate |
| Люба | PILOT_OPERATIONS_OWNER, P1_INCIDENT_COMMANDER, INFRASTRUCTURE_OWNER, DATABASE_OWNER | **PENDING** | `LYUBA_OWNER_ACK=PENDING` — do not fabricate |

```text
MULTI_ROLE_ALLOWED=YES_FOR_SMALL_PILOT
MULTI_ROLE_APPROVED=YES
OWNERSHIP_ACK_COMPLETE=NO
OPS-BLK-002=OPEN_PENDING_OWNER_ACK
OPS-BLK-002_CLOSED=NO
```

**Historical (LEGACY_ONLY — superseded for BINTRANS pilot):** Low-code Week-3 documents referenced other individuals in PM/support contexts. Those records are **not** operational ownership for BINTRANS Control Tower staging.

---

## 2. Minimum ownership model (MANAGEMENT APPROVED)

| ID | Role | Assigned | Management approved |
|---|---|---|---|
| A | PILOT_BUSINESS_OWNER | Феликс | YES |
| B | PILOT_TECHNICAL_OWNER | Марина | YES |
| C | PILOT_OPERATIONS_OWNER | Люба | YES |
| D | P1_INCIDENT_COMMANDER | Люба | YES |
| E | INFRASTRUCTURE_OWNER | Люба | YES |
| F | DATABASE_OWNER | Люба | YES |
| G | SECURITY_OWNER | Марина | YES |
| H | GO_LIVE_AUTHORITY | Феликс | YES |

One individual may cover multiple roles during small controlled pilot — **approved** for Феликс, Марина, and Люба as above.

---

## 3. Acknowledgement model

| Field | Rule |
|---|---|
| MANAGEMENT_APPROVED | Controller assigned named owner (2026-09-03) |
| OWNER_ACKNOWLEDGED | Individual confirmed on-call acceptance |
| CONTACT_METHOD | PENDING until supplied — do not invent phone/email/Telegram |

```text
ACK_STATUS_ALLOWED=PENDING|ACKNOWLEDGED
NO_BLOCKER_CLOSURE_WITH_PENDING_OWNER_ACK=YES
```

---

## 4. Support model (MANAGEMENT APPROVED)

| Field | Value | Status |
|---|---|---|
| SUPPORT_WINDOW | 09:00–18:00 MSK, Monday–Friday | **MANAGEMENT APPROVED** |
| P1_INITIAL_ACK_TARGET | 15 minutes | **MANAGEMENT APPROVED** |
| P2_INITIAL_ACK_TARGET | 30 minutes | **MANAGEMENT APPROVED** |
| P3_INITIAL_ACK_TARGET | Next support window / backlog triage | **MANAGEMENT APPROVED** |

```text
SUPPORT_POLICY_MANAGEMENT_APPROVED=YES
OPS-BLK-003_POLICY_APPROVED=YES
OPS-BLK-003=OPEN_PENDING_OWNER_ACK_DEPENDENCY
OPS-BLK-003_CLOSED=NO
```

### Incident levels (approved)

| Level | Definition |
|---|---|
| **P1** | Pilot unavailable; security/tenant-isolation failure; data integrity threat; critical business flow unavailable |
| **P2** | Major degradation; important workflow unavailable; workaround exists; pilot can continue partially |
| **P3** | Non-critical defect; UX issue; reporting inconsistency; minor operational issue |

**Legacy note:** Low-code pack same-business-day / next-business-day targets remain **HISTORICAL / LEGACY_ONLY** — superseded by approved BINTRANS targets above.

---

## 5. Escalation chain (approved policy)

### P1

Monitoring / Operator → PILOT_OPERATIONS_OWNER (Люба) → P1_INCIDENT_COMMANDER (Люба) → PILOT_TECHNICAL_OWNER (Марина) → PILOT_BUSINESS_OWNER / GO_LIVE_AUTHORITY (Феликс)

### P2

Monitoring / Operator → PILOT_OPERATIONS_OWNER (Люба) → PILOT_TECHNICAL_OWNER (Марина)

### P3

Support queue → backlog / responsible product owner

### Primary pilot incident channel

| Field | Value | Status |
|---|---|---|
| ESCALATION_CHANNEL | BINTRANS Pilot Ops | **MANAGEMENT APPROVED** |
| CHANNEL_ROLE | ALERT_AND_INCIDENT_COORDINATION_CHANNEL | **MANAGEMENT APPROVED** |
| TELEGRAM_ALERT_DELIVERY | Active | OPS-BLK-001 CLOSED |

Telegram is **not** the only durable audit log.

---

## 6. ACK procedure (approved policy)

For an incoming alert:

1. Human sees alert (Telegram / Grafana / Prometheus).
2. Human posts ACK in coordination channel.
3. Incident owner records: timestamp, alert, owner, severity, initial action.
4. For P1/P2: incident remains owned until resolved or explicit handoff.
5. Resolution is explicitly recorded.

```text
ACK | <severity> | <alert/incident> | <owner> | <UTC timestamp>
```

---

## 7. RPO / RTO (management-approved targets)

### Observed staging evidence (HISTORICAL)

| Field | Value |
|---|---|
| RESTORE_DRILL | PASS |
| OBSERVED_TECHNICAL_RESTORE_DURATION | ~6 seconds (isolated DB restore — **not** operational RTO) |
| LATEST_BACKUP | `freight_platform_20260901T190602Z.dump` |
| BACKUP_VALIDATION | PASS |

### Approved targets

| Field | Value | Status |
|---|---|---|
| RPO_TARGET | 24 hours (daily validated logical backup) | **MANAGEMENT APPROVED** |
| RTO_TARGET | 30 minutes (committed operational target) | **MANAGEMENT APPROVED** |
| COMMITTED_OPERATIONAL_RTO | 30 minutes | **RTO_APPROVED=YES** |

```text
RPO_TARGET_MANAGEMENT_APPROVED=YES
RTO_MANAGEMENT_APPROVED=YES
RTO_APPROVED=YES
RPO_OPERATIONALLY_SATISFIED=NO
RPO_GAP_ACCEPTED=NO
OPS-BLK-004=OPEN_PENDING_DAILY_BACKUP_AUTOMATION
OPS-BLK-004_CLOSED=NO
```

**RPO consequence:** Up to one day's changes could require reconstruction if only daily backup is available — target approved, mechanism not yet guaranteed.

---

## 8. Backup requirements and implementation gap

| Field | Value |
|---|---|
| BACKUP_CURRENT_MODE | MANUAL_OPERATOR_INVOCATION |
| BACKUP_SCRIPT | `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh` |
| BACKUP_AUTOMATED | NO |
| BACKUP_SCHEDULE | None automated |
| RPO_IMPLEMENTATION_GAP | **YES** |
| DAILY_VALIDATED_BACKUP_REQUIRED | **YES** |
| RPO_GAP_ACCEPTED | **NO** |
| RPO_GAP_ACCEPTED_BY | — |
| RPO_GAP_ACCEPTED_DATE | — |

Controller rejected accepting the gap without automated daily backup. Target RPO=24h is approved; operational satisfaction requires backup automation follow-up.

### Backup remediation follow-up (TECHNICAL PROPOSAL — not implemented)

```text
BACKUP_FOLLOWUP_REQUIRED=YES
DAILY_BACKUP_TIME_PROPOSED=03:00 MSK
```

Required future capability:

- Automatic daily PostgreSQL logical backup
- Fail-closed validation
- SHA256 generation
- Backup metadata update
- Retention policy
- Failure exit code
- Failure visible to pilot monitoring (`BintransPilotBackupStale` alert remains meaningful)
- No secret leakage
- Restore compatibility preserved

**Do not install cron/systemd in this docs-only task.**

---

## 9. Decision matrix (updated)

| DECISION_ID | TOPIC | APPROVED_VALUE | MANAGEMENT_APPROVED | OWNER_ACK / OPS SATISFIED | BLOCKER |
|---|---|---|---|---|---|
| DEC-OPS-001 | Business Owner | Феликс | YES | ACK YES | OPS-BLK-002 pending others |
| DEC-OPS-002 | Technical Owner | Марина | YES | ACK PENDING | OPS-BLK-002 |
| DEC-OPS-003 | Operations Owner | Люба | YES | ACK PENDING | OPS-BLK-002 |
| DEC-OPS-004 | P1 Incident Commander | Люба | YES | ACK PENDING | OPS-BLK-002 |
| DEC-OPS-005 | Infrastructure Owner | Люба | YES | ACK PENDING | OPS-BLK-002 |
| DEC-OPS-006 | Database Owner | Люба | YES | ACK PENDING | OPS-BLK-002 |
| DEC-OPS-007 | Security Owner | Марина | YES | ACK PENDING | OPS-BLK-002 |
| DEC-OPS-008 | Go-Live Authority | Феликс | YES | ACK YES | OPS-BLK-002 pending others |
| DEC-OPS-009 | Support Window | 09:00–18:00 MSK Mon–Fri | YES | Policy only | OPS-BLK-003 |
| DEC-OPS-010 | P1 ACK target | 15 minutes | YES | Policy only | OPS-BLK-003 |
| DEC-OPS-011 | P2 ACK target | 30 minutes | YES | Policy only | OPS-BLK-003 |
| DEC-OPS-012 | Escalation channel | BINTRANS Pilot Ops | YES | Policy only | OPS-BLK-003 |
| DEC-OPS-013 | RPO | 24h | YES (target) | **NOT operationally satisfied** | OPS-BLK-004 |
| DEC-OPS-014 | RTO | 30 minutes | YES | Approved | OPS-BLK-004 pending backup |

---

## 10. MANAGEMENT APPROVAL RECORD

```text
APPROVED_BY=Феликс
APPROVAL_DATE=2026-09-03
APPROVAL_SOURCE=CONTROLLER_CHAT
MANAGEMENT_APPROVAL_RECORDED=YES

PILOT_BUSINESS_OWNER=Феликс
PILOT_TECHNICAL_OWNER=Марина
PILOT_OPERATIONS_OWNER=Люба
P1_INCIDENT_COMMANDER=Люба
INFRASTRUCTURE_OWNER=Люба
DATABASE_OWNER=Люба
SECURITY_OWNER=Марина
GO_LIVE_AUTHORITY=Феликс

SUPPORT_WINDOW=09:00–18:00 MSK, Monday–Friday
P1_ACK_TARGET=15 minutes
P2_ACK_TARGET=30 minutes
ESCALATION_CHANNEL=BINTRANS Pilot Ops

RPO=24h
RTO=30 minutes

RPO_GAP_ACCEPTED=NO
```

### Owner ACK record (separate from management approval)

```text
FELIX_OWNER_ACK=YES
FELIX_ACK_SOURCE=CONTROLLER_CHAT
FELIX_ACK_DATE=2026-09-03

MARINA_OWNER_ACK=PENDING
LYUBA_OWNER_ACK=PENDING
```

---

## 11. Blocker and pilot readiness summary

```text
OPS-BLK-001=CLOSED
OPS-BLK-002=OPEN_PENDING_OWNER_ACK
OPS-BLK-003=OPEN_PENDING_OWNER_ACK_DEPENDENCY
OPS-BLK-004=OPEN_PENDING_DAILY_BACKUP_AUTOMATION
OPS-BLK-005=OPEN_MEDIUM

OPS_OBS_TELEGRAM_EGRESS_DNS_WORKAROUND=OPEN_NONBLOCKING
PILOT_REPEAT_INTERVAL_REVIEW=RECOMMENDED

PILOT_OPERATIONAL_READINESS=FAIL
CONTROLLED_PILOT_GO_LIVE_RECOMMENDATION=NO_GO
REAL_USER_PILOT_ALLOWED=NO
```

**NEXT_ACTION:** WAIT_FOR_MARINA_AND_LYUBA_OWNER_ACK + CONTROLLER_REVIEW_FOR_BACKUP_AUTOMATION_TASK
