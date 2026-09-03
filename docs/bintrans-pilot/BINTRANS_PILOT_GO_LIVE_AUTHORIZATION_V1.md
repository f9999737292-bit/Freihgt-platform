# BINTRANS Pilot Go-Live Authorization v1

**Status:** ACTIVE — controlled real-user pilot authorized
**Wave:** Controlled Real-User Pilot Go-Live Record v1.0
**Last updated:** 2026-09-03

This document records the final management/controller authorization for the **controlled BINTRANS real-user pilot**. It does **not** authorize production rollout or broad commercial launch.

---

## 1. Go-live authority decision

| Field | Value |
|---|---|
| GO_LIVE_AUTHORITY | Феликс |
| GO_LIVE_DECISION | **GO** |
| GO_LIVE_DECISION_DATE | 2026-09-03 |
| GO_LIVE_DECISION_SOURCE | CONTROLLER_CHAT |
| CONTROLLER_FINAL_PILOT_READINESS_GATE | **PASS** |

---

## 2. Readiness gates

| Gate | Status |
|---|---|
| PILOT_TECHNICAL_READINESS | **PASS** |
| PILOT_OPERATIONAL_READINESS | **PASS** |
| PILOT_SECURITY_READINESS | **PASS** |
| CONTROLLED_PILOT_GO_LIVE_RECOMMENDATION | **GO** |
| REAL_USER_PILOT_ALLOWED | **YES** |
| CONTROLLED_REAL_USER_PILOT | **AUTHORIZED** |

```text
PILOT_STATUS=LIVE_CONTROLLED
```

---

## 3. Explicit non-authorizations

```text
PRODUCTION_GO_LIVE=NOT_AUTHORIZED
BROAD_CUSTOMER_ROLLOUT=NOT_AUTHORIZED
```

---

## 4. Verified pilot baseline (2026-09-03)

| Field | Value |
|---|---|
| MAIN_SHA | `fbc73aaac188d36c3561b777fc6145583b6d52d1` |
| OPS_BLK_001 | CLOSED |
| OPS_BLK_002 | CLOSED |
| OPS_BLK_003 | CLOSED |
| OPS_BLK_004 | CLOSED |
| OPS_BLK_005 | CLOSED |
| CRITICAL_BLOCKERS_REMAINING | 0 |
| HIGH_BLOCKERS_REMAINING | 0 |
| MEDIUM_BLOCKERS_REMAINING | 0 |
| REGISTRY_DIGEST_PINNED_SERVICE_COUNT | 14 |
| LOCAL_ONLY_SERVICE_COUNT | 0 |
| REGISTRY_ACCESS_ON_STAGING | PULL_ONLY |
| TEMPORARY_WRITE_TOKEN_REMOVED | YES |
| DB_MIGRATION_VERSION | 64 |
| DB_DIRTY | false |
| BACKUP_AUTOMATION | ACTIVE |
| RPO_TARGET | 24h |
| RPO_OPERATIONALLY_SATISFIED | YES |
| RTO_TARGET | 30m |
| RTO_APPROVED | YES |
| OWNER_ACK_COMPLETE | YES |
| ALERTING_GATE | PASS |

Evidence references: `BINTRANS_PILOT_OPS_BLK005_CLOSURE_EVIDENCE_V1.md`, `BINTRANS_PILOT_MANAGEMENT_APPROVAL_PACK_V1.md`

---

## 5. Pilot operating conditions

| Field | Value |
|---|---|
| SUPPORT_WINDOW | 09:00–18:00 MSK, Monday–Friday |
| P1_ACK_TARGET | 15 minutes |
| P2_ACK_TARGET | 30 minutes |
| ESCALATION_CHANNEL | BINTRANS Pilot Ops |

| Role | Assigned |
|---|---|
| PILOT_BUSINESS_OWNER | Феликс |
| PILOT_TECHNICAL_OWNER | Марина |
| PILOT_OPERATIONS_OWNER | Люба |
| P1_INCIDENT_COMMANDER | Люба |
| INFRASTRUCTURE_OWNER | Люба |
| DATABASE_OWNER | Люба |
| SECURITY_OWNER | Марина |
| GO_LIVE_AUTHORITY | Феликс |

---

## 6. Scope boundary

### Authorized by GO

- Controlled real-user testing
- Current BINTRANS staging environment (`161.104.57.152`)
- Limited pilot user cohort
- Operator-assisted support
- Approved RFx → execution → shipment → Control Tower flow
- Approved operational monitoring, backup, and support model

### Not authorized by GO

- Production rollout
- Unrestricted customer onboarding
- Broad commercial launch
- Uncontrolled scaling
- Removal of monitoring or backup requirements
- Bypass of change control
- Use of unapproved new product modules

---

## 7. Active canonical state

```text
PILOT_OPERATIONAL_READINESS=PASS
CONTROLLED_PILOT_GO_LIVE_RECOMMENDATION=GO
REAL_USER_PILOT_ALLOWED=YES
CONTROLLED_REAL_USER_PILOT=AUTHORIZED
PILOT_STATUS=LIVE_CONTROLLED
```

**NEXT_ACTION:** BEGIN_CONTROLLED_REAL_USER_PILOT
