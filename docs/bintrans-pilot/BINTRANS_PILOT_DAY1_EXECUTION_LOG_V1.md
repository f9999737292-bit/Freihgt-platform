# BINTRANS Pilot Day-1 Real-User Execution Log v1

**Status:** TEMPLATE — pre-session initialization only  
**Wave:** Controlled Real-User Pilot — Day-1 Session Preparation v1.0  
**Baseline SHA:** `8524ceffa563d94e25a35cad9da20fe25260f120`  
**Staging host:** `161.104.57.152` (operator SSH + local tunnel only)

Do **not** mark any step **PASS** until real humans execute it during the controlled pilot session.

---

## Session header

| Field | Value |
|---|---|
| PILOT_SESSION_ID | PENDING |
| SESSION_DATE | PENDING |
| START_TIME | PENDING |
| END_TIME | PENDING |
| PILOT_SESSION_STATUS | NOT_STARTED |

### Participants (assign before execution)

| Role | Participant |
|---|---|
| SHIPPER_PARTICIPANT | PENDING |
| CARRIER_PARTICIPANT | PENDING |
| DRIVER_PARTICIPANT | PENDING |
| OPERATOR | PENDING |

### Authorization context

```text
PILOT_STATUS=LIVE_CONTROLLED
REAL_USER_PILOT_ALLOWED=YES
CONTROLLED_REAL_USER_PILOT=AUTHORIZED
PRODUCTION_GO_LIVE=NOT_AUTHORIZED
BROAD_CUSTOMER_ROLLOUT=NOT_AUTHORIZED
```

Support window: **09:00–18:00 MSK, Monday–Friday**  
P1 ack target: **15 minutes** · P2 ack target: **30 minutes**  
Escalation channel: **BINTRANS Pilot Ops**

---

## Day-1 precheck summary (2026-09-03)

Read-only agent precheck before first real-user session. No business transactions executed.

| Gate | Result |
|---|---|
| PILOT_DAY1_PRECHECK | **PASS** |
| STAGING_MUTATION | NO |
| BUSINESS_TRANSACTION_CREATED_BY_AGENT | NO |

### Platform

| Check | Result |
|---|---|
| CRITICAL_UNHEALTHY_COUNT | 0 |
| CRITICAL_RESTARTING_COUNT | 0 |
| API_GATEWAY_HEALTH | PASS |
| WEB_ADMIN_LOGIN_HTTP | 200 |
| POSTGRES_RUNNING | YES |
| POSTGRES_HEALTHY | YES |
| DB_MIGRATION_VERSION | 64 |
| DB_DIRTY | false |

### Operations

| Check | Result |
|---|---|
| BACKUP_TIMER_ENABLED | YES |
| BACKUP_TIMER_ACTIVE | YES |
| LATEST_BACKUP_VERIFIED | YES |
| BACKUP_STALE_ALERT_FIRING | NO |
| PROMETHEUS_HEALTH | PASS |
| ALERTMANAGER_HEALTH | PASS |
| GRAFANA_HEALTH | PASS |

### Images

| Check | Result |
|---|---|
| RUNNING_APP_SERVICE_COUNT | 14 |
| REGISTRY_DIGEST_PINNED_SERVICE_COUNT | 14 |
| REGISTRY_ACCESS_MODE | PULL_ONLY |
| OVERALL_REPRODUCIBLE_WITHOUT_LOCAL_CACHE | YES |

### Pilot account inventory (read-only)

Role codes in staging identity model use domain-specific names (`SHIPPER_ADMIN`, `CARRIER_ADMIN`, etc.). Minimum Day-1 role codes checked literally:

| Required role code | ACCOUNT_AVAILABLE | Notes |
|---|---|---|
| SHIPPER | NO | Functional proxy: `SHIPPER_ADMIN` (9 active users); Wave2 fixture buyer is `SHIPPER_ADMIN` / ACTIVE |
| CARRIER | NO | Functional proxy: `CARRIER_ADMIN` (10 active users); Wave2 fixture carrier user **invalid / missing roles** |
| DRIVER | YES | 10 active `DRIVER` users; pilot driver account present |
| OPERATOR | NO | No `OPERATOR` role code; `FORWARDER_MANAGER` (1 active user) may serve operator-assisted verification only after controller assignment |

```text
PILOT_ACCOUNT_GAP=YES
MISSING_ACCOUNT_ROLES=SHIPPER,CARRIER,OPERATOR
```

**Controller action before Day-1:** assign real participants, confirm or provision accounts mapped to shipper/carrier/operator flows. Do **not** treat technical test fixtures as real pilot participants automatically.

---

## Human execution sequence

Operator records evidence during the session. Agent pre-session state is **NOT_RUN** for all steps.

### Step 1 — Shipper login

Human shipper: login via web-admin (operator tunnel).

| Evidence | Status |
|---|---|
| SHIPPER_LOGIN | NOT_RUN |

---

### Step 2 — RFx creation

Human shipper: create RFx, enter required commercial/transport data, save, publish.

| Evidence | Status |
|---|---|
| RFX_CREATE | NOT_RUN |
| RFX_PUBLISH | NOT_RUN |

Capture during session:

| Field | Value |
|---|---|
| RFX_ID | |
| RFX_NUMBER | |

---

### Step 3 — Carrier visibility

Human carrier: login, open tender workspace, find published RFx.

| Evidence | Status |
|---|---|
| CARRIER_LOGIN | NOT_RUN |
| RFX_VISIBLE_TO_CARRIER | NOT_RUN |

---

### Step 4 — Bid

Human carrier: create bid, enter price/terms, submit.

| Evidence | Status |
|---|---|
| BID_CREATE | NOT_RUN |
| BID_SUBMIT | NOT_RUN |

Capture during session:

| Field | Value |
|---|---|
| BID_ID | |

---

### Step 5 — Buyer evaluation

Human shipper: open submitted bid, evaluate, award.

| Evidence | Status |
|---|---|
| BUYER_BID_VISIBLE | NOT_RUN |
| EVALUATION | NOT_RUN |
| AWARD | NOT_RUN |

---

### Step 6 — Execution creation

Observe resulting transport order and shipment.

| Evidence | Status |
|---|---|
| TRANSPORT_ORDER_CREATED | NOT_RUN |
| SHIPMENT_CREATED | NOT_RUN |

Capture during session:

| Field | Value |
|---|---|
| TRANSPORT_ORDER_ID | |
| SHIPMENT_ID | |

---

### Step 7 — Carrier execution

Human carrier/operator: accept, assign vehicle, assign driver.

| Evidence | Status |
|---|---|
| CARRIER_ACCEPT | NOT_RUN |
| VEHICLE_ASSIGN | NOT_RUN |
| DRIVER_ASSIGN | NOT_RUN |

---

### Step 8 — Driver execution

Real driver or approved pilot operator acting through driver flow: start execution, progress to IN_TRANSIT.

| Evidence | Status |
|---|---|
| DRIVER_EXECUTION | NOT_RUN |
| IN_TRANSIT | NOT_RUN |

Do **not** fake driver activity through direct DB/API manipulation.

---

### Step 9 — Event history

Operator verifies real shipment: `/shipments/<SHIPMENT_ID>/events`

| Evidence | Status |
|---|---|
| EVENT_HISTORY | NOT_RUN |
| EVENT_HISTORY_COUNT | |
| EVENT_TIMELINE_CORRECT | |

---

### Step 10 — Control Tower

Operator verifies same shipment in Control Tower.

| Evidence | Status |
|---|---|
| CONTROL_TOWER_VISIBILITY | NOT_RUN |
| CONTROL_TOWER_SHIPMENT_MATCH | |

---

## Day-1 success gate

Future session **PASS** requires all of the following marked **PASS** by human execution:

- SHIPPER_LOGIN, RFX_CREATE, RFX_PUBLISH
- CARRIER_LOGIN, RFX_VISIBLE_TO_CARRIER, BID_CREATE, BID_SUBMIT
- BUYER_BID_VISIBLE, EVALUATION, AWARD
- TRANSPORT_ORDER_CREATED, SHIPMENT_CREATED
- CARRIER_ACCEPT, VEHICLE_ASSIGN, DRIVER_ASSIGN
- IN_TRANSIT
- EVENT_HISTORY, CONTROL_TOWER_VISIBILITY

And:

```text
TENANT_ISOLATION_INCIDENTS=0
DATA_INTEGRITY_INCIDENTS=0
P1_INCIDENTS=0
```

P2/P3 may exist only if recorded and triaged.

---

## Incident classification

Use severity **P1 / P2 / P3** per `BINTRANS_PILOT_INCIDENT_RUNBOOK_INDEX_V1.md`.

### P1 examples

- Tenant isolation failure
- Data leakage
- Authentication bypass
- Data integrity threat
- Entire pilot unavailable
- Critical business flow impossible

**If P1 occurs:** do **not** silently repair. Set `PILOT_SESSION_STATUS=STOPPED_P1`. Notify **BINTRANS Pilot Ops**.

Escalation chain: **Люба → Марина → Феликс**

### P2 examples

- Significant feature failure
- Important workflow requires workaround
- Partial service degradation

### P3 examples

- UX issue
- Text/layout issue
- Minor reporting inconsistency
- Non-blocking inconvenience

### Incident log (session)

| Time (MSK) | Severity | Summary | Status | Owner |
|---|---|---|---|---|
| | | | | |

---

## Session close-out (fill after execution)

| Field | Value |
|---|---|
| REAL_USER_BUSINESS_FLOW_EXECUTED | NO |
| DAY1_SESSION_RESULT | PENDING |
| TENANT_ISOLATION_INCIDENTS | |
| DATA_INTEGRITY_INCIDENTS | |
| P1_INCIDENTS | |
| P2_INCIDENTS | |
| P3_INCIDENTS | |
| OPERATOR_NOTES | |

---

## References

- `BINTRANS_PILOT_GO_LIVE_AUTHORIZATION_V1.md`
- `BINTRANS_PILOT_INCIDENT_RUNBOOK_INDEX_V1.md`
- `BINTRANS_PILOT_OPERATIONAL_DECISIONS_V1.md`
- `docs/BINTRANS_DEDICATED_CONTROL_TOWER_STAGING_RUNBOOK.md`
