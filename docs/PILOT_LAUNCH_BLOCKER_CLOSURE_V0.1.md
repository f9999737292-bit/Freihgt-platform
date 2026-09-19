# Pilot Launch Blocker Closure v0.1

**Task:** Close operational launch blockers from `PILOT_OPERATIONAL_READINESS_AND_HANDOFF_V0.1.md`  
**Branch:** `ops/pilot-launch-blocker-closure-v0.1`  
**Base:** `origin/main` @ `234c8b78d198e1a694757be20fb5e53b32dd77ad`  
**Date:** 2026-08-14  
**Pilot launch executed:** NO  
**Production mutation:** NO  

---

## Git Preflight

| Field | Value |
| --- | --- |
| LOCAL_HEAD (worktree) | `234c8b78d198e1a694757be20fb5e53b32dd77ad` |
| ORIGIN_MAIN_SHA_AT_START | `234c8b78d198e1a694757be20fb5e53b32dd77ad` |
| MAIN_WORKTREE_STATUS | dirty (unrelated changes — not touched) |
| Isolated worktree | `D:\Projects\freight-platform-pilot-blocker-closure` |

---

## Previous State

```text
TECHNICAL_PILOT_GATE=PASS
SECURITY_GATE=PASS
OPERATIONAL_READINESS=CONDITIONAL_PASS
GO_LIVE_RECOMMENDATION=GO_WITH_CONDITIONS
```

---

## Launch Blocker Summary

| Blocker | Before | After | Evidence | Remaining Action |
| --- | --- | --- | --- | --- |
| BLOCKER-001 ALERT_ROUTING | NOT_CONFIGURED | **BLOCKED** | `PILOT_INTERIM_ALERTING_V0.1.md`; staging Prometheus/Grafana available; no Alertmanager | Assign P1/P2 owners + escalation channel; activate interim manual mode |
| BLOCKER-002 RESTORE_DRILL | NOT_VERIFIED | **FAIL** | Isolated drill on disposable PG; backup structurally valid but **empty** (0 schemas) | Generate fresh verified backup from live DB (authorized ops); re-run isolated restore drill |
| BLOCKER-003 ON_CALL_OWNERSHIP | TBD | **BLOCKED** | `PILOT_ON_CALL_ASSIGNMENT_V0.1.md`; partial legacy names only | Complete role assignment + contact channels + ACK |
| BLOCKER-004 PILOT_RELEASE_PINNING | NOT_DEFINED | **PASS** | `PILOT_RELEASE_MANIFEST_V0.1.md`; digests from staging VM | Formal approvals (TBD) |

---

## BLOCKER-001 — Alert Routing

### Current Infrastructure

| Component | Status |
| --- | --- |
| Prometheus | AVAILABLE (`127.0.0.1:9090` on staging VM) |
| Grafana | AVAILABLE (`127.0.0.1:3001` on staging VM) |
| Alertmanager | **MISSING** (not in repo compose / staging stack) |
| Automated routing | NOT_CONFIGURED |

### Decision

Interim manual alerting **documented but not activatable** — P1/P2 owners and escalation channel are TBD.

```text
ALERT_ROUTING=BLOCKED
ALERTING_MODE=INTERIM_MANUAL (PROPOSED — NOT ACTIVE)
BLOCKER_REASON=BLOCKED_OWNERSHIP
```

See `docs/PILOT_INTERIM_ALERTING_V0.1.md` for proposed thresholds and check cadence.

---

## BLOCKER-002 — Restore Drill

### Backup Discovery

| Field | Value |
| --- | --- |
| BACKUP_SCRIPT | `scripts/ops/backup_postgres.sh` (documented in staging ops) |
| BACKUP_FILE | `/protected/bintrans/backups/freight_platform_20260811T083942Z.dump` |
| BACKUP_FORMAT | PostgreSQL custom (PGDMP) |
| BACKUP_DATE | 2026-08-11T08:39:42Z |
| BACKUP_SIZE | 913 bytes |
| BACKUP_SHA256 | `c04d993fedc70b9627b773a367f0a62872fd6feed6ccce7990793bd7e66c6c9b` |
| BACKUP_READABLE | YES (pg_restore -l succeeds) |
| BACKUP_USABLE_FOR_RECOVERY | **NO** |

### Isolated Restore Drill

Executed via `scripts/ops/pilot_restore_drill_isolated.sh` on dedicated staging VM:

| Step | Result |
| --- | --- |
| Target | Disposable container `pilot-restore-drill-pg` (port 55432) |
| Live staging DB impact | **NONE** |
| RESTORE_COMMAND | Completed (exit may warn) |
| DATABASE_CONNECTIVITY | PASS (disposable PG) |
| SCHEMA_PRESENT | **FAIL** (`SCHEMA_COUNT=0`) |
| CRITICAL_TABLES_PRESENT | **FAIL** (`schema_migrations` absent) |
| SELECT_VALIDATION | N/A (no tables) |
| DATA_SAMPLE_VALIDATION | **FAIL** (projection count 0) |

```text
RESTORE_DRILL=FAIL
RESTORE_COMPLETED=YES (command ran; content empty)
BACKUP_VERIFIED_STRUCTURALLY=YES
BACKUP_VERIFIED_OPERATIONALLY=NO
LIVE_STAGING_IMPACT=NONE
```

**Critical finding:** Previous `BACKUP_VERIFIED=YES` flag does not reflect recoverability. A 913-byte dump with zero application schemas cannot satisfy Pilot RPO/RTO.

### Proposed RPO/RTO (Not Approved)

| Field | Value |
| --- | --- |
| PROPOSED_PILOT_RPO | Daily backup cadence (once fresh backup verified) |
| PROPOSED_PILOT_RTO | TBD after successful restore drill with non-empty backup |
| OBSERVED_RESTORE_DURATION | Sub-minute (disposable target; empty content) |

---

## BLOCKER-003 — On-Call Ownership

Partial names exist in legacy low-code Pilot docs only:

- **Артем Асаев** — rollback/release owner (contact not provided)
- **Феликс Асаев** — business/PM/go-no-go (contact not provided)

Required roles for Control Tower Pilot remain largely unassigned:

```text
ON_CALL_OWNERSHIP=BLOCKED
P1_INCIDENT_COMMANDER=TBD
PILOT_OPERATIONS_OWNER=TBD
INFRASTRUCTURE_OWNER=TBD
DATABASE_OWNER=TBD
SECURITY_OWNER=TBD
ESCALATION_CHANNEL=TBD
PILOT_SUPPORT_WINDOW=TBD
```

See `docs/PILOT_ON_CALL_ASSIGNMENT_V0.1.md`.

---

## BLOCKER-004 — Pilot Release Pinning

### Version Analysis (`b75eb3d` vs `234c8b78`)

| Category | Changes |
| --- | --- |
| SECURITY | No regression — baseline verified at `b75eb3d` on Selectel |
| DATA_INTEGRITY | No mandatory delta |
| RUNTIME_CRITICAL | None identified |
| CONTROL_TOWER_PILOT_CRITICAL | Alert-ack features on main — **non-mandatory** for current shadow Pilot |
| NON_CRITICAL | Alert acknowledgement UI/backend |
| DOCS_TESTS_ONLY | Verification docs, parallel engineering |

### Decision

```text
PILOT_RELEASE_ID=CT-PILOT-2026-08-14-b75eb3d
PILOT_GIT_SHA=b75eb3d
VERSION_DECISION=PIN_VERIFIED_STAGING (Option A)
PILOT_RELEASE_PINNING=PASS
IMAGE_DIGEST_PINNING=YES
```

Full manifest: `docs/PILOT_RELEASE_MANIFEST_V0.1.md`

Rollback release = same (`b75eb3d`) — first verified known-good on dedicated VM; no prior documented release.

---

## Non-Blocking Conditions

| ID | Status | Closure |
| --- | --- | --- |
| COND-001 RBAC deny staging identity | Open | BEFORE_SCALE_UP |
| COND-002 Event timeline data | Open | FIRST_REAL_EVENT_HISTORY_OR_DISPOSABLE_FIXTURE |
| COND-003 SHA alignment | **Closed** | NOT_REQUIRED_FOR_CURRENT_PILOT (`b75eb3d` formally pinned) |

---

## Alerting Evidence

```text
ALERTING_MODE=INTERIM_MANUAL (PROPOSED)
MONITORING_DASHBOARD=Grafana + Prometheus + health endpoints
CHECK_CADENCE=PROPOSED (see PILOT_INTERIM_ALERTING_V0.1.md)
P1_THRESHOLD_SET=PROPOSED
P2_THRESHOLD_SET=PROPOSED
OWNER=TBD
ESCALATION=TBD
EXPIRY=BEFORE_SCALE_UP_OR_AUTOMATED_ALERTING
```

---

## Safety Attestation

```text
PRODUCTION_MUTATION=NO
STAGING_BUSINESS_DATA_MUTATION=NO
PRODUCTION_DEPLOY=NO
PILOT_LAUNCH=NO
LIVE_DB_RESTORE=NO
SERVICE_RESTART=NO
SECRET_CHANGE=NO
FORCE_PUSH=NO
PRODUCT_CODE_MODIFIED=NO (docs + ops script only)
```

---

## Final Verdict

```text
ALERT_ROUTING=BLOCKED
RESTORE_DRILL=FAIL
ON_CALL_OWNERSHIP=BLOCKED
PILOT_RELEASE_PINNING=PASS

LAUNCH_BLOCKERS=ALERT_ROUTING, RESTORE_DRILL, ON_CALL_OWNERSHIP

OPERATIONAL_READINESS=FAIL
GO_LIVE_RECOMMENDATION=NO_GO
```

**Rationale:** Unusable backup is a serious operational defect (§48). Alert routing and on-call ownership lack required assignees/channels. Release pinning alone is insufficient for GO.

---

## Next Recommended Step

1. **Authorized ops:** Run fresh PostgreSQL backup on staging; verify size > threshold and non-zero schema count; re-run `scripts/ops/pilot_restore_drill_isolated.sh`.
2. **Management:** Complete `PILOT_ON_CALL_ASSIGNMENT_V0.1.md` with contacts and ACK.
3. **Ops lead:** Activate interim manual alerting per `PILOT_INTERIM_ALERTING_V0.1.md`.
4. After blockers closed: proceed to **CONTROLLED PILOT LAUNCH PLAN v0.1** (separate task).

**Do not launch Pilot from this task.**

---

## Related Artifacts

| File | Purpose |
| --- | --- |
| `docs/PILOT_RELEASE_MANIFEST_V0.1.md` | Formal release + digest pinning |
| `docs/PILOT_ON_CALL_ASSIGNMENT_V0.1.md` | Owner assignment sheet (incomplete) |
| `docs/PILOT_INTERIM_ALERTING_V0.1.md` | Interim manual alerting (incomplete) |
| `docs/PILOT_RUNBOOK_V0.1.md` | Updated runbook references |
| `scripts/ops/pilot_restore_drill_isolated.sh` | Safe isolated restore drill script |
