# Low-code Pilot Week-3 Selectel SSH SG Post-Panel Verification Evidence v0.1

## Summary

Selectel SSH Security Group post-panel verification was executed from the trusted operator workstation.

Trusted-path checks passed (SSH, API health, runtime, UFW database and internal port denial). Selectel Security Group /32 restriction and non-trusted SSH rejection could **not** be independently confirmed. STG-LIM-003 remains open pending non-trusted rejection test or independent Selectel panel evidence.

## Target

Provider:

```text
Selectel
```

Server IP:

```text
161.104.53.221
```

Deployment path:

```text
/opt/bintrans/freight-platform
```

Controlled pilot:

```text
active
```

Production-ready claimed:

```text
no
```

## Verification Matrix

| Test ID | Check | Expected | Actual | Result |
| ------- | ----- | -------- | ------ | ------ |
| PP-SSH-SG-001 | Trusted SSH path | success | SSH_TRUSTED_PATH_OK | PASS |
| PP-SSH-SG-002 | SSH user / hostname | root / gpt-docker | root / gpt-docker | PASS |
| PP-SSH-SG-003 | API health GET | 200 | 200 | PASS |
| PP-SSH-SG-004 | Runtime containers healthy | 10 healthy | 10 healthy | PASS |
| PP-SSH-SG-005 | UFW 5432 denied | deny | deny | PASS |
| PP-SSH-SG-006 | UFW 6379 denied | deny | deny | PASS |
| PP-SSH-SG-007 | UFW internal ports denied | deny | deny | PASS |
| PP-SSH-SG-008 | UFW 22 status | ALLOW or SG-restricted | ALLOW Anywhere | NOTE |
| PP-SSH-SG-009 | Selectel SG panel changed manually | yes | unknown | PENDING |
| PP-SSH-SG-010 | TCP 22 /32 only in Selectel SG | yes | unknown | PENDING |
| PP-SSH-SG-011 | TCP 22 0.0.0.0/0 removed in Selectel SG | yes | unknown | PENDING |
| PP-SSH-SG-012 | Non-trusted SSH rejection | rejected | not_available | PENDING |
| PP-SSH-SG-013 | STG-LIM-003 closed | yes | no | FAIL |

## Trusted Path Verification

SSH from trusted operator workstation:

```text
pass
```

SSH user:

```text
root
```

Hostname:

```text
gpt-docker
```

OS:

```text
Ubuntu 24.04.4 LTS
```

SSH without operator private key (trusted workstation):

```text
Permission denied (publickey) — port 22 reachable from trusted source
```

Operator IP stored in docs:

```text
no
```

## API Verification

Health endpoint:

```text
http://161.104.53.221/health
```

Health status:

```text
200 / pass
```

## Runtime Verification

Containers healthy:

```text
10 healthy
```

Docker compose checked:

```text
attempted — docker compose ps reported no configuration file at deployment path; docker ps used as fallback
```

Container list (sanitized):

```text
freight_api_gateway, freight_low_code_service, freight_identity_service, freight_document_service, freight_rfx_service, freight_company_service, freight_transport_order_service, freight_shipment_service, freight_billing_register_service, freight_postgres — all Up (healthy)
```

Docker port binding note:

```text
internal service ports bound on host interfaces; UFW denies external access to 5432, 6379, 8080, 8088, 3000, 5173
```

## UFW Verification

UFW status checked:

```text
yes
```

UFW 5432 denied:

```text
yes
```

UFW 6379 denied:

```text
yes
```

UFW internal service ports denied:

```text
yes — 8080, 8088, 3000, 5173
```

UFW 80/443 allowed:

```text
yes
```

UFW 22 status:

```text
ALLOW Anywhere
```

Interpretation:

```text
UFW 22 ALLOW Anywhere is acceptable only when Selectel Security Group restricts SSH to trusted operator IP /32 — SG restriction not independently verified in this pack
```

## Selectel Security Group Verification

Selectel SG panel changed manually:

```text
unknown — no independent panel evidence captured in this pack
```

TCP 22 allowed only from trusted operator IP /32:

```text
unknown
```

TCP 22 open to 0.0.0.0/0 removed:

```text
unknown
```

## Non-Trusted Rejection Test

Non-trusted source available:

```text
no
```

Non-trusted SSH rejection result:

```text
not_available
```

External port-scan attempt:

```text
not_available — external check API unavailable from automation environment
```

## Decision

```text
SELECTEL_SSH_SG_TRUSTED_PATH_PASS_NON_TRUSTED_REJECTION_PENDING
```

## STG-LIM-003 Status

```text
OPEN_PENDING_NON_TRUSTED_REJECTION_TEST
```

## Production-ready Status

```text
not claimed
```

## Safety Confirmation

Remote SSH executed:

```text
yes — read-only verification commands only
```

Remote writes executed:

```text
no
```

API POST/PUT/PATCH/DELETE executed:

```text
no
```

Secrets captured:

```text
no
```

Operator IP stored in docs:

```text
no
```

SSH private key path/name stored in docs:

```text
no
```

## Next Recommended Event

```text
perform non-trusted SSH rejection test or capture independent Selectel SG panel evidence confirming TCP 22 /32 only and no 0.0.0.0/0 SSH allow
```

## Next Pack

```text
Selectel SSH SG Non-Trusted Rejection or Panel Evidence Pack v0.1
```
