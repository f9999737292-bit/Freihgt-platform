# Low-code Pilot Week-3 Candidate Plan v0.1

## Purpose

Conservative Week-3 plan after Week-2 cross-entity readiness review (**GO_WITH_CONDITIONS**). Focus on **evidence collection**, **monitoring**, and **operator feedback** — not broad production expansion.

**Prerequisite:** Week-2 closure pack completed.

## Candidate Scope

| Dimension | Week-3 intent |
|-----------|---------------|
| Tenants | One pilot tenant (unchanged) |
| TRANSPORT_ORDER | Continue primary pilot; daily reports |
| SHIPMENT | Monitored limited writes on approved demo entity only |
| BILLING_REGISTER | Monitored limited writes with financial safety checks |
| New entities | **Not by default** — require written approval |
| Broad rollout | **Excluded** |

## Preconditions

Before Week-3 operational open:

- [ ] Week-2 closure doc signed off
- [ ] Cross-entity decision **GO_WITH_CONDITIONS** acknowledged
- [ ] Monitoring templates distributed to operators
- [ ] Stop conditions briefed
- [ ] Auth-on staging verification scheduled (if not repeated since Week-1)

## Recommended Workstreams

### Workstream 1: Monitoring Evidence

**Goal:** Replace preparatory monitoring with **real pilot data**.

| Task | Output |
|------|--------|
| Fill daily reports for TO pilot | `docs/pilot-reports/` entries |
| On first SH write after enablement | SH monitoring report template |
| On first BR write after enablement | BR monitoring report template + financial safety columns |
| Weekly audit gap review | Zero gaps target |
| Evening health-check | Logged in daily report |

**Exit:** At least 3 pilot days with reports OR explicit "no writes" documented.

### Workstream 2: Auth-on Staging Verification

**Goal:** Confirm RBAC on staging matches dev expectations.

| Task | Reference |
|------|-----------|
| Repeat auth-on checklist | `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md` |
| Verify PLATFORM_ADMIN on admin routes | 401/403/200 matrix |
| Verify non-admin blocked | Shipper logist spot-check |
| Document results | Week-3 daily report |

**Exit:** Auth-on staging PASS or documented exceptions with owners.

### Workstream 3: Operator Feedback Collection

**Goal:** First **real** operator feedback for SH/BR limited pilot.

| Task | Output |
|------|--------|
| Distribute quick guides | SH + BR operator guides |
| 15-min walkthrough per entity | Sign-off checklist |
| Feedback form submissions | `LOW_CODE_PILOT_OPERATOR_FEEDBACK_FORM_V0.1.md` |
| Triage P0/P1/P2 | Week-3 fix backlog |

**Exit:** ≥1 feedback form per entity type attempted OR documented blocker.

### Workstream 4: Runtime Fixes If Needed

**Goal:** Smallest safe fix for P0/P1 only.

| Trigger | Action |
|---------|--------|
| P0 stop condition | **Low-code Runtime Pilot Fix Pack v0.1** |
| P1 functional bug | Fix pack with approval; no drive-by refactors |
| P2/P3 | Backlog for later sprint |

**Exit:** No open P0; P1 owned with due dates.

### Workstream 5: Pilot Expansion Decision

**Goal:** Data-driven decision — expand or hold.

| Question | Decision gate |
|----------|---------------|
| Add second SH entity? | Monitoring clean + operator sign-off |
| Add second BR entity? | Financial safety clean + operator sign-off |
| Shipper logist SH/BR write? | Product + security approval |
| Broad rollout? | **Not in Week-3** |

**Exit:** Written expansion decision or explicit "hold" note in Week-3 review.

## Not In Scope

- Broad production rollout
- Multi-tenant expansion
- Batch migration execute
- Import execute in pilot flow
- Template publish without review
- Payment/billing status automation via low-code
- Mobile driver app
- ЭТрН/ЭПД integration
- API contract changes
- Migrations
- Core business logic changes

## Exit Criteria (Week-3 end)

Week-3 considered successful for pilot continuity if:

1. **Monitoring:** Daily reports filed (or zero-write days documented) for ≥3 days
2. **Feedback:** Operator feedback collected for TO; attempt for SH/BR
3. **Auth-on:** Staging verification repeated or scheduled with owner
4. **Health:** No unresolved P0; smoke tests pass on review days
5. **Financial:** No billing/payment side effects from BR pilot writes
6. **Decision:** Week-3 review doc with expand/hold recommendation

## Risks

| Risk | Mitigation |
|------|------------|
| Assuming docs = production ready | Week-3 focuses on real evidence |
| BR financial side effect | Mandatory core GET after every write |
| Scope creep to broad rollout | Expansion workstream gated |
| Missing auth-on on staging | Dedicated workstream |

## Next Pack After Week-3

Depending on Week-3 exit:

- **Stable + evidence:** Pilot expansion decision pack (limited)
- **P0/P1 issues:** Low-code Runtime Pilot Fix Pack v0.1
- **Default:** Week-3 review + updated Week-4 candidate plan

---

Inputs: `LOW_CODE_PILOT_WEEK2_CROSS_ENTITY_READINESS_REVIEW_V0.1.md`, `LOW_CODE_PILOT_WEEK2_CROSS_ENTITY_DECISION_NOTE_V0.1.md`
