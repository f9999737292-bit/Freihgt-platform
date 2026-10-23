# Pilot Interim Alerting v0.1

**Status:** INCOMPLETE — launch blocker **BLOCKER-001** not closed.

Alertmanager is **not deployed** on dedicated Control Tower staging VM. Automated alert routing is unavailable without separate approved deployment.

---

## Mode

```text
INTERIM_ALERTING_MODE=MANUAL_ACTIVE_MONITORING (PROPOSED — NOT ACTIVATED)
ALERTING_MODE=INTERIM_MANUAL
```

Activation requires assigned owners and escalation channel (see `PILOT_ON_CALL_ASSIGNMENT_V0.1.md`).

---

## Monitoring Sources

| Source | Location | Status |
| --- | --- | --- |
| Prometheus | `http://127.0.0.1:9090` (VM localhost via SSH) | AVAILABLE |
| Grafana | `http://127.0.0.1:3001` (VM localhost via SSH) | AVAILABLE |
| Health endpoints | Gateway `/health`, `/ready` | AVAILABLE |
| Container status | `docker ps` | AVAILABLE |
| Gateway metrics | `/metrics` | AVAILABLE |

---

## Proposed Manual Check Cadence

| Phase | Cadence |
| --- | --- |
| Before go-live | Every 15 min for first 2h after any deploy |
| Pilot Day 1 | +15m, +30m, +1h, +2h, +4h, EOD |
| Pilot Day 2–3 | Every 4h during support window |
| **EXPIRY** | Before Pilot scale-up OR before automated Alertmanager deployment |

---

## Proposed Thresholds (Not Approved)

| Condition | Threshold | Severity |
| --- | --- | --- |
| Disk warning | ≥ 80% | P3 |
| Disk critical | ≥ 90% | P1/P2 |
| Health fail | 2 consecutive failures | P1/P2 |
| Readiness fail | 2 consecutive failures | P2 |
| Container unhealthy | Any pilot-critical unhealthy | P2 |
| Container restart | Unexpected restart | P2 investigate |
| Gateway 5xx | > 2% over 5m warning; > 5% critical | P2/P1 |
| Control Tower summary fail | Non-200 authenticated probe | P2 |
| Auth/login outage | Login fail for test identity | P1/P2 |
| PostgreSQL unavailable | `pg_isready` fail | P1 |

---

## Ownership (Required for PASS — Not Met)

| Field | Value |
| --- | --- |
| P1_OWNER_ROLE | TBD |
| P2_OWNER_ROLE | TBD |
| P1_OWNER | TBD |
| P2_OWNER | TBD |
| ESCALATION_CHANNEL | **TBD** |
| ACK_REQUIREMENT | Manual log entry in daily Pilot report |

---

## Verdict

```text
ALERT_ROUTING=BLOCKED
BLOCKER=ESCALATION_CHANNEL_AND_OWNER_NOT_DEFINED
```

**Remaining action:** Assign P1/P2 owners + escalation channel, then mark interim manual mode ACTIVE.
