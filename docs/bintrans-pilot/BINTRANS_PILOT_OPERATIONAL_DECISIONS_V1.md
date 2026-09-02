# BINTRANS Pilot Operational Decisions v1

**Status:** OPEN — awaiting management input  
**Wave:** P0.1 Critical Operations Blocker Closure + Management Decision Package v0.1  
**Base SHA:** `6e4d3e22ec09c91d7d2a57e189918db15c564e69`  
**Primary management pack:** `docs/bintrans-pilot/BINTRANS_PILOT_MANAGEMENT_APPROVAL_PACK_V1.md`

Sections are labeled **APPROVED**, **PROPOSED**, **OPEN**, or **HISTORICAL**. Do not mark APPROVED without explicit evidence.

---

## Blocker register

| Blocker | Topic | Status | Notes |
|---|---|---|---|
| OPS-BLK-001 | Telegram alert delivery | **CLOSED** | FIRING + RESOLVED lifecycle PASS; human confirmation recorded |
| OPS-BLK-002 | On-call ownership | **OPEN_WAITING_FOR_MANAGEMENT_APPROVAL** | Named owners + ACK required |
| OPS-BLK-003 | Support / escalation | **OPEN_WAITING_FOR_MANAGEMENT_APPROVAL** | Window + ACK targets not approved |
| OPS-BLK-004 | RPO / RTO | **OPEN_WAITING_FOR_MANAGEMENT_APPROVAL** | Proposed 24h / 30m; not approved |
| OPS-BLK-005 | Registry / image pinning | **OPEN_MEDIUM** | Out of scope for management pack v0.1 |

```text
PILOT_OPERATIONAL_READINESS=FAIL
CONTROLLED_PILOT_GO_LIVE_RECOMMENDATION=NO_GO
REAL_USER_PILOT_ALLOWED=NO
```

---

## HISTORICAL — OPS-BLK-001 (CLOSED)

| Field | Value |
|---|---|
| **Status** | **CLOSED** |
| **Final verdict** | PASS |
| **Synthetic test** | `telegram-clean-lifecycle-20260902T210510Z` |
| **Root cause** | Upstream partial Telegram IP blocking (default DNS A-record blocked) |
| **Remediation** | Alertmanager `extra_hosts` DNS override to reachable Telegram DC (staging compose) |
| **Human gates** | FIRING confirmation YES; RESOLVED confirmation recorded by controller |
| **Closure date** | 2026-09-02 (controller state) |

Do not reopen OPS-BLK-001 unless Telegram delivery regresses.

---

## DECISION-001 — Critical ownership assignments (OPS-BLK-002)

| Field | Value |
|---|---|
| **Decision** | Assign named accountable individuals for pilot-critical roles |
| **Status** | **OPEN_WAITING_FOR_MANAGEMENT_APPROVAL** |
| **Approver role** | Management / GO_LIVE_AUTHORITY |

See ownership inventory and acknowledgement register in `BINTRANS_PILOT_MANAGEMENT_APPROVAL_PACK_V1.md` §1–3.

| Role | CURRENT | APPROVED |
|---|---|---|
| PILOT_BUSINESS_OWNER | TBD | NO |
| PILOT_TECHNICAL_OWNER | Role label only (LEGACY_ONLY) | NO |
| PILOT_OPERATIONS_OWNER | Low-code legacy reference (LEGACY_ONLY) | NO |
| P1_INCIDENT_COMMANDER | TBD | NO |
| INFRASTRUCTURE_OWNER | Role label only (LEGACY_ONLY) | NO |
| DATABASE_OWNER | TBD | NO |
| SECURITY_OWNER | Role label only (LEGACY_ONLY) | NO |
| GO_LIVE_AUTHORITY | TBD | NO |

---

## DECISION-002 — Support / escalation channel (OPS-BLK-003)

| Field | Value |
|---|---|
| **Decision** | Approve primary support and escalation route |
| **Status** | **OPEN_WAITING_FOR_MANAGEMENT_APPROVAL** |
| **Approver role** | PILOT_BUSINESS_OWNER + PILOT_OPERATIONS_OWNER |

| Field | CURRENT | PROPOSED | APPROVED |
|---|---|---|---|
| PRIMARY_SUPPORT_CHANNEL | NOT_PROVIDED | BINTRANS Pilot Ops (coordination) | NO |
| ESCALATION_CHANNEL | NOT_PROVIDED | Per escalation chain in management pack | NO |
| CHANNEL_ROLE | Telegram alert delivery active | ALERT_AND_INCIDENT_COORDINATION_CHANNEL | NO |

---

## DECISION-003 — Support window / SLA (OPS-BLK-003)

| Field | Value |
|---|---|
| **Decision** | Approve pilot support window and P1/P2 response targets |
| **Status** | **OPEN_WAITING_FOR_MANAGEMENT_APPROVAL** |
| **Approver role** | PILOT_BUSINESS_OWNER |

| Field | CURRENT | PROPOSED | APPROVED |
|---|---|---|---|
| PILOT_SUPPORT_WINDOW | NOT DEFINED | 09:00–18:00 MSK, Mon–Fri | NO |
| P1_INITIAL_ACK_TARGET | LEGACY_ONLY | 15 minutes | NO |
| P2_INITIAL_ACK_TARGET | LEGACY_ONLY | 30 minutes | NO |
| P3_INITIAL_ACK_TARGET | — | Next support window / backlog | NO |

---

## DECISION-004 — Pilot RPO (OPS-BLK-004)

| Field | Value |
|---|---|
| **Decision** | Approve recovery point objective |
| **Status** | **OPEN_WAITING_FOR_MANAGEMENT_APPROVAL** |
| **Approver role** | GO_LIVE_AUTHORITY |

| Field | Value |
|---|---|
| RPO_PROPOSED | 24 hours (daily validated logical backup) |
| RPO_APPROVED | **NO** |
| BACKUP_CURRENT_MODE | Manual operator invocation |
| RPO_IMPLEMENTATION_GAP | **YES** — automated daily backup not active |

See `BINTRANS_PILOT_RPO_RTO_APPROVAL_PACK_V1.md`.

---

## DECISION-005 — Pilot RTO (OPS-BLK-004)

| Field | Value |
|---|---|
| **Decision** | Approve recovery time objective |
| **Status** | **OPEN_WAITING_FOR_MANAGEMENT_APPROVAL** |
| **Approver role** | GO_LIVE_AUTHORITY |

| Field | Value |
|---|---|
| RTO_PROPOSED | 30 minutes (committed operational target) |
| RTO_APPROVED | **NO** |
| OBSERVED_TECHNICAL_RESTORE_DURATION | ~6 seconds (isolated DB restore drill — **not** operational RTO) |
| RTO_APPROVAL_STATUS | PROPOSED_NOT_APPROVED |

---

## DECISION-006 — Local-image acceptance / registry publish (OPS-BLK-005)

| Field | Value |
|---|---|
| **Status** | **OPEN_MEDIUM** — not addressed in management pack v0.1 |
| **Approver role** | PILOT_OPERATIONS_OWNER + GO_LIVE_AUTHORITY |

No change in this task.

---

## Supporting documents

| Document | Purpose |
|---|---|
| `BINTRANS_PILOT_MANAGEMENT_APPROVAL_PACK_V1.md` | Decision matrix, fill-in block, escalation, ACK model |
| `BINTRANS_PILOT_OWNERSHIP_APPROVAL_PACK_V1.md` | Ownership summary |
| `BINTRANS_PILOT_SUPPORT_MODEL_PACK_V1.md` | Support targets summary |
| `BINTRANS_PILOT_RPO_RTO_APPROVAL_PACK_V1.md` | RPO/RTO evidence summary |
| `BINTRANS_PILOT_INCIDENT_RUNBOOK_INDEX_V1.md` | Incident navigation |
| `BINTRANS_PILOT_LOGGING_BASELINE_V1.md` | Logging baseline |

---

```text
MANAGEMENT_ACTION_REQUIRED=YES
NEXT_ACTION=CONTROLLER_MANAGEMENT_DECISION
```
