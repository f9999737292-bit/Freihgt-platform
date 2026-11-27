# Hosted dispatch contract 0.5H

Local contract for J-MON-J-M003 and J-MON-J-F002. Nothing in this file publishes a workflow or starts a run. The classifier candidate `bintrans-autopilot-monitor.yml` is unchanged. The dispatch candidate is `candidate/.github/workflows/bintrans-autopilot-budget-dispatch.yml` and is not copied to `.github/workflows/`.

## Inputs

The adapter POST sends only these inputs. The workflow declares the same three, all required strings.

| Input | Rule | Meaning |
| --- | --- | --- |
| `budget_dispatch_id` | `^[A-Za-z0-9][A-Za-z0-9._-]{0,79}$` | Ledger `dispatch_id` |
| `campaign_id` | same rule, and it must equal the ledger campaign | Campaign binding |
| `expected_head_sha` | `^[0-9a-f]{40}$` | Exact head SHA |

The ledger identity limit is 80 characters, so the workflow uses that tighter bound. No other input is sent. `ref` on the POST remains the branch or tag name. GitHub's workflow dispatch API does not accept a raw commit SHA as `ref`.

## SHA guard

`github.sha` is compared with `expected_head_sha` in `sha-guard` before any checkout, and again as the first step of `trusted-checkout`. A missing value, a bad format, a repository other than `f9999737292-bit/Freihgt-platform`, a stop value other than exact `false`, or a SHA mismatch exits non-zero. The checkout step is not reached. This workflow does not call `gh api`.

Checkout uses the pinned `actions/checkout` commit, `repository: f9999737292-bit/Freihgt-platform`, `ref: inputs.expected_head_sha`, and `persist-credentials: false`. Each job has `timeout-minutes` of at most 10.

The guard does not stop GitHub from scheduling the run. A branch can move after the adapter reads the commit and before GitHub resolves `ref`. The wrong head can consume Actions minutes until the guard exits. `REF_MUTATION_RACE_PROTECTION=DETECT_AFTER_START`. `SHA_GUARD` is not a prevention of that start. A commit read that returns a different SHA raises `ref_changed` and does not POST. That reserve stays held.

## Trusted actor and event

GitHub `actor` is the account that started the workflow run. `triggering_actor` is the account that caused this attempt. A rerun can change `triggering_actor` while `actor` stays the same. `TrustedDispatchPrincipal` supplies the numeric actor id, repository id, workflow id, and branch from configuration. Those values are not workflow inputs.

A trusted correlation requires `repository.id`, `workflow_id`, `head_sha`, `head_branch`, `event`, `actor.id`, `triggering_actor.id`, the run id, `run_attempt`, `campaign_id`, and `budget_dispatch_id`. A missing field is `missing_identity`. A different actor id is `actor_rejected` even when the login matches. A different triggering actor is `triggering_actor_rejected`. `event` must be `workflow_dispatch`. `run_attempt` must be 1. A matching actor id does not release a reserve and does not prove the budget gate.

`inspect_run_inventory` returns every supplied run. Filtering the list by actor or event is rejected. An external dispatch, a manual rerun, an unknown actor, an unexpected event, a repeated correlation id, a run without a reservation, or a wrong SHA stays visible and is an incident. The campaign is blocked and the reserve is not released.

## Correlation

`run-name` is `bintrans-budget {campaign_id} {budget_dispatch_id} {expected_head_sha}`. The list-runs API exposes that string as `name` and `display_title`. That is the observable correlation identifier. It does not authorize the run.

`correlate_run` accepts one run only when the identifier, repository, workflow, head SHA, and `run_attempt` agree. `run_attempt` other than 1 is a mismatch. Zero matches or more than one match raise `dispatch_unknown`. The reserve stays. The POST is not retried. A later unique observation can be bound with `bind_observed_run`; that still does not release the reserve. Selection does not use `created_at` or list position.

## Permissions

`actions: read` and `contents: read` only. No schedule, no `pull_request`, no `pull_request_target`, no write permission, no paid model call, and no private token in the workflow. The job token is the read-only `github.token` and this dispatch candidate does not use it.
