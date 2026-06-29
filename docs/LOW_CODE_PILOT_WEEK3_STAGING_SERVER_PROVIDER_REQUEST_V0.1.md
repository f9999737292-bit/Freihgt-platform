# Low-code Pilot Week-3 Staging Server Provider Request v0.1

## Request Summary

Formal request template to order or allocate a **remote staging server** for low-code Week-3 controlled pilot (PR-GAP-001). **Docs-only** — submit via ticket/email; do not commit provider credentials.

Reference: `LOW_CODE_PILOT_WEEK3_STAGING_SERVER_REQUIREMENTS_V0.1.md`

**Decision:** **STAGING_SERVER_PROVIDER_REQUEST_CREATED**

## Server Configuration To Order

| Field | Requested value |
|-------|-----------------|
| Type | VPS / VDS / Cloud VM |
| Purpose | Remote staging — low-code controlled pilot |
| OS | Ubuntu Server 24.04 LTS x64 |
| CPU | **8 vCPU** (minimum 4 vCPU) |
| RAM | **32 GB** (minimum 16 GB) |
| Disk | **500 GB NVMe SSD** (minimum 200 GB NVMe) |
| Network | 1 Gbps |
| Public IPv4 | Required |
| Backup | Daily snapshot, 7–14 days retention |

## Required Access

- SSH key access (public key provided via secure channel — **not in repo**)
- Sudo or root access for Docker installation
- Named ops contact for handoff

## Required Network Settings

- Firewall / security group enabled
- Inbound: 22 (SSH, IP-restricted preferred), 80, 443
- **Outbound:** internet for updates and Docker pulls
- **Block public inbound:** 5432, 6379, 8080, 8088, 3000, 5173

## Required Backup Settings

- Daily automated snapshot or backup
- Retention 7–14 days
- Confirm restore procedure exists (document yes/no only)

## Required Security Settings

- SSH key auth preferred
- No default weak passwords documented in tickets
- PostgreSQL and Redis **not** exposed to `0.0.0.0/0`
- Staging isolated from production network unless explicitly approved

## Information Provider Must Return

Provider/Ops must return (sanitized — safe for repo intake form):

| Field | Required |
|-------|----------|
| Provider name | yes |
| Public IP | yes |
| Server hostname | yes |
| OS version | yes |
| CPU / RAM / Disk | yes |
| SSH user | yes |
| Sudo/root availability | yes (yes/no) |
| Backup/snapshot status | yes (enabled yes/no) |
| Firewall/security group status | yes (enabled yes/no) |
| Opened ports | yes (list public ports only) |
| Assigned domain/subdomain | if ready |

## Forbidden Data

Do **not** include in repo, tickets copied to repo, or this document:

- Passwords
- SSH private keys
- Tokens / JWT
- `.env` values
- Database credentials
- Raw production data

## Decision

**STAGING_SERVER_PROVIDER_REQUEST_CREATED**

## Next Step

On server delivery → complete **Staging Server Acceptance Checklist v0.1** and **Remote Staging Details Intake Form v0.1**
