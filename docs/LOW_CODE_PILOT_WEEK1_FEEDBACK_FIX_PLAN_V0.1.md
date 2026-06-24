# Low-code Pilot Week-1 Feedback & Fix Plan v0.1

## Summary

Week-1 feedback collection and fix-planning package for the low-code staging pilot. Defines feedback sources, issue severity model (P0/P1/P2), daily plan for Days 1–5, safe fix rules, operator feedback form, and weekly review template.

**Prerequisite:** Day-1 monitoring pack committed (`LOW_CODE_PILOT_DAY1_MONITORING_V0.1.md`).

**Current pilot decision:** **GO_WITH_CONDITIONS** — continue unless P0 stop condition appears.

**This is a docs/planning pack only** — no code changes in this pack.

## Current Commit

| Field | Value |
|-------|-------|
| HEAD | `c411d54` — `docs: add low-code pilot day-1 monitoring` |
| Pack date | 2026-06-24 |
| Branch | `main` |
| Backend / frontend changed in this pack | **no** |

## Scope

**In scope**

- Feedback sources and categories
- P0/P1/P2 severity model
- Week-1 daily plan (Days 1–5)
- Safe fix rules and verification commands
- Operator feedback form and week-1 review template references
- Decision gates for Week 2

**Out of scope**

- Code fixes (separate fix pack if P0/P1 found)
- Revert or history rewrite
- Production import/migration/batch execute
- Template publish
- Manual DB edits
- Auth-on env commit

## Evidence Documents

| Document | Status | Commit |
|----------|--------|--------|
| `LOW_CODE_PILOT_DAY1_MONITORING_V0.1.md` | **Found** | `c411d54` |
| `LOW_CODE_PILOT_DAILY_REPORT_TEMPLATE_V0.1.md` | **Found** | `c411d54` |
| `LOW_CODE_PILOT_FINAL_SMOKE_HANDOFF_V0.1.md` | **Found** | `466d593` |
| `LOW_CODE_PILOT_HANDOFF_NOTE_V0.1.md` | **Found** | `466d593` |
| `LOW_CODE_PILOT_RELEASE_PACKAGE_V0.1.md` | **Found** | `168e2f9` |
| `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md` | **Found** | `168e2f9` |
| `LOW_CODE_PILOT_RELEASE_NOTES_V0.1.md` | **Found** | `168e2f9` |

**Missing evidence docs:** none.

## Feedback Sources

| Source | Owner | Frequency | Output |
|--------|-------|-----------|--------|
| Operator daily report | Operator | Daily EOD | `LOW_CODE_PILOT_DAILY_REPORT_TEMPLATE_V0.1.md` |
| Audit events | Operator / QA | Daily | Event counts, anomalies |
| low-code-service logs | DevOps | Daily + on incident | 5xx, error patterns |
| health-check results | Operator / DevOps | Morning + evening | PASS/FAIL log |
| integration-smoke-test | QA (dev/staging) | Weekly or on demand | Run ID |
| Admin UI feedback | Platform admin | As reported | Operator feedback form |
| Runtime user feedback | Shipper logists | As reported | Operator feedback form |
| Template admin feedback | Platform admin | As reported | Import/export/publish flow |
| Import/export feedback | Platform admin | Per use | Preview/execute issues |
| Migration preview feedback | Platform admin | Per use | Preview vs expectation |
| Support messages / incidents | Pilot lead | As received | Incident log (Day-1 monitoring) |

## Feedback Categories

Tag every issue with one or more categories:

| Category | Examples |
|----------|----------|
| Runtime custom fields | Save fails, wrong values, empty state |
| Entity panels | TO detail panel load/save, validation_context |
| Template admin | Builder, clone, publish, list |
| Import/export | Preview errors, export copy, checksum warnings |
| Migration | Preview SAFE/WARNING/BLOCKED, execute mismatch |
| Batch migration | Preview wizard, batch limits |
| Audit | Missing events, filter broken, wrong actor |
| Permissions/Auth | 403 confusion, admin access, tenant header |
| UX/wording | Unclear errors, labels, tooltips |
| Performance/health | Slow load, 5xx, health-check fail |
| Documentation/operator confusion | Runbook gap, checklist unclear |

## Issue Severity Model

### P0 — stop pilot

| # | Condition |
|---|-----------|
| 1 | Tenant isolation issue |
| 2 | Admin endpoint accessible by non-admin user (auth-on) |
| 3 | Wrong entity/tenant write |
| 4 | health-check failure (low-code-service down) |
| 5 | Repeated low-code-service 5xx (3+ in 15 min) |
| 6 | Audit missing for write/admin operation |
| 7 | Active template wrong version/code unexpectedly |
| 8 | Migration execute result differs from preview |

**Action:** STOP pilot → escalate → **Low-code Runtime Pilot Fix Pack v0.1**

### P1 — fix this week

| # | Condition |
|---|-----------|
| 1 | Custom values save fails for pilot flow |
| 2 | Import/export preview blocks admin flow |
| 3 | Unclear validation error blocks operator |
| 4 | UI console error on pilot page |
| 5 | Permission guard confusing but not unsafe |
| 6 | Audit filter unusable for operator triage |
| 7 | Operator cannot follow runbook due to product gap |

**Action:** Fix in Days 2–3 (small, safe fixes only) — no API contract changes, no migrations, no core business logic changes.

### P2 — backlog

| # | Condition |
|---|-----------|
| 1 | Cosmetic UI issue |
| 2 | Wording improvement |
| 3 | Advanced filtering |
| 4 | Vitest coverage |
| 5 | Batch >100 support |
| 6 | Batch-level audit enhancements |
| 7 | Auto rollback UI |

**Action:** Log in backlog; address in Week-2+ or dedicated polish sprint.

### P3 — note only

Cosmetic non-blocking items, documentation typos, nice-to-have UX — no Week-1 action required.

## Week-1 Daily Plan

### Day 1 — monitor only

- [ ] Morning/evening health-check + audit baseline
- [ ] Collect incidents via daily report + feedback form
- [ ] **No feature changes** unless P0 (then stop + fix pack)
- [ ] Remind users: TO only, one tenant, preview before execute

### Days 2–3 — fix P0/P1 only

- [ ] Triage open issues by severity
- [ ] Fix **P0** immediately (stop pilot if needed)
- [ ] Fix **P1** with smallest safe diff:
  - UI defensive fixes (loading, errors, disable in-flight)
  - Wording / validation messages
  - Permission visibility guardrails
- [ ] **Not allowed:** API contract changes, migrations, core business logic
- [ ] Re-run verification after each fix batch
- [ ] Update operator docs if runbook gap found

### Day 4 — docs + safe polish

- [ ] Operator docs update (checklist, runbook gaps)
- [ ] UI wording/polish **only if safe** (P1/P2 cosmetic)
- [ ] Confirm no unapproved template publish or migration execute
- [ ] Mid-week audit review with pilot lead

### Day 5 — week summary + decision

- [ ] Fill `LOW_CODE_PILOT_WEEK1_REVIEW_TEMPLATE_V0.1.md`
- [ ] Summarize P0/P1/P2 counts
- [ ] **Go/no-go for broader scope** (SHIPMENT / BILLING_REGISTER)
- [ ] Assign Week-2 actions
- [ ] Proceed to **Low-code Pilot Week-1 Review Pack v0.1**

## P0 Stop Conditions

Same as Day-1 monitoring — see `LOW_CODE_PILOT_DAY1_MONITORING_V0.1.md` → Stop Conditions.

On any P0: **STOP** → pilot lead + platform ops → Runtime Pilot Fix Pack.

## P1 Fix-this-week Items

Allowed fix types (Week-1):

| Type | Allowed | Example |
|------|---------|---------|
| Loading/error states | Yes | Spinner on save; toast on failure |
| Button disable in-flight | Yes | Prevent double submit |
| Validation message clarity | Yes | RU/EN missing key, clearer text |
| Safe JSON display | Yes | `<pre>` not v-html |
| Permission UI guard | Yes | Hide admin nav for non-admin |
| Broken link/route | Yes | 404 on pilot page |
| i18n key | Yes | Missing RU label |

Not allowed without separate approval:

- New API endpoints
- API contract changes
- Migrations
- Core business logic changes
- Batch execute policy changes

## P2 Backlog Items

Defer to Week-2+ unless promoted by pilot lead:

- Advanced audit filters
- Vitest unit test coverage
- Batch >100
- Batch-level audit UI
- Auto rollback UI
- Phase 2 entity expansion (SHIPMENT/BILLING_REGISTER)
- Playwright/browser automation

## Safe Fix Rules

1. **Smallest diff** — one issue per fix when possible
2. **No API contract changes** in Week-1
3. **No migrations**
4. **No core business logic changes** in low-code-service domain
5. **Backend fix** — stop and request explicit approval before touching `services/low-code-service`
6. **Preview before execute** — never skip for import/migration
7. **Safety gate** — show git status, diff, checks before commit
8. **Do not commit** `LOW_CODE_ADMIN_AUTH_ENABLED=true`

## Verification Commands

### Daily (operator / QA)

```powershell
cd D:\Projects\freight-platform
make health-check
```

### Weekly or after fix batch (dev/local)

```powershell
cd D:\Projects\freight-platform
make seed-lowcode-demo
make integration-smoke-test

cd apps\web-admin
npm run build
```

### If backend fix ever approved

```powershell
cd D:\Projects\freight-platform\services\low-code-service
go test ./...

cd D:\Projects\freight-platform
make platform-build-service SERVICE=low-code-service
make platform-up-no-build
make health-check
make integration-smoke-test
```

### Low-code API spot-check (read-only)

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
curl.exe -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"
curl.exe -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/audit-events?limit=20"
```

## Operator Feedback Form

Use for every user-reported or operator-found issue:

`docs/LOW_CODE_PILOT_OPERATOR_FEEDBACK_FORM_V0.1.md`

## Weekly Review Template

Fill on Day 5 (or end of Week 1):

`docs/LOW_CODE_PILOT_WEEK1_REVIEW_TEMPLATE_V0.1.md`

## Decision Gates

| Gate | When | Outcome |
|------|------|---------|
| **Day 1 EOD** | After first pilot day | GO / GO_WITH_CONDITIONS / STOPPED |
| **Day 3** | Mid-week triage | P1 fixes on track? Any P0? |
| **Day 5** | Week summary | Week-1 Review Pack; Week 2 scope decision |
| **Scope expansion** | Day 5 only | SHIPMENT/BR only if GO + zero P0 + P1 resolved |

### Week 2 scope options

| Option | Condition |
|--------|-----------|
| Continue TO only | Default; safest |
| Add SHIPMENT panels | GO_WITH_CONDITIONS + operator sign-off |
| Add BILLING_REGISTER | After SHIPMENT stable or explicit product approval |
| Pause pilot | Any unresolved P0 or repeated P1 |

## Verification Run (this pack)

| Check | Result | Notes |
|-------|--------|-------|
| `make health-check` | **PASS** | All 9 services OK |
| `make seed-lowcode-demo` | **PASS** | 6 PUBLISHED templates |
| `make integration-smoke-test` | **PASS** | `TEST-20260624151824` |
| `npm run build` | **PASS** | web-admin build complete |

## Next Action

**Low-code Pilot Week-1 Review Pack v0.1**

If P0 appears during Week-1:

**Low-code Runtime Pilot Fix Pack v0.1**
