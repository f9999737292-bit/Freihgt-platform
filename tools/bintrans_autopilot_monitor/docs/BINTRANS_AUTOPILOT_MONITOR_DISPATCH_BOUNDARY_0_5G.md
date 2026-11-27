# Dispatch boundary 0.5G

Local contract for a future isolated Actions proof. No live credential is configured. The real transport raises before a connection is opened. Tests use the offline transport only. The candidate workflow file is unchanged. This ledger is not a global cap on GitHub billing. `BYPASS_PREVENTION_HOSTED=NOT_PROVEN`.

## Separate pieces

| Piece | Role | Live secret |
| --- | --- | --- |
| Budget gate | Reserves, stop, ledger, local state machine | None |
| Dispatch adapter | Allowlisted workflow dispatch POST | None. Principal classification `SEPARATE` |
| GitHub read adapter | Commit, run, and jobs reads | None. Principal classification `READ_ONLY` |
| Cancellation interface | Cancel one run | None. Principal classification `SEPARATE_FROM_MONITOR` |
| Credential provider | Returns classifications only | `live_secret` raises. `LIVE_CREDENTIALS_CONFIGURED=NO` |
| Offline transport | Scripted responses for tests | None |

The monitor workflow keeps `actions: read` and `contents: read`. This stage does not add a permission to that workflow, does not create a PAT, and does not create a GitHub App.

## Permission matrix for a later isolated proof

| Operation | Principal | GitHub permission needed later | This stage |
| --- | --- | --- | --- |
| List one workflow run | Monitor read | `actions: read` | Offline only |
| List jobs, including later pages | Monitor read | `actions: read` | Offline only. A short page is incomplete |
| Read the commit object for the bound SHA | Monitor read | `contents: read` | Offline only |
| Workflow dispatch | Dispatch principal, not the monitor token | `actions: write` limited to the allowlisted workflow | Disabled |
| Cancel a run | Cancel principal, not the monitor token | `actions: write` for cancel | Disabled |
| Reconcile minutes | Read principal plus an authoritative billing source | Actions job duration is not billing | Fail closed |

## Allowlist

The adapter sends a request only when every one of these matches the campaign:

- repository `owner/name`
- workflow file name, one path segment
- ref that was bound ahead of the POST
- expected 40-character SHA
- method `GET` or `POST`
- path: commit read, workflow dispatch, one run, that run's jobs, or cancel of that run

Any other method, any absolute URL, any other repository, any other path, and any redirect are rejected. A redirect is not followed. If the dispatch POST may already have been sent, the ledger row stays unknown and the reserve stays. The POST is not sent again. There is no exactly-once claim.

## SHA binding

The public workflow endpoint, checked on 2026-11-27 at `https://docs.github.com/en/rest/actions/workflows`, says `ref` is a branch or tag name. It does not accept a raw commit SHA as `ref`. The same page, for API version `2026-03-10`, describes a successful body that includes `workflow_run_id`. Older descriptions of this endpoint use an empty 204. The adapter therefore does not put the expected SHA in `ref`, and it does not substitute an unbound branch name. An empty 204 and a body that only carries `workflow_run_id` are both unconfirmed. Neither proves that the expected SHA ran.

Before the POST it reads `/repos/{owner}/{name}/git/commits/{sha}`. The returned object SHA must equal the expected SHA, and the caller must pass a `TrustedRefBinding` whose ref is the allowlisted ref and whose object SHA is that same SHA. The POST body uses the branch or tag name as `ref`. The dispatch candidate added in 0.5H receives `budget_dispatch_id`, `campaign_id`, and `expected_head_sha`. The classifier candidate still has only `run_id` and `head_sha` and is not the dispatch target. If the commit read does not match, the POST is not sent.

A 204 response with an empty body is not proof that a particular run started. Correlation reads the run name GitHub shows for `run-name`, or an explicit correlation field in an offline fixture. Exactly one run may match. Zero or several matches stay `DISPATCH_UNKNOWN`. The adapter does not choose a run by `created_at` or by list position. A matching name is not authorization; repository, workflow, SHA, and `run_attempt` are checked again.

## Usage

Job start and completion times are a duration estimate. They are not treated as billable minutes. `apply_trusted_usage` rejects a report whose source is not authoritative billing, rejects a report that carries a caller-supplied minutes total, and rejects an incomplete or non-terminal report. The full reserve stays. An unverified `run_attempt` other than 1 records the job minutes that were supplied to reconciliation and blocks the campaign. A run id with no ledger row blocks the campaign. That is detection. It does not prevent the run.

## Who can bypass the local controller

| Action | Who can do it on GitHub | What this ledger does |
| --- | --- | --- |
| Dispatch the workflow | Anyone the repository allows to dispatch that workflow, including an administrator and a person using the Actions UI | Detects an unmatched run id and blocks new local reserves. Does not stop GitHub from billing the run |
| Rerun a job or a workflow | Anyone with rerun permission | `run_attempt` other than 1 blocks the campaign and keeps the minutes |
| Cancel | Anyone with write permission on Actions, or an administrator | The monitor token is not the cancel principal. In-flight cancel remains the operator runbook |
| Edit the workflow file | Anyone with write permission on the default branch, or an administrator | Out of scope. The candidate file in this repository is not the published workflow |
| Change the allowed ref or the required SHA | The local controller configuration, plus anyone who can dispatch a different ref from GitHub | The adapter refuses a ref that was not bound to the expected SHA. GitHub itself can still start another ref |
| Administer the repository | Organization owners and repository administrators | Can change permissions, secrets, and workflows. This ledger cannot see that change until a later read |

Prevention would require GitHub settings that refuse every dispatch except one dedicated principal, with administrators unable to dispatch or rerun. Those settings are not applied here and have not been tested against the live repository. `HOSTED_PERMISSION_ENFORCEMENT=NOT_RUN`.

Detection, after a run exists, is the local block: unknown usage, an unmatched run, a second attempt, or recovery required. Further local reserves deny. GitHub can still bill a run that this process did not create.

## Hosted proof, later and separate

A later controller GO would have to name the isolated repository, the dispatch principal, the cancel principal, and the read principal before any live call. This stage does not create that repository, does not publish the workflow, and does not dispatch it. `HOSTED_003` page 2 stays deferred. The planned reservations remain D1 through D6 at 58 minutes. That number is a reservation model, not a measured GitHub invoice. The cap stays 60. Paid automation stays off.
