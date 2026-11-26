# CI classification

The monitor reports observations. It does not repair CI and it does not retry a job.

## Observed NLO failure used as a fixture

This text was read from CI run `45708992126` during the docs PR stage. That run's head SHA is `0755e2f29c92a96fdf14c4444aa8e3d815c17619`. The failing job is `network-optimizer-chain-persistence-integration`. The log line is:

```text
embedded postgres: no version found matching 16.4.0
```

This preflight did not call GitHub again. `fixtures/nlo_repeated.json` stores that run and that line. A second run id, `90000000002`, is synthetic. It exists so the grouper can count two copies of the same signature. It is not a second live run.

## Labels

When the job name and the exact line match, each observation gets:

| Field | Value |
| --- | --- |
| `KNOWN_ERROR_SIGNATURE` | The line matched. |
| `ENVIRONMENT_SUSPECTED` | The line talks about an embedded Postgres version lookup. |
| `ROOT_CAUSE_UNCONFIRMED` | The log line is not treated as a finished root-cause proof. |
| `ci_result` | `FAILURE` |
| `retry` | false |

`ENVIRONMENT_SUSPECTED` is not a finding that the runner image is the cause. A code pin to `16.4.0`, a cache miss, or a runner image change could produce the same line. This stage does not choose among them.

The overall report stays `FAILURE`. A repeated signature does not become `SUCCESS`. No retry counter increments. The NLO service and PostgreSQL were not changed.

## Other conclusions

| Conclusion | Observation `ci_result` | Overall when it is the strongest result |
| --- | --- | --- |
| `failure` | `FAILURE` | `FAILURE` |
| `cancelled` | `CANCELLED` | `CANCELLED` |
| `timed_out` | `TIMED_OUT` | `TIMED_OUT` |
| `success` | `SUCCESS` | `SUCCESS` only if every job succeeded |

A success job that shares a payload with a failure stays `SUCCESS` on its own row. The report `ci_result` stays `FAILURE`.

An empty observation list, a network error, a rejected repository, or a rejected SHA is `UNKNOWN` or `ERROR`, never `SUCCESS`.

## 0.5B false-success gate

`CI_RESULT=SUCCESS` and exit 0 are allowed only when every one of these is true:

- `run.status` is `completed`
- `run.conclusion` is `success`
- the job list is complete
- every considered job has `status=completed` and `conclusion=success`
- `run_id`, `run_attempt`, and `head_sha` agree
- no job id is missing, and no job id has conflicting records
- the API payload is not incomplete or unknown

`run.conclusion=failure` is `CI_RESULT=FAILURE` and exit 1 even when every visible job says `success`. `cancelled` and `timed_out` stay negative. A null or missing conclusion is not `SUCCESS`.

## Run attempt policy

`RUN_ATTEMPT_POLICY=latest_confirmed_attempt_only`

GitHub's `GET /repos/{owner}/{repo}/actions/runs/{run_id}` returns the latest attempt in `run_attempt`. `GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs` defaults to `filter=latest`, so those jobs belong to that attempt. Each job has its own `id`, `run_id`, `run_attempt`, `name`, `status`, `conclusion`, and `head_sha`. A rerun creates new job ids. Jobs from an older attempt are served by `GET /repos/{owner}/{repo}/actions/runs/{run_id}/attempts/{attempt_number}/jobs`. This candidate does not call that endpoint.

If a fixture also contains older attempts, only jobs whose `run_attempt` equals the run's `run_attempt` are classified. An older success cannot hide a failure on the selected attempt. If the selected attempt has no jobs, or `run_attempt` or `job.id` is missing, the result is `UNKNOWN` or `ERROR` with a nonzero exit. Two records for one `job.id` that disagree are `ERROR`. They are not reduced to the successful record. Two different `job.id` values with the same `name` are both kept.

## Known signatures

The standard Jobs API does not provide `log_excerpt`. The adapter discards any such field and sets `log_excerpt_verified` false. A real adapter result therefore stays `unclassified_failure`, `CI_RESULT=FAILURE`, and `ROOT_CAUSE_UNCONFIRMED`.

`KNOWN_ERROR_SIGNATURE` is used only when a local fixture sets `log_excerpt_verified` true and the excerpt contains the NLO or Playwright fixture line. That does not mean a hosted monitor read the log. This stage does not download logs and does not add GitHub permissions. `J-0.5A-M002` is not fully closed.
