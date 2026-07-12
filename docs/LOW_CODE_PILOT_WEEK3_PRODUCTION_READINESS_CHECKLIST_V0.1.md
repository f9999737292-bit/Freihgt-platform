# Low-code Pilot Week-3 Production Readiness Checklist v0.1

## Checklist Summary

Production readiness checklist for Week-3 low-code pilot review (trigger: **Production review requested**).

**Final status:** **NOT_PRODUCTION_READY** — controlled pilot only.

## Functional Readiness

| criterion | status | evidence | owner | notes |
|-----------|--------|----------|-------|-------|
| TO/SH/BR scenarios operable (demo) | PASS_FOR_CONTROLLED_PILOT | 3/3 operators completed scenarios | Феликс Асаев | demo entities only |
| Custom field templates active | PASS_FOR_CONTROLLED_PILOT | health + template checks in pilot | pilot lead | dev/demo tenant |

## Operator Feedback Readiness

| criterion | status | evidence | owner | notes |
|-----------|--------|----------|-------|-------|
| 3/3 operator feedback | PASS | forms v0.1; intake v0.1 | Феликс Асаев | all 5/5, ready |
| Operator blockers | PASS | замечаний нет | — | no P0/P1/P2 |
| Feedback-based fixes required | PASS | none reported | — | no code changes from intake |

## Runtime Readiness

| criterion | status | evidence | owner | notes |
|-----------|--------|----------|-------|-------|
| health-check 9/9 | PASS | make health-check 2026-06-26 | QA | dev environment |
| low-code-service | PASS | health-check OK | DevOps | — |
| audit GET available | PASS | HTTP 200 | QA | pilot tenant |
| metrics endpoint | PASS | HTTP 200 | DevOps | localhost:8088 |

## Security / Auth Readiness

| criterion | status | evidence | owner | notes |
|-----------|--------|----------|-------|-------|
| Remote Auth-On Repeat | **CLOSED_APPROVED_BY_OWNER_REMOTE_AUTH_ON_VERIFIED** | `docs/LOW_CODE_PILOT_WEEK3_PR_GAP_001_OWNER_APPROVAL_CAPTURE_V0.1.md`, `docs/LOW_CODE_PILOT_WEEK3_PR_GAP_001_CLOSURE_DECISION_NOTE_V0.1.md` | **Феликс Асаев** | PR-GAP-001 closed with owner approval. Remote auth-on verified on Selectel staging. Production-ready not claimed. Remaining staging limitations tracked separately. |
| Production auth policy | PENDING | — | Security | not approved |
| RBAC production review | PENDING | — | Security | out of scope v0.1 pilot |

## Tenant Isolation Readiness

| criterion | status | evidence | owner | notes |
|-----------|--------|----------|-------|-------|
| Tenant isolation production evidence | **PASS** | Tenant Isolation Owner Final Approval v0.1, Tenant Isolation Evidence Review v0.1 | **Феликс Асаев** | PR-GAP-006 closed; optional staging follow-up |
| Cross-tenant leak test (prod) | PENDING | — | Security | not executed |

## Data / Migration Readiness

| criterion | status | evidence | owner | notes |
|-----------|--------|----------|-------|-------|
| Production data approval | **PASS / APPROVED_BY_OWNER** | Production Data Owner Final Approval v0.1, Production Data Policy v0.1 | **Феликс Асаев** | PR-GAP-002 closed; production data use not approved |
| Migration execute policy | PENDING | — | DevOps | no prod migrations approved |
| Template publish policy | PENDING | — | pilot lead | publish blocked without pack |
| Low-code SoT policy | **PASS / APPROVED_BY_SOT_OWNER** | SoT Owner Final Approval v0.1, SoT Owner Approval Gate v0.1, Source-of-Truth Policy v0.1 | **Феликс Асаев** | PR-GAP-010 closed; source-of-truth scope approved |

## Audit / Observability Readiness

| criterion | status | evidence | owner | notes |
|-----------|--------|----------|-------|-------|
| Audit read (dev) | PASS_FOR_CONTROLLED_PILOT | audit GET 200 | pilot lead | dev only |
| Monitoring/alerting production policy | **PASS** | Production Monitoring Policy v0.1, Alert Conditions v0.1, Monitoring Checklist v0.1, Monitoring Owner Final Approval v0.1 | **Артем Асаев** | PR-GAP-004 closed; real config not changed |
| Audit retention production policy | **PASS** | Audit Retention Policy v0.1, Audit Evidence Handling Rules v0.1, Audit Compliance Owner Final Approval v0.1 | **Феликс Асаев** | PR-GAP-005 closed; real config not changed |

## Rollback Readiness

| criterion | status | evidence | owner | notes |
|-----------|--------|----------|-------|-------|
| Rollback plan | **PASS** | Rollback Plan v0.1, Low-code Rollback Procedure v0.1, Rollback Checklist v0.1, Rollback Owner Final Approval v0.1 | **Артем Асаев** | PR-GAP-003 closed; rollback not executed |
| Rollback drill (prod) | PENDING | — | DevOps | not executed |

## Support Readiness

| criterion | status | evidence | owner | notes |
|-----------|--------|----------|-------|-------|
| Support owner | **PASS** | Support Ownership Policy v0.1, Support Escalation Matrix v0.1, Support Ownership Checklist v0.1, Support Owner Final Approval v0.1 | **Артем Асаев** | PR-GAP-007 closed |
| Runbooks for production ops | PENDING | — | pilot lead | controlled pilot runbooks only |

## Governance Approval

| criterion | status | evidence | owner | notes |
|-----------|--------|----------|-------|-------|
| controlled pilot approval | PASS | CONTROLLED_PILOT_APPROVED | Феликс Асаев | active |
| Release owner | **PASS / APPROVED_BY_RELEASE_OWNER** | Release Owner Final Approval v0.1, Release Ownership Policy v0.1 | **Артем Асаев** | PR-GAP-008 closed; no deploy executed |
| Final go/no-go approval | **OWNER_APPROVED_BUT_PRODUCTION_READY_NOT_CLAIMED** | Final Go-No-Go Owner Final Approval v0.1, Staging Hardening Review v0.1 | **Феликс Асаев** | PR-GAP-009 owner approved; production-ready not claimed due to STG-LIM-001..004 |

## Gap Closure Artifacts

| artifact | status | reference |
|----------|--------|-----------|
| Gap Closure Plan | **created** | `LOW_CODE_PILOT_WEEK3_PRODUCTION_READINESS_GAP_CLOSURE_PLAN_V0.1.md` |
| Remaining Gaps Consolidation | **created** | `LOW_CODE_PILOT_WEEK3_REMAINING_GAPS_STATUS_CONSOLIDATION_V0.1.md` |
| No-Server Gap Closure Status | **created** | `LOW_CODE_PILOT_WEEK3_NO_SERVER_GAP_CLOSURE_STATUS_V0.1.md` |
| Gap Tracker | **created** | `LOW_CODE_PILOT_WEEK3_PRODUCTION_READINESS_GAP_TRACKER_V0.1.md` |
| Owner Matrix | **created** | `LOW_CODE_PILOT_WEEK3_PRODUCTION_READINESS_OWNER_MATRIX_V0.1.md` |
| Acceptance Criteria | **created** | `LOW_CODE_PILOT_WEEK3_PRODUCTION_READINESS_ACCEPTANCE_CRITERIA_V0.1.md` |

**Remaining blocker:** **none (PR-GAP-001 closed)** — PR-GAP-009 **OWNER_APPROVED_BUT_PRODUCTION_READY_NOT_CLAIMED** — production-ready not claimed due to staging limitations

## Final Status

| Area | Result |
|------|--------|
| Controlled pilot | **PASS — continue** |
| Production readiness | **NOT APPROVED** |
| PR-GAP-002 | **PASS / APPROVED_BY_OWNER** |
| PR-GAP-008 | **PASS / APPROVED_BY_RELEASE_OWNER** |
| PR-GAP-010 | **PASS / APPROVED_BY_SOT_OWNER** |
| PR-GAP-009 | **OWNER_APPROVED_BUT_PRODUCTION_READY_NOT_CLAIMED** |
| PR-GAP-001 | **CLOSED_APPROVED_BY_OWNER_REMOTE_AUTH_ON_VERIFIED** |
| Staging limitations | **STG-LIM-001..006 OPEN** — STG-LIM-001 DNS pending (`staging.bintrans.ru`); STG-LIM-002 HTTPS pending DNS+SSH; STG-LIM-003 SSH SG pending re-verification |
| Final production readiness | **NOT_PRODUCTION_READY_CONTROLLED_PILOT_ONLY** |
| Decision | `SELECTEL_SSH_SG_NON_TRUSTED_REJECTION_FAILED_PORT_22_PUBLICLY_OPEN` |
| Reason | External non-trusted scan confirms port 22 open; Selectel SG /32 not applied |
| Gap closure plan | **created** — `GAP_CLOSURE_PLAN_CREATED` |
| Evidence prepared | `docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SG_NON_TRUSTED_REJECTION_EVIDENCE_V0.1.md`, `docs/LOW_CODE_PILOT_WEEK3_STG_LIM_003_CLOSURE_CANDIDATE_NOTE_V0.1.md` |

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_READINESS_DECISION_V0.1.md`, `LOW_CODE_PILOT_WEEK3_PRODUCTION_READINESS_GAP_CLOSURE_PLAN_V0.1.md`
