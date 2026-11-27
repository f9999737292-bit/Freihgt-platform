# Hosted security preflight 0.5H

This is the plan for a later isolated proof. It does not create a repository, a GitHub App, a token, or a workflow run. `J-MON-J-F003` stays a document until a separate controller GO.

## Boundaries

| Boundary | What this stage can say |
| --- | --- |
| `BYPASS_DETECTION` | An unmatched run id, a second attempt, a wrong SHA, or a wrong workflow blocks the local campaign and keeps the reserve. |
| `BYPASS_PREVENTION` | Not proven. A person who can dispatch or rerun in GitHub can start a billed run before the ledger sees it. |
| `OPERATOR_TRUST_BOUNDARY` | The host administrator and any repository administrator are outside the ledger. `HOST_ADMIN_TRUST_BOUNDARY=YES`. |
| `GLOBAL_BILLING_CAP` | Not proven. The 60 minute figure is the local ledger cap. |

## Later isolated repository

| Item | Requirement before any live call |
| --- | --- |
| Repository | A new repository that is not this product repository. Not created here. |
| Owner | The controller named in that GO. Not a shared admin group. |
| Dispatch principal | A separate principal whose only write is dispatch of the allowlisted workflow. |
| Cancel path | A different principal from the monitor token. The monitor token is not given cancel. |
| Monitor token | `actions: read` and `contents: read` only. |
| Participants | No write access for the monitor principal. No extra people with dispatch rights. |
| Allowed ref | Branch protection on the named ref. Protection does not remove an administrator's ability to dispatch. |
| Manual rerun | A rerun without a new reservation is a bypass. The campaign blocks. GitHub is not configured here to forbid the rerun button. |
| External run | `observe_external_run` records the incident and blocks further local reserves. It does not cancel the GitHub run. |
| Workflow file | The candidate stays unpublished until the GO that also names the isolated repository. The repository literal in the candidate must be reviewed again for that repository. |

`BYPASS_PREVENTION_HOSTED=NOT_PROVEN`. Detection after a run exists is not prevention. `HOSTED_003` page 2 stays deferred. `J-MON-R2-M003` stays open until that hosted enforcement is actually run.
