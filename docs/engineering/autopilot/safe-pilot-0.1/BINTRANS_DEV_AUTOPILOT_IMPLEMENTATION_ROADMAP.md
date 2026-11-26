# Implementation roadmap

Status: sequence for a later controller decision. Phases A–F are not started by this pilot.

Each phase keeps `AUTOPILOT_ENABLED`, `PAID_AUTOMATION_ENABLED`, `WORKER_ENABLED`, `AUTO_MERGE_ENABLED`, and `AUTO_DEPLOY_ENABLED` off unless that phase's controller approval names the one flag it turns on. No phase turns on merge or deploy.

## PHASE_A — read-only GitHub and CI monitoring

Revision 0.2 reason: J-DA-C002. Phase A does not enable the dispatcher.

DEPENDENCIES: current `main`. No PR #403 merge. The candidate workflow file stays outside the default branch until a controller GO.

REQUIRED_CHANGES: add `.github/workflows/bintrans-autopilot-monitor.yml` only after that GO. `workflow_dispatch` only. Read permissions listed in the security acceptance checklist. No Cursor client. The job reports Issues, pull requests, and Actions run conclusions. It does not write.

TESTS: permission test that a write client is absent; workflow guard still rejects an untrusted event.

SECURITY_GATES: contents read, issues read, pull requests read, actions read. Emergency variable still stops the process.

MAX_COST: `0`.

ROLLBACK: disable the workflow.

CONTROLLER_APPROVAL: required before the job is allowed to run on `main`. Not granted here.

## PHASE_B — approved queue and cost estimate

DEPENDENCIES: Phase A. Issue #313 gates armed: unknown price deny, `require_distinct_provider` true, caps still `0`.

REQUIRED_CHANGES: persist a controller event id on the queue row. Do not treat label or `READY` as approval. Estimate cost from a verified price list. Do not call Cursor.

TESTS: DRY-001 through DRY-006 and DRY-015 against the real queue in an isolated database whose name passes the existing test-database check.

SECURITY_GATES: stale SHA denies; duplicate delivery does not double-reserve.

MAX_COST: `0`.

ROLLBACK: stop accepting controller events. Queue rows stay. No delete.

CONTROLLER_APPROVAL: required for the event schema. Not granted here.

## PHASE_C — one development agent, draft PR

DEPENDENCIES: Phase B. Verified model id and price. Server-owned emergency stop, or the existing Actions variable if the only runner is GitHub Actions.

REQUIRED_CHANGES: one `POST /v1/agents` from the dispatcher, `workOnCurrentBranch` false, dedicated branch, draft pull request. `MAX_CONCURRENT_DEVELOPMENT_AGENTS=1` for this phase.

TESTS: DRY-009, DRY-011, DRY-012, DRY-013, DRY-014. Cancel is available and terminal. No merge assertion.

SECURITY_GATES: budget reserve before create; stop before create; secrets absent from the prompt.

MAX_COST: a controller-chosen per-task cap inside the planning envelope. The `500` figure is not that cap.

ROLLBACK: `AUTOPILOT_EMERGENCY_STOP=true` and cancel the run. Do not delete the branch from an agent.

CONTROLLER_APPROVAL: required for the first paid create. Not granted here.

## PHASE_D — Agent J and verified spend

DEPENDENCIES: Phase C. GitHub App identity for the reviewer. Append-only verdict bound to the SHA.

REQUIRED_CHANGES: `INDEPENDENT_REVIEW_GATE` can be evaluated. It stays blocked if the actor, the SHA, or the CI result is missing. Reconcile `chargedCents`. Unknown usage holds the budget.

TESTS: DRY-007, DRY-008, DRY-010, plus a new-commit staleness case.

SECURITY_GATES: CI success is not a review. Reviewer has no deploy permission. Forged payload does not clear a hold.

MAX_COST: the Phase C cap plus the review task cap. A review does not raise the implementation cap.

ROLLBACK: reviewer app disabled. Gate returns to `BLOCKED`.

CONTROLLER_APPROVAL: required to name the reviewer app. Not granted here.

## PHASE_E — two development agents

DEPENDENCIES: Phase D. Path overlap check and dependency check on the live queue.

REQUIRED_CHANGES: raise the development cap from 1 to 2. Review cap stays 1.

TESTS: two claims, overlapping paths denied, dependency not `COMPLETED` denied.

SECURITY_GATES: no shared lease; no shared tenant role.

MAX_COST: two reserves must both fit. The second denies when the remainder is short.

ROLLBACK: cap back to 1. In-flight leases expire. They are not force-completed.

CONTROLLER_APPROVAL: required. Not granted here.

## PHASE_F — supervised 24/7

DEPENDENCIES: Phases A–E accepted. Alerts on stop, budget hold, and stale head. Emergency stop outside the checkout.

REQUIRED_CHANGES: the existing hourly schedule may run only while the controller flag and the stop source allow it. No resident worker beyond one scheduled pass.

TESTS: stop during a pass, restart with a hold, missing stop source.

SECURITY_GATES: all earlier gates. No auto-merge. No deploy.

MAX_COST: monthly and daily caps chosen by the controller. Planning target remains unauthorized until that choice.

ROLLBACK: variable `AUTOPILOT_ENABLED` not `true`, and emergency stop true.

CONTROLLER_APPROVAL: required separately from every earlier phase. Not granted here.
