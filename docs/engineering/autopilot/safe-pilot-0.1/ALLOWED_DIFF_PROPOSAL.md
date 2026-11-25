# Allowed diff proposal

Nothing in this list is applied. Commit, push, and a draft PR wait for a separate controller GO.

## Docs that may be added after GO

Target directory: `docs/engineering/autopilot/safe-pilot-0.1/`

- `BINTRANS_DEV_AUTOPILOT_24X7_ARCHITECTURE_0_1.md` (0.2)
- `BINTRANS_DEV_AUTOPILOT_GITHUB_CURSOR_INTEGRATION.md` (0.2)
- `BINTRANS_DEV_AUTOPILOT_BUDGET_MODEL.md` (0.1, unchanged bytes)
- `BINTRANS_DEV_AUTOPILOT_SECURITY_MODEL.md` (0.2)
- `BINTRANS_DEV_AUTOPILOT_AGENT_SCHEDULER.md` (0.1, unchanged bytes)
- `BINTRANS_DEV_AUTOPILOT_FAILURE_RECOVERY.md` (0.2)
- `BINTRANS_DEV_AUTOPILOT_DRY_RUN_REPORT.md` (0.2)
- `BINTRANS_DEV_AUTOPILOT_IMPLEMENTATION_ROADMAP.md` (0.2)
- `offline_dry_run.py` (unchanged bytes, labeled `SIMULATED_ONLY`)
- `MANIFEST.sha256`
- `CHANGELOG.md`
- `EMERGENCY_STOP_CANDIDATE.md`
- `DRY_TO_PRODUCT_MATRIX.md`
- `PHASE_A_SECURITY_ACCEPTANCE.md`
- `PR403_STALE_BASE_PLAN.md`
- `ALLOWED_DIFF_PROPOSAL.md`

## Docs that replace a file on main only after a second GO

- `docs/engineering/autopilot/EMERGENCY_STOP.md` may be replaced by `EMERGENCY_STOP_CANDIDATE.md`. Not done now. The live runbook on `main` still contains the file-merge instruction.

## Workflow that must not be committed in the same GO as the docs

- `.github/workflows/bintrans-autopilot-monitor.yml`

Presence on the default branch makes `workflow_dispatch` runnable. That is activation, not documentation. It needs its own controller GO, and even then the job denies unless `AUTOPILOT_EMERGENCY_STOP` is explicitly `false`. This proposal does not set that variable.

## Not in any diff

- `tools/bintrans_autopilot/**`
- `autopilot.config.json`
- SQL migrations
- PR #403 branch
- Issue #313
