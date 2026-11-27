# Monitor 0.5D hosted verification plan

This document is a plan. It does not create a repository, publish a workflow, or dispatch one. `J-MON-R2-M003` stays open. `HOSTED_PERMISSION_ENFORCEMENT=NOT_RUN`.

## Environment choice

Option A, an isolated test repository, is the recommended place for the first hosted proof. Option B, publishing the candidate into `f9999737292-bit/Freihgt-platform`, can prove the real repository gate and the real `vars.AUTOPILOT_EMERGENCY_STOP`, but it also puts a workflow next to production Actions. That step needs its own controller GO after the isolated proof.

| Question | Isolated repository | Main repository |
| --- | --- | --- |
| Repository type | New empty repository owned by the operator. No BINTRANS deploy keys, packages, or environments. | Existing `f9999737292-bit/Freihgt-platform`. |
| Secrets | No production secrets and no copied staging secrets. No PAT. The job uses only its default `GITHUB_TOKEN`. | The job must still receive only `github.token`. Org or environment secrets must not be mapped into the job. |
| Trusted SHA | The commit that adds the candidate workflow in the test repository, recorded before dispatch. | `github.sha` of `refs/heads/main` after a reviewed merge. |
| Placement | `.github/workflows/` of the test repository only, after a publication GO. | Not authorized by this plan. |
| Dispatch | A person runs `workflow_dispatch` once per scenario. | Same, and only after a separate GO. |
| Removal | Delete the workflow file, then delete the test repository, after the evidence is saved outside it. | Removing a published workflow is a later change on `main`. |
| What transfers | GitHub's enforcement of the same `permissions` keys on `GITHUB_TOKEN` is evidence about the product, not about this repository's variables. | The repository name check, the real stop variable, and the absence of extra secrets still have to be shown on the main repository. |
| What does not transfer | A 403 in the test repository does not show how `Freihgt-platform` variables or org secrets behave. | A pass on `main` is the only proof of those controls. |

`ISOLATED_REPOSITORY_REQUIRED=YES` for the first hosted permission probes. No probe in this stage touches a BINTRANS pull request, branch, or workflow.

## Limits

`SUPERSEDED_BY_0_5E`. `J-MON-J-B001` rejected a campaign of 26 runs against a 60 minute cap with no reservation ledger. The current limits and the admission rule are in `BINTRANS_AUTOPILOT_MONITOR_HOSTED_BUDGET_RUNBOOK_0_5E.md`. An operator note is not the limiter.

The candidate still has one job, `ubuntu-latest`, and `timeout-minutes: 10`. The concurrency group does not reject a second dispatch. It can queue one. That is not a budget gate.

## Permission matrix

Every write probe uses a throwaway object in the isolated repository, or a request that does not name a BINTRANS object. A 403 is a pass only when the response body or error name says the token's permission is insufficient. A 404, a network error, or an unrelated 403 is a fail of the evidence, not a pass.

| ID | Preconditions | Expected result | Evidence | PASS | FAIL | Stop |
| --- | --- | --- | --- | --- | --- | --- |
| HOSTED-001 | Stop is exactly `false`. Dispatch one run. | The job token is `github.token` for that job. | A read of the token's permission set or the job log line that the token is the default token, with the secret value redacted. | Identity is the job token and no PAT is present. | A PAT, deploy key, or other credential is present. | Cancel the run. |
| HOSTED-002 | Same, plus a known completed run id in the test repository. | Both candidate GET calls return 200. | Saved status codes, not the raw JSON body. | Run GET and jobs GET are 200. | Any other method is sent, or a GET is denied. | Cancel the run. |
| HOSTED-003 | A run whose job count crosses one page, or a fixture total_count larger than the fetched pages. | The classifier does not report success when pages are incomplete. | Sanitized classification and exit code. | Incomplete pages are not `SUCCESS`. | Exit 0 on an incomplete page set. | Cancel the run. |
| HOSTED-004 | An injected `POST` to create one throwaway issue or label in the test repository. | The token cannot create it. | HTTP status and the permission error name. | 403 with insufficient permissions, and the object does not exist. | The object is created, or the error is not a permission denial. | Delete the test repository if an object was created. |
| HOSTED-005 | A throwaway issue created by a separate admin token, not by the monitor job. | The monitor token cannot edit it. | 403 permission error. The admin token then shows the body unchanged. | No edit. | The body changes. | Admin token reverts it, then the test repository is deleted. |
| HOSTED-006 | The same throwaway issue. | The monitor token cannot delete it. | 403 permission error and the object still exists for the admin token. | Object remains. | Object is gone. | Stop using the monitor token. Admin cleanup only. |
| HOSTED-007 | Stop is `false`. | `git push` with the job token fails. | Permission error. The test branch tip is unchanged. | No new commit. | The branch moves. | Reset is forbidden. Delete the test repository. |
| HOSTED-008 | A throwaway pull request opened by the admin token. | The monitor token cannot merge it. | 403 permission error. The pull request stays open. | Still open. | It merges. | Delete the test repository. |
| HOSTED-009 | The same throwaway pull request. | The monitor token cannot approve it. | 403 permission error. Review list is unchanged. | No approval. | An approval appears. | Admin dismisses it if one appears. |
| HOSTED-010 | The monitor job tries to dispatch a workflow. | The token cannot dispatch. | 403 permission error. No new run appears. | No new run. | A run is created. | Cancel that run. |
| HOSTED-011 | The test repository has no production secrets. The job prints only secret names that are present, never values. | Production secret names are absent. | Log of names only. | No production secret is mounted. | Any production secret name or value appears. | Cancel the run and treat the log as sensitive. |
| HOSTED-012 | The job environment is listed by name, not by value. | `GH_TOKEN` is the only GitHub credential, and it is the job token. | Names only. | No second token variable. | Another credential variable exists. | Cancel the run. |
| HOSTED-013 | Record the expected SHA before dispatch. | Checkout ref is that SHA. | The checkout log shows the pinned action and that SHA. | Exact SHA. | A branch name, tag, or pull-request ref is checked out. | Cancel the run. |
| HOSTED-014 | After checkout, `git config` credential helper and the token file are inspected without printing a token. | `persist-credentials` is false. | Log says credentials were not persisted. | No stored credentials. | A credential file or helper contains a token. | Cancel the run and delete the repository. |
| HOSTED-015 | A real GET is performed. | The Actions log has no raw run or jobs JSON. | Log excerpt around the classify step. | No `total_count` payload and no full job object. | Raw JSON is in the log. | Cancel is too late. Delete the log artifact if one was uploaded. |
| HOSTED-016 | A synthetic secret-shaped string is placed in a job name in the test repository. | The sanitized line says `REDACTED`. | Sanitized stdout only. | The synthetic value is absent. | The value is printed. | Cancel the run. |

## Emergency stop matrix

GitHub resolves `vars` into the step environment when the step is prepared. This plan does not claim that editing `AUTOPILOT_EMERGENCY_STOP` stops a step that has already begun. That was not observed. The guaranteed stop for an in-flight run is the operator cancelling the workflow run in the Actions UI. Acceptable delay for that manual cancel is the time GitHub takes to kill the runner, which this plan does not measure. If the variable cannot be read because it is unset, the candidate's exact-`false` test denies the run.

| ID | Setup | Observable guarantee | Not claimed |
| --- | --- | --- | --- |
| STOP-001 | Variable unset. | The first step exits 2. No checkout log and no `gh api` log. | None beyond that run. |
| STOP-002 | Variable `true`. | Exit 2 before checkout and before GET. | A running step sees the new value. |
| STOP-003 | Variable exactly `false`. | The run may pass the stop steps. | That CI itself succeeded. |
| STOP-004 | Variable `FALSE`, `False`, `0`, or empty. | Exit 2. | Any other spelling is allowed. |
| STOP-005 | Cancel is not required if exit 2 is first. | Checkout does not start. | A later edit of the variable. |
| STOP-006 | The classify step repeats the check. | GET does not start when the step env is not `false`. | A change made after the step env was captured. |
| STOP-007 | Set the variable to `true` after page 1 has been fetched. | This plan expects the current step to keep the old env. | An automatic stop between pages. |
| STOP-008 | Set the variable to `true` after GET and before the Python process is trusted to re-read it. | The Python process sees the env captured at step start. | A live re-read of the repository variable. |
| STOP-009 | Set the variable to `true` after the run has started. | Later steps may or may not observe it. Record the actual result. Do not call a guess a pass. | A universal in-flight stop. |
| STOP-010 | Operator cancels the run. | The run ends as cancelled. | That the classifier exited 2 by itself. |

`STOP_RUNTIME_GUARANTEE=NOT_PROVEN_FOR_IN_FLIGHT_RUN`. The operator sequence is `BINTRANS_AUTOPILOT_MONITOR_EMERGENCY_STOP_RUNBOOK_0_5E.md`. Changing the repository variable is not a proven stop for a step that has already started.

## 0.5E dispatch grouping

Every row above stays mandatory. Grouping only shares a run across checks that can be observed together. It does not delete a check.

| Dispatch | Checks | Reservation |
| --- | --- | --- |
| D1 | STOP-001, STOP-005, STOP-006. Variable unset. | 11 |
| D2 | STOP-002. Variable `true`. STOP-005 and STOP-006 are recorded again. | 11 |
| D3 | STOP-004. One invalid value. Local tests still cover the other spellings. | 11 |
| D4 | One fixture job so D5 has a completed run to read. Timeout 2 minutes. | 3 |
| D5 | STOP-003, HOSTED-001, HOSTED-002, HOSTED-011, HOSTED-012, HOSTED-013, HOSTED-014, HOSTED-015, HOSTED-016. After the first GET, the operator sets the variable and records STOP-007, STOP-008, and STOP-009, then cancels the run for STOP-010. | 11 |
| D6 | HOSTED-004, HOSTED-005, HOSTED-006, HOSTED-007, HOSTED-008, HOSTED-009, HOSTED-010. One throwaway job in the isolated repository. It is not a change to the candidate file. | 11 |
| D7 | HOSTED-003 page 2, a run with at least 101 jobs. | Denied under the 1-minute-per-job floor. |

D1 through D6 reserve 58 minutes. That fits in 60 only when each run is reconciled before the next one starts. HOSTED-003's short-page stop is visible in D5, because that run has fewer than 100 jobs. The live page-2 fetch is not in those 58 minutes. A 101-job fixture is at least 101 minutes under the fail-closed floor, so it is `CONTROLLER_DECISION` and is not dispatched by this plan.

## PostgreSQL budget boundary

The read-only monitor does not call a model, the budget controller, or PostgreSQL. A missing budget test does not authorize paid automation.

| Later stage | Budget rule |
| --- | --- |
| Read-only monitor | No spend. Budget verification is not a substitute for the permission proof. |
| Scheduler | Remains off until a separate GO. A schedule is not added by this plan. |
| Autonomous worker | Remains off. No queue connection. |
| Paid model call | Denied unless a future contract sets a verified price and a non-zero cap. `PAID_AUTOMATION_ENABLED=true` is not spend approval. |
| Repository writes | Not a budget feature. They stay denied by permissions. |

Isolated budget proof, not run now: a disposable database, not staging and not production, with caps at 0, emergency stop set, and `PAID_AUTOMATION_ENABLED=true`. The expected controller result is deny. No migration is applied to a shared database. `POSTGRES_BUDGET=NOT_RUN`.
