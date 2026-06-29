# Low-code Pilot Week-3 Staging Server Requirements v0.1

## Summary

Defines **recommended and minimum server profiles** for remote staging to support PR-GAP-001 auth-on repeat. **Docs-only** — no provisioning executed.

**Decision:** **STAGING_SERVER_REQUIREMENTS_CREATED_PENDING_PROVISIONING**

**PR-GAP-001:** **REMOTE_STAGING_SERVER_REQUIREMENTS_CREATED_PENDING_PROVISIONING**

## Purpose

Provide Ops / Platform with standardized server requirements before ordering or accepting a staging VM for the low-code controlled pilot.

Reference: `LOW_CODE_PILOT_WEEK3_STAGING_SERVER_PROVIDER_REQUEST_V0.1.md`, `LOW_CODE_PILOT_WEEK3_STAGING_SERVER_ACCEPTANCE_CHECKLIST_V0.1.md`

## Recommended Server Profile

| Field | Requirement |
|-------|-------------|
| Type | VPS / VDS / Cloud VM |
| Purpose | Remote staging / controlled pilot |
| OS | Ubuntu Server 24.04 LTS x64 |
| CPU | 8 vCPU |
| RAM | 32 GB |
| Disk | 500 GB NVMe SSD |
| Network | 1 Gbps |
| Public IPv4 | **required** |
| Backup | Daily snapshot, 7–14 days retention |
| Access | SSH key, sudo/root access |

## Minimum Server Profile

| Field | Requirement |
|-------|-------------|
| CPU | 4 vCPU |
| RAM | 16 GB |
| Disk | 200 GB NVMe SSD |
| OS | Ubuntu Server 24.04 LTS x64 |

Below minimum is **not recommended** for full platform stack (PostgreSQL, Redis, API gateway, low-code-service, web-admin build).

## Operating System

- **Ubuntu Server 24.04 LTS x64** only for this pack baseline
- Security updates enabled
- UTC timezone acceptable; document local TZ if different

## Network Requirements

- Stable public IPv4 address
- Outbound internet for Docker image pulls and package updates
- Inbound 80/443 for HTTPS staging access
- SSH (22) — preferably restricted by source IP or VPN policy

## Security Requirements

- Firewall enabled (UFW, cloud security group, or equivalent)
- SSH key authentication preferred; password-only SSH **not recommended**
- No public exposure of database or internal service ports
- Sudo/root access for initial Docker setup only — document who holds access
- **No secrets stored in repo docs**

## Docker Requirements

- Docker Engine installed (planned before deploy)
- Docker Compose v2 installed (planned before deploy)
- Sufficient disk for images and volumes (see storage section)

## Storage / Backup Requirements

- NVMe SSD preferred for database and container I/O
- Daily snapshot or backup enabled
- Retention: 7–14 days minimum
- Document snapshot provider feature (yes/no) — no credentials in docs

## Domain / SSL Requirements

- Domain or subdomain assigned for staging (e.g. `staging.example.com`)
- HTTPS via Let's Encrypt or provider SSL — planned before auth-on repeat
- HTTP (80) allowed temporarily for certificate issuance only

## Ports Policy

### Public ports (allowed)

| Port | Service | Notes |
|------|---------|-------|
| 22 | SSH | Prefer IP-restricted |
| 80 | HTTP | SSL setup / redirect to 443 |
| 443 | HTTPS | Staging web-admin and API via reverse proxy |

### Must not be exposed publicly

| Port | Service |
|------|---------|
| 5432 | PostgreSQL |
| 6379 | Redis |
| 8080 | API gateway (direct) |
| 8088 | low-code-service (direct) |
| 3000 / 5173 | web-admin dev ports |
| Other internal service ports | Platform microservices |

Use reverse proxy on 443 only for external access.

## What Must Not Be Exposed

- Database credentials in documentation
- Raw production data on staging server
- Production network peering without approval
- Unauthenticated admin endpoints on public internet without auth-on plan

## Acceptance Criteria

Server acceptable for next phase when:

1. Meets **minimum** profile (or documented exception approved)
2. Ubuntu 24.04 LTS x64 confirmed
3. Public IPv4 and domain/subdomain planned or assigned
4. SSH key + sudo/root access confirmed (no keys in repo)
5. Firewall enabled; DB ports not public
6. Docker + Compose installation planned or complete
7. Backup/snapshot enabled
8. Details captured in intake form (sanitized)

## Decision

**STAGING_SERVER_REQUIREMENTS_CREATED_PENDING_PROVISIONING**

## Next Steps

1. Submit `LOW_CODE_PILOT_WEEK3_STAGING_SERVER_PROVIDER_REQUEST_V0.1.md` to provider/Ops
2. Complete `LOW_CODE_PILOT_WEEK3_STAGING_SERVER_ACCEPTANCE_CHECKLIST_V0.1.md` on delivery
3. Fill `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_DETAILS_INTAKE_FORM_V0.1.md` (no secrets)
4. Trigger **Remote Auth-On Staging Repeat Pack v0.1**

**Production-ready claimed:** **no**

**Controlled pilot:** **CONTROLLED_PILOT_APPROVED** — active
