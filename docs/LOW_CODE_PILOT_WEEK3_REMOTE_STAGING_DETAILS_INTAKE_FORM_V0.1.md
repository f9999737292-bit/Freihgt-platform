# Low-code Pilot Week-3 Remote Staging Details Intake Form v0.1

## Summary

Intake form template for collecting **remote staging infrastructure details** to unblock PR-GAP-001. **Docs-only** — no deploy, no SSH, no secrets stored in repo.

**Decision:** **REMOTE_STAGING_DETAILS_INTAKE_FORM_CREATED_PENDING_INPUT**

**PR-GAP-001:** **REMOTE_STAGING_DETAILS_PENDING_INPUT**

## Purpose

Capture server, network, access, and runtime readiness **metadata** (not secrets) so Ops can proceed to Remote Auth-On Staging Repeat Pack v0.1.

Reference: `LOW_CODE_PILOT_WEEK3_STAGING_INPUT_FORM_V0.1.md` (post-deploy URLs), `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_READINESS_CHECKLIST_V0.1.md`

## Current Status

| Field | Value |
|-------|-------|
| Intake form | **created** |
| Real staging details | **not provided** |
| Production-ready claimed | **no** |
| Controlled pilot | **CONTROLLED_PILOT_APPROVED** — active |
| Deploy executed | **no** |
| Staging writes executed | **no** |

## Server Details

| Field | Value |
|-------|-------|
| provider | |
| public_ip | |
| domain | |
| OS | |
| CPU | |
| RAM | |
| disk | |
| SSH user | |
| sudo/root access | yes / no |
| Docker installed | yes / no |
| Docker Compose installed | yes / no |

## Network Details

| Field | Value |
|-------|-------|
| firewall enabled | yes / no |
| ports 80/443 available | yes / no |
| SSH restricted by IP or policy | yes / no |
| PostgreSQL external port closed | yes / no |
| HTTPS planned | yes / no |

## Access Details

| Field | Value |
|-------|-------|
| SSH key access available | yes / no |
| repository cloned | yes / no |
| branch main selected | yes / no |
| ops owner name | |
| security owner name | |

**Note:** SSH private keys, passwords, and tokens are **never** stored in this document.

## Runtime Details

| Field | Value |
|-------|-------|
| staging web-admin URL | |
| staging API gateway URL | |
| LOW_CODE_ADMIN_AUTH_ENABLED=true planned | yes / no |
| .env staging prepared (secrets not in docs) | yes / no |
| backup/snapshot enabled | yes / no |

## Domain / SSL Details

| Field | Value |
|-------|-------|
| domain/subdomain assigned | yes / no |
| SSL/TLS certificate planned | yes / no |
| certificate provider | |

## Environment Variables Status

| Item | Ready (yes/no) | Notes |
|------|----------------|-------|
| Database connection vars | | status only — **no values** |
| JWT/auth vars | | status only — **no values** |
| Tenant/demo seed vars | | status only — **no values** |
| LOW_CODE_ADMIN_AUTH_ENABLED | | planned yes/no only |

## Security Rules

- **Never** store SSH private keys in docs or repo
- **Never** store passwords in docs or repo
- **Never** store JWT/tokens in docs or repo
- **Never** store `.env` values in docs or repo
- Only **yes/no** status for secrets/env readiness
- UUIDs and public URLs may be recorded after Ops confirmation (no credentials)

## Required Before Auth-On Repeat

1. Server provisioned and reachable
2. Docker + Docker Compose installed
3. Domain/subdomain and HTTPS planned or active
4. `LOW_CODE_ADMIN_AUTH_ENABLED=true` planned for staging
5. Admin and non-admin test users available (credentials via secure channel)
6. Tenant ID available (UUID only — no secrets)
7. Read-only verification mode approved for repeat pack

## Forbidden Data

- SSH private keys
- Passwords
- JWT / tokens / API keys
- Raw `.env` contents
- Raw production data
- Production database connection strings with credentials

## Decision

**REMOTE_STAGING_DETAILS_INTAKE_FORM_CREATED_PENDING_INPUT**

## Next Steps

1. Ops / Platform / Staging owner completes this form (sanitized fields only)
2. Complete `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_READINESS_CHECKLIST_V0.1.md`
3. Trigger **Low-code Pilot Week-3 Remote Auth-On Staging Repeat Pack v0.1** when details provided

**PR-GAP-001 remains open** until remote auth-on repeat verification completes.

Reference: `LOW_CODE_PILOT_WEEK3_REMOTE_AUTH_ON_STAGING_PREREQUISITES_V0.1.md`
