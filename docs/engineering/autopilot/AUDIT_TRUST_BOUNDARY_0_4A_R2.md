# Audit trust boundary 0.4A R2

Status: the finding gate is blocked. This note does not authorize a reviewer, a CI fetch, or a merge.

## What a packet can prove

An `AuditPacket` is a claim. Its `reviewer`, `verdict`, and `test_result` fields are text supplied by the caller. Matching the string `agent-j` does not identify Agent J. A `PASS` in that text does not prove CI passed, and it does not remove an earlier block.

The gateway records the claim in `AuditJournal` and returns `finding_gate=blocked`. `reviewer_authenticated` and `ci_verified` stay false. `merge_authorized` stays false.

## What is stored

The journal is append-only. Each line carries the hash of the previous line. `update`, `delete`, and `clear` raise `journal_immutable`. A rewritten line fails verification with `journal_tampered`, and the gateway then refuses further submits.

A `CHANGES_REQUIRED` or `BLOCKED` verdict, a `blocks_merge` finding, or a `CRITICAL` or `HIGH` severity writes a block for that SHA. A later packet for the same SHA returns `prior_block_stands`.

## What is still missing

A clear decision needs both of these, read by the trusted workflow on `main`, not copied out of a packet:

1. A GitHub pull-request review whose author is an allowlisted login other than the PR author, for the exact head SHA.
2. Successful GitHub check runs for that same SHA, including `bintrans-autopilot-unit` and `bintrans-autopilot-queue`.

This stage does not call GitHub and does not add a credential. Until a later controller change adds those reads inside the trusted workflow, the gate remains blocked. Agent H cannot supply the missing proof by writing `reviewer=agent-j`.
