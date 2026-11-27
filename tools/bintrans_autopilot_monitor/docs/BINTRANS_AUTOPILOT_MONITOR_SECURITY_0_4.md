# BINTRANS autopilot monitor security 0.4

## Workflow token versus GitHub App

The candidate workflow sets `permissions` for the job's `GITHUB_TOKEN`. That token is created for the job and expires with the job. The keys in the workflow file are the workflow-syntax permissions.

A GitHub App installation has a separate permission set, configured on the app. App permission names, including the app Metadata permission described in GitHub App documentation, are not the same object as a workflow `permissions` key. Granting or omitting a workflow key does not edit the app registration. This prototype does not create or modify a GitHub App.

## Permissions in the candidate

Fetched on 2026-11-26 from the official workflow syntax:

https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax

The permissions table and the YAML example list `actions`, `artifact-metadata`, `attestations`, `checks`, `code-quality`, `contents`, `copilot-requests`, `deployments`, `discussions`, `id-token`, `issues`, `packages`, `pages`, `pull-requests`, `security-events`, `statuses`, and `vulnerability-alerts`. They do not list a key named `metadata`.

`METADATA_PERMISSION_DECISION=OMITTED_NOT_IN_OFFICIAL_WORKFLOW_SYNTAX`

`metadata: read` is not added. Adding a key that the current syntax page does not list could make a future workflow invalid. `artifact-metadata` is a different permission and is not granted.

`J-DA-R2-M001=CLOSED_INVALID_REQUIREMENT`. Agent J closed it because the official syntax does not list a `metadata` key. This release candidate does not add `metadata: read`.

## R2 least privilege

`HISTORICAL_R2_CANDIDATE_NOT_CURRENT`. This section records the unpublished R2 candidate. It is not the 0.5 candidate. The current contract is the section `0.5 current candidate` below.

The R2 candidate's `gh api --method GET` calls were only:

- `repos/${REPOSITORY}/actions/runs`
- `repos/${REPOSITORY}/actions/runs?status=failure`
- `repos/${REPOSITORY}/pulls?state=open`

The R2 candidate had no `/issues` request and no checkout. `J-MON-M001` asked to drop `issues: read` or to justify an issues call. The call was not used, so `issues: read` was removed. `contents: read` was also absent from that R2 file: it did not call a contents endpoint and did not check out code.

R2 granted workflow permissions:

```yaml
actions: read
pull-requests: read
```

Not granted: `contents`, `issues`, `metadata`, `checks`, `statuses`, `id-token`, `deployments`, any `write`, `read-all`, `write-all`.

## Triggers

`on` contains only `workflow_dispatch`. `schedule`, `push`, `pull_request`, `pull_request_target`, and `repository_dispatch` are absent. Any automatic trigger needs a separate controller GO.

The job `if` also requires `github.event_name == 'workflow_dispatch'` and `github.ref == 'refs/heads/main'`. The runner is `ubuntu-latest`. Timeout is 10 minutes. Concurrency does not cancel an in-progress report.

## Stop before read

The first step exits 2 unless `vars.AUTOPILOT_EMERGENCY_STOP` is exactly `false`. Checkout and the `gh api --method GET` steps are later in the file. Checkout is pinned, uses `repository: f9999737292-bit/Freihgt-platform` and `ref: ${{ github.sha }}`, and sets `persist-credentials: false`. A pull-request head is not checked out.

## Untrusted data

Issue text, comments, and job logs are data. `injection_suspected` records marker hits. The report's `actions` list stays empty, `untrusted_text_executed` stays false, and `ci_result` is not rewritten because the text asked for a pass.

The package does not import `subprocess`, does not call `os.system`, and does not open a socket. `reject_mutation` raises `SecurityDeny` for every verb outside `load_runs`, `load_jobs`, and `load_pulls`.

## Dispatcher boundary

`workflow_guard` on main trusts the dispatcher file, which has write permissions and receives `CURSOR_API_KEY` after the guard. The monitor candidate does not call that workflow, does not import that package, and does not read those secrets.

`audit_gateway.py` and `supervisor.py` remain only on draft PR #403. They are not copied and not imported.

## What this document does not prove

Static tests and fixture runs do not prove GitHub's enforcement of `permissions` on a hosted runner. That proof waits for a published workflow and was not run. No production secret, deployment, or privileged runner was added.

`J-MON-R2-M001` is addressed in the inactive candidate: the candidate calls the existing Python engine and returns that engine's exit code. Hosted execution of the candidate was not run, so this is not a production closure.

`J-MON-R3-L001` is addressed in the candidate: GitHub API bodies go to a temporary directory and are removed on exit. The process prints sanitized rows only. A hosted log review was not run.

`J-MON-R2-M003` remains OPEN. `HOSTED_PERMISSION_ENFORCEMENT=NOT_RUN`. This document does not call the monitor production-ready.

## 0.5 current candidate

`CURRENT_CANDIDATE`. This section describes `candidate/.github/workflows/bintrans-autopilot-monitor.yml` as it exists after the 0.5C preflight. `J-MON-J-L001` asked to stop describing that file with the R2 permission and route list.

Granted workflow permissions, and why each one is present:

| Key | Value | Why it is granted |
| --- | --- | --- |
| `actions` | `read` | `GET /repos/${REPOSITORY}/actions/runs/${RUN_ID}` and `GET /repos/${REPOSITORY}/actions/runs/${RUN_ID}/jobs?per_page=100&page=${page}`. |
| `contents` | `read` | The pinned checkout reads the trusted repository tree at `ref: ${{ github.sha }}`. |

Not granted: `pull-requests`, `issues`, `metadata`, `checks`, `statuses`, `id-token`, `deployments`, `artifact-metadata`, any `write`, `read-all`, `write-all`. The candidate does not call `/pulls` or `/issues`.

The only `gh api` commands are `--method GET`. There is no `POST`, `PUT`, `PATCH`, or `DELETE`. Jobs pagination stops at 50 pages. The selected run id and the expected head SHA are operator inputs. The job `if` requires `workflow_dispatch`, `refs/heads/main`, and `f9999737292-bit/Freihgt-platform`.

Checkout uses `actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683`, `repository: f9999737292-bit/Freihgt-platform`, `ref: ${{ github.sha }}`, and `persist-credentials: false`. A pull-request head is not checked out.

The first step exits 2 unless `vars.AUTOPILOT_EMERGENCY_STOP` is exactly `false`, and that step is before checkout. The classify step repeats the same exact-`false` test before `mktemp` and before either GET. This document does not claim that changing the repository variable stops a step that has already started. That behavior was not hosted-tested.

The candidate does not call Cursor, a paid model, or the product dispatcher. It does not download job logs. A Jobs API body without a verified excerpt stays `unclassified_failure`. Local fixture runs and static YAML checks do not close `J-MON-R2-M003`. `HOSTED_PERMISSION_ENFORCEMENT=NOT_RUN`.

`J-MON-J-L001=CLOSED` on the documentation delta. Agent J accepted that delta as docs-only. The closure does not close hosted permission enforcement.
