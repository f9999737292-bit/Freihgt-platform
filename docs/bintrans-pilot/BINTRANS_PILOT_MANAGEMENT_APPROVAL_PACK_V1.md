# BINTRANS Pilot Management Approval Pack v1

**Status:** CLOSED — management approval and owner ACK recorded; OPS-BLK-002/003/004 closed
**Wave:** Management Approval Record + Owner ACK Gate v0.2
**Last updated:** 2026-09-03

This document distinguishes **MANAGEMENT_APPROVED**, **OWNER_ACKNOWLEDGED**, **HISTORICAL**, and **LEGACY_ONLY**.

---

## Purpose

Management decision package for BINTRANS controlled pilot blockers:

| Blocker | Topic | Status |
|---|---|---|
| OPS-BLK-001 | Telegram alert delivery | **CLOSED** |
| OPS-BLK-002 | On-call / ownership | **CLOSED** |
| OPS-BLK-003 | Support / escalation | **CLOSED** |
| OPS-BLK-004 | RPO / RTO | **CLOSED** |
| OPS-BLK-005 | Registry / image pinning | **OPEN_MEDIUM** |

---

## 1. Ownership register (management-approved and owner-acknowledged)

Recorded **2026-09-03**.

| Role | ASSIGNED | MANAGEMENT_APPROVED | OWNER_ACKNOWLEDGED | CONTACT_CHANNEL | ACK_SOURCE | ACK_DATE |
|---|---|---|---|---|---|---|
| PILOT_BUSINESS_OWNER | Феликс | YES | YES | BINTRANS Pilot Ops | CONTROLLER_CHAT | 2026-09-03 |
| PILOT_TECHNICAL_OWNER | Марина | YES | YES | BINTRANS Pilot Ops | BINTRANS_PILOT_OPS_TELEGRAM | 2026-09-03 |
| PILOT_OPERATIONS_OWNER | Люба | YES | YES | BINTRANS Pilot Ops | BINTRANS_PILOT_OPS_TELEGRAM | 2026-09-03 |
| P1_INCIDENT_COMMANDER | Люба | YES | YES | BINTRANS Pilot Ops | BINTRANS_PILOT_OPS_TELEGRAM | 2026-09-03 |
| INFRASTRUCTURE_OWNER | Люба | YES | YES | BINTRANS Pilot Ops | BINTRANS_PILOT_OPS_TELEGRAM | 2026-09-03 |
| DATABASE_OWNER | Люба | YES | YES | BINTRANS Pilot Ops | BINTRANS_PILOT_OPS_TELEGRAM | 2026-09-03 |
| SECURITY_OWNER | Марина | YES | YES | BINTRANS Pilot Ops | BINTRANS_PILOT_OPS_TELEGRAM | 2026-09-03 |
| GO_LIVE_AUTHORITY | Феликс | YES | YES | BINTRANS Pilot Ops | CONTROLLER_CHAT | 2026-09-03 |

### Per-person ACK summary

| Person | Roles | OWNER_ACK | ACK_SOURCE |
|---|---|---|---|
| Феликс | PILOT_BUSINESS_OWNER, GO_LIVE_AUTHORITY | **YES** | CONTROLLER_CHAT |
| Марина | PILOT_TECHNICAL_OWNER, SECURITY_OWNER | **YES** | BINTRANS_PILOT_OPS_TELEGRAM |
| Люба | PILOT_OPERATIONS_OWNER, P1_INCIDENT_COMMANDER, INFRASTRUCTURE_OWNER, DATABASE_OWNER | **YES** | BINTRANS_PILOT_OPS_TELEGRAM |

```text
MULTI_ROLE_ALLOWED=YES_FOR_SMALL_PILOT
MULTI_ROLE_APPROVED=YES
OWNERSHIP_ACK_COMPLETE=YES
ALL_REQUIRED_OWNER_ACK=YES
MARINA_OWNER_ACK=YES
LYUBA_OWNER_ACK=YES
OPS_BLK_002=CLOSED
OPS_BLK_002_CLOSED=YES
```

**LEGACY_ONLY:** Low-code Week-3 documents referenced other individuals in PM/support contexts. Those records are **not** operational ownership for BINTRANS Control Tower staging. `P2_OWNER` is **LEGACY_NON_REQUIRED_FOR_MINIMUM_PILOT_MODEL** — P2 escalation uses PILOT_OPERATIONS_OWNER → PILOT_TECHNICAL_OWNER.

---

## 2. Minimum ownership model (MANAGEMENT APPROVED)

| ID | Role | Assigned | Management approved | Owner ACK |
|---|---|---|---|---|
| A | PILOT_BUSINESS_OWNER | Феликс | YES | YES |
| B | PILOT_TECHNICAL_OWNER | Марина | YES | YES |
| C | PILOT_OPERATIONS_OWNER | Люба | YES | YES |
| D | P1_INCIDENT_COMMANDER | Люба | YES | YES |
| E | INFRASTRUCTURE_OWNER | Люба | YES | YES |
| F | DATABASE_OWNER | Люба | YES | YES |
| G | SECURITY_OWNER | Марина | YES | YES |
| H | GO_LIVE_AUTHORITY | Феликс | YES | YES |

---

## 3. Support model (MANAGEMENT APPROVED — CLOSED)

| Field | Value | Status |
|---|---|---|
| SUPPORT_WINDOW | 09:00–18:00 MSK, Monday–Friday | **APPROVED** |
| P1_INITIAL_ACK_TARGET | 15 minutes | **APPROVED** |
| P2_INITIAL_ACK_TARGET | 30 minutes | **APPROVED** |
| ESCALATION_CHANNEL | BINTRANS Pilot Ops | **APPROVED** |

```text
SUPPORT_POLICY_MANAGEMENT_APPROVED=YES
OPS_BLK_003=CLOSED
OPS_BLK_003_CLOSED=YES
```

---

## 4. Escalation chain (approved policy)

### P1

Monitoring / Operator → PILOT_OPERATIONS_OWNER (Люба) → P1_INCIDENT_COMMANDER (Люба) → PILOT_TECHNICAL_OWNER (Марина) → PILOT_BUSINESS_OWNER / GO_LIVE_AUTHORITY (Феликс)

### P2

Monitoring / Operator → PILOT_OPERATIONS_OWNER (Люба) → PILOT_TECHNICAL_OWNER (Марина)

### P3

Support queue → backlog / responsible product owner

---

## 5. RPO / RTO (CLOSED)

### Current staging evidence

| Field | Value |
|---|---|
| RESTORE_DRILL | PASS |
| OBSERVED_TECHNICAL_RESTORE_DURATION | ~6 seconds (isolated DB restore — **not** full operational RTO) |
| LATEST_VALIDATED_BACKUP | `/protected/bintrans/backups/freight_platform_20260903T000016Z.dump` |
| FIRST_SCHEDULED_BACKUP_UTC | 2026-09-03T00:00:16Z (03:00 MSK) |
| FIRST_REAL_TIMER_TRIGGER_PROVEN | YES |
| BACKUP_VALIDATION | PASS |
| BACKUP_AUTOMATED | YES |
| DAILY_BACKUP_AUTOMATION | ACTIVE |
| DB_MIGRATION_VERSION | 64 |
| DB_DIRTY | false |

### Approved targets

| Field | Value | Status |
|---|---|---|
| RPO_TARGET | 24 hours (daily validated logical backup) | **APPROVED** |
| RTO_TARGET | 30 minutes (committed operational target) | **APPROVED** |

```text
RPO_TARGET_MANAGEMENT_APPROVED=YES
RTO_MANAGEMENT_APPROVED=YES
RTO_APPROVED=YES
BACKUP_AUTOMATED=YES
DAILY_BACKUP_AUTOMATION=ACTIVE
FIRST_SCHEDULED_BACKUP_PROVEN=YES
RPO_OPERATIONALLY_SATISFIED=YES
OPS_BLK_004=CLOSED
OPS_BLK_004_CLOSED=YES
```

### HISTORICAL (superseded pre-automation state)

Prior to PR #92 merge and first timer run on 2026-09-03:

```text
HISTORICAL_BACKUP_CURRENT_MODE=MANUAL_OPERATOR_INVOCATION
HISTORICAL_BACKUP_AUTOMATED=NO
HISTORICAL_RPO_OPERATIONALLY_SATISFIED=NO
HISTORICAL_OPS_BLK_004=OPEN_PENDING_DAILY_BACKUP_AUTOMATION
```

Automated daily backup is now active via `bintrans-pilot-backup.timer` (03:00 MSK).

---

## 6. Decision matrix (current)

| DECISION_ID | TOPIC | APPROVED_VALUE | MANAGEMENT_APPROVED | OWNER_ACK / OPS SATISFIED | BLOCKER |
|---|---|---|---|---|---|
| DEC-OPS-001 | Business Owner | Феликс | YES | ACK YES | CLOSED |
| DEC-OPS-002 | Technical Owner | Марина | YES | ACK YES | CLOSED |
| DEC-OPS-003 | Operations Owner | Люба | YES | ACK YES | CLOSED |
| DEC-OPS-004 | P1 Incident Commander | Люба | YES | ACK YES | CLOSED |
| DEC-OPS-005 | Infrastructure Owner | Люба | YES | ACK YES | CLOSED |
| DEC-OPS-006 | Database Owner | Люба | YES | ACK YES | CLOSED |
| DEC-OPS-007 | Security Owner | Марина | YES | ACK YES | CLOSED |
| DEC-OPS-008 | Go-Live Authority | Феликс | YES | ACK YES | CLOSED |
| DEC-OPS-009 | Support Window | 09:00–18:00 MSK Mon–Fri | YES | Approved | CLOSED |
| DEC-OPS-010 | P1 ACK target | 15 minutes | YES | Approved | CLOSED |
| DEC-OPS-011 | P2 ACK target | 30 minutes | YES | Approved | CLOSED |
| DEC-OPS-012 | Escalation channel | BINTRANS Pilot Ops | YES | Approved | CLOSED |
| DEC-OPS-013 | RPO | 24h | YES | Operationally satisfied | CLOSED |
| DEC-OPS-014 | RTO | 30 minutes | YES | Approved | CLOSED |

---

## 7. MANAGEMENT APPROVAL RECORD

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
```

### Owner ACK record

```text
FELIX_OWNER_ACK=YES
FELIX_ACK_SOURCE=CONTROLLER_CHAT
FELIX_ACK_DATE=2026-09-03

MARINA_OWNER_ACK=YES
MARINA_ACK_SOURCE=BINTRANS_PILOT_OPS_TELEGRAM
MARINA_ACK_DATE=2026-09-03

LYUBA_OWNER_ACK=YES
LYUBA_ACK_SOURCE=BINTRANS_PILOT_OPS_TELEGRAM
LYUBA_ACK_DATE=2026-09-03

ALL_REQUIRED_OWNER_ACK=YES
```

---

## 8. Blocker and pilot readiness summary

```text
OPS_BLK_001=CLOSED
OPS_BLK_002=CLOSED
OPS_BLK_003=CLOSED
OPS_BLK_004=CLOSED
OPS_BLK_005=OPEN_MEDIUM

CRITICAL_BLOCKERS_REMAINING=0
HIGH_BLOCKERS_REMAINING=0
MEDIUM_BLOCKERS_REMAINING=1

PILOT_OPERATIONAL_READINESS=CONDITIONAL_PENDING_OPS_BLK_005
CONTROLLED_PILOT_GO_LIVE_RECOMMENDATION=PENDING_CONTROLLER_FINAL_READINESS_REVIEW
REAL_USER_PILOT_ALLOWED=NO
```

**NEXT_ACTION:** CONTROLLER_OPS_BLK_005_REMEDIATION_AUTHORIZATION
