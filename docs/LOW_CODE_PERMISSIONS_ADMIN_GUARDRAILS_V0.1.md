# Low-code Permissions & Admin Guardrails v0.1

## Summary

Service-level and UI guardrails for low-code **admin** operations. Runtime GET/PUT APIs (`custom-field-values`, public `form-templates`, audit) remain unchanged. Admin routes require `PLATFORM_ADMIN` when `LOW_CODE_ADMIN_AUTH_ENABLED=true`.

## Scope

**In scope**

- `services/low-code-service` — middleware on `/v1/low-code/admin/*`
- `services/identity-service` — expose `roles[]` on login and `/auth/me`
- `packages/shared-go/auth` — shared role constants and helper
- `apps/web-admin` — route middleware, nav filtering, migration button guards, `X-User-ID` header
- Docker Compose env for identity URL and admin auth toggle
- This document and `NEXT_COMMANDS.md` update

**Out of scope**

- Gateway JWT changes (uses existing `X-User-ID` injection when `AUTH_ENABLED=true`)
- Runtime custom field values authorization
- Fine-grained permissions beyond `PLATFORM_ADMIN`
- Tenant-level admin roles (future pack)

## Admin role (v0.1)

| Role | Access |
|------|--------|
| `PLATFORM_ADMIN` | All `/v1/low-code/admin/*` endpoints and admin UI |

Helper: `packages/shared-go/auth/lowcode_admin.go` → `HasLowCodeAdminRole(roles []string)`.

## Backend: low-code-service

### Configuration

| Env | Default | Purpose |
|-----|---------|---------|
| `IDENTITY_SERVICE_URL` | `http://identity-service:8081` | Role lookup base URL |
| `LOW_CODE_ADMIN_AUTH_ENABLED` | `false` | When `false`, admin routes behave as before (no role check) |

### Protected routes

Middleware: `internal/http/middleware/admin_auth.go` → `RequireLowCodeAdmin`.

| Route group | Methods |
|-------------|---------|
| `/v1/low-code/admin/form-templates` | POST, GET, PUT, publish, clone-to-draft |
| `/v1/low-code/admin/custom-field-values` | migration-preview, migrate-to-active, batch-* |

**Unchanged (no admin middleware):**

- `GET/PUT /v1/low-code/custom-field-values`
- `GET /v1/low-code/form-templates` (public)
- `GET /v1/low-code/audit-events`

### Auth flow

1. Read `X-User-ID` and `X-Tenant-ID` from request headers.
2. If `LOW_CODE_ADMIN_AUTH_ENABLED=false` → pass through.
3. If `X-User-ID` missing → `401 UNAUTHORIZED`.
4. Call identity: `GET {IDENTITY_SERVICE_URL}/v1/users/{userId}/roles?tenant_id={tenantId}`.
5. If user lacks `PLATFORM_ADMIN` → `403 FORBIDDEN`.

Identity client: `internal/platform/identity/client.go`.

### Error envelope

| Code | HTTP | When |
|------|------|------|
| `UNAUTHORIZED` | 401 | Missing `X-User-ID` when auth enabled |
| `FORBIDDEN` | 403 | User has no admin role |

## Backend: identity-service

Login and `/auth/me` responses include `roles: string[]` for the user in the current tenant.

Internal endpoint used by low-code-service:

```http
GET /v1/users/{id}/roles?tenant_id={uuid}
```

Returns role codes assigned to the user for that tenant.

## Frontend: web-admin

### Route guard

`middleware/low-code-admin.ts` — redirects non-admins from admin template pages to `/low-code` with toast `lowCode.adminAccessDenied`.

Applied on:

- `/low-code/admin/form-templates`
- `/low-code/admin/form-templates/new`
- `/low-code/admin/form-templates/[id]`

### Navigation and actions

- Low-code hub hides **Form template admin** link unless `isPlatformAdmin()`.
- Custom field values page hides migration / batch migration entry points unless `isPlatformAdmin()`.

### API headers

`composables/useApi.ts` sends `X-User-ID: {authStore.user.id}` on low-code requests (required when backend admin auth is enabled and gateway auth is off).

### Permissions composable

`composables/usePermissions.ts` reads `AuthUser.roles`. Dev fallback: mock auth + `admin@7rights.local` email grants admin for local UI testing.

## Enabling admin auth (pilot)

### Docker Compose

Set on `low-code-service`:

```yaml
LOW_CODE_ADMIN_AUTH_ENABLED: "true"
IDENTITY_SERVICE_URL: http://identity-service:8081
```

Ensure the calling user has `PLATFORM_ADMIN` in identity for the tenant.

### curl (direct to low-code-service, auth enabled)

```powershell
curl -X POST `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  -H "X-User-ID: <PLATFORM_ADMIN_USER_UUID>" `
  -H "Content-Type: application/json" `
  -d '{"entity_type":"TRANSPORT_ORDER","entity_id":"<ENTITY_ID>"}' `
  "http://localhost:8088/v1/low-code/admin/custom-field-values/migration-preview"
```

Without `X-User-ID` → `401`. With non-admin user → `403`.

### Gateway mode

When `AUTH_ENABLED=true`, the gateway sets `X-User-ID` from JWT; client-supplied values are overwritten.

## Verification

```powershell
cd D:\Projects\freight-platform
go test ./...  # in services/low-code-service and services/identity-service
cd apps\web-admin
npm run build
make health-check
make integration-smoke-test
```

With `LOW_CODE_ADMIN_AUTH_ENABLED=false` (default), existing smoke scripts and curl examples continue to work without `X-User-ID`.

## Recommended next pack

**Low-code Entity Integration v0.2** — deepen core-service `validation_context` wiring and expand automated frontend coverage for entity detail panels.
