# Low-code Pilot Week-2 Plan v0.1

## Summary

Operational plan for Week 2 of the low-code staging pilot. Builds on **GO_WITH_CONDITIONS** Week-1 review (`LOW_CODE_PILOT_WEEK1_REVIEW_V0.1.md`).

**Week-2 objective:** Continue TRANSPORT_ORDER pilot, collect **real** operator and user feedback, fix only P0/P1, and prepare **SHIPMENT read-only validation** if Week 2 remains stable.

**No real user feedback collected yet** — Week 2 is the first week for live feedback collection.

## Week-2 Objective

1. Open pilot to limited shipper users on staging (TO custom fields only)
2. File daily reports and operator feedback forms every day
3. Fix P0 immediately; fix P1 with smallest safe diff
4. If Days 1–3 stable (no P0, health OK), run SHIPMENT read-only internal validation
5. End Week 2 with updated review and scope decision for Week 3

## Scope

| Dimension | Week 2 |
|-----------|--------|
| Tenants | **One** pilot tenant |
| Primary entity | **TRANSPORT_ORDER** |
| Template code | `transport_order_default` |
| SHIPMENT | **Read-only internal validation only** — not user-facing rollout |
| BILLING_REGISTER | **Excluded** |
| Admin ops | `PLATFORM_ADMIN` only (auth-on) |
| Batch execute | Only after clean preview; max **100** — defer if possible |

## Included

- TRANSPORT_ORDER custom field read/edit for pilot shipper users
- Daily health-check + audit review
- Operator daily reports (`LOW_CODE_PILOT_DAILY_REPORT_TEMPLATE_V0.1.md`)
- Operator feedback forms (`LOW_CODE_PILOT_OPERATOR_FEEDBACK_FORM_V0.1.md`)
- Staging auth-on repeat verification
- Platform admin template admin (export, import preview; execute → DRAFT only with approval)
- SHIPMENT read-only validation pack (mid-week, if stable):
  - Active template GET for `shipment_default`
  - Custom values GET on demo shipment entity
  - Entity panel load check (no user rollout)

## Excluded

- Broad rollout beyond one pilot tenant
- SHIPMENT user-facing pilot (read-only QA only)
- BILLING_REGISTER production pilot expansion
- Batch migration execute >100
- Automatic migrations on publish
- Mobile driver app
- ЭТрН/ЭПД integration
- API contract changes
- Migrations
- Core business logic changes
- Manual DB edits
- Committing `LOW_CODE_ADMIN_AUTH_ENABLED=true`

## Daily Checks

From `LOW_CODE_PILOT_DAY1_MONITORING_V0.1.md` — apply every pilot day:

| Time | Actions |
|------|---------|
| **Morning** | `make health-check`; active template curl; audit baseline; auth spot-check |
| **Midday** | Audit last 4h; user issues; log feedback forms |
| **Evening** | Health-check; daily report; stop-condition review |

## Feature Usage Limits

| Feature | Week 2 policy |
|---------|---------------|
| Custom values PUT (TO) | **Allowed** for pilot users |
| Template publish | **Approval required** — avoid unless planned |
| Import execute | **DRAFT only** — preview first; rare |
| Migration execute | **Defer** — preview only unless approved single entity |
| Batch migration | **Defer** — preview only |
| SHIPMENT edit | **Not for users** — read-only QA only |

## Monitoring Focus

| Area | Focus |
|------|-------|
| Runtime | TO save success rate; error message clarity |
| Audit | Every PUT has audit event; correct entity_id |
| Security | Non-admin blocked from admin; tenant isolation |
| Health | low-code-service 5xx count |
| Feedback | File forms for every reported issue |
| SHIPMENT (if run) | GET/load only; no writes |

## Fix Policy

| Severity | Policy |
|----------|--------|
| **P0** | Stop pilot → Runtime Pilot Fix Pack |
| **P1** | Fix this week — UI defensive, wording, guards only |
| **P2** | Log to backlog |
| **P3** | Note only |

**Not allowed without approval:** API changes, migrations, backend domain logic changes.

Reference: `LOW_CODE_PILOT_WEEK1_FEEDBACK_FIX_PLAN_V0.1.md`.

## SHIPMENT Read-only Validation

**Gate:** Run only if Week 2 Days 1–3 have **no P0**, health-check passes, and operator confirms TO pilot stable.

**Allowed checks (read-only):**

```powershell
$T = "{pilot_tenant_id}"
$GW = "http://{gateway}/api/v1"

# Active SHIPMENT template
curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/form-templates/active?entity_type=SHIPMENT&template_code=shipment_default"

# Demo shipment custom values GET (replace entity_id)
curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/custom-field-values?entity_type=SHIPMENT&entity_id={shipment_id}&template_code=shipment_default"
```

**UI (platform admin only):**

```text
http://{web-admin}/shipments/{id}
```

Verify: panel loads, values display, **no pilot user edit** enabled for SHIPMENT in Week 2.

**Next pack:** `Low-code Pilot Week-2 SHIPMENT Read-only Validation Pack v0.1`.

## Risks

| Risk | Mitigation |
|------|------------|
| No real feedback collected | Mandatory daily reports + forms |
| Auth-on not repeated on staging | DevOps task Day 1 |
| Scope creep to SHIPMENT users | Read-only QA only; product sign-off for user rollout |
| Unapproved template publish | Operator checklist; audit review |
| P1 issues block operators | Fix pack rules; smallest diff |

## Decision Gates

| Gate | When | Outcome |
|------|------|---------|
| Week 2 Day 1 open | Pilot users enabled | GO / STOP |
| Day 3 | Mid-week triage | SHIPMENT validation go/no-go |
| Day 5 | Week 2 review | Week 3 scope |
| SHIPMENT validation | After Day 3 gate | Proceed to validation pack or defer |

### Week 3 scope options (decide Day 5)

- Continue TO only
- TO + SHIPMENT user pilot (requires product approval)
- Pause for fix pack

## Next Review

Fill updated review at end of Week 2 using:

- `LOW_CODE_PILOT_WEEK1_REVIEW_TEMPLATE_V0.1.md` (adapt for Week 2)
- Real daily reports and feedback forms

**Next pack after Week 2 plan:** Low-code Pilot Week-2 SHIPMENT Read-only Validation Pack v0.1.
