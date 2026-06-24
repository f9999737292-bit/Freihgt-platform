# Low-code Pilot Week-3 Auth-On Staging Runbook v0.1

## Purpose

Safe operational runbook to verify **auth-on** low-code admin RBAC during Week-3 pilot. Confirms `LOW_CODE_ADMIN_AUTH_ENABLED=true` behavior without committing env changes or executing admin writes.

**Reference:** `LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_VERIFICATION_V0.1.md`

## Preconditions

- [ ] `make health-check` green
- [ ] `make seed-lowcode-demo` OK (demo tenant + users seeded)
- [ ] Week-3 monitoring evidence pack complete
- [ ] Tracked `docker-compose.yml` has `LOW_CODE_ADMIN_AUTH_ENABLED: "false"`
- [ ] **Do not commit** override, `.env`, or staging secrets
- [ ] Rollback plan understood (see Rollback section)

## Required Users

| Role | User ID | Email | Use |
|------|---------|-------|-----|
| `PLATFORM_ADMIN` | `8541a3a3-bde7-4fed-9501-37b9953bf904` | `admin@7rights.local` | Positive admin test |
| `SHIPPER_LOGIST` (non-admin) | `008e1462-6f67-4246-b7dc-4aae1669c0c5` | `shipper@7rights.local` | Negative admin test |

**Tenant ID:** `74519f22-ff9b-4a8b-8fff-a958c689682f`

Seed if missing: `make seed-dev-admin` (see `docs/AUTH_RBAC.md`).

## Required Headers

| Header | Required | Purpose |
|--------|----------|---------|
| `X-Tenant-ID` | Always | Tenant scope |
| `X-User-ID` | Auth-on admin routes | Identity lookup for RBAC |

Gateway base: `http://localhost:8080/api/v1/low-code`

## Safe Verification Steps

### Step 1 — Default-off baseline

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo

$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=SHIPMENT&template_code=shipment_default"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates"
```

**Expected:** runtime **200**; admin without user **200** (default-off).

### Step 2 — Enable auth-on temporarily (local only)

Create **gitignored** file `infrastructure/docker-compose/docker-compose.override.yml`:

```yaml
services:
  low-code-service:
    environment:
      LOW_CODE_ADMIN_AUTH_ENABLED: "true"
```

Restart low-code-service only:

```powershell
docker compose -f infrastructure/docker-compose/docker-compose.yml `
  -f infrastructure/docker-compose/docker-compose.override.yml up -d --no-build low-code-service

make health-check
```

**For remote staging:** set env via deployment config — **never commit** to tracked compose.

### Step 3 — Run auth-on checks (read-only)

See sections below.

### Step 4 — Rollback to default-off

See **Rollback To Default-off** section.

## PLATFORM_ADMIN Check

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$ADMIN = "8541a3a3-bde7-4fed-9501-37b9953bf904"

curl.exe -i `
  -H "X-Tenant-ID: $T" `
  -H "X-User-ID: $ADMIN" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates"
```

**Expected:** HTTP **200**; template list returned.

## Non-admin Forbidden Check

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$NONADMIN = "008e1462-6f67-4246-b7dc-4aae1669c0c5"

curl.exe -i `
  -H "X-Tenant-ID: $T" `
  -H "X-User-ID: $NONADMIN" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates"
```

**Expected:** HTTP **403**; body `FORBIDDEN` — `low-code admin access required`.

## Missing User Check

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"

curl.exe -i `
  -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates"
```

**Expected:** HTTP **401**; body `UNAUTHORIZED` — `authenticated user is required for low-code admin operations`.

## Runtime Compatibility Check

With auth-on still enabled:

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=SHIPMENT&template_code=shipment_default"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&template_code=shipment_default"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/audit-events?limit=20"
```

**Expected:** all HTTP **200** without `X-User-ID` (runtime routes not admin-guarded).

## What Not To Run

During auth-on verification packs:

- PUT/save custom-field-values (unless separate controlled write pack)
- POST template create / update / publish / clone
- POST import-preview / import execute
- POST migration-preview / migration execute
- POST batch-migration execute
- Manual DB edits
- Commit `.env`, override, or `LOW_CODE_ADMIN_AUTH_ENABLED=true` to tracked files
- Destructive Docker commands (`docker compose down -v`, etc.)

## Rollback To Default-off

1. Delete `infrastructure/docker-compose/docker-compose.override.yml` (if created)
2. Restore service from tracked compose:

```powershell
cd D:\Projects\freight-platform
make platform-up-no-build
make health-check
```

3. Confirm default-off:

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
curl.exe -i -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/admin/form-templates"
```

**Expected:** HTTP **200** without `X-User-ID`.

4. Regression:

```powershell
make integration-smoke-test
```

## Expected Results

| Check | Auth-on | Default-off |
|-------|---------|-------------|
| Admin + PLATFORM_ADMIN | **200** | **200** (no guard) |
| Admin + non-admin | **403** | **200** (no guard) |
| Admin, no user | **401** | **200** (no guard) |
| Runtime active template | **200** | **200** |
| Runtime custom values | **200** | **200** |
| Audit GET | **200** | **200** |

## Troubleshooting

| Symptom | Likely cause | Action |
|---------|--------------|--------|
| Admin always 200 for non-admin | Auth still off | Verify override applied; restart `low-code-service`; check env in container |
| Admin 401 for PLATFORM_ADMIN | Wrong user ID or identity unreachable | Verify `IDENTITY_SERVICE_URL`; confirm seed user |
| Admin 500 | Identity service down | `make health-check`; restart identity-service |
| Runtime 401 with auth-on | Unexpected — report P1 | Check router guard scope; do not proceed with pilot writes |
| Smoke fails after rollback | Service not recreated | `make platform-up-no-build`; wait for healthy |

## Stop Conditions

**STOP** verification and open **Low-code Runtime Pilot Fix Pack v0.1** if:

- PLATFORM_ADMIN receives **403** on admin list with valid headers
- Non-admin receives **200** on admin list with auth-on enabled
- Runtime GET breaks (**401/403**) with auth-on enabled
- Rollback fails to restore default-off smoke
- Any P0 security finding (wrong tenant data, auth bypass)

**Do not** leave auth-on enabled locally if it breaks normal dev smoke without team agreement.
