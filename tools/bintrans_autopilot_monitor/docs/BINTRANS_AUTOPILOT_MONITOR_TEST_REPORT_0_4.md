# Monitor test report 0.4 R2

These runs are local. They do not prove that a GitHub-hosted workflow accepts the YAML or enforces the permissions. The workflow file is not published.

## Monitor pack

COMMAND:

```text
py -3 -m unittest tests.test_monitor -v
```

Working directory: `D:\Projects\freight-platform-wt\bintrans-dev-autopilot-monitor-0.4-r2`

R2 result, directory `D:\Projects\freight-platform-wt\bintrans-dev-autopilot-monitor-0.4-r2`: `Ran 17 tests in 0.009s` `OK`

R3 result, directory `D:\Projects\freight-platform-wt\autopilot-monitor-0.4-r3\tools\bintrans_autopilot_monitor`, LF candidate: `Ran 17 tests in 0.016s` `OK`

| TEST | FILE | RESULT | EVIDENCE |
| --- | --- | --- | --- |
| MON-001 | `tests/test_monitor.py` | PASS | Declared permissions are `actions: read` and `pull-requests: read`. `workflow_dispatch` is the only trigger. Stop step is before `gh api`. `metadata:`, `issues:`, and `contents:` are absent from the YAML. |
| MON-002 | `tests/test_monitor.py` | PASS | Missing stop exits 2 with `DENIED` and no observations. |
| MON-003 | `tests/test_monitor.py` | PASS | `true`, `False`, `FALSE`, `0`, `no`, empty, and unreadable all stop. |
| MON-004 | `tests/test_monitor.py` | PASS | Failed `example-unit` stays `FAILURE` with run id `45708990004` and SHA `0755e2f29c92a96fdf14c4444aa8e3d815c17619`. |
| MON-005 | `tests/test_monitor.py` | PASS | A success job stays `SUCCESS` and the process exit is 0. |
| MON-006 | `tests/test_monitor.py` | PASS | `cancelled` and `timed_out` keep those conclusions. Overall is not `SUCCESS`. |
| MON-007 | `tests/test_monitor.py` | PASS | NLO line on run `45708992126` and synthetic run `90000000002` groups to count 2. Labels are `KNOWN_ERROR_SIGNATURE`, `ENVIRONMENT_SUSPECTED`, `ROOT_CAUSE_UNCONFIRMED`. `ci_result=FAILURE`. `retry` is false. |
| MON-008 | `tests/test_monitor.py` | PASS | Untrusted issue text does not add an action and does not flip `FAILURE`. |
| MON-009 | `tests/test_monitor.py` | PASS | Write verbs raise `SecurityDeny`. `github_mutations` stays 0. |
| MON-010 | `tests/test_monitor.py` | PASS | No Cursor URL in the package or the workflow. `cursor_calls=0`. |
| MON-011 | `tests/test_monitor.py` | PASS | Two passes over the same failed run share a digest. |
| MON-012 | `tests/test_monitor.py` | PASS | `ConnectionError` yields `UNKNOWN`. `LiveGitHubTransport` yields `ERROR` / `live_github_not_authorized`. No socket. |
| MON-013 | `tests/test_monitor.py` | PASS | Wrong repository, mismatched job SHA, and unexpected SHA are `ERROR`. |
| MON-014 | `tests/test_monitor.py` | PASS | `bintrans_autopilot` is not imported. |
| MON-015 | `tests/test_monitor.py` | PASS | A later call re-reads the stop source. Allow is not cached. |
| MON-016 | `tests/test_monitor.py` | PASS | The three GET paths are `/actions/runs`, `/actions/runs?status=failure`, and `/pulls`. Derived permissions equal the YAML permissions. `issues` and `contents` are not derived and not declared. |
| stop gate | `tests/test_monitor.py` | PASS | Denied stop values leave `transport.loads` at 0. The YAML has one `jobs:` block, no `continue-on-error`, no `always()`, and no `uses:` step. |

## Product-backed tests

`J-MON-R2-M002` is corrected here. The R2 command used `D:\Projects\freight-platform-wt\autopilot-phase-a-docs\tools\bintrans_autopilot\tests` at PR #411 head `0755e2f29c92a96fdf14c4444aa8e3d815c17619`. Those test files are not byte-identical to `origin/main` `c194da0e4943668e66953407eacd87f438e05c0b`. The PR #411 documentation worktree is not the product-byte source for these five tests.

Agent J ran the five tests from `git archive` of `origin/main` `c194da0e4943668e66953407eacd87f438e05c0b`, with `tools/bintrans_autopilot` and `.github/workflows/bintrans-autopilot-dispatcher.yml`. Agent J recorded `5/5 OK`.

R3 repeats the five tests on a full checkout of current `origin/main` `34a328da00a646806caeb5f2294e9de065dacb02`. `git diff c194da0e4943668e66953407eacd87f438e05c0b HEAD -- tools/bintrans_autopilot .github/workflows/bintrans-autopilot-dispatcher.yml` was empty. The only files between those commits are the merged PR #412 `services/rfx-service` allocation-policy files.

COMMAND:

```text
py -3 -m unittest test_dispatcher.DispatcherTests.test_emergency_stop_makes_no_cursor_call test_cost_provider.CostProviderTests.test_unknown_cost_is_not_free_and_blocks_cursor_default test_remediation.RemediationTests.test_pull_request_workflow_cannot_reach_cursor_secrets test_remediation.RemediationTests.test_actions_pull_request_returns_before_autopilot_startup test_models.ModelRoutingTests.test_role_and_risk_select_only_approved_catalog_ids -v
```

Working directory: `D:\Projects\freight-platform-wt\autopilot-monitor-0.4-r3\tools\bintrans_autopilot\tests`

`PYTHONPATH`: `D:\Projects\freight-platform-wt\autopilot-monitor-0.4-r3\tools` and `D:\Projects\freight-platform-wt\autopilot-monitor-0.4-r3\tools\bintrans_autopilot\tests`

RESULT: `Ran 5 tests in 0.010s` `OK`

| TEST | FILE | RESULT | EVIDENCE |
| --- | --- | --- | --- |
| `test_emergency_stop_makes_no_cursor_call` | `tools/bintrans_autopilot/tests/test_dispatcher.py` | PASS | Product dispatcher makes no Cursor create when the stop env is set. Fake GitHub client. |
| `test_unknown_cost_is_not_free_and_blocks_cursor_default` | `tools/bintrans_autopilot/tests/test_cost_provider.py` | PASS | Unverified cost stays `unknown_cost` and does not launch. |
| `test_pull_request_workflow_cannot_reach_cursor_secrets` | `tools/bintrans_autopilot/tests/test_remediation.py` | PASS | Dispatcher workflow has no pull-request trigger. Guard rejects `pull_request_target`. |
| `test_actions_pull_request_returns_before_autopilot_startup` | `tools/bintrans_autopilot/tests/test_remediation.py` | PASS | A pull_request Actions context returns exit 2 before a paid launch. |
| `test_role_and_risk_select_only_approved_catalog_ids` | `tools/bintrans_autopilot/tests/test_models.py` | PASS | `require_distinct_provider: true` raises `provider_unverified` on the fixture catalog. |

## Not run

| TEST | RESULT | EVIDENCE |
| --- | --- | --- |
| `BudgetControllerR2Tests.test_emergency_stop_not_bypassed_by_test_harness` | NOT_RUN | Previously skipped as `postgres_unavailable` while `BINTRANS_AUTOPILOT_QUEUE_DSN` was unset. Not re-run against a database. |
| Hosted workflow syntax and permission enforcement | NOT_RUN | Candidate is local only. |
| Live GitHub API | NOT_RUN | `LiveGitHubTransport` is not armed. |
| Hosted permission enforcement | NOT_RUN | `J-MON-R2-M003` remains OPEN. |
| Workflow execution of `src/bintrans_monitor` | LOCAL_ONLY | 0.5a candidate calls `python3 -m bintrans_monitor.cli`. Hosted workflow execution remains NOT_RUN. |
| PostgreSQL budget emergency-stop proof | NOT_RUN | `BINTRANS_AUTOPILOT_QUEUE_DSN` was unset. |

## 0.5a local rerun

COMMAND:

```text
py -3 -m unittest discover -s tests -v
```

Working directory: `D:\Projects\freight-platform-wt\autopilot-monitor-0.5a\tools\bintrans_autopilot_monitor`

RESULT: `Ran 37 tests in 0.050s` `OK`

That is MON-001 through MON-036 and the stop-gate test. These runs do not prove a GitHub-hosted workflow.
