# Low-code Pilot Week-3 Staging Limitations Review Note v0.1

## Summary

Remaining Selectel staging limitations reviewed after PR-GAP-001 closure.

These limitations are tracked separately from production readiness gaps.

## Decision

```text
STAGING_LIMITATIONS_REVIEWED_PRODUCTION_READY_NOT_CLAIMED
```

## Limitations

| ID | Limitation | Status | Priority |
| -- | ---------- | ------ | -------- |
| STG-LIM-001 | HTTP-only IP access | OPEN | P1 |
| STG-LIM-002 | HTTPS / Certbot not configured | OPEN | P1 |
| STG-LIM-003 | SSH 22 Selectel Security Group restriction pending | OPEN | P0 |
| STG-LIM-004 | Web-admin UI not deployed | OPEN | P2 |
| STG-LIM-005 | Full demo UI seed-data not executed | OPEN | P3 |
| STG-LIM-006 | seed-lowcode-demo custom field values skipped | OPEN | P3 |

## What Is Sufficient for Controlled Pilot

* API reachable at http://161.104.53.221
* Remote auth-on verified (PR-GAP-001 closed)
* Runtime healthy with migrations applied
* Internal ports not exposed externally

## What Is Not Sufficient for Production-ready Claim

* HTTP-only access without TLS
* SSH not restricted at provider Security Group level
* No web-admin UI on staging
* No production readiness decision pack with updated evidence

## Production-ready Status

```text
not claimed
```

## Controlled Pilot

```text
continues
```

## Recommended First Action

```text
Selectel SSH Security Group Restriction Pack v0.1
```
