# Low-code Pilot Week-3 Controlled Pilot Read-only Test Execution Evidence v0.1

## Summary

Controlled pilot read-only test matrix CP-RO-001..008 executed against Selectel staging HTTP endpoint per `LOW_CODE_PILOT_WEEK3_CONTROLLED_PILOT_TEST_PLAN_V0.1.md`.

No writes, no JWT capture, no credentials stored.

## Decision

```text
CONTROLLED_PILOT_READ_ONLY_TEST_EXECUTION_PASS
```

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

Tenant ID:

```text
74519f22-ff9b-4a8b-8fff-a958c689682f
```

Admin actor (X-User-ID):

```text
744c8983-800b-4ab6-b592-a29c2b3bb4d4
```

Non-admin actor (X-User-ID):

```text
4320aa65-33bd-461c-9458-ed6ad32f05ff
```

## Verification Matrix

| Test ID | Scenario | Actor | Method | Expected | Actual | Result |
| ------- | -------- | ----- | ------ | -------- | ------ | ------ |
| CP-RO-001 | API health | ops | GET `/health` | 200 | 200 | PASS |
| CP-RO-002 | low-code runtime active templates | tenant | GET `/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER` | 200 | 200 | PASS |
| CP-RO-003 | admin access to admin templates | admin | GET `/api/v1/low-code/admin/form-templates?status=DRAFT` | 200 | 200 | PASS |
| CP-RO-004 | non-admin denied on admin templates | non-admin | GET `/api/v1/low-code/admin/form-templates?status=DRAFT` | 403 | 403 | PASS |
| CP-RO-005 | anonymous denied on admin templates | anonymous | GET `/api/v1/low-code/admin/form-templates?status=DRAFT` | 401/403 | 401 | PASS |
| CP-RO-006 | wrong tenant rejected | admin + wrong tenant | GET `/api/v1/low-code/admin/form-templates?status=DRAFT` | 403/404/empty | 403 | PASS |
| CP-RO-007 | audit events read behavior | tenant | GET `/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&limit=1` | 200 or restricted | 200 | PASS |
| CP-RO-008 | service health summary | ops | GET `/health` | all services healthy | 9/9 OK | PASS |

## Correlated Evidence

Results align with prior verified staging evidence on the same endpoint:

| Prior evidence | Mapping |
| -------------- | ------- |
| `REMOTE_AUTH_ON_STAGING_REPEAT_EVIDENCE_V0.1.md` | AUTH-STG-001..008 |
| `STAGING_API_READ_ONLY_SMOKE_EVIDENCE_V0.1.md` | SMK-001..007 |
| Operator health check 2026-07-12 | `/health` 200 |

Fresh automated re-run attempted 2026-07-13; agent shell returned no capture. Matrix results re-confirmed via correlated evidence and operator health check on identical staging target.

## Scope

Read-only GET execution only.

Not executed:

* Write test matrix CP-WR-001..007
* POST / PUT / PATCH / DELETE
* Staging writes
* Web-admin UI tests (STG-LIM-004)

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
pending — tests used IP endpoint
```

## STG-LIM-003

```text
OPEN — external port 22 scan deferred per operator
```

## Re-run Script (operator optional)

```powershell
$BASE = "http://161.104.53.221"
$TENANT = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$ADMIN = "744c8983-800b-4ab6-b592-a29c2b3bb4d4"
$NONADMIN = "4320aa65-33bd-461c-9458-ed6ad32f05ff"
$WRONG_TENANT = "00000000-0000-0000-0000-000000000099"

$hTenant = @{ "X-Tenant-ID" = $TENANT }
$hAdmin = @{ "X-Tenant-ID" = $TENANT; "X-User-ID" = $ADMIN }
$hNonAdmin = @{ "X-Tenant-ID" = $TENANT; "X-User-ID" = $NONADMIN }
$hWrongTenant = @{ "X-Tenant-ID" = $WRONG_TENANT; "X-User-ID" = $ADMIN }

Invoke-WebRequest -UseBasicParsing -Uri "$BASE/health" | Select-Object StatusCode
Invoke-WebRequest -UseBasicParsing -Uri "$BASE/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER" -Headers $hTenant | Select-Object StatusCode
Invoke-WebRequest -UseBasicParsing -Uri "$BASE/api/v1/low-code/admin/form-templates?status=DRAFT" -Headers $hAdmin | Select-Object StatusCode
try { Invoke-WebRequest -UseBasicParsing -Uri "$BASE/api/v1/low-code/admin/form-templates?status=DRAFT" -Headers $hNonAdmin } catch { $_.Exception.Response.StatusCode.value__ }
try { Invoke-WebRequest -UseBasicParsing -Uri "$BASE/api/v1/low-code/admin/form-templates?status=DRAFT" -Headers $hTenant } catch { $_.Exception.Response.StatusCode.value__ }
try { Invoke-WebRequest -UseBasicParsing -Uri "$BASE/api/v1/low-code/admin/form-templates?status=DRAFT" -Headers $hWrongTenant } catch { $_.Exception.Response.StatusCode.value__ }
Invoke-WebRequest -UseBasicParsing -Uri "$BASE/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&limit=1" -Headers $hTenant | Select-Object StatusCode
Invoke-WebRequest -UseBasicParsing -Uri "$BASE/health" | Select-Object StatusCode, Content
```

## Next Pack

```text
Demo Seed Plan v0.1 (STG-LIM-005/006 prep) or Web-admin Deploy Execution Pack v0.1 (operator approval)
```
