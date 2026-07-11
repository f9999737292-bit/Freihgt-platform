# Low-code Pilot Week-3 Staging Hardening and Production Readiness Review v0.1

## Summary

Post-PR-GAP-001 closure review of Selectel staging hardening and remaining production readiness limitations.

Remote auth-on verification is complete. This review tracks open staging limitations separately from closed production readiness gaps.

## Decision

```text
STAGING_HARDENING_AND_PRODUCTION_READINESS_REVIEW_COMPLETED
```

## Production-ready Status

```text
not claimed
```

## Controlled Pilot

```text
continues
```

## Staging Target

Provider:

```text
Selectel
```

Server IP:

```text
161.104.53.221
```

Staging URL:

```text
http://161.104.53.221
```

API URL:

```text
http://161.104.53.221/api/v1
```

Low-code API URL:

```text
http://161.104.53.221/api/v1/low-code
```

Deployment path:

```text
/opt/bintrans/freight-platform
```

Deploy commit SHA:

```text
8c8ecfe
```

Tenant code:

```text
dev-bintrans
```

## Verified Baseline (from PR-GAP-001 evidence)

| Item | Status |
| ---- | ------ |
| Platform runtime healthy | PASS — 10 containers |
| Health check | PASS — 9/9 services |
| Migrations | PASS — 11/11 applied |
| Remote auth-on matrix | PASS — CORE_MATRIX_PASS=yes, FULL_MATRIX_PASS=yes |
| LOW_CODE_ADMIN_AUTH_ENABLED | PASS — true |
| Read-only GET verification | PASS |
| No secrets captured | PASS |
| No writes in verification | PASS |

## Network Hardening Review

| Item | Required | Current | Status | Notes |
| ---- | -------- | ------- | ------ | ----- |
| HTTP 80 API via Nginx | yes | yes | PASS | Reverse proxy to gateway |
| HTTPS 443 / TLS | recommended | no | OPEN | No domain configured; Certbot not applied |
| SSH 22 UFW allow | yes | yes | PASS | Required for operator access |
| SSH 22 Selectel Security Group restriction | yes | no | OPEN | Trusted IP restriction pending |
| PostgreSQL 5432 closed externally | yes | yes | PASS | UFW deny |
| Redis 6379 closed externally | yes | yes | PASS | UFW deny |
| API gateway 8080 closed externally | yes | yes | PASS | UFW deny; Nginx front door only |
| low-code-service 8088 closed externally | yes | yes | PASS | UFW deny |
| web-admin ports 3000/5173 closed externally | yes | yes | PASS | UFW deny |

## Runtime and Access Review

| Item | Required | Current | Status | Notes |
| ---- | -------- | ------- | ------ | ----- |
| Docker installed | yes | yes | PASS | |
| Docker Compose installed | yes | yes | PASS | |
| Repo cloned at deployment path | yes | yes | PASS | /opt/bintrans/freight-platform |
| Branch main | yes | yes | PASS | deploy SHA 8c8ecfe |
| staging .env prepared | yes | yes | PASS | values not stored in docs |
| API gateway reachable externally | yes | yes | PASS | http://161.104.53.221 |
| low-code route reachable | yes | yes | PASS | via gateway |
| Web-admin UI deployed | recommended | no | OPEN | Not deployed on staging |
| Public DNS/domain | optional | no | OPEN | IP-only staging by decision |
| Full demo UI seed-data | optional | no | OPEN | seed-demo-data not executed |

## Staging Limitations Tracker

| Limitation ID | Item | Status | Blocks production-ready | Next action |
| ------------- | ---- | ------ | ----------------------- | ----------- |
| STG-LIM-001 | HTTP-only IP access | OPEN | yes | Configure domain + HTTPS when approved |
| STG-LIM-002 | HTTPS / Certbot | OPEN | yes | Requires staging domain decision |
| STG-LIM-003 | SSH 22 Selectel Security Group restriction | OPEN | yes | Restrict SSH to trusted IPs in Selectel SG |
| STG-LIM-004 | Web-admin UI deploy | OPEN | yes | Deploy web-admin behind Nginx when approved |
| STG-LIM-005 | Full demo UI seed-data | OPEN | no | Run seed-demo-data when UI testing approved |
| STG-LIM-006 | seed-lowcode-demo custom field values | OPEN | no | Requires demo entities present |

## Production Readiness Gap Status

| Gap | Status |
| --- | ------ |
| PR-GAP-001 | CLOSED_APPROVED_BY_OWNER_REMOTE_AUTH_ON_VERIFIED |
| PR-GAP-002–008, PR-GAP-010 | CLOSED_APPROVED_BY_OWNER |
| PR-GAP-009 | OWNER_APPROVED_BUT_PRODUCTION_READY_NOT_CLAIMED |
| Open production gaps | 0 |

## Safety Confirmation

Backend code changed:

```text
no
```

Frontend code changed:

```text
no
```

API contracts changed:

```text
no
```

Migrations created:

```text
no
```

Remote SSH executed in this pack:

```text
no
```

Staging writes executed:

```text
no
```

Secrets captured:

```text
no
```

Production-ready claimed:

```text
no
```

## Review Conclusion

PR-GAP-001 closure evidence remains valid. Staging is operational for controlled pilot API verification.

Production-ready cannot be claimed while STG-LIM-001 through STG-LIM-004 remain open.

## Recommended Next Packs

1. Selectel SSH Security Group Restriction Pack v0.1
2. Staging HTTPS and Domain Decision Pack v0.1 (when domain approved)
3. Staging Web-admin Deploy Pack v0.1
4. Staging Demo Seed-data Pack v0.1 (optional, UI testing)

## Next Recommended Event

```text
address STG-LIM-003 SSH Security Group restriction first
```
