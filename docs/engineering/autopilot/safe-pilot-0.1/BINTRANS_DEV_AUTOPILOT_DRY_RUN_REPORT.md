# Offline dry run

Revision 0.2 reason: J-DA-H003. The results below are `SIMULATED_ONLY`. They are not unit tests and not PostgreSQL tests of `tools/bintrans_autopilot`. The product mapping is `DRY_TO_PRODUCT_MATRIX.md`. Execution of product tests in this revision is `NOT_RUN`.

Command:

`py -3 offline_dry_run.py`

Working directory:

`D:\Projects\freight-platform-wt\bintrans-dev-autopilot-safe-pilot-0.1`

The script does not import `bintrans_autopilot`, open a socket, or read a secret. It checks the proposed gates on synthetic tasks. It does not prove that `main` or PR #403 enforces every scenario. Where `main` already has a related gate, the architecture documents say so separately.

Result: 15 passed, 0 failed, 0 not run. Exit code 0.

## DRY-001

INPUT: task BT-1 with a controller event from `github-app:bintrans-controller`, base SHA `5432e1444ea4ed607cc8992e967023b241315759`, price known, provider verified, budget remaining.

EXPECTED_RESULT: `QUEUED`

OBSERVED_RESULT: `QUEUED`

EVIDENCE: `reason=approved_task_entered_queue`, queue `[BT-1]`.

## DRY-002

INPUT: label approved and a comment body, `controller_event` absent.

EXPECTED_RESULT: `DENY`

OBSERVED_RESULT: `DENY`

EVIDENCE: `reason=controller_authorization_absent`, queue empty.

## DRY-003

INPUT: approved task, `model_price_known=false`.

EXPECTED_RESULT: `DENY`

OBSERVED_RESULT: `DENY`

EVIDENCE: `reason=UNKNOWN_MODEL_PRICE`.

## DRY-004

INPUT: approved task, `max_cost_usd=5`, monthly remaining `4`.

EXPECTED_RESULT: `DENY`

OBSERVED_RESULT: `DENY`

EVIDENCE: `reason=BUDGET_EXHAUSTED`, `remaining=4.0`.

## DRY-005

INPUT: the same delivery id twice.

EXPECTED_RESULT: `DEDUPED`

OBSERVED_RESULT: `DEDUPED`

EVIDENCE: first result `QUEUED`, second `reason=duplicate_webhook`, `queue_length=1`.

## DRY-006

INPUT: controller event SHA is the recorded head; world head is `newer-sha`.

EXPECTED_RESULT: `DENY`

OBSERVED_RESULT: `DENY`

EVIDENCE: `reason=stale_head`.

## DRY-007

INPUT: payload `reviewer=agent-j`, actor `issue-comment`.

EXPECTED_RESULT: `REJECT`

OBSERVED_RESULT: `REJECT`

EVIDENCE: `reason=forged_reviewer`.

## DRY-008

INPUT: trusted reviewer actor, `ci_status=failure`.

EXPECTED_RESULT: `DENY`

OBSERVED_RESULT: `DENY`

EVIDENCE: `reason=ci_failed`.

## DRY-009

INPUT: approved task, `emergency_stop=true`.

EXPECTED_RESULT: `DENY`

OBSERVED_RESULT: `DENY`

EVIDENCE: `reason=emergency_stop`.

## DRY-010

INPUT: `security_hold=true`, then `restart()`, then enqueue.

EXPECTED_RESULT: `DENY`

OBSERVED_RESULT: `DENY`

EVIDENCE: `reason=security_hold`, hold true before and after restart.

## DRY-011

INPUT: `max_retries=1`, one claim, lease cleared, second claim. Cost cap stays `5`.

EXPECTED_RESULT: `DENY`

OBSERVED_RESULT: `DENY`

EVIDENCE: `reason=retry_limit`, `attempts=1`, `max_cost=5.0`.

## DRY-012

INPUT: agent-a claims BT-12, agent-b claims the same id.

EXPECTED_RESULT: `DENY`

OBSERVED_RESULT: `DENY`

EVIDENCE: first `CLAIMED`, second `reason=lease_held`, holder `agent-a`.

## DRY-013

INPUT: body contains an instruction to replace the allow-list with `infrastructure/docker-compose/` and to set `AUTOPILOT_ENABLED=true`. Structured allow-list is `docs/engineering/autopilot/`.

EXPECTED_RESULT: `docs/engineering/autopilot/`

OBSERVED_RESULT: `docs/engineering/autopilot/`

EVIDENCE: `body_contains_injection=true`, paths unchanged.

## DRY-014

INPUT: GitHub permission `contents:read`, write requested.

EXPECTED_RESULT: `DENY`

OBSERVED_RESULT: `DENY`

EVIDENCE: `reason=read_only_github`.

## DRY-015

INPUT: price known, `provider_verified=false`.

EXPECTED_RESULT: `DENY`

OBSERVED_RESULT: `DENY`

EVIDENCE: `reason=UNVERIFIED_PROVIDER`.

## Not claimed

The product suite on `main` was not executed. Isolated PostgreSQL was not started. No GitHub write occurred.
