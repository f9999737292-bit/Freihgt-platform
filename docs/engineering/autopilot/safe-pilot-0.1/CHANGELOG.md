# Changelog

Source pack verified against Agent J on 2026-11-26 before any edit. All eight SHA256 values and the simulator hash matched `DOCUMENT_HASHES_VERIFIED` in the J review.

| File | Version | Reason |
| --- | --- | --- |
| `BINTRANS_DEV_AUTOPILOT_BUDGET_MODEL.md` | 0.1 | Unchanged. J hash stands |
| `BINTRANS_DEV_AUTOPILOT_AGENT_SCHEDULER.md` | 0.1 | Unchanged. J hash stands |
| `offline_dry_run.py` | 0.1 | Unchanged. Still `SIMULATED_ONLY` |
| `BINTRANS_DEV_AUTOPILOT_SECURITY_MODEL.md` | 0.2 | J-DA-H001 webhook contract. J-DA-M002 stop variable is fail-closed and is not the Git file |
| `BINTRANS_DEV_AUTOPILOT_24X7_ARCHITECTURE_0_1.md` | 0.2 | J-DA-C002 Phase A is a separate read-only workflow |
| `BINTRANS_DEV_AUTOPILOT_GITHUB_CURSOR_INTEGRATION.md` | 0.2 | J-DA-C002 dispatcher write permissions are not reused |
| `BINTRANS_DEV_AUTOPILOT_IMPLEMENTATION_ROADMAP.md` | 0.2 | J-DA-C002 Phase A does not enable the dispatcher |
| `BINTRANS_DEV_AUTOPILOT_DRY_RUN_REPORT.md` | 0.2 | J-DA-H003 results are not product tests |
| `BINTRANS_DEV_AUTOPILOT_FAILURE_RECOVERY.md` | 0.2 | J-DA-M002 running paid agents need a separate cancel |

New files in this handoff are version 0.2 and have no J hash: the manifest, this changelog, the emergency-stop candidate, the DRY matrix, the acceptance checklist, the PR #403 plan, the allowed-diff proposal, and the monitor workflow candidate.

## 0.3 documentation pull request

Stage `DEV-AUTOPILOT-24X7-PHASE-A-DOCS-PR-0.3` publishes this directory only.

The eight architecture documents and the 0.2 accompanying Markdown named in `LINE_ENDING_CROSSWALK.md` keep the same text as the verified pack. Their git blobs use LF line endings. The pack files on disk used CRLF. `git diff --check` reports a carriage return as trailing whitespace, so the published blobs are LF. No sentence was edited for that normalization. The crosswalk maps each LF blob hash to the CRLF pack hash.

The 0.2 `MANIFEST.sha256` (`1FA8181484457ACC6513B2C210EFEA1BE4E3A64FEC551E4585B5C2DECC32BB2B`) listed two paths this pull request does not contain:

- `.github/workflows/bintrans-autopilot-monitor.yml`
- `offline_dry_run.py`

Those hashes, and the hash of local `check_candidate.py`, are in `EXTERNAL_EVIDENCE.md`. `MANIFEST.sha256` in git lists only files in this directory and does not hash itself.

`ALLOWED_DIFF_PROPOSAL.md` text is unchanged apart from LF normalization. It still names `offline_dry_run.py` as eligible for a documentation GO. This GO excludes that file and excludes the monitor workflow. The exclusion is recorded here rather than by rewriting the proposal.

`OPEN_FINDINGS.md` records `J-DA-R2-M001`. The finding stays open. This pull request does not contain a workflow, so it cannot close that finding.

| File | Version | Reason |
| --- | --- | --- |
| Eight architecture documents and five 0.2 companions | 0.3 bytes, same text | LF normalization only. CRLF hashes remain in `LINE_ENDING_CROSSWALK.md` |
| `CHANGELOG.md` | 0.3 | Records the git manifest split, the LF normalization, and the open finding |
| `MANIFEST.sha256` | 0.3 | Reproducible for included LF blobs only |
| `EXTERNAL_EVIDENCE.md` | 0.3 | Hashes of local files omitted from git |
| `OPEN_FINDINGS.md` | 0.3 | Carries `J-DA-R2-M001` without closing it |
| `LINE_ENDING_CROSSWALK.md` | 0.3 | Maps CRLF pack hashes to LF git blob hashes |
