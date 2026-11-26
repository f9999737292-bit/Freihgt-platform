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
