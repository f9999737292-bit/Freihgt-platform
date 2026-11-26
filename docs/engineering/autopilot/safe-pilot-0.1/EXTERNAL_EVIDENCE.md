# External evidence

These files are not in the git tree. Their SHA256 values were computed from the local Phase A 0.2 pack on 2026-11-26, before this documentation pull request. A clone of the pull request cannot recompute them. They are labeled external so `MANIFEST.sha256` stays limited to blobs that git contains.

| Label | Local path, not in git | SHA256 | Disposition |
| --- | --- | --- | --- |
| `MONITOR_WORKFLOW_CANDIDATE` | `.github/workflows/bintrans-autopilot-monitor.yml` | `A8CF27547D0E6AB98B8C7F7FB03BA9F1EABCD78A0F5A5AB6D0A384F26029D3E9` | `NOT_PUBLISHED`. Publishing it would expose `workflow_dispatch`. |
| `OFFLINE_DRY_RUN` | `docs/engineering/autopilot/safe-pilot-0.1/offline_dry_run.py` | `BA4B648DCFBD9A407DB01F4F877F8355063234417285CB0D23331CA7353D4FB8` | `NOT_PUBLISHED`. Bytes match the 0.1 simulator. Classification remains `SIMULATED_ONLY`. |
| `CHECK_CANDIDATE` | `check_candidate.py` (outside `proposed/`) | `403E10FC77A68E64646C68DF78275F70A82D19078A69ED99D6457B72563198A9` | `NOT_PUBLISHED`. Local static check only. It was not in the 0.2 manifest. |
| `MANIFEST_0_2_WITH_EXCLUDED_PATHS` | `MANIFEST.sha256` as reviewed in the 0.2 pack | `1FA8181484457ACC6513B2C210EFEA1BE4E3A64FEC551E4585B5C2DECC32BB2B` | `SUPERSEDED` for git. Agent J reproduced all 16 local rows. This pull request replaces that file with a manifest of included blobs only. |

`J-DA-R2-M001` is not evidence of a closed fix. See `OPEN_FINDINGS.md`.
