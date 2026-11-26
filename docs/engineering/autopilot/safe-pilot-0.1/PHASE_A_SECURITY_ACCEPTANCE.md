# Phase A security acceptance

This checklist records the candidate. It does not mark Phase A enabled.

| ID | Check | Result |
| --- | --- | --- |
| A01 | Eight documents prepared under `docs/engineering/autopilot/safe-pilot-0.1/` | Prepared locally. Not committed |
| A02 | SHA256 manifest | `MANIFEST.sha256`, recomputed after the 0.2 edits |
| A03 | Workflow permissions are read | contents, actions, issues, pull-requests: read |
| A04 | No Cursor credential or API host in the candidate workflow | Checked by `check_candidate.py` |
| A05 | No schedule | `on: workflow_dispatch` only |
| A06 | Untrusted payload cannot grant permissions | Monitor performs GET and prints a report. No approval write |
| A07 | Emergency stop fail-closed | Any value other than explicit `false` exits 2 before the report |
| A08 | Webhook contract documented | Security model revision 0.2. Receiver not started |
| A09 | DRY mapping is honest | `DRY_TO_PRODUCT_MATRIX.md`. Product tests `NOT_RUN` |
| A10 | PR #403 unchanged | Head remains `a068ab6af64f14699304752956a192746dc23bf7` |
| A11 | Issue #313 unchanged | Still OPEN. No comment and no edit |
| A12 | Paid flags stay off | No config edit. `autopilot_enabled` on main remains false |

`PHASE_A_READY=NO`. The candidate is not on the default branch. A controller GO is required before commit or publish.
