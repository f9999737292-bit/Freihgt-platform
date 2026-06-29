# Low-code Pilot Week-3 Remote Staging Readiness Checklist v0.1

## Summary

Readiness checklist for remote staging environment before PR-GAP-001 auth-on repeat. **Docs-only** — no deploy, no secrets in repo.

Reference: `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_DETAILS_INTAKE_FORM_V0.1.md`

**Decision:** **REMOTE_STAGING_DETAILS_INTAKE_FORM_CREATED_PENDING_INPUT**

## Readiness Checklist

| Item | Status | Evidence | Notes |
|------|--------|----------|-------|
| Server provisioned | **PENDING** | Intake Form v0.1 | Awaiting real details |
| Ubuntu 24.04 LTS x64 confirmed | **PENDING** | Intake Form v0.1 | OS field |
| Public IP available | **PENDING** | Intake Form v0.1 | |
| Domain/subdomain assigned | **PENDING** | Intake Form v0.1 | |
| SSH key access available | **PENDING** | Intake Form v0.1 | keys not stored in docs |
| Sudo/root access available | **PENDING** | Intake Form v0.1 | |
| Firewall enabled | **PENDING** | Intake Form v0.1 | |
| Ports 80/443 available | **PENDING** | Intake Form v0.1 | |
| SSH restricted by IP or policy | **PENDING** | Intake Form v0.1 | |
| PostgreSQL external port closed | **PENDING** | Intake Form v0.1 | |
| Docker installed | **PENDING** | Intake Form v0.1 | |
| Docker Compose installed | **PENDING** | Intake Form v0.1 | |
| Repository cloned | **PENDING** | Intake Form v0.1 | |
| Branch main selected | **PENDING** | Intake Form v0.1 | |
| .env staging prepared without storing secrets | **PENDING** | Intake Form v0.1 | yes/no status only |
| LOW_CODE_ADMIN_AUTH_ENABLED=true planned | **PENDING** | Intake Form v0.1 | |
| HTTPS planned | **PENDING** | Intake Form v0.1 | |
| Backup/snapshot enabled | **PENDING** | Intake Form v0.1 | |
| No secrets stored in docs | **PASS** | This pack | Explicit rule enforced |

## Status Legend

| Status | Meaning |
|--------|---------|
| **PASS** | Complete / verified |
| **PENDING** | Awaiting real staging input |
| **BLOCKED** | Cannot proceed |
| **NOT_APPLICABLE** | Does not apply |

## Decision

**REMOTE_STAGING_DETAILS_INTAKE_FORM_CREATED_PENDING_INPUT**

## Next Pack

**Low-code Pilot Week-3 Remote Auth-On Staging Repeat Pack v0.1** (after staging details provided)
