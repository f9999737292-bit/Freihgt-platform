# Low-code Pilot Week-3 Staging Deploy Runbook v0.1

## Summary

Operational runbook for DevOps/Ops to deploy **remote staging** for freight-platform low-code **auth-on verification** (PR-GAP-001).

**Current status:** **WAIT_FOR_REMOTE_STAGING_DETAILS**

**Decision:** **STAGING_DEPLOY_RUNBOOK_CREATED**

**PR-GAP-001:** **BLOCKED_WAITING_FOR_REMOTE_STAGING**

**Production-ready not claimed.** **Controlled pilot continues** under `CONTROLLED_PILOT_APPROVED`.

**Docs-only pack** — no deploy executed from this document; placeholders only; no secrets in git.

## Purpose

Enable Ops to stand up staging so QA/Security can run **Remote Auth-On Staging Repeat Pack v0.1** with read-only GET matrix.

**Target:** remote staging must provide URL, API URL, auth-on confirmation, service restart confirmation, admin/non-admin users, tenant ID, login/JWT flow, and read-only permission.

## Current Status

| Field | Value |
|-------|-------|
| HEAD (pack baseline) | `01c8158` — remote staging preparation checklist |
| Local auth-on | `AUTH_ON_REPEAT_LOCAL_VERIFIED` |
| Remote staging | **not deployed** |
| Repo deploy automation | **none** — manual Ops steps |

## Target Result

| Deliverable | Description |
|-------------|-------------|
| HTTPS staging URL | e.g. `https://staging.7rights.ru` |
| API base | `https://<staging-domain>/api/v1/low-code` |
| Auth-on | `LOW_CODE_ADMIN_AUTH_ENABLED=true` on low-code-service |
| Test users | PLATFORM_ADMIN + non-admin |
| Tenant | Pilot tenant UUID |
| Handoff | Completed `LOW_CODE_PILOT_WEEK3_STAGING_INPUT_FORM_V0.1.md` |

## Staging Options

| Option | Description | Use when |
|--------|-------------|----------|
| **A. VPS + DNS** | Full staging on VM with `staging.7rights.ru` | Preferred for PR-GAP-001 closure |
| **B. Temporary tunnel** | HTTPS to local Docker via Cloudflare Tunnel / ngrok | Short-term matrix only — see tunnel doc |
| **C. Local only** | localhost + auth-on override | Already done — does not close PR-GAP-001 remote |

## Recommended Path

**Option A (VPS + staging.7rights.ru)** for durable staging evidence.

Option B only with Security approval and explicit note that full staging may still be required.

## Infrastructure Requirements

| Component | Requirement |
|-----------|-------------|
| VM/VPS | Linux (Ubuntu 22.04+ recommended), 4+ GB RAM, 2+ vCPU |
| Public IP | Static or stable |
| Firewall | 443 (HTTPS), 22 (SSH admin only) |
| Reverse proxy | nginx / Caddy / Traefik with TLS |
| Docker | Docker Engine + Compose v2 |
| Git | Clone from `https://github.com/f9999737292-bit/Freihgt-platform.git` |

## DNS Requirements

| Record | Value |
|--------|-------|
| Type | A or CNAME |
| Host | `staging.7rights.ru` (or agreed subdomain) |
| Target | VPS public IP or load balancer |
| TLS | Let's Encrypt or org certificate |

## Server Requirements

- Docker and docker compose plugin installed
- Non-root deploy user with docker group (recommended)
- Sufficient disk for images + PostgreSQL volume
- **No production data** on staging DB
- Staging-only secrets in **server-side env file** — not in git

## Docker Requirements

Reference compose: `infrastructure/docker-compose/docker-compose.yml`

| Service | Staging notes |
|---------|---------------|
| postgres | Staging DB only; unique password |
| identity-service | JWT_SECRET staging-only |
| api-gateway | Port 8080 behind reverse proxy → 443 |
| low-code-service | **Auth-on flag via env override** |
| All services | `make health-check` equivalent after start |

**Do not** commit `docker-compose.override.yml` with secrets to git.

## Repository Checkout

```bash
git clone https://github.com/f9999737292-bit/Freihgt-platform.git
cd freight-platform
git checkout main
git pull
```

Deploy tag/commit: record SHA in staging input form (e.g. `01c8158` or later).

## Environment Configuration

1. Create **server-only** env file (e.g. `/opt/freight-platform/staging.env`) — **never commit**
2. Use placeholders from `LOW_CODE_PILOT_WEEK3_STAGING_ENV_EXAMPLE_V0.1.md`
3. Set `ENVIRONMENT=staging`
4. Point `DATABASE_URL`, `JWT_SECRET`, etc. to staging values
5. Load env when running compose (deployment-specific — e.g. `--env-file staging.env`)

See readiness checklist: `LOW_CODE_PILOT_WEEK3_STAGING_READINESS_CHECKLIST_V0.1.md`

## Auth-On Configuration

| Step | Action |
|------|--------|
| 1 | Set `LOW_CODE_ADMIN_AUTH_ENABLED=true` on **low-code-service** only |
| 2 | Ensure `IDENTITY_SERVICE_URL` reachable from low-code-service |
| 3 | **Restart** low-code-service (not full destructive down) |
| 4 | Verify health: gateway + low-code `/health` |
| 5 | Document confirmation in staging input form |

**Rollback:** set `LOW_CODE_ADMIN_AUTH_ENABLED=false`, restart low-code-service.

Reference: `LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_RUNBOOK_V0.1.md`

## Database / Migration Notes

| Rule | Detail |
|------|--------|
| Migrations | Apply via project migration process on staging DB **once** during initial deploy |
| Data | Staging/test data only — **no production copy** |
| This pack | **Does not** run migrations or writes |

Ops confirms: `migrations applied` in readiness checklist.

## Seed / Test Users

After platform up + migrations:

```bash
# On staging server or CI job with API_GATEWAY_URL pointing to staging
export API_GATEWAY_URL=https://<staging-domain>
make seed-dev-admin    # creates PLATFORM_ADMIN — adjust ADMIN_EMAIL for staging
make seed-lowcode-demo # demo templates for pilot tenant
```

Create **non-admin** user (e.g. `SHIPPER_LOGIST`) via identity seed or admin API — record UUID in input form.

**Dev reference (local — staging UUIDs will differ):**

| Role | Email (example) |
|------|-----------------|
| PLATFORM_ADMIN | `admin@<staging-domain>` |
| Non-admin | `operator@<staging-domain>` |

Passwords: **credentials provided separately / not stored**.

## Tenant Setup

- Create or reuse **pilot tenant** for controlled pilot scope
- Record `X-Tenant-ID` UUID in `LOW_CODE_PILOT_WEEK3_STAGING_INPUT_FORM_V0.1.md`
- Align with `LOW_CODE_PILOT_WEEK3_CONTROLLED_PILOT_SCOPE_CHARTER_V0.1.md`

## Read-Only Verification

After deploy, QA runs **GET only**:

- Admin → admin templates **200**
- Non-admin → admin templates **403**
- Anonymous → admin **401/403**
- Runtime active templates **200**
- Audit GET per staging policy

Matrix: `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_AUTH_ON_TEST_MATRIX_V0.1.md`

**Forbidden:** POST, PUT, PATCH, DELETE, publish, migrate execute, import, batch execute.

## Remote Auth-On Matrix Dependency

PR-GAP-001 closes only after:

1. Staging deployed per this runbook
2. Input form completed
3. **Remote Auth-On Staging Repeat Pack v0.1** executed with evidence doc

## Security Rules

1. No production data on staging
2. No secrets in git or docs (placeholders only)
3. No production-ready claim from deploy alone
4. Read-only QA until Security approves writes (if ever)
5. Rotate staging JWT_SECRET independently from production

## Secrets Handling

| Item | Rule |
|------|------|
| Passwords | Secure channel only |
| JWT / tokens | Not stored in repo |
| `.env` on server | File permissions restricted; not in git |
| Docs | *credentials provided separately / not stored* |

## Rollback Notes

| Scenario | Action |
|----------|--------|
| Auth-on breaks admin UI | `LOW_CODE_ADMIN_AUTH_ENABLED=false`; restart low-code-service |
| Bad deploy | Redeploy previous image/SHA; restore staging DB snapshot if needed |
| Abort pilot on staging | Disable auth-on; document in ops notes |

## Handover To Ops

Deliver to pilot team:

1. Completed `LOW_CODE_PILOT_WEEK3_STAGING_INPUT_FORM_V0.1.md` (no passwords in file)
2. Readiness checklist all **READY** for required rows
3. Read-only permission **yes**
4. Named Ops owner + Security owner

## Required Output From Ops

Fill and return (see input form):

1. Staging URL  
2. API URL  
3. Auth-on confirmed + restart confirmed  
4. Admin/non-admin email + UUID  
5. Tenant ID  
6. Login/JWT flow  
7. Read-only permission **yes**

## Next Pack

| Trigger | Pack |
|---------|------|
| Staging details provided | **Remote Auth-On Staging Repeat Pack v0.1** |
| Temporary tunnel approved | **Temporary Tunnel Auth-On Matrix Pack v0.1** |

Related docs:

- `LOW_CODE_PILOT_WEEK3_STAGING_ENV_EXAMPLE_V0.1.md`
- `LOW_CODE_PILOT_WEEK3_STAGING_INPUT_FORM_V0.1.md`
- `LOW_CODE_PILOT_WEEK3_STAGING_READINESS_CHECKLIST_V0.1.md`
- `LOW_CODE_PILOT_WEEK3_TEMPORARY_TUNNEL_OPTION_V0.1.md`
