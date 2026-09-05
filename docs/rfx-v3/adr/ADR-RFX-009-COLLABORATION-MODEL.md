# ADR-RFX-009: Collaboration Model

**Status:** Accepted (architecture draft)  
**Date:** 2026-09-03

## Context

Enterprise buyers have multiple stakeholders (owner, editor, reviewer, approver) working on the same RFx.

## Decision

Introduce **`RfxCollaborator`** roles: `OWNER`, `EDITOR`, `REVIEWER`, `VIEWER`. Company context resolved server-side from membership — never client-supplied. Approval gates (`PUBLISH`, `AWARD`) optional per tenant policy.

## Consequences

- RBAC extends beyond single owner company user.
- Audit events record actor for all draft/publish/award actions.
- Real-time sync via `rfx.questionnaire.updated` / collaboration events (future).

## References

- [RFX_V3_SECURITY.md](../RFX_V3_SECURITY.md)
- [RFX_V3_DATA_MODEL.md](../RFX_V3_DATA_MODEL.md) §3.6
