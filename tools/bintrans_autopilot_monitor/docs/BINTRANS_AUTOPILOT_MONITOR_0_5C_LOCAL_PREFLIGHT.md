# Monitor 0.5C local security preflight

Stage `MONITOR-0.5C-LOCAL-SECURITY-PREFLIGHT`. Local and offline only. This stage does not publish the candidate and does not dispatch it.

## Baseline

`origin/main` for this branch is `8dfae68712230cc63754f78262dc9a9cf4a206ec`. PR #415 is already merged at `bf45e7c7f7a6d222689500f438ee50224724ca01`. Post-merge CI run `45879887987` succeeded on that merge SHA. The commits after that merge, through `8dfae687`, are RFX evaluator files only. They do not change `tools/bintrans_autopilot_monitor/`, `tools/bintrans_autopilot/`, or `.github/workflows/`.

The active path `.github/workflows/bintrans-autopilot-monitor.yml` is absent. The candidate remains under `tools/bintrans_autopilot_monitor/candidate/.github/workflows/`.

`J-MON-R2-M003` stays open. `HOSTED_PERMISSION_ENFORCEMENT=NOT_RUN`. A local PASS is not hosted permission enforcement.

## Closed 0.5B findings

These stay closed by the classifier already on main, and the new tests repeat them:

- `J-0.5A-C001` and `J-0.5A-H002`: different `job.id` values with the same name are both kept. A failure is not dropped.
- `J-0.5A-H001`: `run.conclusion=failure` stays `FAILURE` when the visible jobs succeeded.
- `J-0.5A-M001`: only the run's `run_attempt` is classified.
- `J-0.5A-M002`: the Jobs API has no verified log excerpt. The adapter does not invent one, so the result stays `unclassified_failure`.

## Defect fixed in this preflight

`cli._load_pages` used to sort `jobs-*.json` lexicographically. `jobs-10.json` was loaded before `jobs-2.json`. A short later page was then treated as a non-final page, so a complete ten-page run could become `UNKNOWN` instead of preserving a real `FAILURE`. The loader now requires contiguous numeric pages `jobs-1.json` through at most `jobs-50.json`.

The classify step fetched with `gh api` after checkout without repeating the emergency-stop test in that step. The candidate now repeats the exact `false` check immediately before `mktemp` and before either GET. The first stop step still runs before checkout. Actions reads `vars.AUTOPILOT_EMERGENCY_STOP` at step start; this candidate does not change repository variables.

The sanitized report now redacts a synthetic cursor-shaped key and `user:password@` URL credentials in addition to the existing GitHub token and bearer patterns. No live secret is used.

## What this stage does not prove

Hosted token enforcement, a real `workflow_dispatch`, and schedule behavior were not run. `AUTOPILOT_ENABLED` stays off. Paid automation and the worker stay off.
