# DRY to product test matrix

Revision reason: J-DA-H003. The simulator `offline_dry_run.py` is `SIMULATED_ONLY`. A row is `UNIT_TESTED` or `POSTGRES_INTEGRATION_TESTED` only when that test was executed. None were executed in this revision, so those columns are `NOT_RUN`. Names below are tests that exist on `origin/main` `5432e1444ea4ed607cc8992e967023b241315759`. Their presence is not a pass.

| DRY_ID | SIMULATOR_TEST | PRODUCT_MODULE | PRODUCT_TEST | EXPECTED_SECURITY_PROPERTY | CURRENT_EVIDENCE | REMAINING_GAP |
| --- | --- | --- | --- | --- | --- | --- |
| DRY-001 | `offline_dry_run.py` enqueue with controller event | `dispatcher.py`, `postgres_queue.py` | none for a GitHub App controller event | Only a controller event enters the queue | `SIMULATED_ONLY` | Product still has no controller-app event. `NOT_RUN` |
| DRY-002 | same, event absent | `dispatcher.py` | `test_remediation.py` `test_contract_approval_requires_allowlisted_actor_and_digest` | A label or comment does not authorize | `SIMULATED_ONLY`. Product test exists and was `NOT_RUN` | Product check is an allowlisted actor plus digest, not the webhook contract |
| DRY-003 | `model_price_known=false` | `models.py` `unpaid_model_reason` | `test_cost_provider.py` `test_unknown_cost_is_not_free_and_blocks_cursor_default` | Unknown price denies | `SIMULATED_ONLY`. Product test `NOT_RUN` | Re-run on current main before any paid GO |
| DRY-004 | remaining budget below task cap | `dispatcher.py`, `budget_controller.py` | `test_dispatcher.py` `test_budget_exceeded_does_not_launch` | Exhausted budget stops dispatch | `SIMULATED_ONLY`. Product test `NOT_RUN` | Caps on main are 0; the test uses a fake client |
| DRY-005 | same delivery id | `postgres_queue.py` | `test_postgres_queue_03b.py` `test_same_idempotency_key_does_not_duplicate` | One delivery does not duplicate work | `SIMULATED_ONLY`. Product test `NOT_RUN` | Simulator key is not the PostgreSQL idempotency key |
| DRY-006 | world head differs | `dispatcher.py` | `test_dispatcher.py` `test_stale_pr_cannot_merge` | Stale head blocks the next grant | `SIMULATED_ONLY`. Product test `NOT_RUN` | Product test is PR identity, not a controller approval TTL |
| DRY-007 | `reviewer=agent-j` from a comment | `audit_gateway.py` on draft PR #403 only | none on `main` | Forged reviewer is rejected | `SIMULATED_ONLY` | Gate is absent on `main`. Draft file was not executed |
| DRY-008 | CI failure with trusted actor | `dispatcher.py` | `test_remediation.py` `test_newer_failed_check_blocks_an_older_success` | CI failure blocks completion | `SIMULATED_ONLY`. Product test `NOT_RUN` | Does not prove the future reviewer gate |
| DRY-009 | `emergency_stop=true` | `config.py` `runtime_enabled` | `test_dispatcher.py` `test_emergency_stop_makes_no_cursor_call` | Stop blocks a new Cursor call | `SIMULATED_ONLY`. Product test `NOT_RUN` | Test covers the env flag, not a missing variable on the monitor |
| DRY-010 | hold survives `restart()` | none on `main` | none | A restart does not clear a security hold | `SIMULATED_ONLY` | No persistent hold store on `main` |
| DRY-011 | second claim past max retries | `dispatcher.py`, `postgres_queue.py` | `test_dispatcher.py` `test_run_error_retries_once_then_blocks`; `test_postgres_queue_03b.py` `test_retries_end_dead_and_nonretry_fails` | Retry cap stops execution and does not raise the cost cap | `SIMULATED_ONLY`. Product tests `NOT_RUN` | Confirm the product retry does not change `MAX_COST` |
| DRY-012 | two agents, one task | `postgres_queue.py`, `dispatcher.py` | `test_postgres_queue_03b.py` `test_two_workers_do_not_both_claim`; `test_dispatcher.py` `test_two_dispatchers_claim_a_task_once` | One lease holder | `SIMULATED_ONLY`. Product tests `NOT_RUN` | Isolated PostgreSQL was not started |
| DRY-013 | injected allow-list in the body | `contract.py`, `dispatcher.py` | `test_dispatcher.py` `test_issue_text_cannot_override_the_contract` | Issue text does not change permissions | `SIMULATED_ONLY`. Product test `NOT_RUN` | Not a full prompt-injection suite |
| DRY-014 | write requested under contents read | candidate monitor workflow | none | Unauthorized GitHub write is denied | `SIMULATED_ONLY` | Workflow is not published, so the permission set is not enforced by GitHub yet |
| DRY-015 | provider unverified | `models.py` | `test_cost_provider.py` `test_config_provider_string_is_not_independence` | Unknown provider denies | `SIMULATED_ONLY`. Product test `NOT_RUN` | `require_distinct_provider` is false in committed config |

## Plan for DRY-016 through DRY-020

These tests are not written and were not run. Classification: `NOT_RUN`.

| DRY_ID | Planned product test | Property | Where it belongs |
| --- | --- | --- | --- |
| DRY-016 | Expired controller event and a reused `approval_id` are denied | TTL and one-time approval | Future controller receiver. Not Phase A |
| DRY-017 | Request without a valid `X-Hub-Signature-256`, and a second `delivery_id`, are denied | HMAC and replay | Future webhook. Phase A does not listen |
| DRY-018 | `unpaid_model_reason` denies when `model` is omitted | Cursor default is not a price | Extend `test_cost_provider.py`. Do not change production flags in that test |
| DRY-019 | Two concurrent `budget_protocol_reserve` calls cannot both succeed past the cap | Reserve race | Isolated PostgreSQL only, never a shared database |
| DRY-020 | `workflow_guard` returns untrusted for a `pull_request_target` context | Untrusted PR workflow cannot reach secrets | `test_remediation.py` already has a pull-request case. Add the `pull_request_target` event name without running that workflow |

No row above is `UNIT_TESTED` or `POSTGRES_INTEGRATION_TESTED` for this revision.
