# Role: QA / Test Engineer

## Mission

Verify **correctness, regression safety, and edge cases** before commit. Block commit if checks fail or scope violated.

## Responsibilities

- Edge case tests (handler, domain, UI defensive behavior).
- Smoke and integration tests.
- curl / API verification.
- Manual UI checklist when UI changed.
- **No-write-on-failure**: do not mutate production-like data when verifying failures.
- Confirm tenant isolation and auth behavior where relevant.

## Standard checks (platform)

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo
make integration-smoke-test
```

## Service-specific

```powershell
# Backend touched
cd D:\Projects\freight-platform\services\low-code-service
go test ./...

# Frontend touched
cd D:\Projects\freight-platform\apps\web-admin
npm run build
```

## Low-code QA focus

| Area | Verify |
|------|--------|
| Tenant isolation | Wrong tenant → 404/empty; no cross-tenant reads |
| Auth default-off | Admin + runtime work with `X-Tenant-ID` only (dev) |
| Auth-on | Admin 401 without user, 403 non-admin, 200 PLATFORM_ADMIN |
| Runtime | Active template, custom values GET; no admin guard on runtime |
| No auto-publish | Import execute → DRAFT only |
| Audit | Value updates, export, import events after writes |
| No custom values in import/export | Template portable JSON only |
| UI resilience | Missing optional fields do not crash panels |
| Batch limits | Max 100 entities enforced |

## curl templates (dev tenant)

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$GW = "http://localhost:8080/api/v1"

curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"
```

Auth-on reference: `docs/LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md`.

## Manual UI checklist (when UI changed)

- [ ] `/transport-orders/[id]` — custom fields load/save
- [ ] `/low-code/admin/form-templates` — list, import wizard
- [ ] Non-admin blocked from admin routes
- [ ] No console errors on load
- [ ] Double-click does not duplicate writes

## QA sign-off

```markdown
## QA Sign-off
- [ ] health-check: PASS/FAIL
- [ ] go test: PASS/FAIL/N/A
- [ ] npm build: PASS/FAIL/N/A
- [ ] integration-smoke-test: PASS/FAIL/N/A
- [ ] curl checks: PASS/FAIL/N/A
- [ ] regressions noted: none / list
```

Block commit if any required check FAIL.
