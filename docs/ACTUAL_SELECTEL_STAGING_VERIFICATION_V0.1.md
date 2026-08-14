# Actual Selectel Staging Verification v0.1

## Executive Result

| Field | Value |
| --- | --- |
| ACTUAL_SELECTEL_STAGING | **FAIL** |
| SECURITY_GATE | **PASS** |
| CONTROL_TOWER_REAL_DATA | **PASS** |
| FINAL_PILOT_VERDICT | **CONDITIONAL_PASS** |
| VERIFICATION_DATE | 2026-08-14 |

**Rationale:** Dedicated Selectel Control Tower staging backend/API/security verification succeeded via SSH read-only probes. Pilot **FAIL** on overall staging gate due to missing public HTTPS/frontend, deployed SHA behind UI-E2E-tested main, empty shipment event timeline for sampled shipment, and RBAC deny test blocked (no low-privilege staging identity in protected env).

---

## Git

| Field | Value |
| --- | --- |
| BASE_SHA | 234c8b78d198e1a694757be20fb5e53b32dd77ad |
| LOCAL_HEAD | 234c8b78d198e1a694757be20fb5e53b32dd77ad |
| ORIGIN_MAIN_SHA_AT_START | 234c8b78d198e1a694757be20fb5e53b32dd77ad |
| VERIFICATION_BRANCH | test/actual-selectel-staging-verification-v0.1 |
| WORKTREE_PATH | D:\Projects\freight-platform-selectel-staging-verification |
| UI_E2E_PREVIOUS_RESULT | PASS (13/13 local) |
| UI_E2E_TESTED_PRODUCT_SHA | 234c8b78d198e1a694757be20fb5e53b32dd77ad |

---

## Environment

| Field | Value |
| --- | --- |
| ENVIRONMENT_IDENTITY | **STAGING_CONFIRMED** |
| CLOUD_PROVIDER | Selectel |
| ENVIRONMENT | STAGING (BINTRANS dedicated Control Tower VM) |
| HOSTNAME | bintrans-control-tower-staging |
| PUBLIC_IP | 161.104.57.152 (API **not** publicly published) |
| PUBLIC_URL | **NOT_CONFIGURED** (loopback-only gateway) |
| FRONTEND_URL | **NOT_DEPLOYED** (no web-admin container) |
| API_URL | http://127.0.0.1:18080 (VM localhost via SSH) |
| DEPLOYMENT_TYPE | DOCKER_COMPOSE |
| DEPLOY_PATH | /opt/bintrans/control-tower-staging |
| COMPOSE_PROJECT | bintrans-ct-staging |
| OLD_SHARED_VPS | 161.104.53.221 — **out of scope** per runbook |

---

## Deployed Version

| Field | Value |
| --- | --- |
| EXPECTED_PILOT_SHA | 234c8b78d198e1a694757be20fb5e53b32dd77ad (origin/main at verification) |
| DEPLOYED_SHA (env) | b75eb3d |
| DEPLOYED_REPO_HEAD (VM) | 4d0cdfb2d4b85e8b00a098111810624affede3f9 |
| IMAGE_TAGS | cr.selcloud.ru/bintrans-staging/*@sha256 (digest-pinned) |
| VERSION_MATCH | **OLDER_THAN_UI_E2E_TESTED_SHA** |
| CONTROL_TOWER_MODE | shadow (PRIMARY disabled — expected) |
| MIGRATION_VERSION | 19 (dirty=false) |

---

## Infrastructure

| Component | Status | Health | Version/Image | Notes |
| --- | --- | --- | --- | --- |
| api-gateway | Up 2d | healthy | api-gateway@sha256:db9714a5… | restarts=0 |
| control-tower-read-model | Up 47h | healthy | read-model@sha256:defe3f66… | restarts=0 |
| identity-service | Up 3d | healthy | bintrans-staging registry | |
| shipment-service | Up 3d | healthy | bintrans-staging registry | |
| document-service | Up 3d | healthy | | |
| billing-register-service | Up 3d | healthy | | |
| postgres | Up 3d | healthy | not publicly bound | |
| redpanda | Up 3d | healthy | not publicly bound | |
| prometheus/grafana | Up 3d | running | loopback only | |
| web-admin/frontend | **absent** | N/A | | browser smoke blocked |

Disk: 8% used (/dev/sda1 99G). Memory: ~1.2Gi used / 7.8Gi total.

---

## Health / TLS / Public Surface

| Check | Result |
| --- | --- |
| EXTERNAL_HEALTH (via SSH localhost) | **200** `/health` |
| EXTERNAL_READY | **200** `/ready` |
| READ_MODEL_HEALTH | **200** |
| HTTPS | **NOT_AVAILABLE** (no public 443 on dedicated VM) |
| TLS_CERT | **NOT_APPLICABLE** |
| HTTP_REDIRECT | N/A |
| PUBLIC_POSTGRES | **CLOSED** |
| PUBLIC_INTERNAL_SERVICES | **CLOSED** (gateway/read-model bound 127.0.0.1 only) |
| PUBLIC_GATEWAY | **NOT_EXPOSED** (by design) |

---

## Endpoint Results

| Endpoint | Auth | Expected | Actual | Result |
| --- | --- | --- | --- | --- |
| GET /health | none | 200 | 200 | PASS |
| GET /ready | none | 200 | 200 | PASS |
| POST /api/v1/auth/login (valid) | test creds | 200 | 200 | PASS |
| GET /api/v1/control-tower/summary | none | 401 | 401 | PASS |
| GET /api/v1/control-tower/summary | invalid token | 401 | 401 | PASS |
| GET /api/v1/control-tower/summary | valid | 200 | 200 | PASS |
| GET /api/v1/shipments | valid | 200 | 200 (5 items) | PASS |
| GET /api/v1/shipments/{foreign} | valid + untrusted tenant header | 404 | 404 | PASS |
| GET /api/v1/shipments/{foreign}/events | valid + untrusted tenant header | 404 | 404 | PASS |
| GET /api/v1/drivers/{foreign} | valid + untrusted tenant header | 404 | 404 | PASS |
| GET /api/v1/vehicles/{foreign} | valid + untrusted tenant header | 404 | 404 | PASS |
| GET /api/v1/shipments/{foreign}?tenant_id=… | valid | 404 | 404 | PASS |
| GET /api/v1/shipments/{own}/events | valid | 200 | 200 (0 events) | PARTIAL |
| GET /api/v1/control-tower/summary?status=IN_TRANSIT | valid | 200 | 200 | PASS |

---

## Security Tests

| ID | Test | Expected | Actual | Result |
| --- | --- | --- | --- | --- |
| SEC-STG-001 | missing auth | 401 | 401 | PASS |
| SEC-STG-002 | invalid auth | 401 | 401 | PASS |
| SEC-STG-003 | untrusted X-Tenant-ID | no foreign data / header ignored | summary digest unchanged | PASS |
| SEC-STG-004 | foreign shipment | 404 | 404 | PASS |
| SEC-STG-005 | foreign shipment events | 404 | 404 | PASS |
| SEC-STG-006 | RBAC denied role | 403/deny | **NOT_RUN** | BLOCKED |
| SEC-STG-007 | query tenant bypass | 404 | 404 | PASS |

---

## Control Tower Tests

| ID | Test | Result | Notes |
| --- | --- | --- | --- |
| CT-STG-001 | summary endpoint | PASS | HTTP 200, latency 24–32ms (3 samples) |
| CT-STG-002 | real backend data | PASS | `source=LEGACY`, `fallbackUsed=false`, `hasDemoIds=false` |
| CT-STG-003 | KPI | PARTIAL | kpiCount=0 |
| CT-STG-004 | active shipments | PARTIAL | summary activeShipments=0; list API returns 5 |
| CT-STG-005 | critical events | PASS | criticalEvents=6 |
| CT-STG-006 | filters | PASS | `?status=IN_TRANSIT` → 200 |
| CT-STG-007 | browser load | BLOCKED | no frontend deployed |
| CT-STG-008 | no demo mode | PASS | no DEMO-* identifiers |
| CT-STG-009 | no unexpected 5xx | PASS | day0 baseline Gateway5xx=0; live probes clean |
| CT-STG-010 | latency observation | PASS | min=24 avg=26 max=32 ms |

---

## Shipment Event History

| ID | Test | Result | Notes |
| --- | --- | --- | --- |
| SHE-STG-001 | own shipment timeline | PARTIAL | HTTP 200, eventCount=0 |
| SHE-STG-002 | derived provenance | BLOCKED | no events to inspect |
| SHE-STG-003 | foreign shipment 404 | PASS | |
| SHE-STG-004 | unknown shipment handling | PASS | foreign UUID → 404 |

---

## Observability (read-only)

| Field | Value |
| --- | --- |
| day0 baseline (2026-08-12) | MatchTotal=5, MismatchTotal=0, Gateway5xx=0, consumer lag=0 |
| API_GATEWAY_ERRORS (30m) | no panic/fatal/5xx in sampled logs |
| SHIPMENT_SERVICE_ERRORS | no panic in 30m sample |
| IDENTITY_SERVICE_ERRORS | no panic in 30m sample |
| CRITICAL_RESTARTS | 0 on pilot-critical services |
| READINESS_FALSE_POSITIVE | not observed |

---

## Blocked Tests

| TEST | BLOCKER | WHY_NOT_BYPASSED | PILOT_IMPACT |
| --- | --- | --- | --- |
| STG-BROWSER-001..008 | NO_FRONTEND_DEPLOYED | Dedicated VM exposes API on loopback only; no web-admin/nginx | HIGH — no browser-level staging smoke |
| SEC-STG-006 RBAC deny | RBAC_DENY_TEST_BLOCKED_NO_SAFE_IDENTITY | No low-privilege staging creds in protected env; will not create users | MEDIUM |
| HTTPS/TLS gate | PUBLIC_HTTPS_NOT_CONFIGURED | Loopback-only architecture on dedicated VM | HIGH for external pilot URL |
| SHE-STG-002 provenance | EMPTY_EVENT_TIMELINE | Sample shipment returned 0 events | MEDIUM |

---

## Defects

### DEFECT-STG-001

| Field | Value |
| --- | --- |
| CLASSIFICATION | DEPLOYMENT_DEFECT |
| SEVERITY | HIGH |
| SCENARIO | Public pilot surface |
| EXPECTED | Public HTTPS frontend + API ingress for staging pilot |
| ACTUAL | Gateway 127.0.0.1:18080 only; no frontend container; no public 80/443 |
| PILOT_IMPACT | Browser staging smoke impossible; TLS gate fails |

### DEFECT-STG-002

| Field | Value |
| --- | --- |
| CLASSIFICATION | DEPLOYMENT_DEFECT |
| SEVERITY | MEDIUM |
| SCENARIO | Version alignment |
| EXPECTED | Staging at or compatible with UI-E2E-tested main (234c8b78) |
| ACTUAL | Deployed b75eb3d / VM checkout 4d0cdfb |
| PILOT_IMPACT | Alert-ack and post-#9 main changes not deployed to staging |

### DEFECT-STG-003

| Field | Value |
| --- | --- |
| CLASSIFICATION | DATA_DEFECT |
| SEVERITY | MEDIUM |
| SCENARIO | Shipment event history |
| EXPECTED | Non-empty timeline for existing staging shipment |
| ACTUAL | HTTP 200 with eventCount=0 for sampled own shipment |
| PILOT_IMPACT | Event history UX cannot be validated on staging data |

### DEFECT-STG-004

| Field | Value |
| --- | --- |
| CLASSIFICATION | DATA_DEFECT |
| SEVERITY | LOW |
| SCENARIO | Control Tower active shipments vs list API |
| EXPECTED | Consistent active shipment representation |
| ACTUAL | summary activeShipments=0 while `/api/v1/shipments` returns 5 |
| PILOT_IMPACT | May reflect shadow/legacy filter semantics; document for operators |

---

## Safety

| Field | Value |
| --- | --- |
| PRODUCTION_MUTATION | NO |
| STAGING_MUTATION | NO |
| STAGING_DEPLOY | NO |
| SERVICE_RESTART | NO |
| DATABASE_WRITE | NO |
| SECRET_CHANGE | NO |
| FORCE_PUSH | NO |

---

## Artifacts

| Artifact | Status |
| --- | --- |
| E2E_TEST_FILES | `scripts/verification/selectel_staging_verify_remote.sh` |
| SCREENSHOTS | NOT_RETAINED_PRIVACY / NOT_APPLICABLE |
| TRACE/VIDEO | NOT_GENERATED |
| HTML_REPORT | NOT_GENERATED |
| day0 baseline reference | `/protected/bintrans/control-tower-observation/day0-baseline-20260812T132921Z.json` (on VM) |

---

## Test Commands Record

```powershell
cd D:\Projects\freight-platform
git fetch origin
git worktree add D:\Projects\freight-platform-selectel-staging-verification `
  -b test/actual-selectel-staging-verification-v0.1 origin/main

ssh bintrans-ct-staging "hostname; docker ps; curl http://127.0.0.1:18080/health"
# Unauthenticated probes (401 expected)
ssh bintrans-ct-staging "curl -w '%{http_code}' http://127.0.0.1:18080/api/v1/control-tower/summary"

# Authenticated verification (credentials sourced from protected staging.env on VM; not printed)
scp scripts/verification/selectel_staging_verify_remote.sh bintrans-ct-staging:/tmp/
ssh bintrans-ct-staging "bash /tmp/selectel_staging_verify_remote.sh"
```

---

## Pilot Mapping

| Field | Value |
| --- | --- |
| UI_E2E | PASS (previous task) |
| ACTUAL_SELECTEL_STAGING | FAIL (pilot surface incomplete) |
| SECURITY_GATE | PASS |
| FINAL_PILOT_VERDICT | CONDITIONAL_PASS |
| REMAINING_BLOCKERS | PUBLIC_FRONTEND_HTTPS; STAGING_SHA_ALIGN; RBAC_DENY_IDENTITY; EVENT_TIMELINE_DATA |

**Note:** Actual Selectel staging **backend** verification passed security and Control Tower real-API checks. Overall staging gate fails pilot PASS criteria due to deployment/operational gaps above — not due to auth bypass or cross-tenant leak.
