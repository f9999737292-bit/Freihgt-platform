# Low-code Runtime Pilot Readiness v0.1

## Summary

Pilot readiness review of the low-code runtime layer at commit `8d975db` (`feat: wire low-code validation context into entity panels`). All automated verification passed; gateway and direct low-code API smoke checks returned HTTP 200. The platform is **ready for a controlled admin/runtime pilot** with **TRANSPORT_ORDER** as the primary scope, expanding to **SHIPMENT** and **BILLING_REGISTER** after initial validation.

**Verdict:** **GO for controlled pilot** (dev/staging first). No blocking defects found. Remaining gaps are documented operational and coverage items, not runtime blockers.

## Current Commit

| Field | Value |
|-------|-------|
| Commit | `8d975db` |
| Message | `feat: wire low-code validation context into entity panels` |
| Branch | `main` |
| Working tree at review | clean |
| Review date | 2026-06-24 |

## Scope

**In scope**

- Runtime API compatibility (GET/PUT templates, values, audit)
- Auth default-off and pilot auth-on documentation
- Entity detail panels (TO / shipment / billing register)
- `validation_context` wiring
- Migration/batch preview (read-only)
- Observability (health, metrics, audit, logs policy)
- Readiness matrix and pilot checklist

**Out of scope**

- Backend business logic changes
- API contract changes
- Database migrations
- New feature UI
- Live toggle of `LOW_CODE_ADMIN_AUTH_ENABLED=true` in Docker Compose (manual pilot checklist only)
- Batch migration execute (preview-only for this review)

## Runtime Compatibility

Demo tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f`  
Demo entity (TO): `2db04b49-665c-469f-bcb1-ffeb1274fedb` (DEMO-TO-001)

| Check | HTTP | Result | When |
|-------|------|--------|------|
| Active template (gateway) | 200 | `PUBLISHED`, `is_active: true` | 2026-06-24 |
| Custom field values GET (gateway) | 200 | 3 fields returned | 2026-06-24 |
| Custom field values PUT **without** `validation_context` | 200 | `status: ok` | 2026-06-24 |
| Custom field values PUT **with** `validation_context` | 200 | `status: ok` | 2026-06-24 |
| Audit events GET (gateway) | 200 | migration + value events | 2026-06-24 |
| Public form templates list | 200 | **PUBLISHED only** (no DRAFT) | 2026-06-24 |
| Integration smoke test | — | **PASSED** (TEST-20260624013051) | 2026-06-24 |

All checks run **without** `X-User-ID` (auth default-off).

### Example commands

```powershell
cd D:\Projects\freight-platform
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"

curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"

curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb"

curl.exe -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&limit=10"
```

## Auth Modes

### Default-off (current Docker Compose)

| Setting | Value |
|---------|-------|
| `LOW_CODE_ADMIN_AUTH_ENABLED` | `"false"` (default) |

| Endpoint class | Behavior |
|----------------|----------|
| Runtime GET/PUT custom-field-values | No role check; no `X-User-ID` required |
| Public form templates / audit | No role check |
| Admin template / migration / batch | No role check (dev convenience) |

Verified: batch migration preview and migration preview succeed without `X-User-ID`.

### Pilot auth-on (manual — do not change compose permanently for this review)

| Setting | Expected behavior |
|---------|-------------------|
| `LOW_CODE_ADMIN_AUTH_ENABLED=true` | Admin `/v1/low-code/admin/*` requires `X-User-ID` + `PLATFORM_ADMIN` |
| Runtime GET/PUT | **Unchanged** — still no admin middleware |
| Missing `X-User-ID` on admin | `401 UNAUTHORIZED` |
| Non-admin user on admin | `403 FORBIDDEN` |

**Pilot enable steps (staging/pilot only):**

1. Set on `low-code-service`: `LOW_CODE_ADMIN_AUTH_ENABLED=true`, `IDENTITY_SERVICE_URL=http://identity-service:8081`
2. Restart `low-code-service` only
3. Confirm pilot user has `PLATFORM_ADMIN` for tenant (`make seed-dev-admin` in dev)
4. Web-admin sends `X-User-ID` from auth store; gateway overwrites from JWT when `AUTH_ENABLED=true`
5. Verify admin curl with `X-User-ID: 8541a3a3-bde7-4fed-9501-37b9953bf904` (dev admin) → 200 on batch preview
6. Verify admin curl **without** `X-User-ID` → 401
7. Rollback: set flag back to `false` and restart service

See `docs/LOW_CODE_PERMISSIONS_ADMIN_GUARDRAILS_V0.1.md`. Middleware unit tests pass (`admin_auth_test.go`); live auth-on toggle not applied during this review to avoid disrupting default-off dev workflows.

## Entity Panel Readiness

| Entity | Page | Panel | validationContext | Unavailable fallback |
|--------|------|-------|-------------------|----------------------|
| TRANSPORT_ORDER | `/transport-orders/[id]` | `LowCodeCustomFieldsPanel` editable | `buildTransportOrderValidationContext` | `CommonApiUnavailableState` + retry |
| SHIPMENT | `/shipments/[id]` | editable | `buildShipmentValidationContext` + route labels | same |
| BILLING_REGISTER | `/billing-registers/[id]` | editable | `buildBillingRegisterValidationContext` | same |

**Demo URLs (after `make seed-demo-data` + `make seed-lowcode-demo`):**

```text
http://localhost:3000/transport-orders/2db04b49-665c-469f-bcb1-ffeb1274fedb
http://localhost:3000/shipments/14d405e2-0152-4030-b356-eec464a3cc66
http://localhost:3000/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796
```

**Automated helper verification:** `node scripts/dev/verify_lowcode_validation_context.mjs` — **OK**

Manual UI browser check optional; code paths for load/save/unavailable states verified in component review.

## validation_context

| Item | Status |
|------|--------|
| Helper `apps/web-admin/utils/lowCodeValidationContext.ts` | Present |
| PUT without context | Works (backward compatible) |
| PUT with compact context | Works |
| Backend parses `entity_status` + `role` | Unchanged |
| Extra context keys | Ignored safely by backend |
| Not used for financial gates | Policy documented |

## Migration Readiness

Preview-only (no execute in this review):

| Check | HTTP | Outcome |
|-------|------|---------|
| Single migration preview | 200 | entity status `SAFE` |
| Batch migration preview | 200 | `summary.total: 1`, `safe: 1` |

Payloads: `scripts/dev/payloads/lowcode_migration_preview_transport_order.json`, `lowcode_batch_migration_preview_transport_order.json`

Execute paths previously verified in earlier packs; pilot policy: **always preview before execute**.

## Batch Migration Readiness

| Guardrail | Status |
|-----------|--------|
| Max 100 entities | Enforced (service + UI) |
| Duplicate ID dedupe | Documented + tested |
| Preview gate before execute | 409 guards |
| Per-entity transaction isolation | Documented |
| Preview-only in this review | OK |

## Audit Readiness

| Item | Status |
|------|--------|
| `GET /v1/low-code/audit-events` | 200 |
| Value update events | Present for demo TO |
| Migration/batch metadata (`batch_id`) | Supported |
| Audit as admin history source of truth | Recommended for pilot |

## Observability

| Item | Result |
|------|--------|
| `make health-check` | **OK** |
| `low-code-service` `/health` | **OK** |
| `low-code-service` `/metrics` | **OK** (200) |
| Batch migration metrics | Present (`lowcode_batch_migration_*`) |
| High-cardinality labels | **Not used** (`tenant_id`, `entity_id`, `batch_id` forbidden — tested) |
| Structured batch logs | Documented in `LOW_CODE_BATCH_MIGRATION_HARDENING_V0.1.md` |
| `value_json` in metrics/logs | Not exposed |

## Readiness Matrix

| # | Dimension | Rating | Evidence |
|---|-----------|--------|----------|
| 1 | Runtime API compatibility | **READY** | Gateway smoke 200; PUT with/without context |
| 2 | Auth default-off compatibility | **READY** | All checks without `X-User-ID`; compose default `false` |
| 3 | Pilot auth-on checklist | **PARTIAL** | Documented + unit tests; not live-toggled in compose |
| 4 | Entity panels | **READY** | TO/SH/BR wired; helper script OK; unavailable fallback in panel |
| 5 | validation_context | **READY** | v0.2 wiring + PUT tests |
| 6 | Conditional required validation | **READY** | Domain/service tests pass |
| 7 | Template lifecycle | **READY** | Public API PUBLISHED only; admin draft path separate |
| 8 | Migration safety | **READY** | Preview SAFE; design + edge case tests |
| 9 | Batch migration safety | **READY** | Preview OK; hardening doc + metrics tests |
| 10 | Audit history | **READY** | GET 200; events for demo entity |
| 11 | Metrics/logs | **READY** | Metrics scrape OK; bounded labels |
| 12 | Admin UX | **READY** | Batch wizard, migration modal, admin guardrails |
| 13 | i18n RU/EN/ZH | **READY** | Low-code keys including `adminAccessDenied` |
| 14 | Tests/build | **PARTIAL** | `go test`, `npm run build`, verification script; no Vitest |
| 15 | Operational docs | **READY** | This doc + 50+ LOW_CODE_* references |

**Totals:** 13 READY / 2 PARTIAL / 0 NOT_READY

## Pilot Checklist

### Before pilot

- [ ] Backup database (staging/pilot environment)
- [ ] `make health-check` — all services OK
- [ ] Confirm seeds are **dev-only**; do not run demo seeds in production
- [ ] Enable `LOW_CODE_ADMIN_AUTH_ENABLED=true` on pilot `low-code-service`
- [ ] Verify pilot operator has `PLATFORM_ADMIN` for tenant
- [ ] Verify tenant isolation (`X-Tenant-ID` required; cross-tenant tests exist)
- [ ] Verify audit events visible for test entity
- [ ] Verify active templates: `GET .../form-templates/active` returns `PUBLISHED` + `is_active: true`
- [ ] Verify public runtime API lists **no DRAFT** templates
- [ ] Run `node scripts/dev/verify_lowcode_validation_context.mjs`
- [ ] Run `make integration-smoke-test`

### During pilot

- [ ] Start with **TRANSPORT_ORDER** entity type only
- [ ] Limit template changes; use **clone-to-draft** for edits
- [ ] Run **migration preview** before any execute
- [ ] Avoid batch execute unless preview is fully `SAFE` / warnings acknowledged
- [ ] Monitor health, `/metrics`, audit log, service logs
- [ ] Do not use `validation_context` for billing/UPD approval decisions

### Rollback

- [ ] Disable `LOW_CODE_ADMIN_AUTH_ENABLED` if auth issues → restart service
- [ ] Keep previous `PUBLISHED` template active (activation policy)
- [ ] Do not manually delete custom field values
- [ ] Use audit log to inspect changes
- [ ] No direct DB edits except emergency DBA procedure

## Rollback Notes

| Issue | Action |
|-------|--------|
| Admin auth blocks operators | Set `LOW_CODE_ADMIN_AUTH_ENABLED=false`, restart `low-code-service` |
| Bad template published | Publish previous version or clone-to-draft fix; migration preview before data move |
| Bad migration execute | Audit inspect; re-run preview; per-entity idempotency documented |
| Low-code service down | Entity detail pages show unavailable panel; core entities unaffected |

## Risks and Gaps

| Risk / gap | Severity | Mitigation |
|------------|----------|------------|
| Pilot auth-on not live-tested in this review | Low | Follow manual checklist on staging before prod |
| No Vitest component tests | Low | Verification script + manual UI spot-check |
| Core BFF does not forward `validation_context` server-side | Low | Frontend sidecar pattern sufficient for pilot |
| FR/DOCUMENT/RFX panels lack v0.2 validationContext | Low | Out of recommended pilot scope |
| Batch execute scale (100 cap) | Medium | Preview first; split batches |
| Client `validation_context.role` not trust boundary | Medium | Policy: soft validation only |

## Recommended Pilot Scope

**Phase 1 (week 1):** TRANSPORT_ORDER custom fields on entity detail + read-only template preview  
**Phase 2:** SHIPMENT custom fields  
**Phase 3:** BILLING_REGISTER custom fields  
**Admin (with auth-on):** template clone-to-draft, migration preview; batch execute only after clean preview  
**Exclude from pilot:** core workflow changes, financial gates, direct DB edits

## Verification Commands

```powershell
cd D:\Projects\freight-platform

git status --short
git log --oneline -5

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

### API smoke (gateway, no X-User-ID)

See **Runtime Compatibility** section above.

### Migration preview

```powershell
curl.exe -X POST `
  -H "Content-Type: application/json" `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  --data-binary "@scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json" `
  http://localhost:8080/api/v1/low-code/admin/custom-field-values/batch-migration-preview
```

## Next Action

**Low-code Permissions Matrix Pack v0.1** — expand beyond single `PLATFORM_ADMIN` role to tenant-scoped permission matrix for admin operations (optional before wider pilot rollout).

If pilot staging finds blockers → **Low-code Runtime Pilot Fix Pack v0.1**.

## Verification Results (this review)

| Check | Result |
|-------|--------|
| `make health-check` | OK |
| `make seed-lowcode-demo` | OK |
| `make integration-smoke-test` | PASSED (TEST-20260624013051) |
| `go test ./...` (low-code-service) | OK |
| `npm run build` (web-admin) | OK |
| `verify_lowcode_validation_context.mjs` | OK |
| Runtime API smoke (gateway) | OK |
| PUT without/with validation_context | OK |
| Migration preview | OK (SAFE) |
| Batch preview | OK (total:1) |
| Public templates (no DRAFT) | OK |
| Metrics (no tenant_id label) | OK |
