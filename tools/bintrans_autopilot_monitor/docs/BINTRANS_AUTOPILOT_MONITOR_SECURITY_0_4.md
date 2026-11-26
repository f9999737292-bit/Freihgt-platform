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

The candidate's `gh api --method GET` calls are only:

- `repos/${REPOSITORY}/actions/runs`
- `repos/${REPOSITORY}/actions/runs?status=failure`
- `repos/${REPOSITORY}/pulls?state=open`

There is no `/issues` request and no checkout. `J-MON-M001` asked to drop `issues: read` or to justify an issues call. The call is not used, so `issues: read` is removed. `contents: read` is also removed: no contents or git endpoint is called.

Granted workflow permissions:

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

## 0.5B current candidate

This section describes the inactive 0.5 candidate. The R2 permission paragraph above describes the earlier unpublished candidate.

The 0.5 candidate grants `actions: read` and `contents: read`. `contents: read` is present because the candidate checks out the trusted repository SHA. It does not grant `pull-requests: read`, `issues: read`, `metadata: read`, or any write permission. `J-0.5A-L001` is this wording difference: the historical MON-001 row in the R2 test report still says `pull-requests: read`.

The candidate does not download job logs. A Jobs API response without a verified excerpt cannot become `KNOWN_ERROR_SIGNATURE`. `J-0.5A-M002` is not fully closed. No live log permission was added.
