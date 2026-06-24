# Low-code Pilot Operator Checklist v0.1

Quick reference for pilot operators. Full context: `LOW_CODE_PILOT_RELEASE_PACKAGE_V0.1.md`, `LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md`.

## Daily Checklist

- [ ] `make health-check` — all services green
- [ ] Review audit events (value updates, admin actions)
- [ ] Scan low-code-service error logs
- [ ] Confirm active template: `transport_order_default` PUBLISHED
- [ ] Spot-check 1–2 pilot entities — custom values look correct
- [ ] Review admin DRAFTs — none published without approval
- [ ] Log issues/blockers in pilot tracker

## Before Template Change

- [ ] **Export** current published template (save JSON with date)
- [ ] Confirm `schema_version: lowcode.template.export.v1`
- [ ] Use **clone-to-draft** for edits (do not edit published directly)
- [ ] Notify pilot lead if template affects live entities

## Before Import

- [ ] Export current template first
- [ ] Run **import-preview** only on test/staging tenant first
- [ ] Review errors and warnings (checksum mismatch = warning)
- [ ] Confirm execute will create **DRAFT only**
- [ ] Do **not** publish until template review complete

## Before Publish

- [ ] Review diff / version compare
- [ ] Confirm no accidental field removals
- [ ] Run **migration preview** if active template changes affect existing values
- [ ] Get explicit approval from pilot admin / product owner
- [ ] Never publish imported DRAFT without review

## Before Migration

- [ ] Run **migration-preview** for all target entities
- [ ] Review SAFE / WARNING / BLOCKED per entity
- [ ] Execute only if SAFE or warnings explicitly accepted
- [ ] Single entity first in Phase 1; batch only after sign-off
- [ ] Batch: max **100** IDs; run **batch-migration-preview** first

## After Migration

- [ ] Verify custom values on affected entities (GET)
- [ ] Check audit events (`entity_id`, `batch_id` if batch)
- [ ] Document migration in pilot log
- [ ] Do not retry execute without new preview

## Audit Review

```powershell
$T = "{pilot_tenant_id}"
$GW = "http://{gateway}/api/v1"

curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/audit-events?entity_type=TRANSPORT_ORDER&limit=50"
```

Look for: value updates, `FORM_TEMPLATE_*`, migration events.

## Error Review

- [ ] Check low-code-service logs for 5xx
- [ ] Check gateway logs for `/api/v1/low-code/*` failures
- [ ] Confirm no raw `value_json` in logs (policy)
- [ ] Escalate repeated errors to platform ops

## When To Stop

Stop pilot and escalate if:

- Non-admin accesses admin API/UI (auth-on)
- Cross-tenant data visible
- Wrong entity/tenant received writes
- Migration outcome ≠ preview expectation
- Audit missing after writes
- Health-check fails
- Repeated low-code 5xx

Mark pilot **BLOCKED** → `Low-code Runtime Pilot Fix Pack v0.1`.

## Who To Notify

| Situation | Notify |
|-----------|--------|
| Auth/access issue | Platform ops + security |
| Data integrity issue | Pilot lead + DBA |
| Template/migration issue | Pilot admin + product |
| Service outage | Platform ops |
| Stop condition triggered | All above + management |

## Commands

```powershell
cd D:\Projects\freight-platform

# Platform health
make health-check

# Demo seed (dev/staging test tenant only — never production)
make seed-lowcode-demo

# Full platform smoke
make integration-smoke-test

# Frontend build check
cd apps\web-admin
npm run build
```

### Quick low-code smoke (dev tenant example)

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$GW = "http://localhost:8080/api/v1"

curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"
```

Auth-on staging: add `-H "X-User-ID: {admin_user_id}"` for admin routes.

See `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md`.
