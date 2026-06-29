# Low-code Pilot Week-3 Staging Server Acceptance Checklist v0.1

## Summary

Acceptance checklist for delivered staging server before deploy and auth-on repeat (PR-GAP-001). **Docs-only** — verify with Ops; no secrets in repo.

Reference: `LOW_CODE_PILOT_WEEK3_STAGING_SERVER_REQUIREMENTS_V0.1.md`

**Decision:** **STAGING_SERVER_REQUIREMENTS_CREATED_PENDING_PROVISIONING**

## Acceptance Checklist

| Item | Status | Evidence | Notes |
|------|--------|----------|-------|
| Server provisioned | **PENDING** | Provider Request v0.1 | Awaiting delivery |
| Ubuntu 24.04 LTS x64 confirmed | **PENDING** | Intake Form v0.1 | |
| CPU >= 8 vCPU preferred | **PENDING** | Intake Form v0.1 | min 4 vCPU |
| RAM >= 32 GB preferred | **PENDING** | Intake Form v0.1 | min 16 GB |
| Disk >= 500 GB NVMe preferred | **PENDING** | Intake Form v0.1 | min 200 GB |
| Public IPv4 available | **PENDING** | Intake Form v0.1 | |
| SSH key access available | **PENDING** | Intake Form v0.1 | keys not in docs |
| Sudo/root access available | **PENDING** | Intake Form v0.1 | |
| Firewall enabled | **PENDING** | Intake Form v0.1 | |
| Ports 80/443 available | **PENDING** | Intake Form v0.1 | |
| SSH restricted by IP or policy | **PENDING** | Intake Form v0.1 | |
| PostgreSQL external port closed | **PENDING** | Intake Form v0.1 | |
| Docker installation planned | **PENDING** | Requirements v0.1 | |
| Docker Compose installation planned | **PENDING** | Requirements v0.1 | |
| Domain/subdomain planned | **PENDING** | Intake Form v0.1 | |
| HTTPS planned | **PENDING** | Intake Form v0.1 | |
| Daily backup/snapshot enabled | **PENDING** | Intake Form v0.1 | |
| No secrets stored in docs | **PASS** | This pack | Explicit rule |

## Status Legend

| Status | Meaning |
|--------|---------|
| **PASS** | Verified |
| **PENDING** | Awaiting server delivery / input |
| **BLOCKED** | Cannot proceed |
| **NOT_APPLICABLE** | Does not apply |

## Decision

**STAGING_SERVER_REQUIREMENTS_CREATED_PENDING_PROVISIONING**

## Next Pack

**Low-code Pilot Week-3 Remote Auth-On Staging Repeat Pack v0.1** (after server provisioned and details provided)
