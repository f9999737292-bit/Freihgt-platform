# Agent scheduler

Status: routing proposal. No agent is authorized to run.

## Competence map

The letter is a skill tag. It is not permission.

| Tag | Competence |
| --- | --- |
| A | Infrastructure and API gateway |
| B | Documents and ЭДО |
| C | TMS, execution, tracking |
| D | NLO optimization |
| E | Analytics and KPI |
| F | Frontend and customer portals |
| G | Finance and commercial research |
| H | Automation and this scheduler |
| J | Independent architecture and security review |

`OWNER_AGENT` on the task must match the tag. A task whose paths are finance-classified cannot be given to a frontend tag. The classifier on `main` already separates frontend, NLO, ЭДО, finance, analytics, TMS, and platform backend from approved paths, not from free text.

Agent J is never the owner of the implementation task it reviews.

## Starting caps

`MAX_CONCURRENT_DEVELOPMENT_AGENTS=2`

`MAX_CONCURRENT_REVIEW_AGENTS=1`

`max_parallel_agents` on `main` is `3`. This pilot proposes `2 + 1` instead, and does not change the committed file. Critical domains already have `max_parallel: 1` for OpenAPI, migrations, identity/gateway, and `packages/shared-go`.

## Eligibility

A task is selected only when every check passes. Any failure denies the claim.

- A controller event exists, is unexpired, and names the current base SHA.
- No other active lease overlaps `ALLOWED_PATHS` or the worktree.
- Dependencies are `COMPLETED`.
- Budget reserve fits.
- `SECURITY_HOLD` is clear.
- The model id is in `GET /v1/models` and has a verified price. Availability is re-checked at claim time. A catalog miss denies.
- The approval has not expired.
- The approved head still matches. A moved head denies.

Priority orders the eligible set. It does not override a deny.

## Selection order

1. Review tasks for Agent J, up to one, and only for a SHA that already has a draft PR and green required CI. The review gate still stays blocked until trusted identity exists, so this step does not run in Phases A–C.
2. Development tasks, up to two, highest priority first, skipping any task whose paths intersect a held lease.

The same task cannot be claimed twice. The PostgreSQL lease is the lock. A second claim receives an empty result, which the queue on `main` already does for one job.

## Model routing

Primary catalog: Cursor models returned by `GET /v1/models`. The id sent to `POST /v1/agents` is one of those ids. Omitting `model` selects Cursor's default and is denied while the price is unknown.

Claude API and DeepSeek API are candidates on paper only. They are not enabled. A cheaper sticker price is not treated as a lower total cost. Total cost includes retries and discarded pull requests.

Choice inputs, once a later phase is approved:

- task class from the existing route table (`ROUTINE` through `SECURITY_CRITICAL`);
- verified price against `MAX_COST_USD`;
- context length published for that model id;
- whether the task is an independent review, which then requires a provider distinct from the implementation provider;
- data policy: customer data and secrets stay out of the prompt;
- observed accepted-output rate from completed tasks. There is no such history yet.

Benchmark design, not executed:

- 10 typical Go tasks;
- 5 frontend tasks;
- 5 CI or debug tasks;
- 5 security-review tasks.

Record `TOTAL_COST`, `ACCEPTED_OUTPUT_RATE`, `REWORK_RATE`, `TIME_TO_ACCEPTED_PR`, `SECURITY_FINDINGS`, and `TOTAL_RETRIES`. No paid benchmark is run in this pilot. Result of the benchmark: `NOT_RUN`.
