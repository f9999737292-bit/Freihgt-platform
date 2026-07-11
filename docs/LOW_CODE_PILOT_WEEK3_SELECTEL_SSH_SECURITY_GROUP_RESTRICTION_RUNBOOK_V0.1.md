# Low-code Pilot Week-3 Selectel SSH Security Group Restriction Runbook v0.1

## Summary

Runbook for restricting SSH port 22 on the Selectel staging server using Security Group rules.

This document is preparation only. It does not constitute execution evidence.

## Target

Provider:

```text
Selectel
```

Server public IP:

```text
161.104.53.221
```

Deployment path:

```text
/opt/bintrans/freight-platform
```

Staging limitation:

```text
STG-LIM-003
```

## Objective

Restrict inbound SSH (TCP 22) at the Selectel Security Group level to trusted operator IP addresses only.

UFW on the VM is not sufficient for this limitation closure — provider Security Group restriction is required.

## Prerequisites

* Selectel control panel access with permission to edit Security Groups
* Trusted operator public IP address identified (not stored in this doc)
* Selectel Console access retained as break-glass fallback
* No production-ready claim

## Required Security Group Rules

### Inbound — allow

| Protocol | Port | Source | Purpose |
| -------- | ---- | ------ | ------- |
| TCP | 22 | trusted operator IP /32 only | SSH administration |
| TCP | 80 | 0.0.0.0/0 | HTTP staging API |
| TCP | 443 | 0.0.0.0/0 | HTTPS when configured |

### Inbound — deny / do not expose

| Protocol | Port | Notes |
| -------- | ---- | ----- |
| TCP | 5432 | PostgreSQL — must remain closed externally |
| TCP | 6379 | Redis — must remain closed externally |
| TCP | 8080 | API gateway — internal only |
| TCP | 8088 | low-code-service — internal only |
| TCP | 3000 | web-admin dev — internal only |
| TCP | 5173 | web-admin dev — internal only |

## Procedure (Selectel Panel)

1. Log in to Selectel control panel.
2. Open the cloud server attached to `161.104.53.221`.
3. Identify the Security Group bound to this server.
4. Review current inbound rules for port 22.
5. Remove or replace any rule allowing SSH from `0.0.0.0/0` or overly broad CIDR.
6. Add inbound TCP 22 rule limited to trusted operator IP /32.
7. If multiple operators require access, add separate /32 rules — do not use broad ranges without approval.
8. Confirm PostgreSQL 5432 and Redis 6379 are not open in Security Group.
9. Confirm HTTP 80 and HTTPS 443 remain open as required for staging API.
10. Save Security Group changes.
11. Verify SSH access from trusted IP succeeds.
12. Verify SSH from non-trusted IP is rejected or times out.
13. Verify staging API remains reachable: `http://161.104.53.221/api/v1/health` (read-only GET).
14. Record sanitized evidence in execution evidence doc — no secrets, no IPs in repo unless separately approved.

## Break-glass

If SSH is locked out:

* Use Selectel Console (VNC/serial) to restore access
* Do not publish credentials or private keys in docs

## Forbidden

* Do not open SSH to 0.0.0.0/0
* Do not open PostgreSQL or Redis externally
* Do not store operator IP, passwords, SSH keys, or .env values in docs
* Do not claim production-ready
* Do not execute changes without explicit operator approval

## Verification (after execution)

| Check | Expected |
| ----- | -------- |
| SSH from trusted IP | success |
| SSH from non-trusted IP | rejected / timeout |
| HTTP API health | 200 |
| STG-LIM-003 status | mitigated pending evidence review |

## Decision (this runbook)

```text
SELECTEL_SSH_SG_RESTRICTION_RUNBOOK_PREPARED_PENDING_EXECUTION
```

## Production-ready Status

```text
not claimed
```

## Next Step After Execution

Capture evidence in Selectel SSH Security Group Restriction Execution Evidence doc and update STG-LIM-003 status.
