# Low-code Pilot Handoff Note v0.1

**Audience:** pilot lead, operator, staging owner  
**Date:** 2026-06-24  
**Decision:** GO_WITH_CONDITIONS (staging/pilot only — not production)

---

## What is ready

Low-code runtime layer is ready for a **limited pilot** on **one staging tenant**:

- Custom fields on **Transport Orders** (`transport_order_default` template)
- Read/edit custom values on entity detail pages
- Admin UI for template management (list, builder, clone, publish)
- Template export/import (import creates **DRAFT only**)
- Migration preview (single entity and batch, max 100)
- Audit log for writes and admin actions
- Permissions and auth-on guardrails (verified locally)

Full evidence: `LOW_CODE_PILOT_FINAL_SMOKE_HANDOFF_V0.1.md`.

---

## How to start pilot

1. Deploy `main` branch (commit `168e2f9` or later) to **staging**.
2. On staging `low-code-service`, set `LOW_CODE_ADMIN_AUTH_ENABLED=true` (via deployment config or gitignored override — **do not commit** to dev compose).
3. Run auth verification: admin without user → 401; non-admin → 403; platform admin → 200. See `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md`.
4. Confirm pilot tenant has **PUBLISHED** template `transport_order_default` for TRANSPORT_ORDER.
5. Operator opens web-admin, logs in as platform admin, walks through:
   - `/low-code/admin/form-templates`
   - `/transport-orders/{id}` (custom fields panel)
   - `/low-code/audit`
6. Enable pilot users (shipper logists) for TO custom field edit only.

Step-by-step: `LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md`.

---

## Who can operate it

| Role | What they can do |
|------|------------------|
| **Platform admin** | All admin UI: templates, import/export, migration, audit |
| **Shipper logist / admin** | Edit custom fields on transport orders (runtime only) |
| **Carrier / finance / driver** | No admin access; runtime per permissions matrix (Phase 1: TO only for shippers) |
| **DevOps / staging owner** | Env flags, deploy, health-check, auth-on toggle |

Login (dev demo — replace with staging credentials):

```text
http://localhost:3000/login
admin@7rights.local / Admin123456!
Tenant: 74519f22-ff9b-4a8b-8fff-a958c689682f
```

---

## What not to do

- **Do not** run production import execute or migration execute without preview
- **Do not** publish imported templates without review
- **Do not** run batch migration >100 entities
- **Do not** edit the database manually
- **Do not** enable auto-publish from import
- **Do not** expand to multiple tenants without sign-off
- **Do not** treat this as a production rollout

---

## Daily checks

Each pilot day (~10 minutes):

1. `make health-check` — all services green
2. Check audit for unexpected writes
3. Scan low-code-service logs for 5xx
4. Confirm active template is still `transport_order_default` PUBLISHED
5. Spot-check 1–2 transport orders — custom values look correct

Full checklist: `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md`.

---

## Stop conditions

**Stop the pilot immediately** and escalate if:

- A non-admin user reaches admin API or `/low-code/admin/*` UI
- Data from another tenant appears
- Writes go to wrong entity or tenant
- Audit is missing after a write
- Health-check fails or low-code-service returns repeated 5xx
- Migration result does not match preview

**Rollback:** keep previous PUBLISHED template; do not publish bad DRAFT; use audit to inspect; DB restore only via DBA backup procedure.

---

## Next 7 days

| Day | Action |
|-----|--------|
| **Day 0** | Staging deploy + auth-on verify + operator browser sign-off |
| **Day 1** | Open pilot to limited shipper users (TO only); run Day-1 monitoring pack |
| **Day 2–3** | Daily checks; collect operator feedback; no template changes unless planned |
| **Day 4–5** | Review audit volume and error logs; assess Phase 2 (SHIPMENT) readiness |
| **Day 6–7** | Pilot retrospective; decide continue / expand / pause |

**Next pack:** Low-code Pilot Day-1 Monitoring Pack v0.1.

**Support docs:**

- Runbook: `LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md`
- Operator checklist: `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md`
- Release notes: `LOW_CODE_PILOT_RELEASE_NOTES_V0.1.md`
- Permissions: `LOW_CODE_PERMISSIONS_MATRIX_V0.1.md`
