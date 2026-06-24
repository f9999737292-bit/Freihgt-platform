# Low-code Pilot Week-2 Cross-Entity Decision Note v0.1

## Decision Summary

**Overall: GO_WITH_CONDITIONS**

Low-code runtime pilot is ready for **continued internal limited pilot** across TRANSPORT_ORDER (primary), SHIPMENT, and BILLING_REGISTER on **approved demo/pilot entities only**.

**Not approved:** broad production rollout, uncontrolled scope expansion, or financial/status automation via custom fields.

**Review date:** 2026-06-24

## Approved Scope

| Entity | Approved mode | Demo entity |
|--------|---------------|-------------|
| TRANSPORT_ORDER | Limited pilot writes (primary) | Pilot tenant entities per runbook |
| SHIPMENT | Limited pilot writes with conditions | DEMO-SH-PLANNED |
| BILLING_REGISTER | Limited pilot writes with conditions + financial guardrails | DEMO-BR-001 |

**Shared requirements:**

- Audit after every write
- Monitoring daily reports when writes occur
- Stop conditions enforced
- One pilot tenant (staging/dev)

## Not Approved

- Broad production rollout (any entity)
- Additional entities without written pilot lead approval
- Billing/payment/shipment **status** changes via low-code custom fields
- invoice/act/UPD operations via custom fields
- migration execute / batch execute / import execute in pilot flow
- Template publish without admin review
- Manual DB edits
- Automated migrations on publish

## Entity Decisions

| Entity | Decision | Notes |
|--------|----------|-------|
| TRANSPORT_ORDER | **READY_LIMITED_PILOT** | Runtime baseline; continue primary pilot |
| SHIPMENT | **READY_LIMITED_PILOT** | Controlled write + enablement + monitoring; no post-enablement events yet |
| BILLING_REGISTER | **READY_LIMITED_PILOT** | Controlled write + financial safety + monitoring; no post-enablement events yet |

## Conditions

1. **No real operator feedback yet** — collect in Week-3 before expansion
2. **Auth-on staging** — repeat verification before staging user expansion
3. **Monitoring reports** — fill SH/BR templates on first pilot writes
4. **Financial safety** — core billing register GET after every BR write
5. **P0 stop** → halt entity writes + Runtime Pilot Fix Pack

## Required Monitoring

| Entity | Doc |
|--------|-----|
| TRANSPORT_ORDER | `LOW_CODE_PILOT_DAY1_MONITORING_V0.1.md` + daily report |
| SHIPMENT | `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_V0.1.md` |
| BILLING_REGISTER | `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_V0.1.md` |

## Stop Conditions

Stop affected entity writes immediately on:

- Wrong tenant or entity
- Audit missing after write
- Active template changed unexpectedly
- Core status / financial side effect (shipment or billing)
- invoice/act/UPD triggered (billing)
- Repeated low-code-service 5xx
- integration-smoke-test failure after write

Full list: cross-entity readiness review doc.

## Week-3 Recommendation

Proceed to **Week-2 Closure & Week-3 Plan Pack** with focus on:

1. Monitoring evidence collection (real writes + daily reports)
2. Operator feedback forms
3. Auth-on staging repeat
4. Fix pack only if P0/P1 emerges
5. **No** broad rollout until evidence + sign-off

See: `LOW_CODE_PILOT_WEEK3_CANDIDATE_PLAN_V0.1.md`

## Owner Actions

| Action | Owner | Due |
|--------|-------|-----|
| Approve Week-2 closure | Pilot lead / PM | Week-2 end |
| Schedule operator walkthrough (TO/SH/BR) | Operator lead | Week-3 Day 1 |
| Repeat auth-on on staging | DevOps + Security | Week-3 |
| First SH/BR monitoring daily report | Operator | On first write |
| Week-3 plan review | PM | After closure pack |

---

Full review: `LOW_CODE_PILOT_WEEK2_CROSS_ENTITY_READINESS_REVIEW_V0.1.md`
