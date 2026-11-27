# Release inventory 0.4 R2

Prepared copy: `tools/bintrans_autopilot_monitor/` on `feat/bintrans-autopilot-monitor-draft-v0.4`.

The verified R2 tree remains at `D:\Projects\freight-platform-wt\bintrans-dev-autopilot-monitor-0.4-r2`. Its manifest file hash is `67206E0BBAEDE4B92D81F24B02FF47FC2570E8388E57632ECD5AF95B6F53F3BD`. This copy changes documentation for `J-MON-R2-M002` and records the open limits. Its own manifest is regenerated after those edits.

The verified preflight tree remains at `D:\Projects\freight-platform-wt\bintrans-dev-autopilot-monitor-0.4`. Its manifest file hash is `C7AC425F4D768077A8C6B180294919635551D2604185286A6915FA748DF0F694`.

The workflow file stays inside this tree. It is not installed at `.github/workflows/bintrans-autopilot-monitor.yml`. Draft PR creation waits for a separate Controller GO. This prototype is not production-ready.

| Path | Role |
| --- | --- |
| `candidate/.github/workflows/bintrans-autopilot-monitor.yml` | Unpublished `workflow_dispatch` candidate. The R2 copy granted `actions: read` and `pull-requests: read`. The current 0.5 file grants `actions: read` and `contents: read` only. |
| `src/bintrans_monitor/__init__.py` | Package exports. |
| `src/bintrans_monitor/engine.py` | Read-only classifier. Stop gate runs before a transport load. |
| `src/bintrans_monitor/workflow_static.py` | Derives required permissions from the GET paths. |
| `src/bintrans_monitor/forbidden_snippets.txt` | Strings that must not appear in the workflow. |
| `tests/test_monitor.py` | MON-001 through MON-016 and the stop-gate test. |
| `fixtures/nlo_repeated.json` | Observed NLO run `45708992126` plus synthetic run `90000000002`. |
| `fixtures/success_run.json` | Success fixture. |
| `fixtures/unit_failure.json` | Non-NLO failure fixture. |
| `fixtures/cancelled_and_timed_out.json` | Cancelled and timed-out fixtures. |
| `fixtures/sha_mismatch.json` | Job SHA that does not match the run. |
| `docs/BINTRANS_AUTOPILOT_MONITOR_DESIGN_0_4.md` | Design, including the R2 permission note. |
| `docs/BINTRANS_AUTOPILOT_MONITOR_SECURITY_0_4.md` | Security contract. `J-DA-R2-M001` is `CLOSED_INVALID_REQUIREMENT`. |
| `docs/BINTRANS_AUTOPILOT_MONITOR_CI_CLASSIFICATION.md` | Failure labels. |
| `docs/BINTRANS_AUTOPILOT_MONITOR_TEST_REPORT_0_4.md` | Commands and results for this candidate. |
| `docs/BINTRANS_AUTOPILOT_PHASE_A_PUBLICATION_PLAN.md` | Future GO list. Not executed. |
| `docs/BINTRANS_AUTOPILOT_MONITOR_RELEASE_INVENTORY_0_4_R2.md` | This inventory. |
| `MANIFEST.sha256` | SHA256 of every file above except itself. |

Hashes are in `MANIFEST.sha256`. Reproduce a row with SHA256 of the file bytes. Do not hash `__pycache__`.

## Current 0.5 candidate

`CURRENT_CANDIDATE`. The table above is the R2 inventory plus the permission correction required by `J-MON-J-L001`. The file list that this tree actually hashes is `MANIFEST.sha256`.

The current workflow file does not grant `pull-requests: read`. It grants `contents: read` because checkout is pinned to `actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683` with `persist-credentials: false` and `ref: ${{ github.sha }}`. Its GitHub API calls are only:

- `GET repos/${REPOSITORY}/actions/runs/${RUN_ID}`
- `GET repos/${REPOSITORY}/actions/runs/${RUN_ID}/jobs?per_page=100&page=${page}`

There is no `/pulls` call and no `/issues` call. The candidate is still not installed at `.github/workflows/bintrans-autopilot-monitor.yml`. Hosted permission enforcement remains `NOT_RUN`.
