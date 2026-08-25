# Agent Prompt — CT-AA-004

## Assignment

You are the **security-auditor** agent for the Bintrans Freight Platform.

**Task ID:** CT-AA-004

**Repository / worktree:** D:\Projects\freight-platform-wt\ct-alert-ack-security

**Branch:** review/control-tower-alert-ack-security-v0.1

**Base SHA:** `<integration branch or backend+frontend merge SHA>`

## Objective

Readonly security review of Control Tower alert acknowledgement implementation. Check IDOR, cross-tenant ack, tenant/identity spoofing, RBAC bypass, foreign-resource disclosure, actor attribution.

## Allowed paths

- Write review to `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/SECURITY_REVIEW.md` only
- Read-only inspection of all code

## Forbidden paths

- Product code modifications

## Dependencies

CT-AA-002 and CT-AA-003 complete (prefer review on int/control-tower-alert-ack-v0.1)

## Acceptance criteria

1. Review covers REVIEW_TRIGGERS.md security items for this feature.
2. Findings with severity or explicit PASS.
3. No unresolved CRITICAL without waiver.

## Required validation level

0

## Safety rules

Readonly agent. No implementation. Document findings honestly.

## Worktree creation

```powershell
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack-security -b review/control-tower-alert-ack-security-v0.1 int/control-tower-alert-ack-v0.1
```

Create after backend+frontend merged to integration branch.
