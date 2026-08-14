# Pilot Operational Readiness & Handoff v0.1

## Executive Result

| Field | Value |
| --- | --- |
| TECHNICAL_PILOT_GATE | **PASS** |
| SECURITY_GATE | **PASS** |
| OPERATIONAL_READINESS | **CONDITIONAL_PASS** |
| GO_LIVE_RECOMMENDATION | **GO_WITH_CONDITIONS** |
| ASSESSMENT_DATE | 2026-08-14 |
| BASE_SHA | 234c8b78d198e1a694757be20fb5e53b32dd77ad |

**Summary:** Technical and security verification gates are closed. Operational controls exist in repository and on dedicated Selectel staging VM, but **alert routing, restore test verification, formal on-call assignment, and Pilot release pinning** remain open. Controlled Pilot launch is recommended **with documented conditions** — not unrestricted GO.

**Companion:** `docs/PILOT_RUNBOOK_V0.1.md`

---

## Previous Gates (Closed)

| Gate | Result | Evidence |
| --- | --- | --- |
| LOCAL_VERIFICATION | PASS | Makefile health-check, local Docker |
| STAGING_SHADOW_VERIFICATION | PASS | Shadow observation baseline on dedicated VM |
| UI_E2E | PASS | 13/13 — `CONTROL_TOWER_UI_E2E_VERIFICATION_V0.1.md` |
| ACTUAL_SELECTEL_STAGING | PASS | `ACTUAL_SELECTEL_STAGING_VERIFICATION_V0.1.md` (closeout) |
| CONTROL_TOWER_REAL_DATA | PASS | Staging API: `fallbackUsed=false`, `hasDemoIds=false` |
| SECURITY_GATE | PASS | No auth/RBAC bypass, no cross-tenant leak |

---

## Pilot Topology

```text
Pilot Client (browser / controlled entry — TBD for production Pilot URL)
        |
        v
Frontend (web-admin) — local UI E2E verified; not on dedicated API VM
        |
        v
API Gateway (127.0.0.1:18080 on dedicated VM — NOT publicly exposed by design)
        |
        +--> Identity Service
        +--> Company Service
        +--> Transport Order Service
        +--> RFx Service
        +--> Shipment Service
        +--> Document Service
        +--> Billing Register Service
        +--> Low-code Service
        +--> Control Tower BFF (shadow read-model)
                  |
                  +--> Shipment / read-model / Kafka consumer
        |
        v
PostgreSQL + Redpanda (internal only)
        |
        v
Observability: Prometheus (127.0.0.1:9090) + Grafana (127.0.0.1:3001)
```

### Selectel Targets

| Host | Role | Pilot primary? |
| --- | --- | --- |
| `161.104.53.221` | LEGACY_SHARED_VPS (`gpt-docker`) | **NO** |
| `161.104.57.152` | ACTUAL_CONTROL_TOWER_STAGING_VM | **YES** (API/runtime evidence) |

| Field | Value |
| --- | --- |
| GATEWAY_BIND | `127.0.0.1:18080` |
| PUBLIC_DIRECT_GATEWAY | NO_BY_DESIGN |
| VERIFICATION_PATH | SSH + LOCALHOST API |
| DEDICATED_VM_PUBLIC_HTTP | NOT_EXPOSED_BY_DESIGN (not a regression) |

---

## System Inventory (Pilot-Critical)

| Component | Role | Pilot-critical | Health | Observability | Owner | Rollback unit |
| --- | --- | --- | --- | --- | --- | --- |
| API Gateway | Ingress, auth, Control Tower BFF | YES | `/health`, `/ready` | `/metrics`, Prometheus | TBD | Container image digest |
| Identity Service | Auth, users, roles | YES | `/health` | `/metrics` | TBD | Container image digest |
| Shipment Service | Shipments, outbox | YES | `/health` | `/metrics` | TBD | Container image digest |
| Control Tower Read Model | Shadow projection consumer | YES | `/health`, `/ready` | `/metrics`, shadow rules | TBD | Container image digest |
| Document Service | Documents milestones | YES | `/health` | `/metrics` | TBD | Container image digest |
| Billing Register Service | Billing milestones | YES | `/health` | `/metrics` | TBD | Container image digest |
| Company / TO / RFx / Low-code | Supporting domain | YES | `/health` | `/metrics` | TBD | Container image digest |
| PostgreSQL | Primary data store | YES | `pg_isready` | Manual/query | TBD | Backup restore + image |
| Redpanda | Kafka messaging | YES | internal | `rpk` (ops) | TBD | Compose profile |
| Prometheus | Metrics | YES (ops) | `/-/healthy` | self | TBD | Compose profile |
| Grafana | Dashboards | PARTIAL | `/api/health` | N/A | TBD | Compose profile |
| web-admin | Control Tower UI | YES | Nuxt build/E2E | Browser | TBD | Static deploy artifact |

---

## Ownership Matrix

| Role | Responsible | Accountable | Status |
| --- | --- | --- | --- |
| Product Owner | TBD | TBD | **MUST_BE_ASSIGNED_BEFORE_GO_LIVE** |
| Technical Owner | TBD | TBD | **MUST_BE_ASSIGNED_BEFORE_GO_LIVE** |
| Backend Owner | TBD | TBD | **MUST_BE_ASSIGNED_BEFORE_GO_LIVE** |
| Frontend Owner | TBD | TBD | **MUST_BE_ASSIGNED_BEFORE_GO_LIVE** |
| DevOps / Infrastructure | TBD | TBD | **MUST_BE_ASSIGNED_BEFORE_GO_LIVE** |
| Database Owner | TBD | TBD | **MUST_BE_ASSIGNED_BEFORE_GO_LIVE** |
| Security Owner | TBD | TBD | **MUST_BE_ASSIGNED_BEFORE_GO_LIVE** |
| Pilot Operations Owner | TBD | TBD | **MUST_BE_ASSIGNED_BEFORE_GO_LIVE** |
| Incident Commander | TBD | TBD | **MUST_BE_ASSIGNED_BEFORE_GO_LIVE** |
| Rollback Owner | Documented: Артем Асаev (rollback docs) | TBD confirm | PARTIAL |
| Business Pilot Owner | Documented: Феликс Асаev (PM notify) | TBD confirm | PARTIAL |

---

## Readiness Scoring

| Domain | Status | Evidence | Remaining gap | Pilot blocking |
| --- | --- | --- | --- | --- |
| Runtime | PASS | Staging 14/14 containers healthy; health/ready 200 | Frontend entry path TBD for live Pilot URL | NO |
| Security | PASS | Staging + UI E2E security gates | RBAC deny identity completeness | NO (see COND-001) |
| Observability | PARTIAL | Prometheus+Grafana up on VM; `/metrics` on services | No Alertmanager; example alerts only | NO (scale-up blocker) |
| Backup | PARTIAL | Script + `BACKUP_VERIFIED=YES` + dump on disk | Restore test not verified; schedule manual | **YES** (restore test) |
| Recovery | PARTIAL | Backup script validates `pg_restore -l` | Full restore procedure not tested | **YES** |
| Rollback | PARTIAL | Digest-pinned images; bintrans runbook; CT rollback scripts | Last-known-good release not formally pinned for Pilot | NO |
| Incident Response | PARTIAL | Multiple runbooks + `PILOT_RUNBOOK_V0.1.md` | On-call routing TBD | **YES** (P1 routing) |
| Ownership | PARTIAL | Rollback/PM names in legacy docs | Most roles TBD | **YES** |
| Pilot Users | PARTIAL | Launch runbook model | No staging Pilot users created | NO (pre-launch task) |
| Pilot Tenants | PARTIAL | Cohort manifest on VM (1 tenant) | Formal onboarding checklist incomplete | NO |
| Change Control | PARTIAL | Git/PR workflow; digest pinning | Change freeze not activated | NO |
| Documentation | PASS | Verification reports + runbooks | This handoff closes PILOT_OPERATIONAL_HANDOFF gap | NO |

---

## Observability

| System | Status | Evidence |
| --- | --- | --- |
| PROMETHEUS | **AVAILABLE** | Running on dedicated VM (`127.0.0.1:9090`, healthy 200); config scrapes all services |
| GRAFANA | **AVAILABLE** | Running on dedicated VM (`127.0.0.1:3001`, healthy 200) |
| ALERTMANAGER | **MISSING** | Not deployed in bintrans staging compose |
| CENTRAL_LOGGING | **MISSING** | Container logs via `docker logs` only; no Loki/ELK |
| TRACING | **NOT_APPLICABLE** | No OpenTelemetry deployment observed |
| REQUEST_ID | **AVAILABLE** | Gateway `X-Request-ID` middleware + structured logs |

### Monitoring verdict (Pilot-minimum)

| Signal | Status |
| --- | --- |
| CPU/RAM/disk (VM) | PARTIAL — manual `df`/`free`; no automated alert |
| Container state/restarts | PARTIAL — `docker ps`; restart count observable |
| HTTP 4xx/5xx | PARTIAL — Prometheus metrics on gateway/services |
| Latency | PARTIAL — histogram metrics exist |
| Control Tower summary | PARTIAL — custom CT metrics + health probes |
| Auth/login | PARTIAL — health + manual login probe |
| DB availability | PARTIAL — `/ready` + `pg_isready` |

---

## Alerting Readiness

| Alert type | Configured | Notes |
| --- | --- | --- |
| Service down | **MISSING** | Policy in `LOW_CODE_PILOT_WEEK3_PRODUCTION_MONITORING_ALERT_CONDITIONS_V0.1.md` |
| 5xx spike | **MISSING** | Example rules in `control_tower_shadow_alerts.example.yml` — not loaded |
| Latency degradation | **MISSING** | Proposed in policy docs |
| CPU/RAM/disk | **MISSING** | Proposed thresholds in runbook |
| DB unavailable | **MISSING** | Manual readiness check only |
| Auth outage | **MISSING** | P0 conditions documented |
| Control Tower outage | **MISSING** | Shadow alert examples exist |

```text
ALERT_ROUTING=NOT_CONFIGURED
WHO_RECEIVES_P1=TBD
WHO_RECEIVES_P2=TBD
```

**Verdict:** `MONITORING=PARTIAL` (metrics present), `ALERTING=PARTIAL` (policy only, no routing).

---

## Logging Readiness

| Field | Status |
| --- | --- |
| REQUEST_ID | YES — gateway middleware |
| SERVICE_IDENTIFICATION | YES — structured JSON logs with `service` field |
| TIMESTAMP | YES — ISO8601 in gateway logs |
| ERROR_LEVEL | YES — slog levels |
| RETENTION | NOT_FORMALIZED — Docker default |
| CENTRALIZATION | NO — per-container only |

### Secret / PII logging risk

| Risk | Assessment |
| --- | --- |
| Authorization token in logs | LOW — gateway logs redact patterns in verification; no evidence of token logging in code review |
| Password logging | LOW — login bodies not logged in gateway |
| PII in logs | MEDIUM operational risk — shipment/customer payloads may appear in debug; restrict log sharing |
| Pilot rule | **No secrets in alert messages or daily reports** |

---

## Backup & Recovery

| Field | Status | Evidence |
| --- | --- | --- |
| BACKUP_IMPLEMENTED | **YES** | `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh` |
| BACKUP_SCHEDULED | **UNKNOWN** | Manual script; no cron evidence on VM |
| LATEST_BACKUP | **CONFIRMED** | `freight_platform_20260811T083942Z.dump` in `/protected/bintrans/backups/` |
| BACKUP_VERIFIED | **YES** | Protected `staging.env` flag; script validates PGDMP + `pg_restore -l` |
| BACKUP_RETENTION | **NOT_FORMALIZED** | Single file observed; Selectel VM daily backup mentioned in legacy docs (7 days) |
| BACKUP_ENCRYPTION | **AT_REST_FS** | `chmod 600` on dump; path protected |
| RESTORE_PROCEDURE | **DOCUMENTED** | Script output references restore; full runbook partial |
| RESTORE_TEST | **NOT_VERIFIED** | No evidence of executed restore drill |

### RPO / RTO

| Field | Value |
| --- | --- |
| APPROVED_RPO | **NOT_SET** |
| APPROVED_RTO | **NOT_SET** |
| PROPOSED_PILOT_RPO | 24h (manual backup cadence + Selectel snapshot) — **PROPOSED** |
| PROPOSED_PILOT_RTO | 4h (authorized restore + validation) — **PROPOSED** |

---

## Rollback

| Field | Status |
| --- | --- |
| ROLLBACK_PROCEDURE | **DOCUMENTED** — `BINTRANS_DEDICATED_CONTROL_TOWER_STAGING_RUNBOOK.md`, image digest env vars |
| LAST_KNOWN_GOOD_RELEASE | **AVAILABLE** — `b75eb3d` / digest-pinned images currently running |
| IMAGE_DIGEST_PINNING | **YES** — `BINTRANS_*_IMAGE` digest overrides in protected env |
| ROLLBACK_VALIDATION | **DOCUMENTED** — health/ready + shadow smoke scripts |
| WHO_AUTHORIZES | Rollback Owner (documented: Артем Асаev — confirm assignment) |

**Deployment mechanism:** Docker Compose on dedicated VM; manual SSH ops; registry `cr.selcloud.ru/bintrans-staging`.

---

## SHA Alignment Decision

| Field | Value |
| --- | --- |
| STAGING_DEPLOYED_SHA | `b75eb3d` |
| UI_E2E / MAIN BASELINE | `234c8b78` |
| DELTA | Alert-acknowledgement features (PR #8–#9), docs, parallel-engineering — **no mandatory tenant/security fixes** in delta |
| MANDATORY_SECURITY_FIXES | Present at `b75eb3d` (tenant isolation verified on staging) |
| **SHA_ALIGNMENT** | **OPTIONAL_BEFORE_PILOT** |

Pilot may proceed on `b75eb3d` for Control Tower shadow observation path. Align to `234c8b78` if alert-ack Pilot scope required.

---

## Change Control (Pilot)

| Policy | Status |
| --- | --- |
| NO_UNREVIEWED_DEPLOYMENTS | Required |
| NO_DIRECT_MAIN_CHANGES | Required |
| NO_FORCE_PUSH | Required |
| RELEASE_SHA_PINNED | Required at launch — **TBD** |
| ROLLBACK_SHA_KNOWN | Yes (`b75eb3d` + digests) |
| CHANGE_LOG_REQUIRED | Yes |
| GO_LIVE_CHANGE_FREEZE | **NOT_ACTIVE** — activate in Controlled Pilot Launch Plan |

### Release identification template

| Field | Value |
| --- | --- |
| PILOT_RELEASE_ID | **TBD** |
| GIT_SHA | `b75eb3d` (staging) or `234c8b78` (if aligned) |
| IMAGE_DIGESTS | Record from protected env at launch |
| DEPLOY_DATE | TBD |
| APPROVED_BY | TBD |
| ROLLBACK_RELEASE | `b75eb3d` + recorded digests |

---

## Known Conditions Register

| ID | Description | Blocking? | Verdict | Owner | Phase |
| --- | --- | --- | --- | --- | --- |
| COND-001 | RBAC deny staging identity missing | NO | **ACCEPTABLE_WITH_CONDITION** | Security/Ops TBD | BEFORE_SCALE_UP |
| COND-002 | Event timeline empty on staging sample | NO | **NON_BLOCKING** (API 200; data/evidence gap) | Ops/Data TBD | FIRST_72H |
| COND-003 | SHA alignment b75eb3d vs 234c8b78 | NO | **NON_BLOCKING** unless alert-ack in Pilot scope | DevOps TBD | BEFORE_GO_LIVE if needed |

### COND-001 Closure Plan

| Field | Value |
| --- | --- |
| REQUIRED_TEST_ROLE | `CONSIGNEE_VIEWER` or equivalent low-privilege |
| REQUIRED_TENANT | Approved cohort tenant (protected manifest) |
| EXPECTED_ACCESS | Control Tower denied / hidden |
| EXPECTED_DENIAL | 403 or route deny |
| WHEN_TO_CLOSE | Before Pilot scale-up; not required for initial controlled launch |

### COND-002 Closure Plan

| Field | Value |
| --- | --- |
| OPTIONS | Natural Pilot shipment history; disposable fixture in future seed task; read-only wait |
| WHEN_TO_CLOSE | First 72h observation or dedicated seed task |
| CURRENT | `GET events` → 200, `eventCount=0` |

---

## Known Defect Register

| ID | Severity | Status | Pilot blocking | Workaround | Source |
| --- | --- | --- | --- | --- | --- |
| DEF-001 | MEDIUM | OPEN | NO | Avoid dashboard route in Pilot demo | UI E2E — `isApiUnavailableError` dashboard import |
| DEF-002 | LOW | OPEN | NO | Use API login for automation | UI E2E — Playwright UI login quirk |
| DEF-003 | LOW | OPEN | NO | Use list API for shipment count | Staging — summary vs list active count |
| DEF-004 | MEDIUM | OPEN | NO | Monitor via API/metrics | Staging — empty event timeline |
| CRITICAL | 0 | — | — | — | — |
| HIGH | 0 | — | — | — | — |

---

## Pilot Users & Tenants (Process — No Creation)

### User categories

| Category | Role | Permissions | Account owner |
| --- | --- | --- | --- |
| Pilot Admin | PLATFORM_ADMIN | Full Control Tower + admin | TBD |
| Shipper user | SHIPPER_* | Tenant-scoped ops | TBD |
| Carrier user | CARRIER_* | Tenant-scoped ops | TBD |
| Low privilege | CONSIGNEE_VIEWER | Deny Control Tower | TBD (COND-001) |
| Support/Ops | Support role | Read + incident | TBD |

### Tenant onboarding checklist

- [ ] Tenant ID assigned and recorded (protected manifest)
- [ ] Company record exists
- [ ] Initial admin provisioned
- [ ] Roles assigned
- [ ] Data isolation validated (staging gate passed)
- [ ] Reference data / test shipment plan
- [ ] Support contact assigned
- [ ] Activation approval recorded

**Current staging cohort:** 1 tenant (`bintrans-staging-controlled` alias) — see protected cohort manifest on VM.

---

## Support Model

| Tier | Scope | Owner |
| --- | --- | --- |
| L1 | Pilot operations, health checks, user reports | TBD |
| L2 | Application engineering, API/Control Tower | TBD |
| L3 | Infrastructure, database, security | TBD |

---

## Escalation Matrix

| Severity | Initial owner | Escalation | Response target | Pilot action |
| --- | --- | --- | --- | --- |
| P1 | Incident Commander TBD | Security + Technical + Rollback Owner | 15 min ack (proposed) | STOP / abort assessment |
| P2 | Pilot Ops TBD | Backend + DevOps | 1h (proposed) | Mitigate / rollback assessment |
| P3 | L1 support | L2 on next business day | 4h (proposed) | Workaround |
| P4 | Monitoring owner | Ticket queue | Next day | Track |

---

## Operational Risk Register

| ID | Risk | Prob | Impact | Severity | Mitigation | Blocker |
| --- | --- | --- | --- | --- | --- | --- |
| R-001 | No alert routing | M | H | HIGH | Configure Alertmanager + routing pre-scale | YES |
| R-002 | Restore untested | L | H | HIGH | Authorized restore drill on staging | YES |
| R-003 | On-call unassigned | M | M | MEDIUM | Assign owners before GO | YES |
| R-004 | Disk fill on VM | L | M | MEDIUM | Monitor `df`; runbook | NO |
| R-005 | Empty event timeline | M | L | LOW | COND-002 plan | NO |
| R-006 | RBAC deny untested | L | M | LOW | COND-001 | NO |
| R-007 | Loopback-only API | — | — | N/A | Expected; SSH/tunnel ops path | NO |

---

## Proposed Pilot SLA/SLO (Not Approved)

| Indicator | Proposed target | Status |
| --- | --- | --- |
| Platform availability | 99.5% Pilot window | PROPOSED |
| API availability | 99.5% | PROPOSED |
| Control Tower summary success | 99% | PROPOSED |
| Auth availability | 99.9% | PROPOSED |
| p95 API latency | < 2s observation | PROPOSED |
| P1 acknowledgement | 15 min | PROPOSED |
| P2 acknowledgement | 1 h | PROPOSED |

---

## Go-Live Checklist

- [x] Security gate pass
- [x] Control Tower real data pass (staging API)
- [x] Tenant isolation pass
- [x] Critical containers healthy (staging evidence)
- [x] Health/readiness pass
- [x] Auth pass (staging)
- [ ] Release SHA approved for Pilot launch
- [ ] Rollback SHA/digests recorded in launch record
- [ ] Monitoring operational with **alert routing**
- [ ] Backup evidence confirmed (< 7 days)
- [ ] Restore procedure **tested**
- [ ] Incident contacts assigned
- [ ] Pilot tenants approved
- [ ] Pilot users approved
- [ ] Known defects accepted (DEF-001..004)
- [ ] RBAC deny gap closed or accepted (COND-001)
- [ ] Event history gap closed or accepted (COND-002)
- [ ] Change freeze active
- [ ] Pilot owner authorizes GO

---

## Go / No-Go Decision

| Criterion | Met? |
| --- | --- |
| TECHNICAL_GATE=PASS | YES |
| SECURITY_GATE=PASS | YES |
| OPERATIONAL_CRITICAL_GAPS=0 | **NO** — alert routing, restore test, ownership |
| ROLLBACK_READY | PARTIAL |
| BACKUP_READY | PARTIAL |
| MONITORING_READY | PARTIAL |
| INCIDENT_OWNERSHIP_READY | NO |

**Decision:** `GO_WITH_CONDITIONS`

### Launch blockers (must close before unrestricted Pilot)

1. Alert routing configured for P1/P2 (or explicit interim manual cadence with assigned owner)
2. Restore drill executed and documented on staging
3. Incident/on-call ownership assigned (minimum: Incident Commander, DevOps, Security, Rollback Owner)
4. `PILOT_RELEASE_ID` + SHA/digests formally approved

### Non-blocking conditions (acceptable at controlled launch)

- COND-001 RBAC deny identity
- COND-002 event timeline data
- COND-003 SHA alignment (unless alert-ack in scope)
- Dedicated VM loopback-only gateway (by design)

---

## First 24 Hours Plan

| Time | Checks |
| --- | --- |
| Launch | Health/ready; auth login; CT summary; container status |
| +15 min | 5xx scan; gateway logs; disk `df -h` |
| +30 min | CT summary latency; critical events; auth |
| +1 h | Repeat health; user feedback |
| +2 h | Prometheus scrape targets; restart count |
| +4 h | Full gate smoke (read-only) |
| End of day | Daily Pilot report; defect review |

---

## First 72 Hours Plan

| Day | Focus |
| --- | --- |
| Day 1 | Stability, auth, Control Tower, incident readiness |
| Day 2 | COND-002 event history observation; performance trends |
| Day 3 | Scale/readiness review; condition closure assessment |

---

## Daily Pilot Report Template

```text
PILOT DAILY STATUS

DATE=
RELEASE=
SYSTEM_STATUS=GREEN|YELLOW|RED
USERS=<count only>
TENANTS=<count only>
ACTIVE_SHIPMENTS=<count only>
P1=
P2=
P3=
5XX_RATE=
LATENCY_OBS=
SECURITY_INCIDENTS=NONE|<redacted summary>
KNOWN_ISSUES=
DECISION=CONTINUE|PAUSE|ROLLBACK_ASSESS
```

---

## Incident Report Template

```text
INCIDENT_ID=
START_TIME=
DETECTED_BY=
SEVERITY=
AFFECTED_COMPONENTS=
AFFECTED_TENANTS=<aliases only>
SYMPTOMS=
ROOT_CAUSE=
MITIGATION=
RECOVERY=
DATA_IMPACT=
SECURITY_IMPACT=
FOLLOW_UP=
REQUEST_IDS=<redacted list>
```

---

## Safety (This Assessment)

| Field | Value |
| --- | --- |
| PRODUCT_CODE_MODIFIED | NO |
| STAGING_MUTATION | NO |
| PRODUCTION_MUTATION | NO |
| DEPLOYMENT_PERFORMED | NO |
| SERVICE_RESTART | NO |
| DATABASE_WRITE | NO |
| SECRET_CHANGE | NO |

---

## Operational Asset Inventory (Repository)

| Asset | Path |
| --- | --- |
| Bintrans staging runbook | `docs/BINTRANS_DEDICATED_CONTROL_TOWER_STAGING_RUNBOOK.md` |
| Staging verification | `docs/ACTUAL_SELECTEL_STAGING_VERIFICATION_V0.1.md` (branch) |
| UI E2E verification | `docs/CONTROL_TOWER_UI_E2E_VERIFICATION_V0.1.md` (branch) |
| CT shadow rollout | `docs/CONTROL_TOWER_SHADOW_ROLLOUT_RUNBOOK.md` |
| Backup script | `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh` |
| Prometheus config | `infrastructure/monitoring/prometheus/prometheus.yml` |
| CT shadow alerts (example) | `infrastructure/monitoring/prometheus/control_tower_shadow_alerts.example.yml` |
| Monitoring policy | `docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_MONITORING_ALERT_CONDITIONS_V0.1.md` |
| Launch runbook (legacy low-code) | `docs/LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md` |
| Rollback checklists | `docs/LOW_CODE_PILOT_WEEK3_ROLLBACK_*.md` |
| Pilot runbook (this pack) | `docs/PILOT_RUNBOOK_V0.1.md` |
| Makefile health-check | `make health-check` |

---

## Next Recommended Step

**Do not launch automatically.**

Proceed to **`CONTROLLED PILOT LAUNCH PLAN v0.1`** after closing launch blockers:

1. Assign operational owners
2. Configure alert routing (or approve interim manual monitoring cadence)
3. Execute authorized restore drill on staging
4. Pin `PILOT_RELEASE_ID` + SHA/digests
5. Activate change freeze
6. Obtain Pilot owner GO signature

---

## Publication

| Field | Value |
| --- | --- |
| BRANCH | `docs/pilot-operational-readiness-v0.1` |
| WORKTREE | `D:\Projects\freight-platform-pilot-readiness` |
| REPORT_FILE | `docs/PILOT_OPERATIONAL_READINESS_AND_HANDOFF_V0.1.md` |
| RUNBOOK_FILE | `docs/PILOT_RUNBOOK_V0.1.md` |
