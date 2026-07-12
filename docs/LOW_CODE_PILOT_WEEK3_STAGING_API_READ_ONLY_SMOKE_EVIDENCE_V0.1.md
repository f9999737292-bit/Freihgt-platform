# Low-code Pilot Week-3 Staging API Read-only Smoke Evidence v0.1

## Summary

Read-only API smoke executed against Selectel staging HTTP endpoint while DNS remains pending.

No writes, no JWT capture, no credentials stored. Full auth-on matrix not re-run in this smoke — anonymous and tenant-header GET checks only.

## Target

Staging URL:

```text
http://161.104.53.221
```

API base:

```text
http://161.104.53.221/api/v1
```

Low-code API:

```text
http://161.104.53.221/api/v1/low-code
```

Tenant header used:

```text
X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f
```

## Verification Matrix

| Test ID | Check | Method | Expected | Actual | Result |
| ------- | ----- | ------ | -------- | ------ | ------ |
| SMK-001 | Gateway health | GET `/health` | 200 | 200 | PASS |
| SMK-002 | Admin route without auth | GET `/api/v1/low-code/admin/form-templates?status=DRAFT` | 401/403 | 401 | PASS |
| SMK-003 | Runtime active template | GET `/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER` | 200 | 200 | PASS |
| SMK-004 | API gateway root | GET `/api/v1` | 200 or 404 | 404 | PASS (non-blocking) |
| SMK-005 | Low-code health path | GET `/api/v1/low-code/health` | 200 or N/A | 404 | PASS (endpoint not exposed) |
| SMK-006 | Audit events read | GET `/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&limit=1` | 200 | 200 | PASS |
| SMK-007 | Admin without tenant header | GET admin templates, no `X-Tenant-ID` | 400/401/403 | 400 | PASS |

## Decision

```text
STAGING_API_READ_ONLY_SMOKE_PASS
```

## Scope

Read-only GET smoke only.

Not executed in this smoke:

* Admin JWT authenticated matrix
* Non-admin JWT matrix
* POST / PUT / PATCH / DELETE
* Staging writes

## Production-ready

```text
not claimed
```

## Safety

Secrets captured:

```text
no
```

JWT/tokens captured:

```text
no
```

Writes executed:

```text
no
```

## DNS Status

```text
pending — smoke used IP endpoint
```

## STG-LIM-003

```text
OPEN — external port 22 scan deferred per operator
```

## Next Pack

```text
Web-admin Deploy Execution Pack v0.1 (after operator approval)
```
