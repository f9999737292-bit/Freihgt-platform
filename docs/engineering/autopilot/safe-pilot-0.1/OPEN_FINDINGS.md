# Open findings

This pull request does not close findings.

## J-DA-R2-M001

| Field | Value |
| --- | --- |
| `FINDING_ID` | `J-DA-R2-M001` |
| `SEVERITY` | `MEDIUM` |
| `STATUS` | `OPEN` |
| `EVIDENCE` | Local candidate `.github/workflows/bintrans-autopilot-monitor.yml` sets `contents: read`, `actions: read`, `issues: read`, and `pull-requests: read`. It does not declare `metadata: read`. SHA256 `A8CF27547D0E6AB98B8C7F7FB03BA9F1EABCD78A0F5A5AB6D0A384F26029D3E9`. Agent J stage `DEV-AUTOPILOT-24X7-PHASE-A-REMEDIATION-0.2-R2-REVIEW` recorded one MEDIUM nit and stated it is not a blocker for the docs-only pull request. That workflow is not a blob in this pull request. |
| `REQUIRED_FIX` | Declare `metadata: read` in the monitor workflow permissions before the separate workflow publication GO. |
| `CARRY_FORWARD_STAGE` | `PHASE_A_MONITOR_WORKFLOW_GO` |

The closing condition is a later review of a published workflow blob that contains `metadata: read`. This file is not that proof.
