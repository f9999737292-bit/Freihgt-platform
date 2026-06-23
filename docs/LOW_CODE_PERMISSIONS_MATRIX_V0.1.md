# Low-code Permissions Matrix v0.1

## Summary

Formal permission matrix for low-code runtime and admin operations before staging/pilot. **v0.1 enforces admin operations at the API layer only when `LOW_CODE_ADMIN_AUTH_ENABLED=true`** (role: `PLATFORM_ADMIN`). Runtime GET/PUT remain tenant-scoped without service-level RBAC in default-off mode. Frontend adds `useLowCodePermissions()` for UI alignment; runtime write is UI-gated by operator roles, not API-gated.

**Review commit:** `2c09aa3` (pilot readiness baseline) + matrix pack changes.

## Scope

**In scope**

- Permission matrix documentation (roles × actions)
- Backend admin guard inventory (already implemented)
- Frontend `useLowCodePermissions()` composable
- UI gates: admin pages, migration/batch buttons, runtime edit visibility
- Auth default-off and pilot auth-on modes
- Verification commands

**Out of scope**

- New database migrations or role seeds
- Service-level RBAC on runtime GET/PUT custom-field-values
- ABAC / entity-ownership checks
- Gateway JWT policy changes
- `TENANT_ADMIN` role (not seeded; use `SHIPPER_ADMIN` / `CARRIER_ADMIN` as tenant admins)
- Audit page UI restriction (documented recommendation only)

## Auth Modes

| Mode | Env | Admin `/admin/*` | Runtime GET/PUT | Audit GET |
|------|-----|------------------|-----------------|-----------|
| **Default-off (dev)** | `LOW_CODE_ADMIN_AUTH_ENABLED=false` | Open (no role check) | Tenant header only | Tenant header only |
| **Pilot auth-on** | `LOW_CODE_ADMIN_AUTH_ENABLED=true` | `PLATFORM_ADMIN` + `X-User-ID` | Unchanged (tenant only) | Unchanged |
| **Unauthenticated curl** | default-off | Works with `X-Tenant-ID` | Works with `X-Tenant-ID` | Works with `X-Tenant-ID` |

## Roles

Seeded in `infrastructure/migrations/000009_seed_roles.up.sql`:

| Role | Scope | Low-code relevance |
|------|-------|-------------------|
| `PLATFORM_ADMIN` | GLOBAL | Full admin low-code; all UI admin gates |
| `SHIPPER_ADMIN` | TENANT | Runtime write (UI); future template publisher |
| `SHIPPER_LOGIST` | TENANT | Runtime write (UI) — transport orders |
| `CARRIER_ADMIN` | TENANT | Runtime write (UI) |
| `CARRIER_DISPATCHER` | TENANT | Runtime write (UI) — shipments |
| `PROCUREMENT_MANAGER` | TENANT | Runtime write (UI) — RFx / freight |
| `FINANCE_MANAGER` | TENANT | Runtime write (UI) — billing |
| `CONSIGNEE_OPERATOR` | TENANT | Runtime write (UI) |
| `DRIVER` | TENANT | **No** admin; **no** runtime write in UI v0.1 |
| `GOV_INSPECTOR` | GLOBAL | Read-only platform (not low-code specific) |

**Not seeded (documented for future):** `TENANT_ADMIN`, `FORWARDER_ADMIN`, `VIEWER`, `READ_ONLY` — map to nearest tenant admin or operator role until added.

## Permission Matrix

Legend: **Y** = allowed | **N** = denied | **T** = tenant-scoped (no role check at API) | **UI** = frontend gate only | **A** = admin API when auth-on

| Action | Default-off API | Auth-on API | UI v0.1 | PLATFORM_ADMIN | SHIPPER_* / LOGIST | CARRIER_* / DISPATCHER | FINANCE | DRIVER | Unauth+curl |
|--------|-----------------|-------------|---------|----------------|--------------------|-----------------------|---------|--------|-------------|
| Read PUBLISHED templates | T | T | Y | Y | Y | Y | Y | Y | T |
| Read active template | T | T | Y | Y | Y | Y | Y | Y | T |
| Read custom field values | T | T | Y | Y | Y | Y | Y | Y | T |
| Edit custom field values | T | T | UI | Y | Y | Y | Y | N | T |
| Read audit events | T | T | Y | Y | Y | Y | Y | Y* | T |
| Create/edit DRAFT template | T | A | UI admin | Y | N | N | N | N | T |
| Publish template | T | A | UI admin | Y | N | N | N | N | T |
| Clone to DRAFT | T | A | UI admin | Y | N | N | N | N | T |
| Migration preview | T | A | UI admin | Y | N | N | N | N | T |
| Migration execute | T | A | UI admin | Y | N | N | N | N | T |
| Batch preview | T | A | UI admin | Y | N | N | N | N | T |
| Batch execute | T | A | UI admin | Y | N | N | N | N | T |

\* DRIVER can open audit page in UI v0.1 (no strict gate); pilot may enable `canViewLowCodeAuditStrict()`.

## Runtime Read Permissions

| Endpoint | Method | Enforcement v0.1 |
|----------|--------|------------------|
| `/v1/low-code/form-templates` | GET | `X-Tenant-ID` required; PUBLISHED only |
| `/v1/low-code/form-templates/active` | GET | Tenant-scoped |
| `/v1/low-code/form-templates/{id}` | GET | Tenant-scoped; published detail |
| `/v1/low-code/custom-field-values` | GET | Tenant-scoped |

No role middleware on runtime read paths (by design).

## Runtime Write Permissions

| Endpoint | Method | Enforcement v0.1 |
|----------|--------|------------------|
| `/v1/low-code/custom-field-values` | PUT | Tenant-scoped; field type/read_only rules |

**UI (v0.1):** `canEditCustomFieldsRuntime()` — operator roles on entity detail panels and custom-field-values page. **API unchanged** — curl without role still works in default-off.

**Pilot recommendation:** keep API tenant-scoped; rely on gateway auth + future Runtime Write RBAC pack for service enforcement.

## Admin Template Permissions

| Endpoint | Guard |
|----------|-------|
| `POST/GET/PUT /v1/low-code/admin/form-templates` | `RequireLowCodeAdmin` |
| `POST .../publish` | same |
| `POST .../clone-to-draft` | same |

**Role when auth-on:** `PLATFORM_ADMIN` only (`packages/shared-go/auth/lowcode_admin.go`).

**UI:** `middleware/low-code-admin.ts` + hub nav via `canAccessLowCodeAdmin()`.

## Migration Permissions

| Endpoint | Guard |
|----------|-------|
| `POST .../admin/custom-field-values/migration-preview` | `RequireLowCodeAdmin` |
| `POST .../admin/custom-field-values/migrate-to-active` | same |
| `POST .../admin/custom-field-values/batch-migration-preview` | same |
| `POST .../admin/custom-field-values/batch-migrate-to-active` | same |

All four routes registered under `adminGuard` in `services/low-code-service/internal/http/router.go`.

**UI:** migration/batch buttons on `/low-code/custom-field-values` use `canRunMigrationPreview()` / `canRunBatchMigrationPreview()`.

## Audit Permissions

| Endpoint | Guard |
|----------|-------|
| `GET /v1/low-code/audit-events` | None (tenant-scoped) |

**UI:** `/low-code/audit` — `auth` middleware only. Strict admin view available via `canViewLowCodeAuditStrict()` (not enabled by default in v0.1).

**Pilot:** audit is source of truth for admin history; restrict audit UI in staging if required.

## Default-off Development Mode

| Check | Expected |
|-------|----------|
| `LOW_CODE_ADMIN_AUTH_ENABLED` | `false` (Docker Compose default) |
| Smoke/curl without `X-User-ID` | Pass |
| Admin migration preview | 200 with tenant header only |
| Runtime PUT | 200 |
| Mock auth admin UI | `admin@7rights.local` → full admin via dev fallback |

## Pilot Auth-on Mode

**Do not enable by default in repo.** Staging/pilot only:

```yaml
# low-code-service environment
LOW_CODE_ADMIN_AUTH_ENABLED: "true"
IDENTITY_SERVICE_URL: http://identity-service:8081
```

| Request | Expected |
|---------|----------|
| Admin POST without `X-User-ID` | `401 UNAUTHORIZED` |
| Admin POST with `SHIPPER_ADMIN` user | `403 FORBIDDEN` |
| Admin POST with `PLATFORM_ADMIN` user | `200` |
| Runtime GET/PUT without `X-User-ID` | Unchanged (200 with tenant) |

**Manual verify (staging):**

```powershell
$T = "<tenant-uuid>"
$ADMIN = "<platform-admin-user-uuid>"
curl.exe -X POST -H "X-Tenant-ID: $T" -H "Content-Type: application/json" `
  --data-binary "@scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json" `
  "http://localhost:8088/v1/low-code/admin/custom-field-values/batch-migration-preview"
# Expect 401 without X-User-ID when auth-on

curl.exe -X POST -H "X-Tenant-ID: $T" -H "X-User-ID: $ADMIN" -H "Content-Type: application/json" `
  --data-binary "@scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json" `
  "http://localhost:8088/v1/low-code/admin/custom-field-values/batch-migration-preview"
# Expect 200 for PLATFORM_ADMIN
```

**Rollback:** set `LOW_CODE_ADMIN_AUTH_ENABLED=false`, restart `low-code-service`.

## Backend Guardrails

| Component | Location |
|-----------|----------|
| Admin middleware | `internal/http/middleware/admin_auth.go` |
| Router wiring | `internal/http/router.go` — single `adminGuard` on both admin route groups |
| Role constants | `packages/shared-go/auth/lowcode_admin.go` |
| Identity lookup | `internal/platform/identity/client.go` |
| Config | `LOW_CODE_ADMIN_AUTH_ENABLED`, `IDENTITY_SERVICE_URL` |

**Tests:** `admin_auth_test.go` — disabled pass-through, 401 missing user, 403 non-admin (SHIPPER_ADMIN, DRIVER), 200 PLATFORM_ADMIN.

## Frontend Guardrails

| Component | Location |
|-----------|----------|
| Permission matrix composable | `composables/useLowCodePermissions.ts` |
| Admin route middleware | `middleware/low-code-admin.ts` |
| Hub admin nav | `pages/low-code/index.vue` |
| Migration buttons | `pages/low-code/custom-field-values/index.vue` |
| Entity panel edit | `:editable="canEditCustomFieldsRuntime()"` on detail pages |
| Legacy helper | `composables/usePermissions.ts` — `isPlatformAdmin()` |

**Dev fallback:** mock auth + `admin@7rights.local` grants all checks (see `AUTH_RBAC.md`).

## Tests

| Layer | Coverage |
|-------|----------|
| Backend middleware | `go test ./internal/http/middleware/...` |
| Shared role helper | `packages/shared-go/auth/lowcode_admin_test.go` |
| Frontend | `npm run build` (no Vitest) |

## Known Limitations

| Limitation | Mitigation |
|------------|------------|
| Runtime PUT not API role-gated | UI gate + tenant scope; future pack |
| Audit UI open to all auth users | Use `canViewLowCodeAuditStrict()` in staging if needed |
| No `TENANT_ADMIN` role seeded | Use SHIPPER_ADMIN / CARRIER_ADMIN |
| Entity ownership not checked | Document pilot scope per entity type |
| `validation_context.role` not trusted for finance | Policy docs |
| Auth-on not live-tested in CI | Manual staging checklist |

## Recommended Pilot Settings

```text
LOW_CODE_ADMIN_AUTH_ENABLED=true
IDENTITY_SERVICE_URL=http://identity-service:8081
Pilot operator role: PLATFORM_ADMIN
Runtime: tenant-scoped X-Tenant-ID on all calls
Audit: enable review of /low-code/audit after template changes
DRAFT templates: admin UI only; verify public API returns PUBLISHED only
```

## Verification Commands

```powershell
cd D:\Projects\freight-platform

make health-check
make seed-lowcode-demo
make integration-smoke-test

cd services\low-code-service
go test ./...

cd ..\..\apps\web-admin
npm run build

cd ..\..
node scripts/dev/verify_lowcode_validation_context.mjs
```

### Default-off API smoke (no X-User-ID)

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"

curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"

curl.exe -X POST -H "X-Tenant-ID: $T" -H "Content-Type: application/json" `
  --data-binary "@scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json" `
  "http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migration-preview"
```

## Next Action

**Low-code Template Import/Export Design Pack v0.1** — design-only pack for template portability between environments.

If staging pilot finds blockers → **Low-code Runtime Pilot Fix Pack v0.1**.

## Related Docs

- `docs/LOW_CODE_PERMISSIONS_ADMIN_GUARDRAILS_V0.1.md`
- `docs/LOW_CODE_RUNTIME_PILOT_READINESS_V0.1.md`
- `docs/AUTH_RBAC.md`
- `docs/LOW_CODE_RUNTIME_INTEGRATION_POLICY_V0.1.md`
