# BINTRANS Pilot Management Approval Pack v1

**Status:** OPEN — awaiting explicit management/controller approval  
**Wave:** Management Ownership + Support + RPO/RTO Decision Package v0.1  
**Base SHA:** `6e4d3e22ec09c91d7d2a57e189918db15c564e69`  
**Do not mark APPROVED without named individual, contact method, and acknowledgement record.**

---

## Purpose

Single management-facing package for closure prerequisites of:

| Blocker | Topic | Status |
|---|---|---|
| OPS-BLK-002 | On-call / ownership | OPEN_WAITING_FOR_MANAGEMENT_APPROVAL |
| OPS-BLK-003 | Support / escalation | OPEN_WAITING_FOR_MANAGEMENT_APPROVAL |
| OPS-BLK-004 | RPO / RTO approval | OPEN_WAITING_FOR_MANAGEMENT_APPROVAL |
| OPS-BLK-005 | Registry / image pinning | OPEN_MEDIUM — **out of scope for this pack** |

This document distinguishes **APPROVED**, **PROPOSED**, **LEGACY_ONLY**, **TBD**, and **NOT_FOUND**.

---

## 1. Ownership inventory (OPS-BLK-002)

A name in a legacy low-code pilot document does **not** constitute BINTRANS pilot operational approval or on-call acceptance.

| Role | CURRENT_VALUE | SOURCE | ACK_STATUS | APPROVED_STATUS |
|---|---|---|---|---|
| PILOT_BUSINESS_OWNER | TBD | — | PENDING | NOT_FOUND |
| PILOT_TECHNICAL_OWNER | Platform team (role label only) | `workstream-status-v0.1.md` WS PLAT | PENDING | LEGACY_ONLY |
| PILOT_OPERATIONS_OWNER | Артем Асаev (low-code support context) | `LOW_CODE_PILOT_WEEK3_SUPPORT_OWNERSHIP_POLICY_V0.1.md` | PENDING | LEGACY_ONLY — **not verified for BINTRANS pilot ops** |
| P1_INCIDENT_COMMANDER | TBD | — | PENDING | NOT_FOUND |
| P2_INCIDENT_OWNER | Support owner (role label only) | `LOW_CODE_PILOT_WEEK3_SUPPORT_ESCALATION_MATRIX_V0.1.md` | PENDING | LEGACY_ONLY |
| INFRASTRUCTURE_OWNER | DevOps (role label only) | `workstream-status-v0.1.md` | PENDING | LEGACY_ONLY |
| DATABASE_OWNER | TBD | — | PENDING | NOT_FOUND |
| SECURITY_OWNER | Security/Architecture (role label only) | escalation matrix | PENDING | LEGACY_ONLY |
| GO_LIVE_AUTHORITY | TBD | — | PENDING | NOT_FOUND |

**Historical note (not operational approval):** Low-code Week-3 governance documents reference individuals in PM / go-no-go contexts. Those records apply to the **low-code controlled pilot documentation track**, not automatically to BINTRANS Control Tower staging operational roles. Do **not** infer BINTRANS on-call ownership from historical mentions alone.

---

## 2. Minimum ownership model (PROPOSED — not approved)

Minimum required roles for controlled BINTRANS pilot:

| ID | Role | Required for pilot |
|---|---|---|
| A | PILOT_BUSINESS_OWNER | YES |
| B | PILOT_TECHNICAL_OWNER | YES |
| C | PILOT_OPERATIONS_OWNER | YES |
| D | P1_INCIDENT_COMMANDER | YES |
| E | INFRASTRUCTURE_OWNER | YES |
| F | DATABASE_OWNER | YES |
| G | SECURITY_OWNER | YES |
| H | GO_LIVE_AUTHORITY | YES |

```text
MULTI_ROLE_ALLOWED=YES_FOR_SMALL_PILOT
MULTI_ROLE_APPROVAL_STATUS=PROPOSED_NOT_APPROVED
```

One accountable individual **may** cover multiple roles during a small controlled pilot, but **every role must have an explicit named owner** before OPS-BLK-002 can close. Role labels alone (`Platform team`, `DevOps`) are insufficient.

---

## 3. Acknowledgement model (PROPOSED — not approved)

Each role requires an explicit acknowledgement record before OPS-BLK-002 closure:

| Field | Required |
|---|---|
| NAME | Named individual |
| ROLE | From minimum model |
| CONTACT_METHOD | Email / phone / Telegram / ticket queue — **do not invent** |
| ACK_STATUS | `PENDING` or `ACKNOWLEDGED` |
| ACK_TIMESTAMP | UTC ISO-8601 when acknowledged |
| APPROVED_BY | Management authority who recorded acceptance |

```text
ACK_STATUS_ALLOWED=PENDING|ACKNOWLEDGED
NO_ROLE_MAY_CLOSE_WITH_ACK_STATUS=PENDING
OWNERSHIP_ACK_COMPLETE=NO
```

### Acknowledgement register (empty — management to fill)

| Role | NAME | CONTACT_METHOD | ACK_STATUS | ACK_TIMESTAMP | APPROVED_BY |
|---|---|---|---|---|---|
| PILOT_BUSINESS_OWNER | | | PENDING | | |
| PILOT_TECHNICAL_OWNER | | | PENDING | | |
| PILOT_OPERATIONS_OWNER | | | PENDING | | |
| P1_INCIDENT_COMMANDER | | | PENDING | | |
| INFRASTRUCTURE_OWNER | | | PENDING | | |
| DATABASE_OWNER | | | PENDING | | |
| SECURITY_OWNER | | | PENDING | | |
| GO_LIVE_AUTHORITY | | | PENDING | | |

---

## 4. Support model (OPS-BLK-003) — PROPOSED

Unless existing approved values are found, propose:

| Field | PROPOSED_VALUE | APPROVAL_STATUS |
|---|---|---|
| SUPPORT_WINDOW | 09:00–18:00 MSK, Monday–Friday | PROPOSED_NOT_APPROVED |
| P1_INITIAL_ACK_TARGET | 15 minutes | PROPOSED_NOT_APPROVED |
| P2_INITIAL_ACK_TARGET | 30 minutes | PROPOSED_NOT_APPROVED |
| P3_INITIAL_ACK_TARGET | Next support window / backlog triage | PROPOSED_NOT_APPROVED |

### Incident levels (PROPOSED)

| Level | Definition |
|---|---|
| **P1** | Pilot unavailable; security/tenant-isolation failure; data integrity threat; critical business flow unavailable |
| **P2** | Major degradation; important workflow unavailable; workaround exists; pilot can continue partially |
| **P3** | Non-critical defect; UX issue; reporting inconsistency; minor operational issue |

**Current approved support values:** NOT_FOUND for BINTRANS pilot. Low-code pack documents same-business-day / next-business-day targets — treat as **LEGACY_ONLY**, not BINTRANS-approved.

---

## 5. Escalation chain (PROPOSED)

### P1

Monitoring / Operator → PILOT_OPERATIONS_OWNER → P1_INCIDENT_COMMANDER → PILOT_TECHNICAL_OWNER → PILOT_BUSINESS_OWNER / GO_LIVE_AUTHORITY

### P2

Monitoring / Operator → PILOT_OPERATIONS_OWNER → PILOT_TECHNICAL_OWNER

### P3

Support queue → backlog / responsible product owner

### Primary pilot incident channel

| Field | Value | Status |
|---|---|---|
| CHANNEL_NAME | BINTRANS Pilot Ops (Telegram group) | OPERATIONAL — alert delivery proven OPS-BLK-001 |
| CHANNEL_ROLE | ALERT_AND_INCIDENT_COORDINATION_CHANNEL | PROPOSED_NOT_APPROVED as sole support route |
| CHAT_ID | `-5081547385` | Configured in Alertmanager (protected env) |

Telegram is **not** the only durable audit log. Incidents must also be recorded in operator incident notes / ticket system when assigned.

**Links:** `docs/bintrans-pilot/BINTRANS_PILOT_INCIDENT_RUNBOOK_INDEX_V1.md`, `docs/LOW_CODE_PILOT_WEEK3_SUPPORT_ESCALATION_MATRIX_V0.1.md`

---

## 6. ACK procedure (PROPOSED — documentation only)

For an incoming alert:

1. Human sees alert (Telegram / Grafana / Prometheus).
2. Human posts ACK in coordination channel.
3. Incident owner records: timestamp, alert, owner, severity, initial action.
4. For P1/P2: incident remains owned until resolved or explicit handoff.
5. Resolution is explicitly recorded.

### Minimal ACK format

```text
ACK | <severity> | <alert/incident> | <owner> | <UTC timestamp>
```

Example (illustrative only):

```text
ACK | P1 | BintransPilotServiceDown | <owner> | 2026-09-02T21:00:00Z
```

No Telegram bot workflow is implemented in this task.

---

## 7. RPO / RTO (OPS-BLK-004) — PROPOSED

### Observed staging evidence (historical — not approval)

| Field | Value | Source |
|---|---|---|
| RESTORE_DRILL | PASS | P0 isolated drill |
| OBSERVED_TECHNICAL_RESTORE_DURATION | ~6 seconds (DB restore on disposable target) | staging restore drill |
| LATEST_BACKUP | `freight_platform_20260901T190602Z.dump` | `/protected/bintrans/backups/` |
| BACKUP_VALIDATION | PASS (`pg_restore -l`, SHA256 recorded) | backup scripts |

### Proposed pilot targets

| Field | PROPOSED_VALUE | APPROVED |
|---|---|---|
| RPO | 24 hours (daily validated logical backup) | **NO** |
| RTO | 30 minutes (committed operational target) | **NO** |

**RPO consequence:** In worst case, up to one day's changes could require reconstruction if only daily backup is available.

**RTO distinction:**

| Term | Meaning |
|---|---|
| OBSERVED_TECHNICAL_RESTORE_DURATION | Seconds-level DB restore observed in isolated drill |
| COMMITTED_OPERATIONAL_RTO | 30 minutes — allows incident recognition, operator action, backup selection, restore, verification, service recovery |

```text
RPO_APPROVED=NO
RTO_APPROVED=NO
RTO_APPROVAL_STATUS=PROPOSED_NOT_APPROVED
```

---

## 8. Backup requirements for RPO

| Field | Value |
|---|---|
| BACKUP_CURRENT_MODE | Manual operator invocation |
| BACKUP_SCRIPT | `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh` |
| BACKUP_AUTOMATED | NO — no cron/systemd schedule in repository |
| BACKUP_SCHEDULE | None automated; operator-run on demand |
| BACKUP_VALIDATION_AUTOMATED | Partial — freshness metric via `bintrans_pilot_backup_metrics.sh` after manual backup |
| BACKUP_STORAGE | `/protected/bintrans/backups/` |
| METADATA_UPDATE | `bintrans_pilot_backup_metadata_update.sh` → protected `staging.env` |

```text
RPO_IMPLEMENTATION_GAP=YES
DAILY_VALIDATED_BACKUP_REQUIRED=YES
```

**Gap:** Proposed RPO of 24h requires a **daily validated backup cadence**. Current mechanism is manual. Management may approve RPO=24h only with explicit acceptance of the gap **or** authorization to implement automated daily backup before pilot go-live.

Provider snapshot (Selectel VM) may exist separately — not substituted here without verified schedule and retention evidence.

---

## 9. Decision matrix

| DECISION_ID | TOPIC | PROPOSED_VALUE | CURRENT_STATUS | MANAGEMENT_INPUT_REQUIRED | BLOCKER |
|---|---|---|---|---|---|
| DEC-OPS-001 | Business Owner | Named individual | TBD | YES | OPS-BLK-002 |
| DEC-OPS-002 | Technical Owner | Named individual | LEGACY_ONLY label | YES | OPS-BLK-002 |
| DEC-OPS-003 | Operations Owner | Named individual | LEGACY_ONLY (low-code) | YES | OPS-BLK-002 |
| DEC-OPS-004 | P1 Incident Commander | Named individual | TBD | YES | OPS-BLK-002 |
| DEC-OPS-005 | Infrastructure Owner | Named individual | LEGACY_ONLY label | YES | OPS-BLK-002 |
| DEC-OPS-006 | Database Owner | Named individual | TBD | YES | OPS-BLK-002 |
| DEC-OPS-007 | Security Owner | Named individual | LEGACY_ONLY label | YES | OPS-BLK-002 |
| DEC-OPS-008 | Go-Live Authority | Named individual | TBD | YES | OPS-BLK-002 |
| DEC-OPS-009 | Support Window | 09:00–18:00 MSK Mon–Fri | NOT DEFINED | YES | OPS-BLK-003 |
| DEC-OPS-010 | P1 ACK target | 15 minutes | LEGACY_ONLY (same day) | YES | OPS-BLK-003 |
| DEC-OPS-011 | P2 ACK target | 30 minutes | LEGACY_ONLY (next day) | YES | OPS-BLK-003 |
| DEC-OPS-012 | Escalation channel | BINTRANS Pilot Ops (coordination) | Telegram configured; policy not approved | YES | OPS-BLK-003 |
| DEC-OPS-013 | RPO | 24h daily validated backup | Mechanism manual; gap exists | YES | OPS-BLK-004 |
| DEC-OPS-014 | RTO | 30 minutes operational | Observed restore seconds; not certified | YES | OPS-BLK-004 |

No row marked APPROVED unless explicit evidence exists. Current evidence supports **PROPOSED** or **LEGACY_ONLY** only.

---

## 10. MANAGEMENT APPROVAL REQUIRED

Controller / management: complete and return (sanitized — no secrets):

```text
PILOT_BUSINESS_OWNER=
PILOT_TECHNICAL_OWNER=
PILOT_OPERATIONS_OWNER=
P1_INCIDENT_COMMANDER=
INFRASTRUCTURE_OWNER=
DATABASE_OWNER=
SECURITY_OWNER=
GO_LIVE_AUTHORITY=

SUPPORT_WINDOW=
P1_ACK_TARGET=
P2_ACK_TARGET=
ESCALATION_CHANNEL=

RPO=
RTO=

APPROVED_BY=
APPROVAL_DATE=
```

Optional risk acceptance if RPO gap accepted without automated daily backup:

```text
RPO_GAP_ACCEPTED=YES|NO
RPO_GAP_ACCEPTED_BY=
RPO_GAP_ACCEPTED_DATE=
```

---

## 11. Blocker status summary

```text
OPS-BLK-001=CLOSED (Telegram alert lifecycle PASS — see operational decisions doc)
OPS-BLK-002=OPEN_WAITING_FOR_MANAGEMENT_APPROVAL
OPS-BLK-003=OPEN_WAITING_FOR_MANAGEMENT_APPROVAL
OPS-BLK-004=OPEN_WAITING_FOR_MANAGEMENT_APPROVAL
OPS-BLK-005=OPEN_MEDIUM (not in scope)

PILOT_OPERATIONAL_READINESS=FAIL
CONTROLLED_PILOT_GO_LIVE_RECOMMENDATION=NO_GO
REAL_USER_PILOT_ALLOWED=NO
```

**NEXT_ACTION:** CONTROLLER_MANAGEMENT_DECISION
