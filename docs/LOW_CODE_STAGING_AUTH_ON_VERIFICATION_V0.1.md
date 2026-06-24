# Low-code Staging Auth-On Verification v0.1

## Summary

Verified `LOW_CODE_ADMIN_AUTH_ENABLED=true` on `low-code-service` in local Docker: admin low-code endpoints require `X-User-ID` and `PLATFORM_ADMIN`; runtime read/write endpoints remain tenant-scoped without admin guard. Default-off dev mode restored and re-verified. **Docs-only pack** — no backend logic, API contract, or migration changes.

## Current Commit

| Field | Value |
|-------|-------|
| Baseline commit | `da5af8e` — `docs: add low-code runtime pilot staging checklist` |
| Verification date | 2026-06-24 |
| Branch | `main` |

## Scope

**In scope**

- Pre-flight git/health baseline
- Default-off smoke (admin + runtime without `X-User-ID`)
- Temporary auth-on via local Docker Compose override (gitignored)
- Admin endpoint negative/positive checks
- Runtime endpoint compatibility with auth-on enabled
- Restore default-off + integration smoke
- Existing `admin_auth_test.go` coverage confirmation

**Out of scope**

- Permanent compose/env changes
- Backend guard logic changes
- New migrations
- Production/staging deployment

## Default-off Baseline

Pre-flight:

```text
git status --short   → clean
git log --oneline -12 → da5af8e at HEAD
make health-check    → OK (all services)
make seed-lowcode-demo → OK (templates + demo values present)
```

Curl without `X-User-ID` (tenant `74519f22-ff9b-4a8b-8fff-a958c689682f`):

| Endpoint | HTTP |
|----------|------|
| `GET .../form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default` | 200 |
| `GET .../admin/form-templates` | 200 |

## Auth-on Setup

**Method:** Temporary local override (gitignored, not committed):

`infrastructure/docker-compose/docker-compose.override.yml`:

```yaml
services:
  low-code-service:
    environment:
      LOW_CODE_ADMIN_AUTH_ENABLED: "true"
```

**Restart:**

```powershell
docker compose -f infrastructure/docker-compose/docker-compose.yml `
  -f infrastructure/docker-compose/docker-compose.override.yml up -d --no-build low-code-service
make health-check
```

Tracked `docker-compose.yml` remains `LOW_CODE_ADMIN_AUTH_ENABLED: "false"`. Override file removed after verification.

## Admin Endpoint Checks

Gateway: `http://localhost:8080/api/v1/low-code/admin/...`  
Tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f`

| Endpoint | No user | Non-admin | PLATFORM_ADMIN |
|----------|---------|-----------|----------------|
| `GET /admin/form-templates` | 401 | 403 | 200 |
| `GET /admin/form-templates/{id}/export` | 401 | — | 200 |
| `POST /admin/custom-field-values/migration-preview` | 401 | — | 200 |
| `POST /admin/custom-field-values/batch-migration-preview` | 401 | — | 200 |
| `POST /admin/form-templates/import-preview` | 401 | — | 200 |

**401 body (no user):** `UNAUTHORIZED` — `authenticated user is required for low-code admin operations`  
**403 body (non-admin):** `FORBIDDEN` — `low-code admin access required`

Template ID for export: `b1111111-1111-4111-8111-111111111102`  
Payloads: `scripts/dev/payloads/migration-edge-cases/safe_transport_order.json`, `scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json`, `scripts/dev/payloads/template-import-export-edge-cases/new_version_request.json`

## Runtime Endpoint Checks

With auth-on enabled, runtime routes (no `adminGuard` in `router.go`) behave as designed:

| Endpoint | No `X-User-ID` | Notes |
|----------|----------------|-------|
| `GET .../form-templates/active` | 200 | Unchanged |
| `GET .../form-templates?entity_type=TRANSPORT_ORDER` | 200 | Public PUBLISHED list |
| `GET .../form-templates/{id}` | 200 | Public detail |
| `GET .../custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=...` | 200 | Tenant-scoped |
| `PUT .../custom-field-values` | Not guard-protected | Malformed body → 400 `VALIDATION_ERROR` (not 401/403); confirms route is outside `RequireLowCodeAdmin` |

Runtime PUT was not executed with valid payload (read-only verification policy). Router registers `PUT /custom-field-values` outside the admin sub-router guarded by `RequireLowCodeAdmin`.

## PLATFORM_ADMIN Check

| Field | Value |
|-------|-------|
| User ID | `8541a3a3-bde7-4fed-9501-37b9953bf904` |
| Email | `admin@7rights.local` |
| Role | `PLATFORM_ADMIN` |

All five admin endpoints returned **200** with `X-User-ID` + `X-Tenant-ID`.

## DRIVER/Non-admin Check

Dev tenant has no dedicated `DRIVER` login user. Negative test used **shipper** (non-admin):

| Field | Value |
|-------|-------|
| User ID | `008e1462-6f67-4246-b7dc-4aae1669c0c5` |
| Email | `shipper@7rights.local` |
| Role | `SHIPPER_LOGIST` |

`GET /admin/form-templates` → **403 FORBIDDEN** (same guard path as `DRIVER` per `admin_auth_test.go`).

## Restore Default-off

1. Deleted `infrastructure/docker-compose/docker-compose.override.yml`
2. `make platform-up-no-build` (recreated `low-code-service` with compose default `false`)
3. `make health-check` → OK
4. `GET /admin/form-templates` without `X-User-ID` → **200**
5. `make integration-smoke-test` → **SMOKE TEST PASSED**

## Verification Results

| Check | Result |
|-------|--------|
| Pre-flight clean tree | Pass |
| Default-off baseline | Pass |
| Auth-on temporary enable | Pass (override, not committed) |
| Admin without user → 401 | Pass |
| Non-admin → 403 | Pass |
| PLATFORM_ADMIN → 200 | Pass |
| Runtime endpoints in auth-on | Pass (unchanged) |
| Default-off restored | Pass |
| `make integration-smoke-test` | Pass |
| `go test ./...` (low-code-service) | Pass |
| `npm run build` (web-admin) | Pass |
| `make health-check` | Pass |

## Issues Found

None blocking pilot auth-on. Minor note: dev demo seed has no `DRIVER` user — use `shipper@7rights.local` or create a driver user for pilot negative tests.

## Recommended Staging Settings

For pilot/staging only on `low-code-service`:

```yaml
LOW_CODE_ADMIN_AUTH_ENABLED: "true"
IDENTITY_SERVICE_URL: http://identity-service:8081
```

Keep **default-off** (`false`) in tracked compose for local dev smoke. Use environment-specific override or deployment config for staging.

Ensure gateway/web-admin forwards `X-User-ID` from authenticated sessions for admin UI routes.

## Next Action

**Low-code Pilot Go/No-Go Review Pack v0.1** — no auth blockers found.

Unit tests already cover guard behavior in `services/low-code-service/internal/http/middleware/admin_auth_test.go` (disabled pass-through, 401 missing user, 403 DRIVER/SHIPPER_ADMIN, 200 PLATFORM_ADMIN). No new tests added.
